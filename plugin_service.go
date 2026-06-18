package main

import (
	"fmt"

	"reqost/internal/plugins"
)

// PluginService is the Wails-facing facade for the plugins package: list
// available plugins, toggle them on/off, and surface the plugin directory.
// Actual hook invocation happens inside ExecService at send time.
type PluginService struct {
	mgr *plugins.Manager
}

func NewPluginService() *PluginService {
	mgr, err := plugins.Open()
	if err != nil {
		// Logged but non-fatal — the app still runs without plugins.
		fmt.Printf("plugins: %v\n", err)
		return &PluginService{}
	}
	return &PluginService{mgr: mgr}
}

func (s *PluginService) Dir() string {
	if s.mgr == nil {
		return ""
	}
	return s.mgr.Dir()
}

func (s *PluginService) List() ([]plugins.Plugin, error) {
	if s.mgr == nil {
		return []plugins.Plugin{}, nil
	}
	return s.mgr.List()
}

func (s *PluginService) SetEnabled(path string, enabled bool) {
	if s.mgr == nil {
		return
	}
	s.mgr.SetEnabled(path, enabled)
}

// manager exposes the loader so other services (ExecService) can run hooks.
func (s *PluginService) manager() *plugins.Manager { return s.mgr }
