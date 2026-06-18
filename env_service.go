package main

import "reqost/internal/envstore"

// EnvService exposes environment persistence to the frontend. The frontend owns
// editing; this just loads/saves the whole document.
type EnvService struct {
	store *envstore.Store
}

func NewEnvService() (*EnvService, error) {
	store, err := envstore.Open()
	if err != nil {
		return nil, err
	}
	return &EnvService{store: store}, nil
}

// LoadEnvironments returns the persisted environments + active selection.
func (s *EnvService) LoadEnvironments() (envstore.State, error) {
	return s.store.Load()
}

// SaveEnvironments persists the whole environment document.
func (s *EnvService) SaveEnvironments(state envstore.State) error {
	return s.store.Save(state)
}
