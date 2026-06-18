package index

import (
	"path/filepath"
	"testing"

	"reqost/internal/collection"
)

// TestAddItemsMerge verifies the OpenAPI import path: AddItems inserts a flat
// tree without clearing existing data.
func TestAddItemsMerge(t *testing.T) {
	db := tempDB(t)
	existing, _ := db.CreateNode("", "Existing", "folder", "")

	items := []collection.FlatItem{
		{ID: "root1", Name: "API", Type: "folder"},
		{ID: "f1", Name: "pets", ParentID: "root1", Type: "folder"},
		{ID: "r1", Name: "List", ParentID: "f1", Type: "request", Method: "GET", URL: "http://x/pets", BodyType: "none"},
	}
	if err := db.AddItems(items); err != nil {
		t.Fatalf("AddItems: %v", err)
	}

	roots, _ := db.GetChildren("")
	if len(roots) != 2 { // Existing + API
		t.Fatalf("want 2 roots, got %d", len(roots))
	}
	pets, _ := db.GetChildren("root1")
	if len(pets) != 1 || pets[0].Name != "pets" {
		t.Errorf("pets folder not merged: %+v", pets)
	}
	if d, _ := db.GetRequestDetail("r1"); d == nil || d.URL != "http://x/pets" {
		t.Errorf("request detail not stored: %+v", d)
	}
	if hits, _ := db.Search("List"); len(hits) == 0 {
		t.Error("imported request not searchable")
	}
	_ = existing
}

func tempDB(t *testing.T) *DB {
	t.Helper()
	db, err := OpenAt(filepath.Join(t.TempDir(), "index.db"))
	if err != nil {
		t.Fatalf("OpenAt: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// TestDeleteNodeAfterReads reproduces the bug where deleting a node after
// browsing the tree failed with SQLITE_BUSY (the old DeleteNode mixed tx.Query
// and tx.Exec in one transaction). It must now fully remove the subtree.
func TestDeleteNodeAfterReads(t *testing.T) {
	db := tempDB(t)

	f, err := db.CreateNode("", "F", "folder", "")
	if err != nil {
		t.Fatalf("create F: %v", err)
	}
	r, _ := db.CreateNode(f.ID, "R", "request", "GET")
	sf, _ := db.CreateNode(f.ID, "SF", "folder", "")
	r2, _ := db.CreateNode(sf.ID, "R2", "request", "POST")

	// Browse the tree first — this is what the UI does before a delete and what
	// used to leave the connection in a state that made the delete BUSY.
	for _, id := range []string{"", f.ID, sf.ID} {
		if _, err := db.GetChildren(id); err != nil {
			t.Fatalf("GetChildren(%q): %v", id, err)
		}
	}
	if _, err := db.GetRequestDetail(r2.ID); err != nil {
		t.Fatalf("GetRequestDetail: %v", err)
	}
	if _, err := db.Search("R2"); err != nil {
		t.Fatalf("Search: %v", err)
	}

	if err := db.DeleteNode(f.ID); err != nil {
		t.Fatalf("DeleteNode: %v", err)
	}

	for _, id := range []string{r.ID, r2.ID} {
		if d, _ := db.GetRequestDetail(id); d != nil {
			t.Errorf("detail for %s still present after delete", id)
		}
	}
	if roots, _ := db.GetChildren(""); len(roots) != 0 {
		t.Errorf("root not empty after delete: %d nodes", len(roots))
	}
	if hits, _ := db.Search("R2"); len(hits) != 0 {
		t.Errorf("FTS not purged: %d hits", len(hits))
	}
}

// TestDeleteLargeFolder deletes a folder with many descendants. With the old
// per-id FTS delete this degraded to O(N × rows) and froze the UI; it must now
// complete quickly and remove everything.
func TestDeleteLargeFolder(t *testing.T) {
	db := tempDB(t)
	f, _ := db.CreateNode("", "Big", "folder", "")
	for i := 0; i < 600; i++ { // > chunk size (500) to exercise chunking
		if _, err := db.CreateNode(f.ID, "req", "request", "GET"); err != nil {
			t.Fatalf("create %d: %v", i, err)
		}
	}
	if kids, _ := db.GetChildren(f.ID); len(kids) != 600 {
		t.Fatalf("expected 600 children, got %d", len(kids))
	}
	if err := db.DeleteNode(f.ID); err != nil {
		t.Fatalf("DeleteNode: %v", err)
	}
	if roots, _ := db.GetChildren(""); len(roots) != 0 {
		t.Errorf("root not empty: %d", len(roots))
	}
	if hits, _ := db.Search("req"); len(hits) != 0 {
		t.Errorf("FTS not purged: %d hits", len(hits))
	}
}

// TestImportItemsPreservesEdits verifies that a re-import keeps the user's
// in-app edits to a request that exists in both versions (same id) instead of
// wiping the detail back to the imported state. Only renamed nodes refresh
// their FTS row, and the FTS row for renamed-but-edited rows must keep the
// edited url/body so search stays consistent with the displayed data.
func TestImportItemsPreservesEdits(t *testing.T) {
	db := tempDB(t)

	first := []collection.FlatItem{
		{ID: "f1", Name: "API", Type: "folder", SortOrder: 0},
		{ID: "r1", Name: "GetUser", ParentID: "f1", Type: "request", Method: "GET", URL: "http://orig/u", Body: "orig-body", BodyType: "none"},
		{ID: "r2", Name: "DeleteUser", ParentID: "f1", Type: "request", Method: "DELETE", URL: "http://orig/d", BodyType: "none"},
	}
	if err := db.ImportItems("/tmp/c.json", 1, first); err != nil {
		t.Fatalf("first import: %v", err)
	}

	// User edits r1.
	d, _ := db.GetRequestDetail("r1")
	d.URL = "http://edited/u"
	d.Body = "edited-body"
	if err := db.SaveRequest(*d); err != nil {
		t.Fatalf("save edit: %v", err)
	}

	// Re-import: r1 renamed, r2 removed, r3 added.
	second := []collection.FlatItem{
		{ID: "f1", Name: "API", Type: "folder", SortOrder: 0},
		{ID: "r1", Name: "FetchUser", ParentID: "f1", Type: "request", Method: "GET", URL: "http://orig/u", BodyType: "none"},
		{ID: "r3", Name: "ListUsers", ParentID: "f1", Type: "request", Method: "GET", URL: "http://orig/list", BodyType: "none"},
	}
	if err := db.ImportItems("/tmp/c.json", 2, second); err != nil {
		t.Fatalf("re-import: %v", err)
	}

	// r1's tree fields refreshed (name), but detail preserved.
	r1, _ := db.GetRequestDetail("r1")
	if r1 == nil {
		t.Fatal("r1 missing after re-import")
	}
	if r1.Name != "FetchUser" {
		t.Errorf("r1 name not updated: %q", r1.Name)
	}
	if r1.URL != "http://edited/u" || r1.Body != "edited-body" {
		t.Errorf("r1 edits lost: url=%q body=%q", r1.URL, r1.Body)
	}

	// r2 deleted.
	if r2, _ := db.GetRequestDetail("r2"); r2 != nil {
		t.Error("r2 should be gone")
	}

	// r3 inserted.
	if r3, _ := db.GetRequestDetail("r3"); r3 == nil {
		t.Error("r3 missing")
	}

	// FTS reflects renamed name AND kept user-edited url.
	if hits, _ := db.Search("FetchUser"); len(hits) == 0 {
		t.Error("search for renamed name miss")
	}
	if hits, _ := db.Search("edited"); len(hits) == 0 {
		t.Error("FTS should contain user-edited url/body, not the import's orig url")
	}
}

// TestMoveNodeReorders re-parents a node and verifies sibling sort_order is
// densely rewritten and self-descendant cycles are rejected.
func TestMoveNodeReorders(t *testing.T) {
	db := tempDB(t)
	a, _ := db.CreateNode("", "A", "folder", "")
	b, _ := db.CreateNode("", "B", "folder", "")
	c, _ := db.CreateNode("", "C", "folder", "")

	// Move C to index 0 (before A).
	if err := db.MoveNode(c.ID, "", 0); err != nil {
		t.Fatalf("move C to top: %v", err)
	}
	got, _ := db.GetChildren("")
	want := []string{c.ID, a.ID, b.ID}
	for i, n := range got {
		if n.ID != want[i] {
			t.Errorf("order[%d] = %q, want %q", i, n.ID, want[i])
		}
	}

	// Move A inside B (becomes B's child).
	if err := db.MoveNode(a.ID, b.ID, 0); err != nil {
		t.Fatalf("move A into B: %v", err)
	}
	bKids, _ := db.GetChildren(b.ID)
	if len(bKids) != 1 || bKids[0].ID != a.ID {
		t.Errorf("A not inside B: %+v", bKids)
	}

	// Cycle: trying to move B into its own descendant A must fail.
	if err := db.MoveNode(b.ID, a.ID, 0); err == nil {
		t.Error("moving B into descendant A should error")
	}

	// Moving B into itself must fail.
	if err := db.MoveNode(b.ID, b.ID, 0); err == nil {
		t.Error("moving B into itself should error")
	}
}

// TestDeleteLeaf checks deleting a single request leaf.
func TestDeleteLeaf(t *testing.T) {
	db := tempDB(t)
	f, _ := db.CreateNode("", "F", "folder", "")
	r, _ := db.CreateNode(f.ID, "R", "request", "GET")
	_, _ = db.GetChildren(f.ID)

	if err := db.DeleteNode(r.ID); err != nil {
		t.Fatalf("DeleteNode leaf: %v", err)
	}
	if d, _ := db.GetRequestDetail(r.ID); d != nil {
		t.Error("leaf detail still present")
	}
	if kids, _ := db.GetChildren(f.ID); len(kids) != 0 {
		t.Errorf("folder still has %d children", len(kids))
	}
}
