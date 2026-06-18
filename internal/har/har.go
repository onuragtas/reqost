// Package har parses HTTP Archive (HAR) 1.2 exports into reqost FlatItems.
//
// HAR is what every browser's DevTools "Save all as HAR" produces — a JSON
// document with `log.entries[]`, each holding a `request` and `response`. We
// import only the request side; responses are discarded.
package har

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"reqost/internal/collection"
)

// Parse converts a HAR JSON document into FlatItems grouped under a single
// folder named after the importedAt timestamp. Caller passes the bytes; we
// don't read files so the same path works for "paste HAR" + file imports.
//
// The returned slice is ready for index.AddItems (merge, no clear).
func Parse(data []byte) ([]collection.FlatItem, error) {
	var doc struct {
		Log struct {
			Entries []entry `json:"entries"`
		} `json:"log"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("har: invalid JSON: %w", err)
	}
	if len(doc.Log.Entries) == 0 {
		return nil, fmt.Errorf("har: no entries")
	}

	rootID := "har-" + newID()
	rootName := "HAR " + time.Now().Format("2006-01-02 15:04")
	root := collection.FlatItem{
		ID:        rootID,
		Name:      rootName,
		ParentID:  "",
		Type:      "folder",
		SortOrder: 0,
	}
	items := []collection.FlatItem{root}

	for i, e := range doc.Log.Entries {
		req := e.Request
		method := strings.ToUpper(req.Method)
		if method == "" {
			method = "GET"
		}

		hdrs := make([]header, 0, len(req.Headers))
		for _, h := range req.Headers {
			// Skip pseudo-headers like ":authority", ":method" — HTTP/2 leftovers
			// the user can't usefully resend through net/http.
			if strings.HasPrefix(h.Name, ":") {
				continue
			}
			hdrs = append(hdrs, header{Key: h.Name, Value: h.Value, Enabled: true})
		}
		hb, _ := json.Marshal(hdrs)

		body, bodyType := bodyOf(req)

		// Name = METHOD path?query (truncated)
		name := fmt.Sprintf("%s %s", method, shortPath(req.URL))

		items = append(items, collection.FlatItem{
			ID:          "har-r-" + newID(),
			Name:        name,
			ParentID:    rootID,
			Type:        "request",
			Method:      method,
			SortOrder:   i,
			URL:         req.URL,
			HeadersJSON: string(hb),
			Body:        body,
			BodyType:    bodyType,
		})
	}
	return items, nil
}

func bodyOf(r request) (string, string) {
	if r.PostData == nil {
		return "", "none"
	}
	mime := strings.ToLower(r.PostData.MimeType)
	if r.PostData.Text != "" {
		switch {
		case strings.Contains(mime, "json"):
			return r.PostData.Text, "json"
		case strings.Contains(mime, "x-www-form-urlencoded"):
			// We *could* split into FormFields here, but the urlencoded raw
			// text body works for replay too. Keep this simple; the user can
			// switch the body type if they want the kv editor.
			return r.PostData.Text, "raw"
		default:
			return r.PostData.Text, "raw"
		}
	}
	return "", "none"
}

func shortPath(rawurl string) string {
	// Try to extract just the path+query, no host noise.
	if i := strings.Index(rawurl, "://"); i >= 0 {
		rest := rawurl[i+3:]
		if j := strings.Index(rest, "/"); j >= 0 {
			p := rest[j:]
			if len(p) > 60 {
				return p[:60] + "…"
			}
			return p
		}
	}
	if len(rawurl) > 60 {
		return rawurl[:60] + "…"
	}
	return rawurl
}

func newID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// HAR 1.2 minimal subset — only the fields we actually consume.
type entry struct {
	Request request `json:"request"`
}

type request struct {
	Method   string    `json:"method"`
	URL      string    `json:"url"`
	Headers  []nameVal `json:"headers"`
	PostData *postData `json:"postData,omitempty"`
}

type postData struct {
	MimeType string `json:"mimeType"`
	Text     string `json:"text"`
}

type nameVal struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// header matches the frontend HeaderRow shape (JSON tags align with what the
// rest of the codebase already persists).
type header struct {
	Key     string `json:"key"`
	Value   string `json:"value"`
	Enabled bool   `json:"enabled"`
}
