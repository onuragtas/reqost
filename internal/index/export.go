package index

import (
	"encoding/json"
	"fmt"
	"strings"
)

const postmanSchema = "https://schema.getpostman.com/json/collection/v2.1.0/collection.json"

// Postman v2.1 export shapes. Dedicated structs (not the import-side
// collection.* types) so the JSON tags produce spec-clean output.
type expCollection struct {
	Info expInfo    `json:"info"`
	Item []*expItem `json:"item"`
}
type expInfo struct {
	Name   string `json:"name"`
	Schema string `json:"schema"`
}
type expItem struct {
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	Item        []*expItem  `json:"item,omitempty"`
	Request     *expRequest `json:"request,omitempty"`
	Event       []expEvent  `json:"event,omitempty"`
}
type expRequest struct {
	Method string      `json:"method"`
	Header []expHeader `json:"header,omitempty"`
	Body   *expBody    `json:"body,omitempty"`
	URL    string      `json:"url"`
}
type expHeader struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}
type expBody struct {
	Mode string `json:"mode"`
	Raw  string `json:"raw"`
}
type expEvent struct {
	Listen string    `json:"listen"`
	Script expScript `json:"script"`
}
type expScript struct {
	Type string   `json:"type"`
	Exec []string `json:"exec"`
}

type exportRow struct {
	id, name, parent, typ, method              string
	url, headers, body, pre, post, description string
	isRequest                                  bool
}

// ExportJSON rebuilds the whole index into an indented Postman v2.1 document.
func (db *DB) ExportJSON(name string) (string, error) {
	rows, err := db.conn.Query(`
		SELECT t.id, t.name, COALESCE(t.parent_id,''), t.type, t.method, t.sort_order,
		       COALESCE(d.url,''), COALESCE(d.headers_json,'[]'), COALESCE(d.body,''),
		       COALESCE(d.pre_script,''), COALESCE(d.post_script,''), COALESCE(d.description,'')
		FROM tree t LEFT JOIN detail d ON d.id = t.id
		ORDER BY t.sort_order`)
	if err != nil {
		return "", fmt.Errorf("export query: %w", err)
	}
	defer rows.Close()

	children := map[string][]*exportRow{}
	for rows.Next() {
		var r exportRow
		var order int
		if err := rows.Scan(&r.id, &r.name, &r.parent, &r.typ, &r.method, &order,
			&r.url, &r.headers, &r.body, &r.pre, &r.post, &r.description); err != nil {
			return "", err
		}
		r.isRequest = r.typ == "request"
		children[r.parent] = append(children[r.parent], &r)
	}
	if err := rows.Err(); err != nil {
		return "", err
	}

	if name == "" {
		name = "reqost export"
	}
	col := expCollection{
		Info: expInfo{Name: name, Schema: postmanSchema},
		Item: buildItems(children, ""),
	}
	out, err := json.MarshalIndent(col, "", "\t")
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func buildItems(children map[string][]*exportRow, parent string) []*expItem {
	var items []*expItem
	for _, r := range children[parent] {
		item := &expItem{Name: r.name, Description: r.description}
		if r.isRequest {
			item.Request = &expRequest{
				Method: orDefault(r.method, "GET"),
				Header: parseExpHeaders(r.headers),
				URL:    r.url,
			}
			if strings.TrimSpace(r.body) != "" {
				item.Request.Body = &expBody{Mode: "raw", Raw: r.body}
			}
			item.Event = buildEvents(r.pre, r.post)
		} else {
			item.Item = buildItems(children, r.id)
		}
		items = append(items, item)
	}
	return items
}

func parseExpHeaders(headersJSON string) []expHeader {
	var raw []struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}
	if err := json.Unmarshal([]byte(headersJSON), &raw); err != nil {
		return nil
	}
	out := make([]expHeader, 0, len(raw))
	for _, h := range raw {
		if h.Key == "" {
			continue
		}
		out = append(out, expHeader{Key: h.Key, Value: h.Value})
	}
	return out
}

func buildEvents(pre, post string) []expEvent {
	var evs []expEvent
	if strings.TrimSpace(pre) != "" {
		evs = append(evs, expEvent{Listen: "prerequest", Script: expScript{Type: "text/javascript", Exec: strings.Split(pre, "\n")}})
	}
	if strings.TrimSpace(post) != "" {
		evs = append(evs, expEvent{Listen: "test", Script: expScript{Type: "text/javascript", Exec: strings.Split(post, "\n")}})
	}
	return evs
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
