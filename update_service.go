package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"reqost/internal/update"
)

// UpdateService exposes the self-update flow to the frontend. Two-step on
// purpose: Check returns metadata so the UI can show release notes + a confirm
// button before Apply touches the binary.
//
// The download URLs are kept server-side (cached in lastInfo) so they never
// need to round-trip through the frontend — Apply() uses the cached result.
type UpdateService struct {
	lastInfo *update.Info
}

func NewUpdateService() *UpdateService { return &UpdateService{} }

// CurrentVersion returns the build-injected Version string ("dev" in unstamped
// local builds). Used by the Settings panel header.
func (s *UpdateService) CurrentVersion() string { return update.Version }

// RepoSlug returns the owner/repo the updater queries. Exposed for the UI's
// "view releases" link.
func (s *UpdateService) RepoSlug() string { return update.RepoSlug }

// CheckForUpdate queries the latest GitHub release and reports whether a newer
// version is available for this platform. The result is cached so ApplyUpdate
// can use the original Info (including internal download URLs).
func (s *UpdateService) CheckForUpdate() (*update.Info, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30_000_000_000) // 30s
	defer cancel()
	info, err := update.Check(ctx)
	if err == nil && info != nil && info.Available {
		s.lastInfo = info
	}
	return info, err
}

// ApplyUpdate downloads and installs the cached update. On success the running
// binary has been replaced; the caller should prompt for restart.
func (s *UpdateService) ApplyUpdate() error {
	if s.lastInfo == nil || !s.lastInfo.Available {
		return fmt.Errorf("no update available — run CheckForUpdate first")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 300_000_000_000) // 5m
	defer cancel()
	return update.Apply(ctx, s.lastInfo)
}

// RestartApp relaunches the (now-updated) app and exits the current process.
//
// We spawn a small detached relauncher that waits for THIS pid to die, then
// launches a fresh instance — so there's never two instances overlapping and
// the new binary (already swapped in by ApplyUpdate) is the one that starts.
// The current process is exited shortly after, letting the JS call return first.
func (s *UpdateService) RestartApp() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if resolved, e := filepath.EvalSymlinks(exe); e == nil {
		exe = resolved
	}
	pid := os.Getpid()

	var relaunch *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		// Prefer reopening the .app bundle via LaunchServices (dock/Finder
		// integration); fall back to the raw binary for non-bundled builds.
		target := exe
		if app := appBundle(exe); app != "" {
			target = app
		}
		script := fmt.Sprintf(`while kill -0 %d 2>/dev/null; do sleep 0.2; done; open -n %q`, pid, target)
		relaunch = exec.Command("/bin/sh", "-c", script)
	case "linux":
		script := fmt.Sprintf(`while kill -0 %d 2>/dev/null; do sleep 0.2; done; exec %q`, pid, exe)
		relaunch = exec.Command("/bin/sh", "-c", script)
	default: // windows and others: start a fresh instance immediately
		relaunch = exec.Command(exe)
	}

	if err := relaunch.Start(); err != nil {
		return fmt.Errorf("spawn relauncher: %w", err)
	}
	_ = relaunch.Process.Release() // detach; it outlives us

	// Exit after a beat so this RPC can return to the UI ("Restarting…") first.
	go func() {
		time.Sleep(400 * time.Millisecond)
		os.Exit(0)
	}()
	return nil
}

// appBundle returns the enclosing .../Name.app for a macOS bundle executable
// (.../Name.app/Contents/MacOS/binary), or "" if exe isn't inside a bundle.
func appBundle(exe string) string {
	macos := filepath.Dir(exe)
	if filepath.Base(macos) != "MacOS" {
		return ""
	}
	contents := filepath.Dir(macos)
	if filepath.Base(contents) != "Contents" {
		return ""
	}
	app := filepath.Dir(contents)
	if strings.HasSuffix(app, ".app") {
		return app
	}
	return ""
}
