package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
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

// PickImportEnv opens a native file dialog and imports a Postman environment
// JSON file. Returns "" if cancelled.
func (s *EnvService) PickImportEnv() (string, error) {
	if s.dialog == nil {
		return "", fmt.Errorf("dialog unavailable")
	}
	d := s.dialog.OpenFile()
	d.SetTitle("Import Postman environment")
	d.AddFilter("JSON", "*.json")
	d.CanChooseFiles(true)
	path, err := d.PromptForSingleSelection()
	if err != nil {
		return "", err
	}
	if path == "" {
		return "", nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read env file: %w", err)
	}
	name, vars, err := collection.ParseEnvBytes(data)
	if err != nil {
		return "", fmt.Errorf("parse env: %w", err)
	}
	return path, s.mergeCollectionVars(name, vars)
}

// ImportEnvFromURL fetches a Postman environment JSON from a URL and imports it.
func (s *EnvService) ImportEnvFromURL(rawURL string) error {
	if rawURL == "" {
		return fmt.Errorf("URL is empty")
	}
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("fetch failed: %w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	name, vars, err := collection.ParseEnvBytes(data)
	if err != nil {
		return fmt.Errorf("parse env: %w", err)
	}
	return s.mergeCollectionVars(name, vars)
}

// mergeCollectionVars upserts an environment named `name` with the given
// variables. If an environment with that name already exists its vars are
// replaced; otherwise a new one is created.
func (s *EnvService) mergeCollectionVars(name string, vars []collection.CollectionVar) error {
	st, err := s.store.Load()
	if err != nil {
		return err
	}
	env := findEnvByName(st.Environments, name)
	if env == nil {
		id := randEnvID()
		st.Environments = append(st.Environments, envstore.Environment{
			ID:   id,
			Name: name,
			Vars: collVarsToEnv(vars),
		})
	} else {
		env.Vars = collVarsToEnv(vars)
	}
	return s.store.Save(st)
}

func findEnvByName(envs []envstore.Environment, name string) *envstore.Environment {
	for i := range envs {
		if strings.EqualFold(envs[i].Name, name) {
			return &envs[i]
		}
	}
	return nil
}

func collVarsToEnv(vars []collection.CollectionVar) []envstore.Var {
	out := make([]envstore.Var, 0, len(vars))
	for _, v := range vars {
		out = append(out, envstore.Var{Key: v.Key, Value: v.Value, Enabled: v.Enabled})
	}
	return out
}

func randEnvID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("env-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}
