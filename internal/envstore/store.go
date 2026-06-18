// Package envstore persists environments (named bags of variables) to a JSON
// file in the user cache dir. It is intentionally dumb storage: the frontend
// owns all editing logic; the backend just loads and saves the whole state and
// resolves the active variable map for interpolation.
package envstore

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type Var struct {
	Key     string `json:"key"`
	Value   string `json:"value"`
	Enabled bool   `json:"enabled"`
	// Secret marks a variable as sensitive — UI masks the value and offers
	// opt-in reveal. Disk storage stays plaintext for now; this is presentation
	// + clipboard-leak guard. Real at-rest secrecy = OS keychain, future work.
	Secret bool `json:"secret,omitempty"`
}

type Environment struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Vars []Var  `json:"vars"`
}

// State is the whole persisted document.
type State struct {
	ActiveID     string        `json:"activeId"`
	Environments []Environment `json:"environments"`
}

type Store struct {
	mu   sync.Mutex
	path string
}

func Open() (*Store, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return nil, fmt.Errorf("user cache dir: %w", err)
	}
	dir := filepath.Join(cacheDir, "reqost")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create cache dir: %w", err)
	}
	return &Store{path: filepath.Join(dir, "environments.json")}, nil
}

// Load returns the persisted state. A missing file yields an empty state, not
// an error.
func (s *Store) Load() (State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var st State
	data, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		st.Environments = []Environment{}
		return st, nil
	}
	if err != nil {
		return st, fmt.Errorf("read environments: %w", err)
	}
	if err := json.Unmarshal(data, &st); err != nil {
		return st, fmt.Errorf("parse environments: %w", err)
	}
	if st.Environments == nil {
		st.Environments = []Environment{}
	}
	return st, nil
}

// Save writes the whole state atomically (temp file + rename).
func (s *Store) Save(st State) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write environments: %w", err)
	}
	return os.Rename(tmp, s.path)
}

// ActiveVars resolves the active environment's enabled vars into a flat map
// ready for {{interpolation}}.
func (s *Store) ActiveVars() (map[string]string, error) {
	st, err := s.Load()
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	for _, env := range st.Environments {
		if env.ID != st.ActiveID {
			continue
		}
		for _, v := range env.Vars {
			if v.Enabled && v.Key != "" {
				out[v.Key] = v.Value
			}
		}
	}
	return out, nil
}
