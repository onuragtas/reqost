package main

import (
	"fmt"
	"log"
	"os"

	"github.com/wailsapp/wails/v3/pkg/application"

	"reqost/internal/collection"
	"reqost/internal/index"
	"reqost/internal/openapi"
	"reqost/internal/watcher"
)

// EventEmitter is satisfied by app.Event in Wails v3.
type EventEmitter interface {
	Emit(string, ...any) bool
}

type CollectionService struct {
	db      *index.DB
	watch   *watcher.Watcher
	emitter EventEmitter
	dialog  *application.DialogManager
}

func NewCollectionService() (*CollectionService, error) {
	db, err := index.Open()
	if err != nil {
		return nil, fmt.Errorf("open index: %w", err)
	}
	svc := &CollectionService{db: db}

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

func (s *CollectionService) setEmitter(e EventEmitter) {
	s.emitter = e
}

func (s *CollectionService) setDialog(d *application.DialogManager) {
	s.dialog = d
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

// MoveNode re-parents a node and sets its position among siblings (0-based).
// Used by the sidebar drag-and-drop. Moving into self or a descendant returns
// an error.
func (s *CollectionService) MoveNode(id, newParentID string, newIndex int) error {
	return s.db.MoveNode(id, newParentID, newIndex)
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

	items, err := collection.ParseFile(path)
	if err != nil {
		s.emit("collection:error", err.Error())
		return
	}

	if err := s.db.ImportItems(path, currentMtime, items); err != nil {
		s.emit("collection:error", fmt.Sprintf("index: %v", err))
		return
	}

	// The SQLite index is the source of truth (edits are written to it, not the
	// file), so we intentionally do NOT auto-watch the file for re-import — that
	// would clobber user edits. Import is a deliberate, user-initiated replace.

	log.Printf("indexed %d items from %s", len(items), path)
	s.emit("collection:ready", path)
}

func (s *CollectionService) emit(event string, data ...any) {
	if s.emitter != nil {
		s.emitter.Emit(event, data...)
	}
}
