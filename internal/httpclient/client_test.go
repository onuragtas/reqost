package httpclient

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func TestExecute(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-On"); got != "yes" {
			t.Errorf("enabled header not sent: got %q", got)
		}
		if got := r.Header.Get("X-Off"); got != "" {
			t.Errorf("disabled header was sent: %q", got)
		}
		body, _ := io.ReadAll(r.Body)
		w.Header().Set("X-Method", r.Method)
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("echo:" + string(body)))
	}))
	defer srv.Close()

	resp, err := New().Execute(context.Background(), Request{
		Method:   "POST",
		URL:      srv.URL,
		BodyType: "raw",
		Body:     "hello",
		Headers: []Header{
			{Key: "X-On", Value: "yes", Enabled: true},
			{Key: "X-Off", Value: "no", Enabled: false},
		},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if resp.Status != http.StatusCreated {
		t.Errorf("status = %d, want 201", resp.Status)
	}
	if resp.Body != "echo:hello" {
		t.Errorf("body = %q, want echo:hello", resp.Body)
	}
	if resp.Timing.TotalMs <= 0 {
		t.Errorf("TotalMs not measured: %v", resp.Timing.TotalMs)
	}
	if findHeader(resp.Headers, "X-Method") != "POST" {
		t.Errorf("response header X-Method missing/wrong: %v", resp.Headers)
	}
}

func TestInterpolate(t *testing.T) {
	vars := map[string]string{"baseUrl": "https://api.test", "ver": "v2"}
	got := interpolate("{{baseUrl}}/{{ver}}/users/{{missing}}", vars)
	want := "https://api.test/v2/users/{{missing}}"
	if got != want {
		t.Errorf("interpolate = %q, want %q", got, want)
	}
}

func TestDynamicVariables(t *testing.T) {
	// $timestamp must be an int.
	ts := interpolate("{{$timestamp}}", nil)
	if _, err := strconv.Atoi(ts); err != nil {
		t.Errorf("$timestamp not numeric: %q", ts)
	}

	// $guid must look like a UUIDv4.
	g := interpolate("{{$guid}}", nil)
	if len(g) != 36 || g[8] != '-' || g[14] != '4' {
		t.Errorf("$guid not a UUIDv4: %q", g)
	}

	// $randomInt must parse as int.
	ri := interpolate("{{$randomInt}}", nil)
	if _, err := strconv.Atoi(ri); err != nil {
		t.Errorf("$randomInt not numeric: %q", ri)
	}

	// Unknown $-prefixed key must stay as-is (helps the user spot typos).
	u := interpolate("{{$nopeSuchVar}}", nil)
	if u != "{{$nopeSuchVar}}" {
		t.Errorf("unknown dynamic var resolved: %q", u)
	}

	// Mixed static + dynamic in one string.
	mixed := interpolate("{{baseUrl}}/{{$randomEmail}}", map[string]string{"baseUrl": "http://x"})
	if !strings.HasPrefix(mixed, "http://x/") || !strings.Contains(mixed, "@example.com") {
		t.Errorf("mixed = %q", mixed)
	}
}

func TestExecuteVariablesAndAuth(t *testing.T) {
	var gotAuth, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
	}))
	defer srv.Close()

	_, err := New().Execute(context.Background(), Request{
		Method:    "GET",
		URL:       srv.URL + "/{{ver}}/me",
		Variables: map[string]string{"ver": "v3", "tok": "secret123"},
		Auth:      &Auth{Type: "bearer", Token: "{{tok}}"},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if gotPath != "/v3/me" {
		t.Errorf("path = %q, want /v3/me", gotPath)
	}
	if gotAuth != "Bearer secret123" {
		t.Errorf("auth = %q, want Bearer secret123", gotAuth)
	}
}

func TestExecuteUrlencoded(t *testing.T) {
	var ct, got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ct = r.Header.Get("Content-Type")
		b, _ := io.ReadAll(r.Body)
		got = string(b)
	}))
	defer srv.Close()

	_, err := New().Execute(context.Background(), Request{
		Method: "POST", URL: srv.URL, BodyType: "urlencoded",
		FormFields: []FormField{
			{Key: "a", Value: "1", Type: "text", Enabled: true},
			{Key: "b", Value: "2", Type: "text", Enabled: false},
		},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if ct != "application/x-www-form-urlencoded" {
		t.Errorf("content-type = %q", ct)
	}
	if got != "a=1" {
		t.Errorf("body = %q, want a=1 (disabled b dropped)", got)
	}
}

func TestExecuteUnsupportedProtocol(t *testing.T) {
	_, err := New().Execute(context.Background(), Request{Protocol: "grpc", URL: "http://x"})
	if err == nil {
		t.Fatal("expected error for unsupported protocol")
	}
}

func findHeader(hs []Header, key string) string {
	for _, h := range hs {
		if h.Key == key {
			return h.Value
		}
	}
	return ""
}
