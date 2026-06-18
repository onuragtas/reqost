package main

import (
	"context"

	"reqost/internal/update"
)

// UpdateService exposes the self-update flow to the frontend. Two-step on
// purpose: Check returns metadata so the UI can show release notes + a confirm
// button before Apply touches the binary.
type UpdateService struct{}

func NewUpdateService() *UpdateService { return &UpdateService{} }

// CurrentVersion returns the build-injected Version string ("dev" in unstamped
// local builds). Used by the Settings panel header.
func (s *UpdateService) CurrentVersion() string { return update.Version }

// RepoSlug returns the owner/repo the updater queries. Exposed for the UI's
// "view releases" link.
func (s *UpdateService) RepoSlug() string { return update.RepoSlug }

// CheckForUpdate queries the latest GitHub release and reports whether a newer
// version is available for this platform.
func (s *UpdateService) CheckForUpdate() (*update.Info, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30_000_000_000) // 30s
	defer cancel()
	return update.Check(ctx)
}

// ApplyUpdate downloads and installs the update described by info. On success
// the running binary has been replaced; the caller should prompt for restart.
func (s *UpdateService) ApplyUpdate(info update.Info) error {
	ctx, cancel := context.WithTimeout(context.Background(), 300_000_000_000) // 5m
	defer cancel()
	return update.Apply(ctx, &info)
}
