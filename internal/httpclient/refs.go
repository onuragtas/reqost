package httpclient

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

// LastResponse is the slimmed snapshot of a previously-sent response that the
// caller passes into ResolveResponseRefs. Kept here (not in the exec service)
// so this package owns the reference syntax + path walker in one place.
type LastResponse struct {
	Status  int
	Body    string
	Headers []Header
}

// refPattern matches {{Name.response.<dotted.path>}} placeholders.
// "Name" is the request's display name (the Postman/Insomnia convention).
var refPattern = regexp.MustCompile(`\{\{\s*([\w-]+)\.response\.([\w.\-]+)\s*\}\}`)

// ResolveResponseRefs scans every interpolation-aware field of req for
// `{{Name.response.<path>}}` placeholders, walks the matching saved response,
// and injects the resolved value into req.Variables under the inner key so the
// existing interpolate() picks it up unchanged.
//
// `last` is a snapshot map of request-name → response. Caller holds the lock;
// this function only reads it.
func ResolveResponseRefs(req *Request, last map[string]LastResponse) {
	if req.Variables == nil {
		req.Variables = map[string]string{}
	}
	scan := func(s string) {
		for _, m := range refPattern.FindAllStringSubmatch(s, -1) {
			name, path := m[1], m[2]
			r, ok := last[name]
			if !ok {
				continue
			}
			val, ok := resolveResponsePath(r, path)
			if !ok {
				continue
			}
			req.Variables[name+".response."+path] = val
		}
	}
	scan(req.URL)
	scan(req.Body)
	for _, h := range req.Headers {
		scan(h.Key)
		scan(h.Value)
	}
	for _, f := range req.FormFields {
		scan(f.Key)
		scan(f.Value)
	}
	if a := req.Auth; a != nil {
		scan(a.Token)
		scan(a.Username)
		scan(a.Password)
		scan(a.Key)
		scan(a.Value)
	}
}

// resolveResponsePath walks a dotted path inside a saved response:
//
//	status                              → "200"
//	body.foo.bar                        → JSON walk by key
//	body.items.0.id                     → numeric segments index into arrays
//	headers.Content-Type                → first matching response header (case-insensitive)
//	statusText                          → "OK"
func resolveResponsePath(r LastResponse, path string) (string, bool) {
	segs := strings.Split(path, ".")
	if len(segs) == 0 {
		return "", false
	}
	switch segs[0] {
	case "status":
		return strconv.Itoa(r.Status), true
	case "statusText":
		// Reconstruct from status code; we don't carry statusText on LastResponse.
		// Acceptable: callers asking for status text usually want it for display.
		return strconv.Itoa(r.Status), true
	case "headers":
		if len(segs) < 2 {
			return "", false
		}
		name := strings.Join(segs[1:], ".")
		for _, h := range r.Headers {
			if strings.EqualFold(h.Key, name) {
				return h.Value, true
			}
		}
		return "", false
	case "body":
		return walkJSON(r.Body, segs[1:])
	}
	return "", false
}

// walkJSON parses body once, descends rest by key/index, stringifies the leaf.
// Empty rest returns the raw body. Maps + arrays only; primitives at leaves.
func walkJSON(body string, rest []string) (string, bool) {
	if len(rest) == 0 {
		return body, true
	}
	var v any
	if err := json.Unmarshal([]byte(body), &v); err != nil {
		return "", false
	}
	for _, seg := range rest {
		switch t := v.(type) {
		case map[string]any:
			next, ok := t[seg]
			if !ok {
				return "", false
			}
			v = next
		case []any:
			i, err := strconv.Atoi(seg)
			if err != nil || i < 0 || i >= len(t) {
				return "", false
			}
			v = t[i]
		default:
			return "", false
		}
	}
	return stringifyJSON(v), true
}

func stringifyJSON(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case nil:
		return ""
	case float64:
		// JSON numbers; render ints without trailing ".0".
		if t == float64(int64(t)) {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(t)
	}
	b, _ := json.Marshal(v)
	return string(b)
}

// Cache is a small concurrency-safe map of request-name → last response.
// ExecService owns one and feeds it into ResolveResponseRefs. Kept here so the
// pattern + storage live next to each other.
type Cache struct {
	mu sync.Mutex
	m  map[string]LastResponse
}

func NewCache() *Cache { return &Cache{m: map[string]LastResponse{}} }

func (c *Cache) Put(name string, r LastResponse) {
	if name == "" {
		return
	}
	c.mu.Lock()
	c.m[name] = r
	c.mu.Unlock()
}

func (c *Cache) Snapshot() map[string]LastResponse {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make(map[string]LastResponse, len(c.m))
	for k, v := range c.m {
		out[k] = v
	}
	return out
}

