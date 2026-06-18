// Package workspaces persists the list of available reqost workspaces
// (a workspace = one isolated SQLite collection index + metadata) and
// resolves their on-disk paths.
//
// Layout under the user cache dir:
//
//	reqost/
//	  workspaces.json          // { activeId, workspaces: [{id, name}] }
//	  workspaces/<id>/index.db // per-workspace SQLite file
package workspaces

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Workspace struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
}

type State struct {
	ActiveID   string      `json:"activeId"`
	Workspaces []Workspace `json:"workspaces"`
}

type Store struct {
	mu   sync.Mutex
	dir  string
	path string
	st   State
}

func Open() (*Store, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(cacheDir, "reqost")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	s := &Store{dir: dir, path: filepath.Join(dir, "workspaces.json")}
	if err := s.load(); err != nil {
		return nil, err
	}
	if len(s.st.Workspaces) == 0 {
		// Seed a default workspace + migrate the legacy single-DB cache
		// (`reqost/index.db`) into it on first run.
		def := Workspace{ID: newID(), Name: "Default", CreatedAt: time.Now()}
		s.st.Workspaces = []Workspace{def}
		s.st.ActiveID = def.ID
		_ = s.migrateLegacy(def.ID)
		_ = s.save()
	}
	return s, nil
}

// migrateLegacy moves the historical `reqost/index.db` into the new
// per-workspace path the first time we boot in multi-workspace mode.
func (s *Store) migrateLegacy(targetID string) error {
	legacy := filepath.Join(s.dir, "index.db")
	if _, err := os.Stat(legacy); err != nil {
		return nil
	}
	dest := s.DBPath(targetID)
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	if _, err := os.Stat(dest); err == nil {
		return nil // never clobber
	}
	return os.Rename(legacy, dest)
}

func (s *Store) load() error {
	data, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	return json.Unmarshal(data, &s.st)
}

func (s *Store) save() error {
	tmp := s.path + ".tmp"
	data, err := json.MarshalIndent(s.st, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func (s *Store) List() []Workspace {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Workspace, len(s.st.Workspaces))
	copy(out, s.st.Workspaces)
	return out
}

func (s *Store) ActiveID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.st.ActiveID
}

// DBPath returns the SQLite path for the given workspace id. Caller is
// responsible for opening/closing.
func (s *Store) DBPath(id string) string {
	return filepath.Join(s.dir, "workspaces", id, "index.db")
}

func (s *Store) Create(name string) (Workspace, error) {
	if name == "" {
		name = "Untitled"
	}
	w := Workspace{ID: newID(), Name: name, CreatedAt: time.Now()}
	if err := os.MkdirAll(filepath.Dir(s.DBPath(w.ID)), 0o755); err != nil {
		return Workspace{}, err
	}
	s.mu.Lock()
	s.st.Workspaces = append(s.st.Workspaces, w)
	s.mu.Unlock()
	return w, s.save()
}

func (s *Store) Rename(id, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.st.Workspaces {
		if s.st.Workspaces[i].ID == id {
			s.st.Workspaces[i].Name = name
			return s.save()
		}
	}
	return fmt.Errorf("workspace %s not found", id)
}

func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.st.Workspaces) <= 1 {
		return fmt.Errorf("cannot delete the last workspace")
	}
	out := s.st.Workspaces[:0]
	for _, w := range s.st.Workspaces {
		if w.ID != id {
			out = append(out, w)
		}
	}
	s.st.Workspaces = out
	if s.st.ActiveID == id {
		s.st.ActiveID = s.st.Workspaces[0].ID
	}
	// Wipe the DB file & dir best-effort.
	_ = os.RemoveAll(filepath.Dir(s.DBPath(id)))
	return s.save()
}

func (s *Store) SetActive(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, w := range s.st.Workspaces {
		if w.ID == id {
			s.st.ActiveID = id
			return s.save()
		}
	}
	return fmt.Errorf("workspace %s not found", id)
}

func newID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return "ws-" + hex.EncodeToString(b[:])
}
