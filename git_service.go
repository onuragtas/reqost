package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// GitService wraps the local `git` binary for the "export current workspace
// to a git repo" workflow. Avoids a go-git dependency; the user already has
// git on PATH if they care about this feature.
type GitService struct {
	collSvc *CollectionService
}

func NewGitService(c *CollectionService) *GitService { return &GitService{collSvc: c} }

// GitStatus is a coarse report on a working tree.
type GitStatus struct {
	Branch  string `json:"branch"`
	Status  string `json:"status"`  // `git status --porcelain` (empty = clean)
	HasRepo bool   `json:"hasRepo"`
	// Remote tracking — empty when the local branch has no upstream
	// configured (e.g. fresh `git init` with no remote yet).
	Remote  string `json:"remote"`
	Upstream string `json:"upstream"` // e.g. "origin/master"
	Ahead   int    `json:"ahead"`    // commits to push
	Behind  int    `json:"behind"`   // commits to pull
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

// Status reports branch + porcelain status + tracking info. Tracking fields
// stay empty when the local branch has no upstream configured.
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

	// Remote name (first listed). With no remote, `git remote` is empty.
	if rs, err := capture(dir, "git", "remote"); err == nil {
		if line := strings.TrimSpace(strings.SplitN(rs, "\n", 2)[0]); line != "" {
			out.Remote = line
		}
	}
	// Upstream (e.g. origin/master). When unset the command exits non-zero.
	if up, err := capture(dir, "git", "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}"); err == nil {
		out.Upstream = strings.TrimSpace(up)
		// rev-list left<right gives "<ahead> <behind>" when upstream exists.
		if cnt, err := capture(dir, "git", "rev-list", "--left-right", "--count", "HEAD..."+out.Upstream); err == nil {
			parts := strings.Fields(strings.TrimSpace(cnt))
			if len(parts) == 2 {
				out.Ahead, _ = strconv.Atoi(parts[0])
				out.Behind, _ = strconv.Atoi(parts[1])
			}
		}
	}
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

// Push pushes the current branch. When the local branch has no upstream
// yet (e.g. just `git init && git remote add`), the first push sets it.
func (s *GitService) Push(dir string) error {
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		return fmt.Errorf("not a git repo: %s", dir)
	}
	branch, err := capture(dir, "git", "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return err
	}
	branch = strings.TrimSpace(branch)
	if branch == "HEAD" {
		return fmt.Errorf("detached HEAD — checkout a branch first")
	}
	// If upstream is missing, default to `origin <branch>` (with -u so the
	// link sticks for future plain `git push`/`git pull`).
	if _, err := capture(dir, "git", "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}"); err != nil {
		return runGit(dir, "push", "-u", "origin", branch)
	}
	return runGit(dir, "push")
}

// Pull fetches + merges the tracked branch. Refuses on a dirty working tree
// to avoid surprise conflicts.
func (s *GitService) Pull(dir string) error {
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		return fmt.Errorf("not a git repo: %s", dir)
	}
	if st, _ := capture(dir, "git", "status", "--porcelain"); strings.TrimSpace(st) != "" {
		return fmt.Errorf("working tree is dirty — commit first or discard changes")
	}
	if _, err := capture(dir, "git", "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}"); err != nil {
		return fmt.Errorf("current branch has no upstream — set a remote first")
	}
	return runGit(dir, "pull", "--ff-only")
}

// Fetch updates remote refs without merging — useful so Status's ahead/behind
// reflects what the server actually has.
func (s *GitService) Fetch(dir string) error {
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		return fmt.Errorf("not a git repo: %s", dir)
	}
	return runGit(dir, "fetch", "--quiet")
}

// SetRemote adds or rewrites a remote URL (defaults to `origin`).
func (s *GitService) SetRemote(dir, name, url string) error {
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		return fmt.Errorf("not a git repo: %s", dir)
	}
	if name == "" {
		name = "origin"
	}
	// add → if already present, rewrite via set-url.
	if err := runGit(dir, "remote", "add", name, url); err == nil {
		return nil
	}
	return runGit(dir, "remote", "set-url", name, url)
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
