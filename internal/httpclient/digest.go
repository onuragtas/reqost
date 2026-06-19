package httpclient

import (
	"crypto/md5"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"
)

// buildDigestHeader takes the WWW-Authenticate challenge from a 401 and
// returns the Authorization header value to attach on the retry. RFC 7616
// algorithms `MD5`, `MD5-sess`, `SHA-256`, `SHA-256-sess` are supported.
// qop=auth is implemented; qop=auth-int (body-hash) is not (rare in practice).
func buildDigestHeader(challenge string, a *Auth, method, fullURL string, vars map[string]string) (string, bool) {
	if a == nil {
		return "", false
	}
	user := interpolate(a.Username, vars)
	pass := interpolate(a.Password, vars)
	if user == "" {
		return "", false
	}

	params := parseChallengeParams(strings.TrimSpace(strings.TrimPrefix(strings.ToLower(challenge), "digest ")))
	// parseChallengeParams lowercases keys but keeps quoted values intact; the
	// case-insensitive scheme strip above means we lose original case on the
	// challenge tail. Re-parse the original to preserve realm casing.
	rawParams := parseChallengeParams(strings.TrimSpace(challenge[len("Digest "):]))

	realm := rawParams["realm"]
	nonce := rawParams["nonce"]
	opaque := rawParams["opaque"]
	algorithm := strings.ToUpper(rawParams["algorithm"])
	if algorithm == "" {
		algorithm = "MD5"
	}
	qopList := rawParams["qop"]
	qop := ""
	for _, q := range strings.Split(qopList, ",") {
		if strings.TrimSpace(q) == "auth" {
			qop = "auth"
			break
		}
	}

	u, err := url.Parse(fullURL)
	if err != nil {
		return "", false
	}
	uri := u.RequestURI()

	h := pickHasher(algorithm)
	ha1 := h(user + ":" + realm + ":" + pass)
	_ = params // reserved for future qop=auth-int handling
	var cnonce string
	if qop == "auth" || strings.HasSuffix(algorithm, "-SESS") {
		cnonce = randHex(16)
	}
	if strings.HasSuffix(algorithm, "-SESS") {
		ha1 = h(ha1 + ":" + nonce + ":" + cnonce)
	}
	return assembleDigest(method, uri, user, realm, nonce, opaque, algorithm, qop, ha1, h, cnonce), true
}

func assembleDigest(method, uri, user, realm, nonce, opaque, algorithm, qop string,
	ha1 string, hash func(string) string, cnonce string) string {

	ha2 := hash(method + ":" + uri)
	nc := "00000001"
	var response string
	if qop == "auth" {
		response = hash(ha1 + ":" + nonce + ":" + nc + ":" + cnonce + ":" + qop + ":" + ha2)
	} else {
		response = hash(ha1 + ":" + nonce + ":" + ha2)
	}

	parts := []string{
		fmt.Sprintf(`username="%s"`, user),
		fmt.Sprintf(`realm="%s"`, realm),
		fmt.Sprintf(`nonce="%s"`, nonce),
		fmt.Sprintf(`uri="%s"`, uri),
		fmt.Sprintf(`response="%s"`, response),
		fmt.Sprintf(`algorithm=%s`, algorithm),
	}
	if qop == "auth" {
		parts = append(parts,
			fmt.Sprintf(`qop=%s`, qop),
			fmt.Sprintf(`nc=%s`, nc),
			fmt.Sprintf(`cnonce="%s"`, cnonce),
		)
	}
	if opaque != "" {
		parts = append(parts, fmt.Sprintf(`opaque="%s"`, opaque))
	}
	return "Digest " + strings.Join(parts, ", ")
}

func pickHasher(algorithm string) func(string) string {
	if strings.HasPrefix(strings.ToUpper(algorithm), "SHA-256") {
		return func(s string) string {
			sum := sha256.Sum256([]byte(s))
			return hex.EncodeToString(sum[:])
		}
	}
	return func(s string) string {
		sum := md5.Sum([]byte(s))
		return hex.EncodeToString(sum[:])
	}
}

// parseChallengeParams turns `realm="Acme", nonce="x"` into a map. Quoted and
// unquoted values both accepted. Keys lowercased; values preserved.
func parseChallengeParams(s string) map[string]string {
	out := map[string]string{}
	i := 0
	for i < len(s) {
		// skip whitespace/commas
		for i < len(s) && (s[i] == ' ' || s[i] == ',' || s[i] == '\t') {
			i++
		}
		// key
		ks := i
		for i < len(s) && s[i] != '=' && s[i] != ',' {
			i++
		}
		key := strings.ToLower(strings.TrimSpace(s[ks:i]))
		if i >= len(s) || s[i] != '=' {
			if key != "" {
				out[key] = ""
			}
			continue
		}
		i++ // skip =
		var val string
		if i < len(s) && s[i] == '"' {
			i++
			vs := i
			for i < len(s) && s[i] != '"' {
				i++
			}
			val = s[vs:i]
			if i < len(s) {
				i++ // closing "
			}
		} else {
			vs := i
			for i < len(s) && s[i] != ',' {
				i++
			}
			val = strings.TrimSpace(s[vs:i])
		}
		if key != "" {
			out[key] = val
		}
	}
	return out
}

func randHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
