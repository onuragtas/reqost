package index

import (
	"encoding/json"
	"testing"
)

// expDoc mirrors just enough of the Postman v2.1 export shape to assert on
// in tests, independent of the unexported expCollection/expItem types.
type expDoc struct {
	Info struct {
		Name   string `json:"name"`
		Schema string `json:"schema"`
	} `json:"info"`
	Item []expDocItem `json:"item"`
}
type expDocItem struct {
	Name    string       `json:"name"`
	Item    []expDocItem `json:"item"`
	Request *struct {
		Method string `json:"method"`
		URL    string `json:"url"`
	} `json:"request"`
}

func buildExportFixture(t *testing.T) (db *DB, folderID, loginID, pingID string) {
	t.Helper()
	db = tempDB(t)
	folder, err := db.CreateNode("", "Auth", "folder", "")
	if err != nil {
		t.Fatalf("create folder: %v", err)
	}
	login, err := db.CreateNode(folder.ID, "Login", "request", "POST")
	if err != nil {
		t.Fatalf("create login: %v", err)
	}
	if err := db.SaveRequest(RequestDetail{ID: login.ID, Name: "Login", Method: "POST", URL: "https://api.example.com/login", Body: `{"user":"x"}`}); err != nil {
		t.Fatalf("save login: %v", err)
	}
	ping, err := db.CreateNode("", "Ping", "request", "GET")
	if err != nil {
		t.Fatalf("create ping: %v", err)
	}
	if err := db.SaveRequest(RequestDetail{ID: ping.ID, Name: "Ping", Method: "GET", URL: "https://api.example.com/ping"}); err != nil {
		t.Fatalf("save ping: %v", err)
	}
	return db, folder.ID, login.ID, ping.ID
}

// TestExportJSONWholeCollection locks in the unscoped (rootID="") behavior:
// every top-level node appears, folders nest their children.
func TestExportJSONWholeCollection(t *testing.T) {
	db, _, _, _ := buildExportFixture(t)
	raw, err := db.ExportJSON("Whole", "")
	if err != nil {
		t.Fatalf("ExportJSON: %v", err)
	}
	var doc expDoc
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if doc.Info.Name != "Whole" {
		t.Errorf("info.name = %q", doc.Info.Name)
	}
	if doc.Info.Schema == "" {
		t.Error("info.schema empty — not a valid Postman v2.1 document")
	}
	if len(doc.Item) != 2 {
		t.Fatalf("want 2 top-level items (Auth folder + Ping), got %d: %+v", len(doc.Item), doc.Item)
	}
	var authFolder *expDocItem
	for i := range doc.Item {
		if doc.Item[i].Name == "Auth" {
			authFolder = &doc.Item[i]
		}
	}
	if authFolder == nil || len(authFolder.Item) != 1 || authFolder.Item[0].Name != "Login" {
		t.Errorf("Auth folder didn't nest Login: %+v", authFolder)
	}
}

// TestExportJSONScopedToFolder verifies right-click "Export…" on a folder:
// only that folder's children appear, as top-level items — Ping (a sibling
// outside the folder) must not leak in.
func TestExportJSONScopedToFolder(t *testing.T) {
	db, folderID, _, _ := buildExportFixture(t)
	raw, err := db.ExportJSON("Auth", folderID)
	if err != nil {
		t.Fatalf("ExportJSON: %v", err)
	}
	var doc expDoc
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(doc.Item) != 1 || doc.Item[0].Name != "Login" {
		t.Fatalf("want just [Login], got %+v", doc.Item)
	}
	if doc.Item[0].Request == nil || doc.Item[0].Request.URL != "https://api.example.com/login" {
		t.Errorf("Login request not exported correctly: %+v", doc.Item[0].Request)
	}
}

// TestExportJSONScopedToRequest verifies right-click "Export…" on a single
// request: a one-item collection containing just that request.
func TestExportJSONScopedToRequest(t *testing.T) {
	db, _, loginID, _ := buildExportFixture(t)
	raw, err := db.ExportJSON("Login", loginID)
	if err != nil {
		t.Fatalf("ExportJSON: %v", err)
	}
	var doc expDoc
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(doc.Item) != 1 {
		t.Fatalf("want exactly 1 item, got %d: %+v", len(doc.Item), doc.Item)
	}
	item := doc.Item[0]
	if item.Name != "Login" || item.Request == nil {
		t.Fatalf("expected a single Login request item, got %+v", item)
	}
	if item.Request.Method != "POST" || item.Request.URL != "https://api.example.com/login" {
		t.Errorf("request fields wrong: %+v", item.Request)
	}
	if len(item.Item) != 0 {
		t.Errorf("a request item should not carry nested Item: %+v", item.Item)
	}
}
