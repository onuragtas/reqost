package index

import (
	"database/sql"
	"fmt"
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
	if query == "" {
		return []TreeNode{}, nil
	}
	rows, err := db.conn.Query(`
		SELECT t.id, t.name, COALESCE(t.parent_id, ''), t.type, t.method, t.sort_order,
		       EXISTS(SELECT 1 FROM tree c WHERE c.parent_id = t.id) AS has_children
		FROM search_fts f
		JOIN tree t ON t.id = f.id
		WHERE search_fts MATCH ?
		ORDER BY rank
		LIMIT 300`, query)
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}
	defer rows.Close()
	return scanNodes(rows)
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
		       d.body_type, d.form_fields, d.graphql_vars, d.grpc_method, d.auth_json, d.settings_json
		FROM detail d
		JOIN tree t ON t.id = d.id
		WHERE d.id = ?`, id).Scan(
		&d.ID, &d.Name, &d.Method, &d.URL,
		&d.Headers, &d.Body, &d.PreScript, &d.PostScript, &d.Description,
		&d.BodyType, &d.FormFields, &d.GraphqlVars, &d.GrpcMethod, &d.Auth, &d.Settings,
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
