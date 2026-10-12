package kb

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	EnvDataDir = "KNOWLEDGE_BASE_DATA"
	envRuntime = "RUNTIME_DIR"
)

func dataRoot() string {
	if d := strings.TrimSpace(os.Getenv(EnvDataDir)); d != "" {
		return d
	}
	if d := strings.TrimSpace(os.Getenv(envRuntime)); d != "" {
		return filepath.Join(d, "backend", "data")
	}
	return filepath.Join(os.TempDir(), "knowledge-base")
}

func workTreeDir(knowledgeID string) string {
	id := filepath.Base(strings.TrimSpace(knowledgeID))
	if id == "" || id == "." || id == ".." {
		id = "unknown"
	}
	return filepath.Join(dataRoot(), "repos", id)
}

func removeWorkTree(knowledgeID string) {
	dest := workTreeDir(knowledgeID)
	root := filepath.Join(dataRoot(), "repos")
	rel, err := filepath.Rel(root, dest)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
		return
	}
	_ = os.RemoveAll(dest)
}

func gitEnv() []string {
	return append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
}

func gitCommand(ctx context.Context, dir string, args ...string) *exec.Cmd {
	var cmd *exec.Cmd
	if strings.TrimSpace(dir) != "" {
		cmd = exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	} else {
		cmd = exec.CommandContext(ctx, "git", args...)
	}
	cmd.Env = gitEnv()
	return cmd
}

func isGitWorkTree(ctx context.Context, dir string) bool {
	if strings.TrimSpace(dir) == "" {
		return false
	}
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return false
	}
	out, err := gitCommand(ctx, dir, "rev-parse", "--is-inside-work-tree").Output()
	return err == nil && strings.TrimSpace(string(out)) == "true"
}

func gitHEAD(ctx context.Context, dir string) (string, error) {
	out, err := gitCommand(ctx, dir, "rev-parse", "HEAD").Output()
	if err != nil {
		return "", Wrap(ErrSourceUnavailable, err, "git rev-parse HEAD in %q", dir)
	}
	return strings.TrimSpace(string(out)), nil
}

func gitTrackedFiles(ctx context.Context, dir string) ([]string, error) {
	out, err := gitCommand(ctx, dir, "ls-tree", "-r", "--name-only", "HEAD").Output()
	if err != nil {
		return nil, Wrap(ErrSourceUnavailable, err, "git ls-tree in %q", dir)
	}
	var files []string
	for _, line := range bytes.Split(out, []byte("\n")) {
		rel := filepath.ToSlash(strings.TrimSpace(string(line)))
		if rel == "" {
			continue
		}
		files = append(files, rel)
	}
	return files, nil
}

func ensureClone(ctx context.Context, remote, ref, dest string) (string, error) {
	remote = strings.TrimSpace(remote)
	ref = strings.TrimSpace(ref)
	dest = filepath.Clean(strings.TrimSpace(dest))
	if remote == "" {
		return "", Errorf(ErrInvalidQuery, "git clone URL is required")
	}
	if dest == "" || dest == "." {
		return "", Errorf(ErrInvalidQuery, "git work tree path is required")
	}
	if isGitWorkTree(ctx, dest) {
		if err := refreshWorkTree(ctx, dest, ref); err == nil {
			return gitHEAD(ctx, dest)
		}
		if err := os.RemoveAll(dest); err != nil {
			return "", Wrap(ErrSourceUnavailable, err, "replace work tree %q", dest)
		}
	} else if _, err := os.Stat(dest); err == nil {
		if err := os.RemoveAll(dest); err != nil {
			return "", Wrap(ErrSourceUnavailable, err, "replace %q", dest)
		}
	}
	if err := cloneFresh(ctx, remote, ref, dest); err != nil {
		return "", err
	}
	return gitHEAD(ctx, dest)
}

func cloneFresh(ctx context.Context, remote, ref, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return Wrap(ErrSourceUnavailable, err, "create clone parent for %q", dest)
	}
	args := []string{"clone", "--depth", "1"}
	if ref != "" {
		args = append(args, "--branch", ref)
	}
	args = append(args, remote, dest)
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Env = gitEnv()
	if out, err := cmd.CombinedOutput(); err != nil {
		_ = os.RemoveAll(dest)
		return Wrap(ErrSourceUnavailable, err, "git clone %s: %s", remote, strings.TrimSpace(string(out)))
	}
	return nil
}

func refreshWorkTree(ctx context.Context, dest, ref string) error {
	fetch := []string{"fetch", "--depth", "1", "origin"}
	if ref != "" {
		fetch = append(fetch, ref)
	}
	if out, err := gitCommand(ctx, dest, fetch...).CombinedOutput(); err != nil {
		return Wrap(ErrSourceUnavailable, err, "git fetch: %s", strings.TrimSpace(string(out)))
	}
	checkout := "FETCH_HEAD"
	if ref == "" {
		if out, err := gitCommand(ctx, dest, "pull", "--ff-only").CombinedOutput(); err != nil {
			return Wrap(ErrSourceUnavailable, err, "git pull: %s", strings.TrimSpace(string(out)))
		}
		return nil
	}
	if out, err := gitCommand(ctx, dest, "checkout", "--force", checkout).CombinedOutput(); err != nil {
		return Wrap(ErrSourceUnavailable, err, "git checkout: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

func repoDisplayName(loc string) string {
	s := strings.TrimSpace(loc)
	s = strings.TrimSuffix(s, ".git")
	s = strings.TrimRight(s, "/")
	if i := strings.LastIndexAny(s, "/:"); i >= 0 {
		s = s[i+1:]
	}
	if s == "" {
		return loc
	}
	return s
}
