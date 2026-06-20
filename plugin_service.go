package main

import (
	"fmt"

	"reqost/internal/plugins"
)

// PluginService is the Wails-facing facade for the plugins package: list
// available plugins, toggle them on/off, and surface the plugin directory.
// Actual hook invocation happens inside ExecService at send time.
type PluginService struct {
	mgr     *plugins.Manager
	emitter EventEmitter
}

// setEmitter wires the Wails event bus so plugin console output (and reload
// notifications) reach the frontend Plugin Console panel.
func (s *PluginService) setEmitter(e EventEmitter) {
	s.emitter = e
	plugins.SetConsoleSink(func(plugin, level, message string) {
		if s.emitter == nil {
			return
		}
		s.emitter.Emit("plugin:console", map[string]any{
			"plugin":  plugin,
			"level":   level,
			"message": message,
		})
	})
}

// Reload re-scans the plugin directory so newly-dropped .js files appear in
// the UI without a full app restart. Returns the refreshed plugin list.
func (s *PluginService) Reload() ([]plugins.Plugin, error) {
	if s.mgr == nil {
		return []plugins.Plugin{}, nil
	}
	return s.mgr.Reload()
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
