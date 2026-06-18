package httpclient

import "testing"

func TestResolveResponseRefs(t *testing.T) {
	last := map[string]LastResponse{
		"Login": {
			Status: 200,
			Body:   `{"access_token":"abc","user":{"id":42,"name":"Alex"},"items":[{"sku":"X1"},{"sku":"X2"}]}`,
			Headers: []Header{
				{Key: "Content-Type", Value: "application/json"},
				{Key: "X-Request-ID", Value: "req-123"},
			},
		},
	}

	cases := []struct {
		raw, want string
	}{
		{"{{Login.response.body.access_token}}", "abc"},
		{"{{Login.response.body.user.id}}", "42"},
		{"{{Login.response.body.user.name}}", "Alex"},
		{"{{Login.response.body.items.0.sku}}", "X1"},
		{"{{Login.response.body.items.1.sku}}", "X2"},
		{"{{Login.response.headers.X-Request-ID}}", "req-123"},
		{"{{Login.response.headers.content-type}}", "application/json"},
		{"{{Login.response.status}}", "200"},
	}

	for _, tc := range cases {
		req := Request{URL: tc.raw}
		ResolveResponseRefs(&req, last)
		got := interpolate(req.URL, req.Variables)
		if got != tc.want {
			t.Errorf("ref %q = %q, want %q", tc.raw, got, tc.want)
		}
	}
}

func TestResolveResponseRefsMissingNameLeavesPlaceholder(t *testing.T) {
	req := Request{URL: "{{Unknown.response.body.x}}"}
	ResolveResponseRefs(&req, map[string]LastResponse{})
	got := interpolate(req.URL, req.Variables)
	if got != "{{Unknown.response.body.x}}" {
		t.Errorf("missing ref should pass through: %q", got)
	}
}

func TestResolveResponseRefsScansHeadersAndBody(t *testing.T) {
	last := map[string]LastResponse{
		"Login": {Status: 200, Body: `{"token":"sekret"}`, Headers: nil},
	}
	req := Request{
		URL:  "https://api/{{Login.response.body.token}}",
		Body: `{"echo":"{{Login.response.body.token}}"}`,
		Headers: []Header{
			{Key: "X-Auth", Value: "Bearer {{Login.response.body.token}}", Enabled: true},
		},
	}
	ResolveResponseRefs(&req, last)
	if got := interpolate(req.URL, req.Variables); got != "https://api/sekret" {
		t.Errorf("url = %q", got)
	}
	if got := interpolate(req.Body, req.Variables); got != `{"echo":"sekret"}` {
		t.Errorf("body = %q", got)
	}
	if got := interpolate(req.Headers[0].Value, req.Variables); got != "Bearer sekret" {
		t.Errorf("header = %q", got)
	}
}
