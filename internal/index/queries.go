package index

import (
	"database/sql"
	"fmt"
	"strings"
	"unicode"
)

type TreeNode struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	ParentID    string `json:"parentId"`
	Type        string `json:"type"`
	Method      string `json:"method"`
	SortOrder   int    `json:"sortOrder"`
	HasChildren bool   `json:"hasChildren"`
}

type RequestDetail struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Method      string `json:"method"`
	URL         string `json:"url"`
	Headers     string `json:"headers"`
	Body        string `json:"body"`
	PreScript   string `json:"preScript"`
	PostScript  string `json:"postScript"`
	Description string `json:"description"`
	BodyType    string `json:"bodyType"`
	FormFields  string `json:"formFields"` // JSON array
	GraphqlVars string `json:"graphqlVars"`
	GrpcMethod  string `json:"grpcMethod"`
	Auth        string `json:"auth"`     // JSON object
	Settings    string `json:"settings"` // JSON object: per-request execution settings
	Examples    string `json:"examples"` // JSON array of saved request+response snapshots
}

const nodeSelect = `
	SELECT t.id, t.name, COALESCE(t.parent_id, ''), t.type, t.method, t.sort_order,
	       EXISTS(SELECT 1 FROM tree c WHERE c.parent_id = t.id) AS has_children
	FROM tree t`

func (db *DB) GetChildren(parentID string) ([]TreeNode, error) {
	var (
		rows *sql.Rows
		err  error
	)
	if parentID == "" {
		rows, err = db.conn.Query(nodeSelect + ` WHERE t.parent_id IS NULL ORDER BY t.sort_order`)
	} else {
		rows, err = db.conn.Query(nodeSelect+` WHERE t.parent_id = ? ORDER BY t.sort_order`, parentID)
	}
	if err != nil {
		return nil, fmt.Errorf("get children: %w", err)
	}
	defer rows.Close()
	return scanNodes(rows)
}

func (db *DB) Search(query string) ([]TreeNode, error) {
	fts := buildFTSQuery(query)
	if fts == "" {
		return []TreeNode{}, nil
	}
	rows, err := db.conn.Query(`
		SELECT t.id, t.name, COALESCE(t.parent_id, ''), t.type, t.method, t.sort_order,
		       EXISTS(SELECT 1 FROM tree c WHERE c.parent_id = t.id) AS has_children
		FROM search_fts f
		JOIN tree t ON t.id = f.id
		WHERE search_fts MATCH ?
		ORDER BY rank
		LIMIT 300`, fts)
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}
	defer rows.Close()
	return scanNodes(rows)
}

// buildFTSQuery turns user input into a safe FTS5 MATCH expression.
//
//   - Punctuation that isn't part of an identifier is stripped (so the user
//     can type "YAPI ve KREDİ BANKASI A.Ş." and we don't try to send the dots
//     and the period as FTS5 syntax).
//   - Every character is run through normalizeForSearch — Turkish letters
//     fold to ASCII (İ/I/ı → i, ş → s, ğ → g, ç → c, ö → o, ü → u) so the
//     query lines up with what we stored in search_fts. SQLite's unicode61
//     tokenizer does NOT fold ı→i on its own; the dotless i isn't classified
//     as a diacritic.
//   - Each remaining token gets a trailing `*` for prefix match — so typing
//     "yapı" finds "YAPI KREDI", "kred" finds "kredi", etc.
//   - Tokens are AND-joined so multi-word searches narrow as expected.
func buildFTSQuery(input string) string {
	folded := normalizeForSearch(input)
	var b strings.Builder
	for _, r := range folded {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r):
			b.WriteRune(r)
		default:
			b.WriteRune(' ')
		}
	}
	tokens := strings.Fields(b.String())
	if len(tokens) == 0 {
		return ""
	}
	parts := make([]string, 0, len(tokens))
	for _, t := range tokens {
		// FTS5 prefix syntax: `bareword*`. Bareword must be alphanumeric, no
		// quotes. We already stripped everything else, so `t` is safe to drop in.
		parts = append(parts, t+"*")
	}
	return strings.Join(parts, " AND ")
}

// normalizeForSearch lower-cases the input and ASCII-folds the handful of
// Turkish letters the standard Unicode case-folding tables miss. This runs
// both at index time (so "YAPI" goes into search_fts as "yapi") and at query
// time (so "yapı" also becomes "yapi"), keeping the two sides aligned.
func normalizeForSearch(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch r {
		case 'ı', 'I', 'İ':
			b.WriteRune('i')
		case 'ş', 'Ş':
			b.WriteRune('s')
		case 'ğ', 'Ğ':
			b.WriteRune('g')
		case 'ç', 'Ç':
			b.WriteRune('c')
		case 'ö', 'Ö':
			b.WriteRune('o')
		case 'ü', 'Ü':
			b.WriteRune('u')
		default:
			b.WriteRune(unicode.ToLower(r))
		}
	}
	return b.String()
}

// RequestsUnder returns every request node at or below rootID (empty == whole
// collection) in depth-first, sort_order sequence — the order a runner fires
// them in.
func (db *DB) RequestsUnder(rootID string) ([]TreeNode, error) {
	var out []TreeNode
	var walk func(parentID string) error
	walk = func(parentID string) error {
		children, err := db.GetChildren(parentID)
		if err != nil {
			return err
		}
		for _, c := range children {
			if c.Type == "request" {
				out = append(out, c)
			} else {
				if err := walk(c.ID); err != nil {
					return err
				}
			}
		}
		return nil
	}
	// A request rootID has no children; treat it as a single-request run.
	if rootID != "" {
		node, err := db.GetChildren(rootID)
		if err != nil {
			return nil, err
		}
		if len(node) == 0 {
			// Could be a leaf request; check its own type via detail existence.
			if d, _ := db.GetRequestDetail(rootID); d != nil {
				return []TreeNode{{ID: rootID, Name: d.Name, Type: "request", Method: d.Method}}, nil
			}
		}
	}
	if err := walk(rootID); err != nil {
		return nil, err
	}
	if out == nil {
		out = []TreeNode{}
	}
	return out, nil
}

func (db *DB) GetRequestDetail(id string) (*RequestDetail, error) {
	var d RequestDetail
	err := db.conn.QueryRow(`
		SELECT d.id, t.name, t.method, d.url, d.headers_json, d.body, d.pre_script, d.post_script, d.description,
		       d.body_type, d.form_fields, d.graphql_vars, d.grpc_method, d.auth_json, d.settings_json, d.examples_json
		FROM detail d
		JOIN tree t ON t.id = d.id
		WHERE d.id = ?`, id).Scan(
		&d.ID, &d.Name, &d.Method, &d.URL,
		&d.Headers, &d.Body, &d.PreScript, &d.PostScript, &d.Description,
		&d.BodyType, &d.FormFields, &d.GraphqlVars, &d.GrpcMethod, &d.Auth, &d.Settings, &d.Examples,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get request detail: %w", err)
	}
	return &d, nil
}

func scanNodes(rows *sql.Rows) ([]TreeNode, error) {
	var nodes []TreeNode
	for rows.Next() {
		var n TreeNode
		if err := rows.Scan(&n.ID, &n.Name, &n.ParentID, &n.Type, &n.Method, &n.SortOrder, &n.HasChildren); err != nil {
			return nil, err
		}
		nodes = append(nodes, n)
	}
	if nodes == nil {
		nodes = []TreeNode{}
	}
	return nodes, rows.Err()
}
