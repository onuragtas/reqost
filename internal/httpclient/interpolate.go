package httpclient

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	mrand "math/rand"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// varPattern matches Postman-style placeholders: {{ name }}, {{baseUrl}},
// {{$timestamp}}, {{$randomInt}}, {{$guid}} etc.  Keys may contain word chars,
// dot, dash, and an optional leading $ for dynamic builtins.
var varPattern = regexp.MustCompile(`\{\{\s*(\$?[\w.\-]+)\s*\}\}`)

// interpolate replaces every {{key}} in s. Static vars come from the map;
// $-prefixed keys are dynamic Postman-style helpers (timestamp, randomInt,
// guid, faker.js subset). Unknown keys are left untouched so the user can see
// what failed to resolve.
func interpolate(s string, vars map[string]string) string {
	if s == "" {
		return s
	}
	if !strings.Contains(s, "{{") {
		return s
	}
	return varPattern.ReplaceAllStringFunc(s, func(m string) string {
		key := varPattern.FindStringSubmatch(m)[1]
		if strings.HasPrefix(key, "$") {
			if v, ok := dynamicVar(key); ok {
				return v
			}
			return m
		}
		if v, ok := vars[key]; ok {
			return v
		}
		return m
	})
}

// dynamicVar returns the value for a Postman-style $-prefixed placeholder.
// Names mirror Postman's "Dynamic variables" set (subset covering the most
// commonly used helpers).
func dynamicVar(name string) (string, bool) {
	switch name {
	case "$timestamp":
		return strconv.FormatInt(time.Now().Unix(), 10), true
	case "$isoTimestamp":
		return time.Now().UTC().Format(time.RFC3339), true
	case "$unixEpochMs":
		return strconv.FormatInt(time.Now().UnixMilli(), 10), true
	case "$guid", "$randomUUID":
		return newUUIDv4(), true
	case "$randomInt":
		return strconv.Itoa(mrand.Intn(1000)), true
	case "$randomBoolean":
		if mrand.Intn(2) == 0 {
			return "false", true
		}
		return "true", true
	case "$randomEmail":
		return fmt.Sprintf("%s@example.com", randomWord(8)), true
	case "$randomFirstName":
		return pick(firstNames), true
	case "$randomLastName":
		return pick(lastNames), true
	case "$randomFullName":
		return pick(firstNames) + " " + pick(lastNames), true
	case "$randomUserName":
		return strings.ToLower(pick(firstNames)) + strconv.Itoa(mrand.Intn(100)), true
	case "$randomPassword":
		return randomWord(12), true
	case "$randomCity":
		return pick(cities), true
	case "$randomCountry":
		return pick(countries), true
	case "$randomCountryCode":
		return pick(countryCodes), true
	case "$randomPhoneNumber":
		return fmt.Sprintf("+1-%03d-%03d-%04d", mrand.Intn(900)+100, mrand.Intn(900)+100, mrand.Intn(10000)), true
	case "$randomUrl":
		return "https://" + randomWord(8) + ".example.com", true
	case "$randomIP":
		return fmt.Sprintf("%d.%d.%d.%d", mrand.Intn(256), mrand.Intn(256), mrand.Intn(256), mrand.Intn(256)), true
	case "$randomColor":
		return pick(colors), true
	case "$randomCompanyName":
		return pick(companies), true
	case "$randomLoremWord":
		return pick(loremWords), true
	case "$randomLoremSentence":
		ws := make([]string, 8+mrand.Intn(6))
		for i := range ws {
			ws[i] = pick(loremWords)
		}
		s := strings.Join(ws, " ")
		return strings.ToUpper(s[:1]) + s[1:] + ".", true
	}
	return "", false
}

// newUUIDv4 returns a random v4 UUID like "f47ac10b-58cc-4372-a567-0e02b2c3d479".
// We avoid the google/uuid dep and roll it from crypto/rand.
func newUUIDv4() string {
	var b [16]byte
	_, err := rand.Read(b[:])
	if err != nil {
		// Fallback to math/rand — UUID does not have to be cryptographically
		// strong for placeholder use.
		mrand.Read(b[:])
	}
	b[6] = (b[6] & 0x0f) | 0x40 // v4
	b[8] = (b[8] & 0x3f) | 0x80 // RFC 4122 variant
	h := hex.EncodeToString(b[:])
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:32]
}

// randomWord returns n random lowercase alphabetic chars.
func randomWord(n int) string {
	const alphabet = "abcdefghijklmnopqrstuvwxyz"
	b := make([]byte, n)
	for i := range b {
		b[i] = alphabet[mrand.Intn(len(alphabet))]
	}
	return string(b)
}

func pick(arr []string) string {
	return arr[mrand.Intn(len(arr))]
}

// Tiny corpora for the faker.js-style placeholders. Postman's lists are huge
// but this slice is enough for sample data in tests / examples.
var (
	firstNames = []string{
		"Alex", "Jordan", "Taylor", "Casey", "Morgan", "Riley", "Avery", "Quinn",
		"Sam", "Drew", "Cameron", "Skyler", "Jamie", "Reese", "Parker", "Rowan",
	}
	lastNames = []string{
		"Smith", "Johnson", "Williams", "Brown", "Davis", "Miller", "Wilson",
		"Moore", "Taylor", "Anderson", "Thomas", "Jackson", "White", "Harris",
	}
	cities = []string{
		"Istanbul", "Berlin", "London", "Paris", "New York", "Tokyo", "Madrid",
		"Rome", "Amsterdam", "Vienna", "Lisbon", "Dublin", "Sofia", "Athens",
	}
	countries = []string{
		"Türkiye", "Germany", "United Kingdom", "France", "United States",
		"Japan", "Spain", "Italy", "Netherlands", "Austria", "Portugal", "Ireland",
	}
	countryCodes = []string{"TR", "DE", "GB", "FR", "US", "JP", "ES", "IT", "NL", "AT", "PT", "IE"}
	colors       = []string{"red", "green", "blue", "orange", "purple", "cyan", "magenta", "yellow"}
	companies    = []string{
		"Acme Inc", "Globex", "Initech", "Hooli", "Stark Industries",
		"Wayne Enterprises", "Umbrella Corp", "Wonka", "Cyberdyne", "Tyrell",
	}
	loremWords = []string{
		"lorem", "ipsum", "dolor", "sit", "amet", "consectetur", "adipiscing",
		"elit", "sed", "do", "eiusmod", "tempor", "incididunt", "ut", "labore",
		"et", "dolore", "magna", "aliqua",
	}
)
