// Package update implements a minimal self-update flow on top of GitHub
// Releases.
//
// Flow:
//   - Check() queries the repo's `releases/latest` and, if the tag is newer
//     than the embedded Version, returns an Info describing the platform asset
//     to download.
//   - Apply(info) downloads the asset (tar.gz / zip), verifies it against the
//     companion .sha256 file, extracts the binary, and replaces the running
//     executable via minio/selfupdate (cross-platform, including Windows).
//
// Version and RepoSlug are overridable at link time:
//
//	go build -ldflags "-X reqost/internal/update.Version=v1.2.3 \
//	                   -X reqost/internal/update.RepoSlug=owner/repo"
package update

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/minio/selfupdate"
)

// Version is the running binary's version, injected via -ldflags at build time.
// In development builds it stays as "dev" and Check() always reports an update.
var Version = "dev"

// RepoSlug is the "owner/repo" the updater queries on GitHub. Overridable via
// -ldflags so a fork can point its build at a different release stream.
var RepoSlug = "onuragtas/reqost"

// Info summarizes a release check result. The download URLs are kept internal;
// the frontend only needs the version metadata to render the prompt.
type Info struct {
	Current   string `json:"current"`
	Latest    string `json:"latest"`
	Available bool   `json:"available"`
	Notes     string `json:"notes"`
	AssetURL  string `json:"-"`
	SHA256URL string `json:"-"`
	AssetName string `json:"assetName"`
}

type ghAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

type ghRelease struct {
	TagName string    `json:"tag_name"`
	Body    string    `json:"body"`
	Assets  []ghAsset `json:"assets"`
}

const releasesAPI = "https://api.github.com/repos/%s/releases/latest"

var httpClient = &http.Client{Timeout: 30 * time.Second}

// Check queries the latest release. Returns Info with Available=false if the
// current version is at or above the tag, or no asset matches this platform.
func Check(ctx context.Context) (*Info, error) {
	url := fmt.Sprintf(releasesAPI, RepoSlug)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("check releases: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("no releases published yet for %s", RepoSlug)
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("github releases: %s", resp.Status)
	}

	var rel ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, fmt.Errorf("decode release: %w", err)
	}

	info := &Info{Current: Version, Latest: rel.TagName, Notes: rel.Body}
	if !isNewer(Version, rel.TagName) {
		return info, nil
	}

	assetName, ext := platformAssetName()
	info.AssetName = assetName
	for _, a := range rel.Assets {
		if a.Name == assetName {
			info.AssetURL = a.BrowserDownloadURL
		}
		if a.Name == assetName+".sha256" {
			info.SHA256URL = a.BrowserDownloadURL
		}
	}
	if info.AssetURL == "" {
		return info, fmt.Errorf("release %s has no asset for %s/%s (looking for %s)",
			rel.TagName, runtime.GOOS, runtime.GOARCH, assetName)
	}
	info.Available = true
	_ = ext
	return info, nil
}

// Apply downloads + verifies + installs the update described by info. The
// caller should prompt the user to restart on success.
func Apply(ctx context.Context, info *Info) error {
	if info == nil || !info.Available || info.AssetURL == "" {
		return errors.New("no update available")
	}

	archive, err := download(ctx, info.AssetURL)
	if err != nil {
		return fmt.Errorf("download asset: %w", err)
	}

	if info.SHA256URL != "" {
		expected, err := fetchExpectedSHA(ctx, info.SHA256URL)
		if err != nil {
			return fmt.Errorf("read sha256: %w", err)
		}
		got := sha256.Sum256(archive)
		if hex.EncodeToString(got[:]) != expected {
			return fmt.Errorf("sha256 mismatch (got %x, want %s)", got, expected)
		}
	}

	binary, err := extractBinary(archive, info.AssetName)
	if err != nil {
		return fmt.Errorf("extract binary: %w", err)
	}

	if err := selfupdate.Apply(bytes.NewReader(binary), selfupdate.Options{}); err != nil {
		// selfupdate attempts a rollback internally; surface the original error
		// so the UI can show what happened.
		return fmt.Errorf("apply update: %w", err)
	}
	return nil
}

// platformAssetName returns the CI naming convention used in build.yml:
//
//	reqost-{GOOS}-{GOARCH}.{tar.gz|zip}
func platformAssetName() (name, ext string) {
	ext = "tar.gz"
	if runtime.GOOS == "windows" {
		ext = "zip"
	}
	return fmt.Sprintf("reqost-%s-%s.%s", runtime.GOOS, runtime.GOARCH, ext), ext
}

// isNewer compares two semver-ish tags (v1.2.3 / 1.2.3). Falls back to a string
// compare for non-semver tags so unusual versions never block updates.
func isNewer(current, latest string) bool {
	c := strings.TrimPrefix(current, "v")
	l := strings.TrimPrefix(latest, "v")
	if c == l {
		return false
	}
	if c == "" || c == "dev" {
		return true
	}

	cp := strings.Split(c, ".")
	lp := strings.Split(l, ".")
	for i := 0; i < len(cp) && i < len(lp); i++ {
		ci, li := atoi(cp[i]), atoi(lp[i])
		if li > ci {
			return true
		}
		if li < ci {
			return false
		}
	}
	return len(lp) > len(cp)
}

func atoi(s string) int {
	// Stop at first non-digit so pre-release suffixes like "1-rc1" still compare
	// the numeric prefix.
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			break
		}
		n = n*10 + int(r-'0')
	}
	return n
}

func download(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("download %s: %s", url, resp.Status)
	}
	// 200 MiB cap: way over the realistic binary size, but bounded so a bad
	// asset URL can't hand-grenade memory.
	return io.ReadAll(io.LimitReader(resp.Body, 200<<20))
}

func fetchExpectedSHA(ctx context.Context, url string) (string, error) {
	body, err := download(ctx, url)
	if err != nil {
		return "", err
	}
	// CI writes "<hex>  filename" (shasum / sha256sum / our PowerShell variant).
	// Accept either that or a bare hex digest.
	line := strings.TrimSpace(string(body))
	if i := strings.IndexAny(line, " \t"); i > 0 {
		line = line[:i]
	}
	return strings.ToLower(line), nil
}

// extractBinary returns the embedded reqost / reqost.exe entry from a tar.gz
// (unix) or zip (windows) archive.
func extractBinary(archive []byte, assetName string) ([]byte, error) {
	if strings.HasSuffix(assetName, ".zip") {
		return extractZip(archive)
	}
	return extractTarGz(archive)
}

func extractTarGz(b []byte) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		if h.Typeflag != tar.TypeReg {
			continue
		}
		if base(h.Name) == "reqost" {
			return io.ReadAll(io.LimitReader(tr, 200<<20))
		}
	}
	return nil, errors.New("tar.gz: reqost binary not found in archive")
}

func extractZip(b []byte) ([]byte, error) {
	r, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		return nil, err
	}
	for _, f := range r.File {
		if base(f.Name) == "reqost.exe" {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer rc.Close()
			return io.ReadAll(io.LimitReader(rc, 200<<20))
		}
	}
	return nil, errors.New("zip: reqost.exe not found in archive")
}

func base(p string) string {
	if i := strings.LastIndexAny(p, "/\\"); i >= 0 {
		return p[i+1:]
	}
	return p
}
