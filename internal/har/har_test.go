package har

import (
	"strings"
	"testing"
)

const sampleHAR = `{
  "log": {
    "entries": [
      {
        "request": {
          "method": "GET",
          "url": "https://api.example.com/users?limit=10",
          "headers": [
            {"name": "Authorization", "value": "Bearer abc"},
            {"name": ":authority", "value": "api.example.com"}
          ]
        }
      },
      {
        "request": {
          "method": "POST",
          "url": "https://api.example.com/users",
          "headers": [{"name": "Content-Type", "value": "application/json"}],
          "postData": {"mimeType": "application/json", "text": "{\"name\":\"x\"}"}
        }
      }
    ]
  }
}`

func TestParseHAR(t *testing.T) {
	items, err := Parse([]byte(sampleHAR))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	// 1 folder + 2 requests
	if len(items) != 3 {
		t.Fatalf("want 3 items, got %d", len(items))
	}
	if items[0].Type != "folder" {
		t.Errorf("first item should be folder, got %s", items[0].Type)
	}

	get := items[1]
	if get.Method != "GET" || !strings.Contains(get.URL, "api.example.com") {
		t.Errorf("GET request malformed: %+v", get)
	}
	// :authority pseudo-header must be filtered out.
	if strings.Contains(get.HeadersJSON, ":authority") {
		t.Error("HTTP/2 pseudo-header leaked into HeadersJSON")
	}
	if !strings.Contains(get.HeadersJSON, "Authorization") {
		t.Errorf("Authorization header missing: %s", get.HeadersJSON)
	}

	post := items[2]
	if post.Method != "POST" || post.BodyType != "json" {
		t.Errorf("POST request malformed: %+v", post)
	}
	if !strings.Contains(post.Body, `"name":"x"`) {
		t.Errorf("body lost: %s", post.Body)
	}
}

func TestParseEmpty(t *testing.T) {
	_, err := Parse([]byte(`{"log":{"entries":[]}}`))
	if err == nil {
		t.Error("expected error on empty entries")
	}
}

func TestParseInvalidJSON(t *testing.T) {
	_, err := Parse([]byte(`not json`))
	if err == nil {
		t.Error("expected error on invalid JSON")
	}
}
