package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"reqost/internal/collection"
	"reqost/internal/har"
	"reqost/internal/index"
	"reqost/internal/openapi"
	"reqost/internal/watcher"
	"reqost/internal/workspaces"
)

// EventEmitter is satisfied by app.Event in Wails v3.
type EventEmitter interface {
	Emit(string, ...any) bool
}

type CollectionService struct {
	mu      sync.Mutex
	db      *index.DB
	watch   *watcher.Watcher
	emitter EventEmitter
	dialog  *application.DialogManager
	envSvc  *EnvService
	ws      *workspaces.Store
}

func NewCollectionService() (*CollectionService, error) {
	ws, err := workspaces.Open()
	if err != nil {
		return nil, fmt.Errorf("open workspaces: %w", err)
	}
	db, err := index.OpenAt(ws.DBPath(ws.ActiveID()))
	if err != nil {
		return nil, fmt.Errorf("open index: %w", err)
	}
	svc := &CollectionService{db: db, ws: ws}

	w, err := watcher.New(func(path string) {
		svc.reimport(path)
	})
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("create watcher: %w", err)
	}
	svc.watch = w
	return svc, nil
}

// ── Workspaces ────────────────────────────────────────────────────────────

// ListWorkspaces returns all known workspaces.
func (s *CollectionService) ListWorkspaces() []workspaces.Workspace {
	return s.ws.List()
}

// ActiveWorkspaceID returns the currently active workspace id.
func (s *CollectionService) ActiveWorkspaceID() string { return s.ws.ActiveID() }

// CreateWorkspace adds a new workspace and returns it. Doesn't switch to it —
// the frontend usually wants to confirm + then SwitchWorkspace.
func (s *CollectionService) CreateWorkspace(name string) (workspaces.Workspace, error) {
	return s.ws.Create(name)
}

// RenameWorkspace updates the display name.
func (s *CollectionService) RenameWorkspace(id, name string) error {
	return s.ws.Rename(id, name)
}

// DeleteWorkspace removes a workspace AND its DB file. Refuses to delete the
// last remaining one.
func (s *CollectionService) DeleteWorkspace(id string) error {
	return s.ws.Delete(id)
}

// SwitchWorkspace tears down the current index, opens the requested
// workspace's DB, and emits collection:ready so the frontend reloads the
// tree. Concurrent calls are serialized.
func (s *CollectionService) SwitchWorkspace(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if id == s.ws.ActiveID() {
		return nil
	}
	if err := s.ws.SetActive(id); err != nil {
		return err
	}
	if s.db != nil {
		_ = s.db.Close()
	}
	newDB, err := index.OpenAt(s.ws.DBPath(id))
	if err != nil {
		return fmt.Errorf("open workspace db: %w", err)
	}
	s.db = newDB
	s.emit("collection:ready", "workspace-switch")
	return nil
}

func (s *CollectionService) setEmitter(e EventEmitter) {
	s.emitter = e
}

func (s *CollectionService) setDialog(d *application.DialogManager) {
	s.dialog = d
}

func (s *CollectionService) setEnvSvc(e *EnvService) {
	s.envSvc = e
}

// PickImport opens a native open-file dialog and imports the chosen collection.
// Returns the selected path ("" if the user cancelled). Import runs async and
// emits the usual collection:* events.
func (s *CollectionService) PickImport() (string, error) {
	if s.dialog == nil {
		return "", fmt.Errorf("dialog unavailable")
	}
	d := s.dialog.OpenFile()
	d.SetTitle("Import Postman collection")
	d.AddFilter("JSON", "*.json")
	d.CanChooseFiles(true)
	path, err := d.PromptForSingleSelection()
	if err != nil {
		return "", err
	}
	if path != "" {
		go s.reimport(path)
	}
	return path, nil
}

// PickImportOpenAPI opens a native dialog, parses an OpenAPI 3 / Swagger 2 spec
// and MERGES it into the current collection (under a folder named after the
// spec). Returns the selected path ("" if cancelled).
func (s *CollectionService) PickImportOpenAPI() (string, error) {
	if s.dialog == nil {
		return "", fmt.Errorf("dialog unavailable")
	}
	d := s.dialog.OpenFile()
	d.SetTitle("Import OpenAPI / Swagger spec")
	d.AddFilter("OpenAPI (JSON/YAML)", "*.json;*.yaml;*.yml")
	d.CanChooseFiles(true)
	path, err := d.PromptForSingleSelection()
	if err != nil {
		return "", err
	}
	if path == "" {
		return "", nil
	}

	items, _, err := openapi.Parse(path)
	if err != nil {
		s.emit("collection:error", err.Error())
		return "", err
	}
	if err := s.db.AddItems(items); err != nil {
		s.emit("collection:error", fmt.Sprintf("import openapi: %v", err))
		return "", err
	}
	s.emit("collection:ready", path)
	return path, nil
}

// ImportHARBytes merges a HAR JSON document (typically pasted from browser
// DevTools' "Save all as HAR") into the current collection. Each entry becomes
// a request under a new HAR-tagged folder. Returns the number of imported
// requests, or an error if the JSON is not valid HAR.
func (s *CollectionService) ImportHARBytes(data string) (int, error) {
	items, err := har.Parse([]byte(data))
	if err != nil {
		return 0, err
	}
	if err := s.db.AddItems(items); err != nil {
		return 0, fmt.Errorf("index har: %w", err)
	}
	s.emit("collection:ready", "har-paste")
	// Item 0 is the wrapping folder; rest are requests.
	return len(items) - 1, nil
}

// PickExport opens a native save-file dialog and writes the Postman export to
// the chosen path. Returns the path ("" if cancelled).
func (s *CollectionService) PickExport(name string) (string, error) {
	if s.dialog == nil {
		return "", fmt.Errorf("dialog unavailable")
	}
	d := s.dialog.SaveFile()
	d.SetFilename("reqost-collection.json")
	path, err := d.PromptForSingleSelection()
	if err != nil {
		return "", err
	}
	if path == "" {
		return "", nil
	}
	if err := s.ExportCollectionToFile(path, name); err != nil {
		return "", err
	}
	return path, nil
}

// GetRootItems returns the top-level nodes in the collection tree.
func (s *CollectionService) GetRootItems() ([]index.TreeNode, error) {
	return s.db.GetChildren("")
}

// GetChildren returns the direct children of a folder node.
func (s *CollectionService) GetChildren(parentID string) ([]index.TreeNode, error) {
	return s.db.GetChildren(parentID)
}

// Search runs FTS5 full-text search and returns matching nodes.
func (s *CollectionService) Search(query string) ([]index.TreeNode, error) {
	return s.db.Search(query)
}

// GetRequestDetail loads body/headers/scripts for a request from SQLite.
// This is the ONLY place heavy content is read — not during tree rendering.
func (s *CollectionService) GetRequestDetail(id string) (*index.RequestDetail, error) {
	return s.db.GetRequestDetail(id)
}

// SaveRequest persists edits to an existing request into the index.
func (s *CollectionService) SaveRequest(d index.RequestDetail) error {
	return s.db.SaveRequest(d)
}

// CreateRequest adds a new empty request under parentID (empty == root).
func (s *CollectionService) CreateRequest(parentID, name, method string) (index.TreeNode, error) {
	if method == "" {
		method = "GET"
	}
	return s.db.CreateNode(parentID, name, "request", method)
}

// CreateFolder adds a new folder under parentID (empty == root).
func (s *CollectionService) CreateFolder(parentID, name string) (index.TreeNode, error) {
	return s.db.CreateNode(parentID, name, "folder", "")
}

// RenameNode changes a node's display name.
func (s *CollectionService) RenameNode(id, name string) error {
	return s.db.RenameNode(id, name)
}

// DeleteNode removes a node and all of its descendants.
func (s *CollectionService) DeleteNode(id string) error {
	return s.db.DeleteNode(id)
}

// GetFolderContext loads the folder's inheritance JSON blob (shared headers /
// auth / scripts). Returns "{}" if empty.
func (s *CollectionService) GetFolderContext(id string) (string, error) {
	return s.db.GetFolderContext(id)
}

// SetFolderContext persists the folder's inheritance JSON blob.
func (s *CollectionService) SetFolderContext(id, contextJSON string) error {
	return s.db.SetFolderContext(id, contextJSON)
}

// AncestorContexts returns the folder-context JSON for each ancestor of id,
// root-to-immediate-parent order. The frontend merges these at send time
// (child overrides parent).
func (s *CollectionService) AncestorContexts(id string) ([]string, error) {
	return s.db.AncestorContexts(id)
}

// MoveNode re-parents a node and sets its position among siblings (0-based).
// Used by the sidebar drag-and-drop. Moving into self or a descendant returns
// an error.
func (s *CollectionService) MoveNode(id, newParentID string, newIndex int) error {
	return s.db.MoveNode(id, newParentID, newIndex)
}

// ClearAll removes every item from the collection index.
func (s *CollectionService) ClearAll() error {
	return s.db.ClearAll()
}

// DuplicateNode creates a deep copy of a request or folder (including all
// descendants) as the next sibling of the original.
func (s *CollectionService) DuplicateNode(id string) (index.TreeNode, error) {
	return s.db.DuplicateNode(id)
}

// ListRequestsUnder returns the requests at/below a node in run order. Used by
// the Collection Runner.
func (s *CollectionService) ListRequestsUnder(id string) ([]index.TreeNode, error) {
	return s.db.RequestsUnder(id)
}

// ExportCollection renders the whole index as a Postman v2.1 JSON document.
func (s *CollectionService) ExportCollection(name string) (string, error) {
	return s.db.ExportJSON(name)
}

// ExportCollectionToFile writes the Postman v2.1 export to path. Reliable from
// the webview (the browser download path does not work in WKWebView).
func (s *CollectionService) ExportCollectionToFile(path, name string) error {
	data, err := s.db.ExportJSON(name)
	if err != nil {
		return err
	}
	return os.WriteFile(path, []byte(data), 0o644)
}

// ImportCollection parses collection.json into the SQLite index.
// Skips re-parse if the file mtime is unchanged.
// Returns immediately; emits "collection:ready" or "collection:error" when done.
func (s *CollectionService) ImportCollection(path string) error {
	go s.reimport(path)
	return nil
}

func (s *CollectionService) reimport(path string) {
	info, err := os.Stat(path)
	if err != nil {
		s.emit("collection:error", fmt.Sprintf("stat %s: %v", path, err))
		return
	}

	currentMtime := info.ModTime().Unix()
	storedMtime, err := s.db.GetMtime(path)
	if err != nil {
		log.Printf("get mtime for %s: %v", path, err)
	}

	if currentMtime == storedMtime {
		s.emit("collection:ready", path)
		return
	}

	s.emit("collection:importing", path)
	log.Printf("importing %s ...", path)

	items, vars, err := collection.ParseFile(path)
	if err != nil {
		s.emit("collection:error", err.Error())
		return
	}

	// MergeItems upserts without deleting — so items imported from other sources
	// (Postman API, OpenAPI) are not wiped when a local file is imported.
	if err := s.db.MergeItems(items); err != nil {
		s.emit("collection:error", fmt.Sprintf("index: %v", err))
		return
	}
	if err := s.db.SetMtime(path, currentMtime); err != nil {
		log.Printf("set mtime %s: %v", path, err)
	}

	if len(vars) > 0 && s.envSvc != nil {
		s.envSvc.mergeCollectionVars(path, vars)
	}

	log.Printf("indexed %d items from %s", len(items), path)
	s.emit("collection:ready", path)
}

// ImportFromURL fetches a URL and imports it as a Postman collection or
// OpenAPI/Swagger spec. It auto-detects the format from the content.
// The import is asynchronous; it emits the usual collection:* events.
// Supported URL types:
//   - Any direct JSON/YAML URL (GitHub raw, gist, etc.)
//   - Postman public share links (getpostman.com/collections/…)
//   - Postman API URLs with ?access_key=… or X-Api-Key header
func (s *CollectionService) ImportFromURL(rawURL string) error {
	if rawURL == "" {
		return fmt.Errorf("URL is empty")
	}
	// Normalise: Postman share links redirect; follow them.
	go func() {
		s.emit("collection:importing", rawURL)

		client := &http.Client{
			Timeout: 30 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) > 10 {
					return fmt.Errorf("too many redirects")
				}
				return nil
			},
		}

		req, err := http.NewRequest("GET", rawURL, nil)
		if err != nil {
			s.emit("collection:error", fmt.Sprintf("invalid URL: %v", err))
			return
		}
		req.Header.Set("Accept", "application/json, application/yaml, text/yaml, */*")
		req.Header.Set("User-Agent", "reqost/1.0")

		resp, err := client.Do(req)
		if err != nil {
			s.emit("collection:error", fmt.Sprintf("fetch failed: %v", err))
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			s.emit("collection:error", fmt.Sprintf("HTTP %d from %s", resp.StatusCode, rawURL))
			return
		}

		body, err := io.ReadAll(io.LimitReader(resp.Body, 50<<20)) // 50 MiB cap
		if err != nil {
			s.emit("collection:error", fmt.Sprintf("read response: %v", err))
			return
		}

		ct := resp.Header.Get("Content-Type")
		isYAML := strings.Contains(ct, "yaml") || strings.HasSuffix(strings.ToLower(rawURL), ".yaml") || strings.HasSuffix(strings.ToLower(rawURL), ".yml")

		// Try Postman environment file (JSON only, no items).
		if !isYAML {
			if name, vars, err := collection.ParseEnvBytes(body); err == nil && len(vars) > 0 {
				if s.envSvc != nil {
					s.envSvc.mergeCollectionVars(name, vars)
				}
				log.Printf("imported env %q (%d vars) from URL %s", name, len(vars), rawURL)
				s.emit("collection:ready", rawURL)
				return
			}
		}

		// Try Postman collection (JSON only).
		if !isYAML {
			if items, vars, err := collection.ParseBytes(body); err == nil && len(items) > 0 {
				if err := s.db.MergeItems(items); err != nil {
					s.emit("collection:error", fmt.Sprintf("index collection: %v", err))
					return
				}
				if len(vars) > 0 && s.envSvc != nil {
					s.envSvc.mergeCollectionVars(rawURL, vars)
				}
				log.Printf("imported %d items from URL %s", len(items), rawURL)
				s.emit("collection:ready", rawURL)
				return
			}
		}

		// Fall back to OpenAPI / Swagger (JSON or YAML).
		items, _, err := openapi.ParseBytes(body)
		if err != nil {
			s.emit("collection:error", fmt.Sprintf("unrecognised format: %v", err))
			return
		}
		if err := s.db.AddItems(items); err != nil {
			s.emit("collection:error", fmt.Sprintf("index openapi: %v", err))
			return
		}
		log.Printf("imported OpenAPI %d items from URL %s", len(items), rawURL)
		s.emit("collection:ready", rawURL)
	}()
	return nil
}

// ImportAllFromPostman fetches every collection and environment from the
// Postman API using the given API key, then imports them all.
// Collections are fetched in parallel (up to 8 concurrent requests); SQLite
// writes are serialised. Environments are fetched in parallel afterward.
func (s *CollectionService) ImportAllFromPostman(apiKey string) error {
	if strings.TrimSpace(apiKey) == "" {
		return fmt.Errorf("API key is required")
	}
	go func() {
		s.emit("collection:importing", "Connecting to Postman…")

		// Shared HTTP client — reuses connections across goroutines.
		client := &http.Client{Timeout: 30 * time.Second}

		get := func(url string) ([]byte, error) {
			req, err := http.NewRequest("GET", url, nil)
			if err != nil {
				return nil, err
			}
			req.Header.Set("X-Api-Key", apiKey)
			req.Header.Set("Accept", "application/json")
			resp, err := client.Do(req)
			if err != nil {
				return nil, err
			}
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusUnauthorized {
				return nil, fmt.Errorf("invalid Postman API key")
			}
			if resp.StatusCode != http.StatusOK {
				return nil, fmt.Errorf("HTTP %d for %s", resp.StatusCode, url)
			}
			return io.ReadAll(io.LimitReader(resp.Body, 50<<20))
		}

		// ── 1. List collections ──────────────────────────────────────────────
		listData, err := get("https://api.getpostman.com/collections")
		if err != nil {
			s.emit("collection:error", err.Error())
			return
		}
		var colList struct {
			Collections []struct {
				UID  string `json:"uid"`
				Name string `json:"name"`
			} `json:"collections"`
		}
		if err := json.Unmarshal(listData, &colList); err != nil {
			s.emit("collection:error", "parse collection list: "+err.Error())
			return
		}
		total := len(colList.Collections)

		// ── 2. Fetch all collections in parallel ─────────────────────────────
		type fetchedCol struct {
			sortOrder int
			name      string
			wrapped   []collection.FlatItem
			vars      []collection.CollectionVar
		}

		const concurrency = 8
		sem := make(chan struct{}, concurrency)
		resultsCh := make(chan fetchedCol, total)
		var fetched atomic.Int32
		var wg sync.WaitGroup

		for i, c := range colList.Collections {
			wg.Add(1)
			sem <- struct{}{} // acquire slot
			go func(i int, uid, name string) {
				defer wg.Done()
				defer func() { <-sem }()

				n := int(fetched.Add(1))
				s.emit("collection:importing", fmt.Sprintf("Fetching %d/%d: %s", n, total, name))

				data, err := get("https://api.getpostman.com/collections/" + uid)
				if err != nil {
					log.Printf("postman: skip %s: %v", name, err)
					return
				}
				items, vars, err := collection.ParseBytes(data)
				if err != nil || len(items) == 0 {
					log.Printf("postman: parse %s: %v", name, err)
					return
				}

				rootID := "postman-col-" + uid
				wrapped := make([]collection.FlatItem, 0, len(items)+1)
				wrapped = append(wrapped, collection.FlatItem{
					ID: rootID, Name: name, Type: "folder", SortOrder: i,
				})
				for _, item := range items {
					if item.ParentID == "" {
						item.ParentID = rootID
					}
					wrapped = append(wrapped, item)
				}
				resultsCh <- fetchedCol{i, name, wrapped, vars}
			}(i, c.UID, c.Name)
		}

		wg.Wait()
		close(resultsCh)

		// ── 3. Index sequentially (SQLite single-writer) ─────────────────────
		s.emit("collection:importing", fmt.Sprintf("Indexing %d collections…", total))
		imported := 0
		for r := range resultsCh {
			if err := s.db.AddItems(r.wrapped); err != nil {
				log.Printf("postman: index %s: %v", r.name, err)
				continue
			}
			if len(r.vars) > 0 && s.envSvc != nil {
				s.envSvc.mergeCollectionVars(r.name, r.vars)
			}
			imported++
		}

		// ── 4. Fetch environments in parallel ────────────────────────────────
		envData, err := get("https://api.getpostman.com/environments")
		if err == nil && s.envSvc != nil {
			var envList struct {
				Environments []struct {
					UID  string `json:"uid"`
					Name string `json:"name"`
				} `json:"environments"`
			}
			if json.Unmarshal(envData, &envList) == nil {
				type fetchedEnv struct {
					name string
					vars []collection.CollectionVar
				}
				envTotal := len(envList.Environments)
				envCh := make(chan fetchedEnv, envTotal)
				var envWg sync.WaitGroup

				for _, e := range envList.Environments {
					envWg.Add(1)
					sem <- struct{}{}
					go func(uid, name string) {
						defer envWg.Done()
						defer func() { <-sem }()
						data, err := get("https://api.getpostman.com/environments/" + uid)
						if err != nil {
							log.Printf("postman: skip env %s: %v", name, err)
							return
						}
						n, vars, err := collection.ParseEnvBytes(data)
						if err != nil || len(vars) == 0 {
							return
						}
						envCh <- fetchedEnv{n, vars}
					}(e.UID, e.Name)
				}
				envWg.Wait()
				close(envCh)
				for r := range envCh {
					s.envSvc.mergeCollectionVars(r.name, r.vars)
				}
			}
		}

		log.Printf("postman import: %d/%d collections", imported, total)
		s.emit("collection:ready", fmt.Sprintf("Imported %d collections from Postman", imported))
	}()
	return nil
}

func (s *CollectionService) emit(event string, data ...any) {
	if s.emitter != nil {
		s.emitter.Emit(event, data...)
	}
}
