package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// GitService wraps the local `git` binary for the "export current workspace
// to a git repo" workflow. Avoids a go-git dependency; the user already has
// git on PATH if they care about this feature.
type GitService struct {
	collSvc *CollectionService
}

func NewGitService(c *CollectionService) *GitService { return &GitService{collSvc: c} }

// GitStatus is a coarse report on a working tree. Output is what `git status
// --porcelain` returns (empty = clean).
type GitStatus struct {
	Branch  string `json:"branch"`
	Status  string `json:"status"`  // porcelain dump
	HasRepo bool   `json:"hasRepo"`
}

// Init turns dir into a git repo (idempotent if already one).
func (s *GitService) Init(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
		return nil
	}
	return runGit(dir, "init")
}

// Status reports branch + porcelain status.
func (s *GitService) Status(dir string) (*GitStatus, error) {
	out := &GitStatus{}
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		return out, nil
	}
	out.HasRepo = true
	branch, _ := capture(dir, "git", "rev-parse", "--abbrev-ref", "HEAD")
	out.Branch = strings.TrimSpace(branch)
	st, _ := capture(dir, "git", "status", "--porcelain")
	out.Status = st
	return out, nil
}

// Export writes the active workspace's collection as Postman v2.1 JSON into
// dir/collection.json. Caller then Commits.
func (s *GitService) Export(dir, name string) error {
	if s.collSvc == nil || s.collSvc.db == nil {
		return fmt.Errorf("collection unavailable")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if name == "" {
		name = "reqost"
	}
	json, err := s.collSvc.db.ExportJSON(name)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "collection.json"), []byte(json), 0o644)
}

// Commit stages everything in dir and commits with message.
func (s *GitService) Commit(dir, message string) error {
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		return fmt.Errorf("not a git repo: %s", dir)
	}
	if message == "" {
		message = "reqost: snapshot"
	}
	if err := runGit(dir, "add", "-A"); err != nil {
		return err
	}
	// Allow empty commits so the user can mark a checkpoint after a re-export.
	return runGit(dir, "commit", "--allow-empty", "-m", message)
}

// Branches returns local branch names, current branch first.
func (s *GitService) Branches(dir string) ([]string, error) {
	out, err := capture(dir, "git", "branch", "--list", "--format=%(refname:short)")
	if err != nil {
		return nil, err
	}
	cur, _ := capture(dir, "git", "rev-parse", "--abbrev-ref", "HEAD")
	cur = strings.TrimSpace(cur)
	var names []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			names = append(names, line)
		}
	}
	// Put current first.
	for i, n := range names {
		if n == cur {
			names[0], names[i] = names[i], names[0]
			break
		}
	}
	return names, nil
}

// Checkout switches branches. Creates the branch if it doesn't exist yet.
func (s *GitService) Checkout(dir, branch string) error {
	if branch == "" {
		return fmt.Errorf("branch required")
	}
	if err := runGit(dir, "checkout", branch); err == nil {
		return nil
	}
	return runGit(dir, "checkout", "-b", branch)
}

func runGit(dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git %s: %s", strings.Join(args, " "), strings.TrimSpace(string(out)))
	}
	return nil
}

func capture(dir, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	return string(out), err
}
