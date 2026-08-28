// Package appdir resolves the one directory reqost treats as durable
// storage: the SQLite indexes, workspaces.json, environments.json, plugin
// state, and design.yaml. See Dir for why this must not be a cache
// directory.
package appdir

import (
	"io/fs"
	"os"
	"path/filepath"
)

// Dir returns reqost's persistent data directory, creating it if needed.
//
// This is deliberately os.UserConfigDir() (~/Library/Application Support on
// macOS), not os.UserCacheDir(): the OS is free to purge a cache directory
// at any time to reclaim disk space, silently deleting whatever's in it —
// that's not hypothetical, it's what happened to an earlier version of this
// app that stored the index there and lost a user's collection.
//
// On first call, if the old cache-dir location still holds data from that
// earlier version, it's moved (not copied) into the new location so nothing
// already on disk is lost by the switch.
func Dir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "reqost")
	if _, err := os.Stat(dir); err == nil {
		return dir, nil // already set up, or already migrated
	}
	if err := os.MkdirAll(base, 0o755); err != nil {
		return "", err
	}

	if oldCacheDir, err := os.UserCacheDir(); err == nil {
		oldDir := filepath.Join(oldCacheDir, "reqost")
		if _, err := os.Stat(oldDir); err == nil {
			if err := os.Rename(oldDir, dir); err == nil {
				return dir, nil
			}
			// Rename can fail across a device boundary (unusual for
			// Caches/Application Support, which are normally on the same
			// volume, but not guaranteed on every setup) — fall back to a
			// plain copy so the migration still succeeds.
			if err := copyDir(oldDir, dir); err != nil {
				return "", err
			}
			_ = os.RemoveAll(oldDir)
			return dir, nil
		}
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
}
