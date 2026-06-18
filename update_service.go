package main

import (
	"context"
	"fmt"

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
