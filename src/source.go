package kb

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func loadFile(ctx context.Context, k Knowledge, path string) ([]byte, string, error) {
	full := joinSource(k.Source.Location, path)
	if full == "" {
		return nil, "", Errorf(ErrSourceUnavailable, "knowledge %q has no readable location", k.ID)
	}
	content, err := os.ReadFile(full)
	if err != nil {
		return nil, "", Wrap(ErrSourceUnavailable, err, "read %q", full)
	}
	return content, sourceRevision(ctx, k, full), nil
}

func joinSource(location, path string) string {
	location = strings.TrimSpace(location)
	path = strings.TrimSpace(path)
	switch {
	case location == "":
		return path
	case path == "":
		return location
	}
	if info, err := os.Stat(location); err == nil && info.IsDir() {
		return filepath.Join(location, path)
	}
	return location
}

func sourceRevision(ctx context.Context, k Knowledge, file string) string {
	if k.Kind != KindSourceCodeRepository || k.Provider != ProviderGitLocal {
		return k.Revision
	}
	dir := strings.TrimSpace(k.Source.Location)
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		dir = filepath.Dir(file)
	}
	out, err := exec.CommandContext(ctx, "git", "-C", dir, "rev-parse", "HEAD").Output()
	if err != nil {
		return k.Revision
	}
	return strings.TrimSpace(string(out))
}
