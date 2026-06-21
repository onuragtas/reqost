package main

import "github.com/wailsapp/wails/v3/pkg/application"

// WindowService bridges the custom Vue title bar to native window controls.
// reqost draws its own title bar (Wails `MacTitleBarHiddenInset`), so the
// platform's "double-click the title bar to zoom" affordance no longer fires
// for free — we wire it back up by exposing the controls to JS.
type WindowService struct {
	window *application.WebviewWindow
}

func NewWindowService() *WindowService { return &WindowService{} }

func (s *WindowService) setWindow(w *application.WebviewWindow) { s.window = w }

// ToggleMaximise is what the title-bar dblclick handler calls. On macOS this
// uses the system Zoom action (smart fit), which is the actual native default
// behaviour for a title-bar double-click. On other platforms it falls back to
// the Maximise/Unmaximise toggle.
func (s *WindowService) ToggleMaximise() {
	if s.window == nil {
		return
	}
	if s.window.IsMaximised() {
		s.window.UnMaximise()
	} else {
		s.window.Maximise()
	}
}

// Minimise hides the window to the dock / taskbar.
func (s *WindowService) Minimise() {
	if s.window != nil {
		s.window.Minimise()
	}
}

// Close requests window close (same as the OS close button).
func (s *WindowService) Close() {
	if s.window != nil {
		s.window.Close()
	}
}
