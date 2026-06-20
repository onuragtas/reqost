package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"reqost/internal/collection"
	"reqost/internal/envstore"
)

// EnvService exposes environment persistence to the frontend. The frontend owns
// editing; this just loads/saves the whole document and handles imports.
type EnvService struct {
	store  *envstore.Store
	dialog *application.DialogManager
}

func NewEnvService() (*EnvService, error) {
	store, err := envstore.Open()
	if err != nil {
		return nil, err
	}
	return &EnvService{store: store}, nil
}

func (s *EnvService) setDialog(d *application.DialogManager) {
	s.dialog = d
}

// LoadEnvironments returns the persisted environments + active selection.
func (s *EnvService) LoadEnvironments() (envstore.State, error) {
	return s.store.Load()
}

// SaveEnvironments persists the whole environment document.
func (s *EnvService) SaveEnvironments(state envstore.State) error {
	return s.store.Save(state)
}

// exportRaw returns the JSON-marshalled current state — used by
// CollectionService.ExportWorkspaceZip so the whole workspace can be packed.
func (s *EnvService) exportRaw() ([]byte, error) {
	st, err := s.store.Load()
	if err != nil {
		return nil, err
	}
	return json.MarshalIndent(st, "", "  ")
}

// importRaw merges an envstore.State JSON blob into the current store.
// Environments with matching id are overwritten; new ids are appended.
func (s *EnvService) importRaw(data []byte) error {
	var incoming envstore.State
	if err := json.Unmarshal(data, &incoming); err != nil {
		return err
	}
	cur, err := s.store.Load()
	if err != nil {
		return err
	}
	byID := map[string]int{}
	for i, e := range cur.Environments {
		byID[e.ID] = i
	}
	for _, e := range incoming.Environments {
		if idx, ok := byID[e.ID]; ok {
			cur.Environments[idx] = e
		} else {
			cur.Environments = append(cur.Environments, e)
		}
	}
	if cur.ActiveID == "" {
		cur.ActiveID = incoming.ActiveID
	}
	return s.store.Save(cur)
}

// PickImportEnv opens a native file dialog and imports a Postman environment
// export JSON. Returns the environment name on success.
func (s *EnvService) PickImportEnv() (string, error) {
	if s.dialog == nil {
		return "", fmt.Errorf("dialog unavailable")
	}
	d := s.dialog.OpenFile()
	d.SetTitle("Import Postman environment")
	d.AddFilter("JSON", "*.json")
	d.CanChooseFiles(true)
	path, err := d.PromptForSingleSelection()
	if err != nil || path == "" {
		return "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read file: %w", err)
	}
	name, vars, err := collection.ParseEnvBytes(data)
	if err != nil {
		// Might be a collection with only root variables — try that.
		_, collVars, cerr := collection.ParseBytes(data)
		if cerr != nil || len(collVars) == 0 {
			return "", fmt.Errorf("not a Postman environment file: %w", err)
		}
		name = strings.TrimSuffix(filepath.Base(path), ".json")
		vars = collVars
	}
	s.mergeCollectionVars(name, vars)
	log.Printf("imported env %q (%d vars) from %s", name, len(vars), path)
	return name, nil
}

// ImportEnvFromURL fetches a Postman environment JSON from a URL and imports it.
// Returns the imported environment name on success.
func (s *EnvService) ImportEnvFromURL(rawURL string) (string, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(rawURL)
	if err != nil {
		return "", fmt.Errorf("fetch: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return "", fmt.Errorf("read: %w", err)
	}
	name, vars, err := collection.ParseEnvBytes(data)
	if err != nil {
		return "", err
	}
	s.mergeCollectionVars(name, vars)
	return name, nil
}

// mergeCollectionVars upserts an environment named after sourceName with the
// given variables. If an environment with that name already exists, its
// variables are replaced. The environment is NOT set as active.
func (s *EnvService) mergeCollectionVars(sourceName string, vars []collection.CollectionVar) {
	if len(vars) == 0 {
		return
	}
	st, err := s.store.Load()
	if err != nil {
		log.Printf("mergeCollectionVars: load: %v", err)
		return
	}

	// Derive a friendly name — strip any path prefix and extension.
	name := sourceName
	if idx := strings.LastIndexAny(name, "/\\"); idx >= 0 {
		name = name[idx+1:]
	}
	name = strings.TrimSuffix(name, ".json")
	if name == "" {
		name = "Collection Variables"
	}

	newVars := make([]envstore.Var, 0, len(vars))
	for _, v := range vars {
		newVars = append(newVars, envstore.Var{Key: v.Key, Value: v.Value, Enabled: v.Enabled})
	}

	found := false
	for i, env := range st.Environments {
		if env.Name == name {
			st.Environments[i].Vars = newVars
			found = true
			break
		}
	}
	if !found {
		newID := fmt.Sprintf("colvar-%d", time.Now().UnixNano())
		st.Environments = append(st.Environments, envstore.Environment{
			ID:   newID,
			Name: name,
			Vars: newVars,
		})
	}

	if err := s.store.Save(st); err != nil {
		log.Printf("mergeCollectionVars: save: %v", err)
	}
}
