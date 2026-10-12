package kb

import (
	"context"
	"os"
	"path/filepath"
	"strings"
)

func loadFile(ctx context.Context, k Knowledge, root, path string) ([]byte, string, error) {
	full := joinSource(root, path)
	if full == "" {
		full = joinSource(k.Source.Location, path)
	}
	if full == "" {
		return nil, "", Errorf(ErrSourceUnavailable, "knowledge %q has no readable location", k.ID)
	}
	content, err := os.ReadFile(full)
	if err != nil {
		return nil, "", Wrap(ErrSourceUnavailable, err, "read %q", full)
	}
	return content, sourceRevision(ctx, k, root), nil
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

func sourceRevision(ctx context.Context, k Knowledge, root string) string {
	if k.Kind != KindSourceCodeRepository || k.Provider != ProviderGit {
		return k.Revision
	}
	dir := strings.TrimSpace(root)
	if dir == "" {
		dir = workTreeDir(k.ID)
	}
	rev, err := gitHEAD(ctx, dir)
	if err != nil {
		return k.Revision
	}
	return rev
}
