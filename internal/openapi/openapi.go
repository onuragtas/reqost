// Package openapi converts an OpenAPI 3 / Swagger 2 spec (JSON or YAML) into the
// flat collection items the index understands. Operations are grouped into
// folders by their first tag under a root folder named after the spec title.
package openapi

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"reqost/internal/collection"
)

type spec struct {
	OpenAPI    string                `yaml:"openapi"`
	Swagger    string                `yaml:"swagger"`
	Info       info                  `yaml:"info"`
	Servers    []server              `yaml:"servers"`
	Host       string                `yaml:"host"`     // swagger 2
	BasePath   string                `yaml:"basePath"` // swagger 2
	Schemes    []string              `yaml:"schemes"`  // swagger 2
	Paths      map[string]pathItem   `yaml:"paths"`
	Components components            `yaml:"components"`  // openapi 3
	Defs       map[string]schemaNode `yaml:"definitions"` // swagger 2
}

type info struct {
	Title string `yaml:"title"`
}
type server struct {
	URL string `yaml:"url"`
}
type components struct {
	Schemas map[string]schemaNode `yaml:"schemas"`
}

type pathItem struct {
	Get        *operation  `yaml:"get"`
	Post       *operation  `yaml:"post"`
	Put        *operation  `yaml:"put"`
	Patch      *operation  `yaml:"patch"`
	Delete     *operation  `yaml:"delete"`
	Head       *operation  `yaml:"head"`
	Options    *operation  `yaml:"options"`
	Parameters []parameter `yaml:"parameters"`
}

func (p pathItem) byMethod() map[string]*operation {
	return map[string]*operation{
		"GET": p.Get, "POST": p.Post, "PUT": p.Put, "PATCH": p.Patch,
		"DELETE": p.Delete, "HEAD": p.Head, "OPTIONS": p.Options,
	}
}

type operation struct {
	Tags        []string     `yaml:"tags"`
	Summary     string       `yaml:"summary"`
	OperationID string       `yaml:"operationId"`
	Description string       `yaml:"description"`
	Parameters  []parameter  `yaml:"parameters"`
	RequestBody *requestBody `yaml:"requestBody"` // openapi 3
}

type parameter struct {
	Name     string     `yaml:"name"`
	In       string     `yaml:"in"` // query | header | path | body (swagger 2)
	Required bool       `yaml:"required"`
	Schema   schemaNode `yaml:"schema"`
}

type requestBody struct {
	Content map[string]mediaType `yaml:"content"`
}
type mediaType struct {
	Schema  schemaNode `yaml:"schema"`
	Example any        `yaml:"example"`
}

// schemaNode is a permissive subset of JSON Schema used to synthesize example
// bodies. Ref is resolved against components/schemas (or definitions).
type schemaNode struct {
	Ref        string                `yaml:"$ref"`
	Type       string                `yaml:"type"`
	Example    any                   `yaml:"example"`
	Properties map[string]schemaNode `yaml:"properties"`
	Items      *schemaNode           `yaml:"items"`
}

// Parse reads a spec file and returns flat items plus the collection title.
func Parse(path string) ([]collection.FlatItem, string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", fmt.Errorf("read spec: %w", err)
	}
	var s spec
	if err := yaml.Unmarshal(data, &s); err != nil {
		return nil, "", fmt.Errorf("parse spec: %w", err)
	}
	if len(s.Paths) == 0 {
		return nil, "", fmt.Errorf("no paths found — not a recognizable OpenAPI/Swagger spec")
	}

	title := s.Info.Title
	if title == "" {
		title = "Imported API"
	}
	base := s.baseURL()
	schemas := s.schemaIndex()

	var items []collection.FlatItem
	rootID := newID()
	items = append(items, collection.FlatItem{ID: rootID, Name: title, Type: "folder"})

	// Stable folder ids per tag, created on first use.
	folderID := map[string]string{}
	order := map[string]int{} // sort order within each parent

	ensureFolder := func(tag string) string {
		if id, ok := folderID[tag]; ok {
			return id
		}
		id := newID()
		folderID[tag] = id
		items = append(items, collection.FlatItem{
			ID: id, Name: tag, ParentID: rootID, Type: "folder", SortOrder: order[rootID],
		})
		order[rootID]++
		return id
	}

	// Deterministic path ordering.
	paths := make([]string, 0, len(s.Paths))
	for p := range s.Paths {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	for _, p := range paths {
		pi := s.Paths[p]
		methods := pi.byMethod()
		for _, m := range []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"} {
			op := methods[m]
			if op == nil {
				continue
			}
			tag := "default"
			if len(op.Tags) > 0 && op.Tags[0] != "" {
				tag = op.Tags[0]
			}
			parentID := ensureFolder(tag)

			params := append(append([]parameter{}, pi.Parameters...), op.Parameters...)
			items = append(items, buildRequest(op, m, p, base, params, schemas, parentID, order[parentID]))
			order[parentID]++
		}
	}
	return items, title, nil
}

func buildRequest(op *operation, method, path, base string, params []parameter, schemas map[string]schemaNode, parentID string, sortOrder int) collection.FlatItem {
	name := op.Summary
	if name == "" {
		name = op.OperationID
	}
	if name == "" {
		name = method + " " + path
	}

	url := strings.TrimRight(base, "/") + path
	var headers []collection.Header
	var query []string
	for _, prm := range params {
		switch prm.In {
		case "header":
			headers = append(headers, collection.Header{Key: prm.Name, Value: ""})
		case "query":
			query = append(query, prm.Name+"=")
		}
	}
	if len(query) > 0 {
		url += "?" + strings.Join(query, "&")
	}

	bodyType := "none"
	body := ""
	if op.RequestBody != nil {
		if mt, ok := op.RequestBody.Content["application/json"]; ok {
			bodyType = "json"
			body = jsonExample(mt, schemas)
			headers = append(headers, collection.Header{Key: "Content-Type", Value: "application/json"})
		}
	}

	return collection.FlatItem{
		ID:          newID(),
		Name:        name,
		ParentID:    parentID,
		Type:        "request",
		Method:      method,
		SortOrder:   sortOrder,
		URL:         url,
		HeadersJSON: marshalHeaders(headers),
		Body:        body,
		BodyType:    bodyType,
		FormFields:  "[]",
		Description: op.Description,
	}
}

func jsonExample(mt mediaType, schemas map[string]schemaNode) string {
	var v any
	if mt.Example != nil {
		v = mt.Example
	} else {
		v = sample(mt.Schema, schemas, 0)
	}
	if v == nil {
		return ""
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return ""
	}
	return string(b)
}

// sample synthesizes an example value from a schema, resolving $ref. depth caps
// recursion against cyclic schemas.
func sample(s schemaNode, schemas map[string]schemaNode, depth int) any {
	if depth > 6 {
		return nil
	}
	if s.Ref != "" {
		if resolved, ok := schemas[refName(s.Ref)]; ok {
			return sample(resolved, schemas, depth+1)
		}
		return nil
	}
	if s.Example != nil {
		return s.Example
	}
	switch s.Type {
	case "object", "":
		if len(s.Properties) == 0 {
			if s.Type == "" {
				return nil
			}
			return map[string]any{}
		}
		obj := map[string]any{}
		for name, p := range s.Properties {
			obj[name] = sample(p, schemas, depth+1)
		}
		return obj
	case "array":
		if s.Items == nil {
			return []any{}
		}
		return []any{sample(*s.Items, schemas, depth+1)}
	case "string":
		return ""
	case "integer", "number":
		return 0
	case "boolean":
		return false
	default:
		return nil
	}
}

func (s spec) baseURL() string {
	if len(s.Servers) > 0 && s.Servers[0].URL != "" {
		return s.Servers[0].URL
	}
	if s.Host != "" { // swagger 2
		scheme := "https"
		if len(s.Schemes) > 0 {
			scheme = s.Schemes[0]
		}
		return scheme + "://" + s.Host + s.BasePath
	}
	return ""
}

func (s spec) schemaIndex() map[string]schemaNode {
	if len(s.Components.Schemas) > 0 {
		return s.Components.Schemas
	}
	return s.Defs // swagger 2
}

func refName(ref string) string {
	i := strings.LastIndex(ref, "/")
	if i == -1 {
		return ref
	}
	return ref[i+1:]
}

func marshalHeaders(headers []collection.Header) string {
	if len(headers) == 0 {
		return "[]"
	}
	b, _ := json.Marshal(headers)
	return string(b)
}

func newID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return "oa-" + hex.EncodeToString(b[:])
}
