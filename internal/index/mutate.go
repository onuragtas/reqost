package index

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"reqost/internal/collection"
)

// NewID returns a random node id for user-created tree nodes. Imported nodes
// keep their Postman ids; created ones get these.
func NewID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return "rq-" + hex.EncodeToString(b[:])
}

// reindexFTS rewrites the FTS row for one node.
func reindexFTS(tx *sql.Tx, id, name, url, body string) error {
	if _, err := tx.Exec("DELETE FROM search_fts WHERE id = ?", id); err != nil {
		return err
	}
	_, err := tx.Exec("INSERT INTO search_fts (id, name, url, body) VALUES (?, ?, ?, ?)", id, name, url, body)
	return err
}

// SaveRequest persists edits to an existing request: tree (name/method),
// detail (url/headers/body/scripts/description) and the FTS row.
func (db *DB) SaveRequest(d RequestDetail) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck

	if _, err := tx.Exec(`UPDATE tree SET name = ?, method = ? WHERE id = ?`, d.Name, d.Method, d.ID); err != nil {
		return fmt.Errorf("update tree: %w", err)
	}
	if _, err := tx.Exec(`
		UPDATE detail SET url = ?, headers_json = ?, body = ?, pre_script = ?, post_script = ?, description = ?,
		                  body_type = ?, form_fields = ?, graphql_vars = ?, grpc_method = ?, auth_json = ?, settings_json = ?
		WHERE id = ?`,
		d.URL, nonEmptyJSON(d.Headers), d.Body, d.PreScript, d.PostScript, d.Description,
		d.BodyType, nonEmptyJSON(d.FormFields), d.GraphqlVars, d.GrpcMethod, d.Auth, nonEmptyJSONObject(d.Settings), d.ID); err != nil {
		return fmt.Errorf("update detail: %w", err)
	}
	if err := reindexFTS(tx, d.ID, d.Name, d.URL, d.Body); err != nil {
		return fmt.Errorf("reindex: %w", err)
	}
	return tx.Commit()
}

// CreateNode inserts a new folder or request under parentID (empty == root) and
// returns the created node. Requests get an empty detail row.
func (db *DB) CreateNode(parentID, name, nodeType, method string) (TreeNode, error) {
	id := NewID()
	tx, err := db.conn.Begin()
	if err != nil {
		return TreeNode{}, err
	}
	defer tx.Rollback() //nolint:errcheck

	// Append: sort_order after the current max among siblings.
	var order int
	row := tx.QueryRow(`SELECT COALESCE(MAX(sort_order)+1, 0) FROM tree WHERE parent_id IS ?`, nullStr(parentID))
	if err := row.Scan(&order); err != nil {
		return TreeNode{}, fmt.Errorf("next order: %w", err)
	}

	if _, err := tx.Exec(`
		INSERT INTO tree (id, name, parent_id, type, method, sort_order)
		VALUES (?, ?, ?, ?, ?, ?)`, id, name, nullStr(parentID), nodeType, method, order); err != nil {
		return TreeNode{}, fmt.Errorf("insert tree: %w", err)
	}
	if nodeType == "request" {
		if _, err := tx.Exec(`INSERT INTO detail (id) VALUES (?)`, id); err != nil {
			return TreeNode{}, fmt.Errorf("insert detail: %w", err)
		}
	}
	if err := reindexFTS(tx, id, name, "", ""); err != nil {
		return TreeNode{}, fmt.Errorf("reindex: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return TreeNode{}, err
	}
	return TreeNode{ID: id, Name: name, ParentID: parentID, Type: nodeType, Method: method, SortOrder: order}, nil
}

// AddItems inserts flat items into the index WITHOUT clearing existing data —
// used to merge an imported OpenAPI spec into the current collection. Callers
// must supply unique ids and consistent ParentIDs.
func (db *DB) AddItems(items []collection.FlatItem) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck

	for _, item := range items {
		parentID := nullStr(item.ParentID)
		if _, err := tx.Exec(`INSERT INTO tree (id, name, parent_id, type, method, sort_order) VALUES (?, ?, ?, ?, ?, ?)`,
			item.ID, item.Name, parentID, item.Type, item.Method, item.SortOrder); err != nil {
			return fmt.Errorf("insert tree %s: %w", item.ID, err)
		}
		if item.Type == "request" {
			ff := item.FormFields
			if ff == "" {
				ff = "[]"
			}
			if _, err := tx.Exec(`
				INSERT INTO detail (id, url, headers_json, body, pre_script, post_script, description,
				                    body_type, form_fields, graphql_vars, auth_json)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				item.ID, item.URL, nonEmptyJSON(item.HeadersJSON), item.Body, item.PreScript, item.PostScript,
				item.Description, item.BodyType, ff, item.GraphqlVars, item.AuthJSON); err != nil {
				return fmt.Errorf("insert detail %s: %w", item.ID, err)
			}
		}
		if err := reindexFTS(tx, item.ID, item.Name, item.URL, item.Body); err != nil {
			return fmt.Errorf("reindex %s: %w", item.ID, err)
		}
	}
	return tx.Commit()
}

// RenameNode updates a node's display name (and its FTS name).
func (db *DB) RenameNode(id, name string) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	if _, err := tx.Exec(`UPDATE tree SET name = ? WHERE id = ?`, name, id); err != nil {
		return err
	}
	// Keep url/body in FTS by reading them back if this is a request.
	var url, body string
	_ = tx.QueryRow(`SELECT COALESCE(url,''), COALESCE(body,'') FROM detail WHERE id = ?`, id).Scan(&url, &body)
	if err := reindexFTS(tx, id, name, url, body); err != nil {
		return err
	}
	return tx.Commit()
}

// DeleteNode removes a node and all its descendants. The descendant ids are
// read OUTSIDE the write transaction (a single recursive CTE), then the
// transaction does writes only. Mixing tx.Query and tx.Exec in one transaction
// on modernc.org/sqlite + WAL can self-lock with SQLITE_BUSY, which silently
// failed deletes from the UI.
func (db *DB) DeleteNode(id string) error {
	ids, err := db.subtreeIDs(id)
	if err != nil {
		return fmt.Errorf("collect subtree: %w", err)
	}

	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck

	// Delete with chunked `id IN (...)` rather than one statement per id.
	// search_fts has `id UNINDEXED` (no index), so a per-id delete full-scans the
	// whole FTS table EACH time — fine for one request, but O(N × rows) for a
	// folder with many descendants, which froze the UI. One IN-statement scans
	// the FTS once per chunk instead.
	for _, t := range []string{"detail", "search_fts", "tree"} {
		if err := deleteByIDs(tx, t, ids); err != nil {
			return fmt.Errorf("delete from %s: %w", t, err)
		}
	}
	return tx.Commit()
}

// deleteByIDs deletes rows whose id is in ids, in chunks to stay under SQLite's
// bound-parameter limit.
func deleteByIDs(tx *sql.Tx, table string, ids []string) error {
	const chunk = 500
	for start := 0; start < len(ids); start += chunk {
		end := start + chunk
		if end > len(ids) {
			end = len(ids)
		}
		batch := ids[start:end]
		ph := make([]string, len(batch))
		args := make([]any, len(batch))
		for i, id := range batch {
			ph[i] = "?"
			args[i] = id
		}
		q := "DELETE FROM " + table + " WHERE id IN (" + strings.Join(ph, ",") + ")"
		if _, err := tx.Exec(q, args...); err != nil {
			return err
		}
	}
	return nil
}

// ErrInvalidMove is returned when MoveNode would create a cycle or move a node
// into itself.
var ErrInvalidMove = errors.New("invalid move: cannot move into self or descendant")

// MoveNode reparents id under newParentID (empty == root) at the given
// 0-based index among that parent's children. Sibling sort_order values are
// rewritten densely (0..N) in one transaction. Reads (subtree, siblings) run
// on db.conn so writes inside the transaction don't self-deadlock on WAL.
func (db *DB) MoveNode(id, newParentID string, newIndex int) error {
	if id == "" {
		return fmt.Errorf("MoveNode: id required")
	}
	if id == newParentID {
		return ErrInvalidMove
	}

	// Reject moving into our own subtree.
	descendants, err := db.subtreeIDs(id)
	if err != nil {
		return fmt.Errorf("collect subtree: %w", err)
	}
	if newParentID != "" {
		for _, d := range descendants {
			if d == newParentID {
				return ErrInvalidMove
			}
		}
	}

	// Read the destination siblings (excluding id) on db.conn before opening
	// the write transaction. Same WAL precaution as DeleteNode.
	var (
		siblings []string
		rows     *sql.Rows
	)
	if newParentID == "" {
		rows, err = db.conn.Query(`SELECT id FROM tree WHERE parent_id IS NULL AND id != ? ORDER BY sort_order, id`, id)
	} else {
		rows, err = db.conn.Query(`SELECT id FROM tree WHERE parent_id = ? AND id != ? ORDER BY sort_order, id`, newParentID, id)
	}
	if err != nil {
		return fmt.Errorf("read siblings: %w", err)
	}
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			rows.Close()
			return err
		}
		siblings = append(siblings, s)
	}
	rows.Close()

	if newIndex < 0 {
		newIndex = 0
	}
	if newIndex > len(siblings) {
		newIndex = len(siblings)
	}

	// ordered = siblings[:newIndex] + [id] + siblings[newIndex:]
	ordered := make([]string, 0, len(siblings)+1)
	ordered = append(ordered, siblings[:newIndex]...)
	ordered = append(ordered, id)
	ordered = append(ordered, siblings[newIndex:]...)

	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck

	if _, err := tx.Exec(`UPDATE tree SET parent_id = ? WHERE id = ?`, nullStr(newParentID), id); err != nil {
		return fmt.Errorf("reparent: %w", err)
	}
	for i, sid := range ordered {
		if _, err := tx.Exec(`UPDATE tree SET sort_order = ? WHERE id = ?`, i, sid); err != nil {
			return fmt.Errorf("reorder: %w", err)
		}
	}
	return tx.Commit()
}

// DuplicateNode creates a deep copy of id (including all descendants for
// folders) inserted as the next sibling of the original. Returns the new root.
// All reads happen on db.conn before the write transaction (WAL precaution).
func (db *DB) DuplicateNode(id string) (TreeNode, error) {
	type srcRow struct {
		id, name, parentID, nodeType, method string
		sortOrder                            int
	}

	// 1. Read source node.
	var src srcRow
	err := db.conn.QueryRow(`
		SELECT id, name, COALESCE(parent_id,''), type, method, sort_order
		FROM tree WHERE id = ?`, id).Scan(
		&src.id, &src.name, &src.parentID, &src.nodeType, &src.method, &src.sortOrder)
	if errors.Is(err, sql.ErrNoRows) {
		return TreeNode{}, fmt.Errorf("node %s not found", id)
	}
	if err != nil {
		return TreeNode{}, fmt.Errorf("read source: %w", err)
	}

	// 2. Read full subtree via recursive CTE.
	subtreeRows, err := db.conn.Query(`
		WITH RECURSIVE sub(id) AS (
			SELECT ?
			UNION ALL
			SELECT t.id FROM tree t JOIN sub ON t.parent_id = sub.id
		)
		SELECT t.id, t.name, COALESCE(t.parent_id,''), t.type, t.method, t.sort_order
		FROM tree t JOIN sub ON t.id = sub.id`, id)
	if err != nil {
		return TreeNode{}, fmt.Errorf("read subtree: %w", err)
	}
	var nodes []srcRow
	for subtreeRows.Next() {
		var n srcRow
		if err := subtreeRows.Scan(&n.id, &n.name, &n.parentID, &n.nodeType, &n.method, &n.sortOrder); err != nil {
			subtreeRows.Close()
			return TreeNode{}, err
		}
		nodes = append(nodes, n)
	}
	subtreeRows.Close()
	if err := subtreeRows.Err(); err != nil {
		return TreeNode{}, err
	}

	// 3. Read detail rows for request nodes.
	type detailRow struct {
		url, headers, body, preScript, postScript, description string
		bodyType, formFields, graphqlVars, grpcMethod, auth, settings string
	}
	details := make(map[string]detailRow, len(nodes))
	for _, n := range nodes {
		if n.nodeType != "request" {
			continue
		}
		var d detailRow
		e := db.conn.QueryRow(`
			SELECT COALESCE(url,''), COALESCE(headers_json,'[]'), COALESCE(body,''),
			       COALESCE(pre_script,''), COALESCE(post_script,''), COALESCE(description,''),
			       COALESCE(body_type,''), COALESCE(form_fields,'[]'), COALESCE(graphql_vars,''),
			       COALESCE(grpc_method,''), COALESCE(auth_json,'{}'), COALESCE(settings_json,'{}')
			FROM detail WHERE id = ?`, n.id).Scan(
			&d.url, &d.headers, &d.body, &d.preScript, &d.postScript, &d.description,
			&d.bodyType, &d.formFields, &d.graphqlVars, &d.grpcMethod, &d.auth, &d.settings)
		if e != nil && !errors.Is(e, sql.ErrNoRows) {
			return TreeNode{}, fmt.Errorf("read detail %s: %w", n.id, e)
		}
		details[n.id] = d
	}

	// 4. Next sort_order slot for the duplicated root.
	var nextOrder int
	if err := db.conn.QueryRow(`SELECT COALESCE(MAX(sort_order)+1, 0) FROM tree WHERE parent_id IS ?`,
		nullStr(src.parentID)).Scan(&nextOrder); err != nil {
		return TreeNode{}, fmt.Errorf("next order: %w", err)
	}

	// 5. Assign new IDs to every node in the subtree.
	idMap := make(map[string]string, len(nodes))
	for _, n := range nodes {
		idMap[n.id] = NewID()
	}

	// 6. Write transaction — Exec only, no Query (WAL precaution).
	tx, err := db.conn.Begin()
	if err != nil {
		return TreeNode{}, err
	}
	defer tx.Rollback() //nolint:errcheck

	var rootNode TreeNode
	for i, n := range nodes {
		newID := idMap[n.id]
		name := n.name
		var newParentID string
		var newOrder int

		if i == 0 {
			newParentID = src.parentID
			newOrder = nextOrder
			name = "Copy of " + name
		} else {
			newParentID = idMap[n.parentID]
			newOrder = n.sortOrder
		}

		if _, err := tx.Exec(`INSERT INTO tree (id, name, parent_id, type, method, sort_order) VALUES (?, ?, ?, ?, ?, ?)`,
			newID, name, nullStr(newParentID), n.nodeType, n.method, newOrder); err != nil {
			return TreeNode{}, fmt.Errorf("insert tree %s: %w", newID, err)
		}

		if n.nodeType == "request" {
			d := details[n.id]
			if _, err := tx.Exec(`
				INSERT INTO detail (id, url, headers_json, body, pre_script, post_script, description,
				                    body_type, form_fields, graphql_vars, grpc_method, auth_json, settings_json)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				newID, d.url, d.headers, d.body, d.preScript, d.postScript, d.description,
				d.bodyType, d.formFields, d.graphqlVars, d.grpcMethod, d.auth, d.settings); err != nil {
				return TreeNode{}, fmt.Errorf("insert detail %s: %w", newID, err)
			}
		}

		det := details[n.id]
		if err := reindexFTS(tx, newID, name, det.url, det.body); err != nil {
			return TreeNode{}, fmt.Errorf("reindex %s: %w", newID, err)
		}

		if i == 0 {
			rootNode = TreeNode{ID: newID, Name: name, ParentID: newParentID, Type: n.nodeType, Method: n.method, SortOrder: newOrder}
		}
	}

	return rootNode, tx.Commit()
}

// subtreeIDs returns id plus all descendant ids via a recursive CTE, read on a
// plain (non-transaction) connection so the cursor is closed before any write.
func (db *DB) subtreeIDs(id string) ([]string, error) {
	rows, err := db.conn.Query(`
		WITH RECURSIVE sub(id) AS (
			SELECT ?
			UNION ALL
			SELECT t.id FROM tree t JOIN sub ON t.parent_id = sub.id
		)
		SELECT id FROM sub`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		ids = append(ids, s)
	}
	return ids, rows.Err()
}

// nonEmptyJSON normalizes an empty headers payload to "[]".
func nonEmptyJSON(s string) string {
	if s == "" {
		return "[]"
	}
	return s
}

// nonEmptyJSONObject normalizes an empty payload to "{}".
func nonEmptyJSONObject(s string) string {
	if s == "" {
		return "{}"
	}
	return s
}
