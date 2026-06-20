// Package plugins runs user-supplied JS hooks against requests and responses.
//
// A plugin is a single .js file that exports any of these functions:
//
//	function onPreSend(req) { ... return req }    // mutate before send
//	function onPostReceive(req, resp) { ... }     // observe / log
//	function onTransformBody(req) { return req }  // alias for onPreSend
//
// The sandbox is goja (pure-Go JS, same engine the test/pre scripts use), so
// users get familiar semantics. No file system, no network, no `require` —
// pluginsdir is the security boundary: only files inside it execute.
package plugins

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/dop251/goja"
)

func jsonUnmarshal(data []byte, v any) error      { return json.Unmarshal(data, v) }
func jsonMarshalIndent(v any) ([]byte, error)     { return json.MarshalIndent(v, "", "  ") }

type Plugin struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Enabled bool   `json:"enabled"`
	source  string
}

// Manager loads + runs plugins. Reads the directory lazily so newly dropped
// files are picked up next call.
type Manager struct {
	dir string

	mu       sync.Mutex
	enabled  map[string]bool // path → enabled
	prefsPath string
}

func Open() (*Manager, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(cacheDir, "reqost", "plugins")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	m := &Manager{
		dir:       dir,
		enabled:   map[string]bool{},
		prefsPath: filepath.Join(cacheDir, "reqost", "plugins.json"),
	}
	m.loadPrefs()
	return m, nil
}

// Dir is exposed so the frontend can show + open the folder in a file manager.
func (m *Manager) Dir() string { return m.dir }

// List returns every .js file in the plugins dir along with its enabled flag.
func (m *Manager) List() ([]Plugin, error) {
	entries, err := os.ReadDir(m.dir)
	if err != nil {
		return nil, err
	}
	out := make([]Plugin, 0, len(entries))
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".js") {
			continue
		}
		p := filepath.Join(m.dir, e.Name())
		out = append(out, Plugin{
			Name:    e.Name(),
			Path:    p,
			Enabled: m.enabled[p],
		})
	}
	return out, nil
}

func (m *Manager) SetEnabled(path string, on bool) {
	m.mu.Lock()
	m.enabled[path] = on
	m.savePrefs()
	m.mu.Unlock()
}

// Hooks is the bag of optional functions a plugin exports.
type Hooks struct {
	PreSend       goja.Callable
	PostReceive   goja.Callable
	TransformBody goja.Callable
}

// Loaded is a parsed plugin ready to run.
type Loaded struct {
	Name  string
	vm    *goja.Runtime
	hooks Hooks
}

// ConsoleSink receives every console.{log,info,warn,error} call from any
// plugin. Caller wires it to the Wails emitter so the frontend Plugin
// Console panel sees the output.
type ConsoleSink func(plugin, level, message string)

var consoleSink ConsoleSink

// SetConsoleSink installs a process-wide console hook. Pass nil to detach.
func SetConsoleSink(s ConsoleSink) { consoleSink = s }

// LoadEnabled returns one runtime per enabled plugin. Caller invokes the
// hooks; we own no shared mutable state across runtimes.
func (m *Manager) LoadEnabled() ([]Loaded, error) {
	plugins, err := m.List()
	if err != nil {
		return nil, err
	}
	var out []Loaded
	for _, p := range plugins {
		if !p.Enabled {
			continue
		}
		src, err := os.ReadFile(p.Path)
		if err != nil {
			continue
		}
		l, err := load(p.Name, string(src))
		if err != nil {
			continue
		}
		out = append(out, l)
	}
	return out, nil
}

func load(name, src string) (Loaded, error) {
	vm := goja.New()
	t := time.AfterFunc(2*time.Second, func() { vm.Interrupt("plugin load timeout") })
	defer t.Stop()

	// console.{log,info,warn,error} → ConsoleSink. Without this hook plugin
	// output goes to /dev/null and the user can't tell why a plugin is
	// misbehaving. We forward each call as one event.
	installConsole(vm, name)

	if _, err := vm.RunString(src); err != nil {
		return Loaded{}, fmt.Errorf("plugin %s: %w", name, err)
	}
	hooks := Hooks{}
	if fn, ok := goja.AssertFunction(vm.Get("onPreSend")); ok {
		hooks.PreSend = fn
	}
	if fn, ok := goja.AssertFunction(vm.Get("onPostReceive")); ok {
		hooks.PostReceive = fn
	}
	if fn, ok := goja.AssertFunction(vm.Get("onTransformBody")); ok {
		hooks.TransformBody = fn
	}
	return Loaded{Name: name, vm: vm, hooks: hooks}, nil
}

// installConsole exposes a minimal `console` object to the plugin's runtime.
// Each call funnels into ConsoleSink with a level tag.
func installConsole(vm *goja.Runtime, pluginName string) {
	console := vm.NewObject()
	emit := func(level string) func(call goja.FunctionCall) goja.Value {
		return func(call goja.FunctionCall) goja.Value {
			if consoleSink == nil {
				return goja.Undefined()
			}
			parts := make([]string, 0, len(call.Arguments))
			for _, a := range call.Arguments {
				parts = append(parts, a.String())
			}
			consoleSink(pluginName, level, strings.Join(parts, " "))
			return goja.Undefined()
		}
	}
	_ = console.Set("log", emit("log"))
	_ = console.Set("info", emit("info"))
	_ = console.Set("warn", emit("warn"))
	_ = console.Set("error", emit("error"))
	_ = vm.Set("console", console)
}

// Reload drops cached plugin runtimes by clearing the manager's read-through
// behaviour: nothing's actually cached (LoadEnabled reads every time), but
// calling List forces a re-scan of the directory so newly-added files appear
// without an app restart. Returns the fresh list.
func (m *Manager) Reload() ([]Plugin, error) { return m.List() }

// RunPreSend mutates req via every enabled plugin's onPreSend / onTransformBody.
// Each plugin gets a 2-second watchdog. A panic in one plugin doesn't stop
// the rest.
func RunPreSend(loaded []Loaded, req map[string]any) map[string]any {
	for _, l := range loaded {
		req = runHook(l, l.hooks.PreSend, req)
		req = runHook(l, l.hooks.TransformBody, req)
	}
	return req
}

// RunPostReceive runs every enabled plugin's onPostReceive(req, resp).
func RunPostReceive(loaded []Loaded, req map[string]any, resp map[string]any) {
	for _, l := range loaded {
		if l.hooks.PostReceive == nil {
			continue
		}
		t := time.AfterFunc(2*time.Second, func() { l.vm.Interrupt("plugin timeout") })
		_, _ = l.hooks.PostReceive(goja.Undefined(), l.vm.ToValue(req), l.vm.ToValue(resp))
		t.Stop()
	}
}

func runHook(l Loaded, fn goja.Callable, req map[string]any) map[string]any {
	if fn == nil {
		return req
	}
	t := time.AfterFunc(2*time.Second, func() { l.vm.Interrupt("plugin timeout") })
	defer t.Stop()
	val, err := fn(goja.Undefined(), l.vm.ToValue(req))
	if err != nil || val == nil || goja.IsUndefined(val) || goja.IsNull(val) {
		return req
	}
	out, ok := val.Export().(map[string]any)
	if !ok {
		return req
	}
	return out
}

// ── prefs persistence ──────────────────────────────────────────────────────

type prefs struct {
	Enabled map[string]bool `json:"enabled"`
}

func (m *Manager) loadPrefs() {
	data, err := os.ReadFile(m.prefsPath)
	if err != nil {
		return
	}
	var p prefs
	if err := jsonUnmarshal(data, &p); err == nil {
		m.enabled = p.Enabled
		if m.enabled == nil {
			m.enabled = map[string]bool{}
		}
	}
}

func (m *Manager) savePrefs() {
	data, err := jsonMarshalIndent(prefs{Enabled: m.enabled})
	if err != nil {
		return
	}
	_ = os.WriteFile(m.prefsPath, data, 0o644)
}
