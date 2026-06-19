package index

import (
	"testing"

	"reqost/internal/collection"
)

// TestSearchTurkishCaseAndPrefix covers the two complaints users hit on real
// banking-API style collections:
//
//   - "yapı" must find "YAPI ve KREDİ BANKASI A.Ş." (Turkish case-folding,
//     diacritic-insensitive thanks to the unicode61 tokenizer).
//   - Partial typing must match — "yapı" finds "YAPI" even though FTS5's
//     default behaviour is whole-token equality. We handle that by appending
//     `*` to each token in the query builder.
func TestSearchTurkishCaseAndPrefix(t *testing.T) {
	db := tempDB(t)
	items := []collection.FlatItem{
		{ID: "r1", Name: "YAPI ve KREDİ BANKASI A.Ş.", ParentID: "", Type: "request", Method: "GET", URL: "https://yapikredi.com.tr/login"},
		{ID: "r2", Name: "Garanti BBVA", ParentID: "", Type: "request", Method: "GET", URL: "https://garanti.com.tr/x"},
		{ID: "r3", Name: "Şirket Logosu", ParentID: "", Type: "request", Method: "GET", URL: "https://example.com/sirket"},
	}
	if err := db.AddItems(items); err != nil {
		t.Fatalf("AddItems: %v", err)
	}

	cases := []struct {
		query     string
		wantHasID string
		notWantID string
	}{
		{"yapı", "r1", "r2"},        // diacritic + case fold
		{"YAPI", "r1", "r2"},        // upper-case input
		{"yap", "r1", ""},           // prefix match
		{"kred", "r1", ""},          // mid-name prefix
		{"BANKASI", "r1", "r2"},     // exact uppercase
		{"garanti", "r2", "r1"},     // sanity check the negative case
		{"şirket", "r3", "r1"},      // diacritic-aware
		{"sirket", "r3", "r1"},      // no diacritic in query also matches
	}

	for _, c := range cases {
		hits, err := db.Search(c.query)
		if err != nil {
			t.Errorf("Search(%q): %v", c.query, err)
			continue
		}
		found := false
		negFound := false
		for _, h := range hits {
			if h.ID == c.wantHasID {
				found = true
			}
			if c.notWantID != "" && h.ID == c.notWantID {
				negFound = true
			}
		}
		if !found {
			t.Errorf("Search(%q): expected to find %q, got %d hits", c.query, c.wantHasID, len(hits))
		}
		if negFound {
			t.Errorf("Search(%q): did NOT expect %q in hits", c.query, c.notWantID)
		}
	}
}

func TestSearchEmptyAndPunctuation(t *testing.T) {
	db := tempDB(t)
	_ = db.AddItems([]collection.FlatItem{
		{ID: "r1", Name: "Foo Bar", ParentID: "", Type: "request", Method: "GET", URL: "https://example.com/path"},
	})

	if hits, _ := db.Search(""); len(hits) != 0 {
		t.Errorf("empty query should return no hits")
	}
	// Pure punctuation must not error and must return no hits.
	if hits, err := db.Search("..."); err != nil || len(hits) != 0 {
		t.Errorf("punct-only query: hits=%d err=%v", len(hits), err)
	}
	// Punctuation between tokens IS treated as a separator (FTS5 is
	// token-based), and each piece is matched as a prefix — so "Foo Bar"
	// is still discoverable via either word, with or without padding dots.
	hits, _ := db.Search("foo")
	if len(hits) == 0 {
		t.Errorf("foo should find Foo Bar")
	}
	hits, _ = db.Search("foo. bar.")
	if len(hits) == 0 {
		t.Errorf("trailing-dot tokens should still find Foo Bar")
	}
}
