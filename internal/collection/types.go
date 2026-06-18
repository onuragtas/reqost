package collection

import "encoding/json"

// Postman Collection Format v2.1

// CollectionVar is a single entry from a Postman collection's root `variable`
// array or a Postman environment export's `values` array.
type CollectionVar struct {
	Key     string `json:"key"`
	Value   string `json:"value"`
	Enabled bool   `json:"enabled"`
	// Postman env exports use "type" (default|secret) instead of a bool.
	Type string `json:"type,omitempty"`
}

type Collection struct {
	Info     CollectionInfo  `json:"info"`
	Item     []Item          `json:"item"`
	Variable []CollectionVar `json:"variable,omitempty"`
}

type CollectionInfo struct {
	ID     string `json:"_postman_id"`
	Name   string `json:"name"`
	Schema string `json:"schema"`
}

type Item struct {
	ID          string   `json:"id,omitempty"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Item        []Item   `json:"item,omitempty"`
	Request     *Request `json:"request,omitempty"`
	Event       []Event  `json:"event,omitempty"`
}

type Request struct {
	Method string     `json:"method,omitempty"`
	URL    PostmanURL `json:"url,omitempty"`
	Header []Header   `json:"header,omitempty"`
	Body   *Body      `json:"body,omitempty"`
	Auth   *Auth      `json:"auth,omitempty"`
}

// Auth is the Postman request-level auth block. Each scheme stores its params as
// a key/value list.
type Auth struct {
	Type   string      `json:"type"`
	Bearer []AuthParam `json:"bearer,omitempty"`
	Basic  []AuthParam `json:"basic,omitempty"`
	APIKey []AuthParam `json:"apikey,omitempty"`
}

type AuthParam struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type FormParam struct {
	Key      string `json:"key"`
	Value    string `json:"value"`
	Type     string `json:"type,omitempty"` // formdata: text|file
	Src      string `json:"src,omitempty"`  // formdata file path
	Disabled bool   `json:"disabled,omitempty"`
}

type GraphQLBody struct {
	Query     string `json:"query"`
	Variables string `json:"variables"`
}

// PostmanURL handles both string and object forms of the url field.
type PostmanURL struct {
	Raw string
}

func (u *PostmanURL) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		u.Raw = s
		return nil
	}
	var obj struct {
		Raw string `json:"raw"`
	}
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}
	u.Raw = obj.Raw
	return nil
}

type Header struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type Body struct {
	Mode       string       `json:"mode,omitempty"`
	Raw        string       `json:"raw,omitempty"`
	URLEncoded []FormParam  `json:"urlencoded,omitempty"`
	FormData   []FormParam  `json:"formdata,omitempty"`
	GraphQL    *GraphQLBody `json:"graphql,omitempty"`
}

type Event struct {
	Listen string `json:"listen"`
	Script Script `json:"script"`
}

type Script struct {
	Type string   `json:"type,omitempty"`
	Exec []string `json:"exec,omitempty"`
}
