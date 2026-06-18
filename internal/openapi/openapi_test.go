package openapi

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

const spec3 = `
openapi: 3.0.0
info: { title: Petstore }
servers: [ { url: https://api.example.com/v1 } ]
paths:
  /pets:
    get:
      summary: List pets
      tags: [pets]
      parameters:
        - { name: limit, in: query }
        - { name: X-Token, in: header }
    post:
      summary: Create pet
      tags: [pets]
      requestBody:
        content:
          application/json:
            schema: { $ref: '#/components/schemas/Pet' }
  /health:
    get:
      summary: Health
components:
  schemas:
    Pet:
      type: object
      properties:
        id: { type: integer }
        name: { type: string }
`

func TestParseOpenAPI3(t *testing.T) {
	p := filepath.Join(t.TempDir(), "spec.yaml")
	if err := os.WriteFile(p, []byte(spec3), 0o644); err != nil {
		t.Fatal(err)
	}
	items, title, err := Parse(p)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if title != "Petstore" {
		t.Errorf("title = %q", title)
	}

	byName := map[string]int{}
	var root, pets, health, list, create string
	for _, it := range items {
		byName[it.Name]++
		switch it.Name {
		case "Petstore":
			root = it.ID
		case "pets":
			pets = it.ID
		case "default":
			health = it.ID
		case "List pets":
			list = it.ID
		case "Create pet":
			create = it.ID
		}
	}
	if root == "" || pets == "" || health == "" {
		t.Fatalf("missing folders: %+v", byName)
	}

	find := func(id string) collectionItem {
		for _, it := range items {
			if it.ID == id {
				return collectionItem{it.Name, it.Method, it.URL, it.Body, it.BodyType, it.ParentID, it.HeadersJSON}
			}
		}
		t.Fatalf("item %s not found", id)
		return collectionItem{}
	}

	lp := find(list)
	if lp.parent != pets {
		t.Errorf("List pets not under pets folder")
	}
	if lp.url != "https://api.example.com/v1/pets?limit=" {
		t.Errorf("List pets url = %q", lp.url)
	}
	if !containsHeader(lp.headers, "X-Token") {
		t.Errorf("header param missing: %s", lp.headers)
	}

	cp := find(create)
	if cp.bodyType != "json" {
		t.Errorf("create bodyType = %q", cp.bodyType)
	}
	var body map[string]any
	if err := json.Unmarshal([]byte(cp.body), &body); err != nil {
		t.Fatalf("create body not json: %q", cp.body)
	}
	if _, ok := body["name"]; !ok {
		t.Errorf("create body missing $ref-resolved fields: %v", body)
	}
}

type collectionItem struct {
	name, method, url, body, bodyType, parent, headers string
}

func containsHeader(headersJSON, key string) bool {
	var hs []map[string]string
	json.Unmarshal([]byte(headersJSON), &hs)
	for _, h := range hs {
		if h["key"] == key {
			return true
		}
	}
	return false
}
