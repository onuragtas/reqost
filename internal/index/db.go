package index

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"

	"reqost/internal/collection"
)

const schema = `
PRAGMA journal_mode=WAL;
PRAGMA synchronous=NORMAL;
PRAGMA cache_size=-32000;
PRAGMA busy_timeout=5000;

CREATE TABLE IF NOT EXISTS meta (
	path  TEXT PRIMARY KEY,
	mtime INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS tree (
	id         TEXT PRIMARY KEY,
	name       TEXT NOT NULL,
	parent_id  TEXT,
	type       TEXT NOT NULL,
	method     TEXT NOT NULL DEFAULT '',
	sort_order INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_tree_parent ON tree(parent_id);

CREATE TABLE IF NOT EXISTS detail (
	id            TEXT PRIMARY KEY,
	url           TEXT NOT NULL DEFAULT '',
	headers_json  TEXT NOT NULL DEFAULT '[]',
	body          TEXT NOT NULL DEFAULT '',
	pre_script    TEXT NOT NULL DEFAULT '',
	post_script   TEXT NOT NULL DEFAULT '',
	description   TEXT NOT NULL DEFAULT '',
	body_type     TEXT NOT NULL DEFAULT '',
	form_fields   TEXT NOT NULL DEFAULT '[]',
	graphql_vars  TEXT NOT NULL DEFAULT '',
	grpc_method   TEXT NOT NULL DEFAULT '',
	auth_json     TEXT NOT NULL DEFAULT '',
	settings_json TEXT NOT NULL DEFAULT '{}'
);

CREATE VIRTUAL TABLE IF NOT EXISTS search_fts USING fts5(
	id UNINDEXED,
	name,
	url,
	body
);
`

type DB struct {
	conn *sql.DB
}

func Open() (*DB, error) {
	path, err := dbPath()
	if err != nil {
		return nil, err
	}
	return OpenAt(path)
}

// OpenAt opens (or creates) an index at an explicit path. Used by Open and by
// tests that need an isolated database.
func OpenAt(path string) (*DB, error) {
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	conn.SetMaxOpenConns(1)
	if _, err := conn.Exec(schema); err != nil {
		conn.Close()
		return nil, fmt.Errorf("init schema: %w", err)
	}
	migrate(conn)
	return &DB{conn: conn}, nil
}

// migrate adds columns introduced after the initial schema to pre-existing
// databases. ALTER TABLE ADD COLUMN errors (duplicate column) are ignored.
func migrate(conn *sql.DB) {
	for _, stmt := range []string{
		`ALTER TABLE detail ADD COLUMN body_type TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE detail ADD COLUMN form_fields TEXT NOT NULL DEFAULT '[]'`,
		`ALTER TABLE detail ADD COLUMN graphql_vars TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE detail ADD COLUMN grpc_method TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE detail ADD COLUMN auth_json TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE detail ADD COLUMN settings_json TEXT NOT NULL DEFAULT '{}'`,
	} {
		_, _ = conn.Exec(stmt) // ignore "duplicate column name"
	}
}

func (db *DB) Close() error {
	return db.conn.Close()
}

func dbPath() (string, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("user cache dir: %w", err)
	}
	dir := filepath.Join(cacheDir, "reqost")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create cache dir: %w", err)
	}
	log.Default().Printf("using index at %s", dir)
	return filepath.Join(dir, "index.db"), nil
}

func (db *DB) GetMtime(path string) (int64, error) {
	var mtime int64
	err := db.conn.QueryRow("SELECT mtime FROM meta WHERE path = ?", path).Scan(&mtime)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	return mtime, err
}

// ImportItems merges a fresh parse into the index incrementally:
//   - Nodes present only in the existing index are deleted (with their detail
//     + FTS rows).
//   - Nodes shared by id between import and existing rows have their `tree`
//     fields (name/parent/type/method/sort_order) updated, but their `detail`
//     row is left untouched so in-app edits survive a re-import.
//   - Nodes new in the import are inserted (tree + detail + FTS).
//
// FTS is refreshed for new rows and for any existing row whose name changed,
// using the kept detail.url/body so the FTS index stays consistent with
// what's actually stored.
//
// Pre-tx reads (existing ids + name/url/body) run on db.conn to avoid the
// SQLite WAL Query+Exec self-deadlock the rest of mutate.go is structured
// around.
func (db *DB) ImportItems(path string, mtime int64, items []collection.FlatItem) error {
	type existingRow struct {
		name, url, body string
	}
	existing := map[string]existingRow{}
	rows, err := db.conn.Query(`
		SELECT t.id, t.name, COALESCE(d.url, ''), COALESCE(d.body, '')
		FROM tree t LEFT JOIN detail d ON t.id = d.id`)
	if err != nil {
		return fmt.Errorf("read existing: %w", err)
	}
	for rows.Next() {
		var id string
		var r existingRow
		if err := rows.Scan(&id, &r.name, &r.url, &r.body); err != nil {
			rows.Close()
			return err
		}
		existing[id] = r
	}
	rows.Close()

	newIDs := make(map[string]struct{}, len(items))
	for _, it := range items {
		newIDs[it.ID] = struct{}{}
	}
	var obsolete []string
	for id := range existing {
		if _, keep := newIDs[id]; !keep {
			obsolete = append(obsolete, id)
		}
	}

	tx, err := db.conn.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	if len(obsolete) > 0 {
		for _, t := range []string{"detail", "search_fts", "tree"} {
			if err := deleteByIDs(tx, t, obsolete); err != nil {
				return fmt.Errorf("delete obsolete from %s: %w", t, err)
			}
		}
	}

	treeInsert, err := tx.Prepare(`
		INSERT INTO tree (id, name, parent_id, type, method, sort_order)
		VALUES (?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer treeInsert.Close()

	treeUpdate, err := tx.Prepare(`
		UPDATE tree SET name = ?, parent_id = ?, type = ?, method = ?, sort_order = ?
		WHERE id = ?`)
	if err != nil {
		return err
	}
	defer treeUpdate.Close()

	detailInsert, err := tx.Prepare(`
		INSERT INTO detail (id, url, headers_json, body, pre_script, post_script, description,
		                    body_type, form_fields, graphql_vars, auth_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer detailInsert.Close()

	ftsDelete, err := tx.Prepare(`DELETE FROM search_fts WHERE id = ?`)
	if err != nil {
		return err
	}
	defer ftsDelete.Close()

	ftsInsert, err := tx.Prepare(`
		INSERT INTO search_fts (id, name, url, body)
		VALUES (?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer ftsInsert.Close()

	for _, item := range items {
		parentID := nullStr(item.ParentID)
		prev, isExisting := existing[item.ID]

		if isExisting {
			if _, err := treeUpdate.Exec(item.Name, parentID, item.Type, item.Method, item.SortOrder, item.ID); err != nil {
				return fmt.Errorf("update tree %s: %w", item.ID, err)
			}
			// Only refresh FTS if the displayed name changed; keep the detail
			// columns the user may have edited (url/body) as the source of truth.
			if prev.name != item.Name {
				if _, err := ftsDelete.Exec(item.ID); err != nil {
					return fmt.Errorf("refresh fts %s: %w", item.ID, err)
				}
				if _, err := ftsInsert.Exec(item.ID, item.Name, prev.url, prev.body); err != nil {
					return fmt.Errorf("refresh fts %s: %w", item.ID, err)
				}
			}
			continue
		}

		// New node: full insert.
		if _, err := treeInsert.Exec(item.ID, item.Name, parentID, item.Type, item.Method, item.SortOrder); err != nil {
			return fmt.Errorf("insert tree %s: %w", item.ID, err)
		}
		if item.Type == "request" {
			formFields := item.FormFields
			if formFields == "" {
				formFields = "[]"
			}
			if _, err := detailInsert.Exec(item.ID, item.URL, item.HeadersJSON, item.Body, item.PreScript, item.PostScript, item.Description,
				item.BodyType, formFields, item.GraphqlVars, item.AuthJSON); err != nil {
				return fmt.Errorf("insert detail %s: %w", item.ID, err)
			}
		}
		if _, err := ftsInsert.Exec(item.ID, item.Name, item.URL, item.Body); err != nil {
			return fmt.Errorf("insert fts %s: %w", item.ID, err)
		}
	}

	// meta is a single-row table per the schema's intent; clear-then-insert.
	if _, err := tx.Exec("DELETE FROM meta"); err != nil {
		return fmt.Errorf("clear meta: %w", err)
	}
	if _, err := tx.Exec("INSERT INTO meta (path, mtime) VALUES (?, ?)", path, mtime); err != nil {
		return fmt.Errorf("insert meta: %w", err)
	}

	return tx.Commit()
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}
