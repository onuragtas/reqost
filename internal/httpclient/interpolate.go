package httpclient

import "regexp"

// varPattern matches Postman-style placeholders: {{ name }}, {{baseUrl}}, etc.
// Keys may contain word chars, dot, dash.
var varPattern = regexp.MustCompile(`\{\{\s*([\w.\-]+)\s*\}\}`)

// interpolate replaces every {{key}} in s with vars[key]. Unknown keys are left
// untouched so the user can see what failed to resolve.
func interpolate(s string, vars map[string]string) string {
	if s == "" || len(vars) == 0 {
		return s
	}
	return varPattern.ReplaceAllStringFunc(s, func(m string) string {
		key := varPattern.FindStringSubmatch(m)[1]
		if v, ok := vars[key]; ok {
			return v
		}
		return m
	})
}
