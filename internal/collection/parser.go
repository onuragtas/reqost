package collection

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type FlatItem struct {
	ID          string
	Name        string
	ParentID    string // empty == root (stored as NULL)
	Type        string // "folder" | "request"
	Method      string
	SortOrder   int
	URL         string
	HeadersJSON string
	Body        string
	PreScript   string
	PostScript  string
	Description string
	BodyType    string // "none" | "raw" | "urlencoded" | "formdata" | "graphql"
	FormFields  string // JSON array for urlencoded/formdata
	GraphqlVars string
	AuthJSON    string // JSON object matching the frontend Auth shape
}

func ParseFile(path string) ([]FlatItem, []CollectionVar, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("read collection: %w", err)
	}
	return ParseBytes(data)
}

// ParseBytes parses a Postman Collection v2.1 JSON from raw bytes.
// It also handles the Postman API envelope: {"collection": {...}}.
// Returns the flat item list and any root-level collection variables.
func ParseBytes(data []byte) ([]FlatItem, []CollectionVar, error) {
	// Postman API wraps the collection: {"collection": { "info":…, "item":… }}
	var envelope struct {
		Collection *Collection `json:"collection"`
	}
	if err := json.Unmarshal(data, &envelope); err == nil && envelope.Collection != nil {
		col := envelope.Collection
		var items []FlatItem
		for i, item := range col.Item {
			flatten(&items, item, "", i)
		}
		return items, normaliseVars(col.Variable), nil
	}

	var col Collection
	if err := json.Unmarshal(data, &col); err != nil {
		return nil, nil, fmt.Errorf("parse collection json: %w", err)
	}
	// Require at least an info block or items so we don't misidentify OpenAPI as a collection.
	if col.Info.Name == "" && len(col.Item) == 0 {
		return nil, nil, fmt.Errorf("not a Postman collection")
	}

	var items []FlatItem
	for i, item := range col.Item {
		flatten(&items, item, "", i)
	}
	return items, normaliseVars(col.Variable), nil
}

// ParseEnvBytes parses a Postman environment export JSON.
// Returns the environment name and its variables, or an error if the bytes
// don't look like a Postman environment file.
func ParseEnvBytes(data []byte) (name string, vars []CollectionVar, err error) {
	var env struct {
		Name   string          `json:"name"`
		Values []CollectionVar `json:"values"`
		Scope  string          `json:"_postman_variable_scope"`
	}
	if err := json.Unmarshal(data, &env); err != nil {
		return "", nil, fmt.Errorf("parse env json: %w", err)
	}
	if env.Scope != "environment" && env.Scope != "globals" && env.Name == "" {
		return "", nil, fmt.Errorf("not a Postman environment file")
	}
	name = env.Name
	if name == "" {
		name = "Imported environment"
	}
	return name, normaliseVars(env.Values), nil
}

// normaliseVars converts Postman variable entries: if `enabled` is unset but
// `type` is not "secret", treat the variable as enabled.
func normaliseVars(in []CollectionVar) []CollectionVar {
	out := make([]CollectionVar, 0, len(in))
	for _, v := range in {
		if v.Key == "" {
			continue
		}
		enabled := v.Enabled || v.Type == "default" || (v.Type == "" && !v.Enabled)
		out = append(out, CollectionVar{Key: v.Key, Value: v.Value, Enabled: enabled})
	}
	return out
}

func flatten(out *[]FlatItem, item Item, parentID string, order int) {
	id := item.ID
	if id == "" {
		id = fmt.Sprintf("%s|%s|%d", parentID, item.Name, order)
	}

	isFolder := item.Request == nil
	if isFolder {
		*out = append(*out, FlatItem{
			ID:          id,
			Name:        item.Name,
			ParentID:    parentID,
			Type:        "folder",
			SortOrder:   order,
			Description: item.Description,
		})
		for i, child := range item.Item {
			flatten(out, child, id, i)
		}
		return
	}

	req := item.Request
	bodyType, body, formFields, graphqlVars := convertBody(req.Body)
	pre, post := extractScripts(item.Event)

	*out = append(*out, FlatItem{
		ID:          id,
		Name:        item.Name,
		ParentID:    parentID,
		Type:        "request",
		Method:      req.Method,
		SortOrder:   order,
		URL:         req.URL.Raw,
		HeadersJSON: marshalHeaders(req.Header),
		Body:        body,
		PreScript:   pre,
		PostScript:  post,
		Description: item.Description,
		BodyType:    bodyType,
		FormFields:  formFields,
		GraphqlVars: graphqlVars,
		AuthJSON:    convertAuth(req.Auth),
	})
}

// convertBody maps a Postman body block to our (bodyType, raw, formFieldsJSON,
// graphqlVars) shape.
func convertBody(b *Body) (bodyType, raw, formFields, graphqlVars string) {
	if b == nil {
		return "none", "", "", ""
	}
	switch b.Mode {
	case "raw":
		return "raw", b.Raw, "", ""
	case "urlencoded":
		return "urlencoded", "", marshalForm(b.URLEncoded), ""
	case "formdata":
		return "formdata", "", marshalForm(b.FormData), ""
	case "graphql":
		if b.GraphQL != nil {
			return "graphql", b.GraphQL.Query, "", b.GraphQL.Variables
		}
		return "graphql", "", "", ""
	default:
		if b.Raw != "" {
			return "raw", b.Raw, "", ""
		}
		return "none", "", "", ""
	}
}

func marshalForm(params []FormParam) string {
	type row struct {
		Key     string `json:"key"`
		Value   string `json:"value"`
		Type    string `json:"type"`
		Enabled bool   `json:"enabled"`
	}
	rows := make([]row, 0, len(params))
	for _, p := range params {
		typ := p.Type
		if typ != "file" {
			typ = "text"
		}
		val := p.Value
		if typ == "file" && val == "" {
			val = p.Src
		}
		rows = append(rows, row{Key: p.Key, Value: val, Type: typ, Enabled: !p.Disabled})
	}
	b, _ := json.Marshal(rows)
	return string(b)
}

// convertAuth maps a Postman auth block to the frontend Auth JSON shape
// ({type, token, username, password, key, value}). Returns "" for none.
func convertAuth(a *Auth) string {
	if a == nil || a.Type == "" || a.Type == "noauth" {
		return ""
	}
	out := map[string]string{
		"type": "none", "token": "", "username": "", "password": "", "key": "", "value": "",
	}
	switch a.Type {
	case "bearer":
		out["type"] = "bearer"
		out["token"] = authParam(a.Bearer, "token")
	case "basic":
		out["type"] = "basic"
		out["username"] = authParam(a.Basic, "username")
		out["password"] = authParam(a.Basic, "password")
	case "apikey":
		out["type"] = "apikey"
		out["key"] = authParam(a.APIKey, "key")
		out["value"] = authParam(a.APIKey, "value")
	default:
		return "" // unsupported scheme → leave as none
	}
	b, _ := json.Marshal(out)
	return string(b)
}

func authParam(params []AuthParam, key string) string {
	for _, p := range params {
		if p.Key == key {
			return p.Value
		}
	}
	return ""
}

func marshalHeaders(headers []Header) string {
	if len(headers) == 0 {
		return "[]"
	}
	b, _ := json.Marshal(headers)
	return string(b)
}

func extractScripts(events []Event) (pre, post string) {
	for _, e := range events {
		s := strings.Join(e.Script.Exec, "\n")
		switch e.Listen {
		case "prerequest":
			pre = s
		case "test":
			post = s
		}
	}
	return
}
