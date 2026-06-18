package collection

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func writeTemp(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "c.json")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestParseBodyModesAndAuth(t *testing.T) {
	col := `{
	  "info": {"name": "t", "schema": "x"},
	  "item": [
	    {"name": "urlenc", "request": {"method": "POST",
	      "auth": {"type": "bearer", "bearer": [{"key": "token", "value": "abc"}]},
	      "url": {"raw": "http://x"},
	      "body": {"mode": "urlencoded", "urlencoded": [{"key": "a", "value": "1"}, {"key": "b", "value": "2", "disabled": true}]}}},
	    {"name": "gql", "request": {"method": "POST", "url": {"raw": "http://x"},
	      "body": {"mode": "graphql", "graphql": {"query": "query { x }", "variables": "{\"v\":1}"}}}},
	    {"name": "basic", "request": {"method": "GET", "url": {"raw": "http://x"},
	      "auth": {"type": "basic", "basic": [{"key": "username", "value": "u"}, {"key": "password", "value": "p"}]}}}
	  ]
	}`
	items, _, err := ParseFile(writeTemp(t, col))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("want 3 items, got %d", len(items))
	}

	urlenc := items[0]
	if urlenc.BodyType != "urlencoded" {
		t.Errorf("urlenc bodyType = %q", urlenc.BodyType)
	}
	var rows []map[string]any
	json.Unmarshal([]byte(urlenc.FormFields), &rows)
	if len(rows) != 2 || rows[0]["key"] != "a" || rows[1]["enabled"] != false {
		t.Errorf("urlenc formFields wrong: %s", urlenc.FormFields)
	}
	if urlenc.AuthJSON == "" {
		t.Fatal("bearer auth not parsed")
	}
	var auth map[string]string
	json.Unmarshal([]byte(urlenc.AuthJSON), &auth)
	if auth["type"] != "bearer" || auth["token"] != "abc" {
		t.Errorf("bearer auth = %v", auth)
	}

	gql := items[1]
	if gql.BodyType != "graphql" || gql.Body != "query { x }" || gql.GraphqlVars != `{"v":1}` {
		t.Errorf("gql wrong: type=%q body=%q vars=%q", gql.BodyType, gql.Body, gql.GraphqlVars)
	}

	basic := items[2]
	var auth2 map[string]string
	json.Unmarshal([]byte(basic.AuthJSON), &auth2)
	if auth2["type"] != "basic" || auth2["username"] != "u" || auth2["password"] != "p" {
		t.Errorf("basic auth = %v", auth2)
	}
}
