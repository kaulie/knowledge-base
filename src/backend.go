package kb

import (
	"context"
	"os/exec"
	"strings"
)

// Backend is one (kind, provider) implementation.
type Backend interface {
	Kind() string
	Provider() string
	List(ctx context.Context, location string, suffixes []string) ([]ListedFile, error)
}

type backendKey struct{ kind, provider string }

var implemented = map[backendKey]Backend{
	{KindFilesystem, ProviderLocal}:              filesystemLocal{},
	{KindSourceCodeRepository, ProviderGitLocal}: gitLocal{},
}

func ImplementedBackends() []Backend {
	return []Backend{filesystemLocal{}, gitLocal{}}
}

func ResolveBackend(kind, provider string) (Backend, error) {
	k, p, err := normalizeKindProvider(kind, provider)
	if err != nil {
		return nil, err
	}
	if b, ok := implemented[backendKey{k, p}]; ok {
		return b, nil
	}
	return nil, Errorf(ErrInvalidQuery, "knowledge %s/%s is not implemented (have filesystem/local, source_code_repository/git-local)", k, p)
}

func normalizeKindProvider(kind, provider string) (string, string, error) {
	k := strings.ToLower(strings.TrimSpace(kind))
	p := strings.ToLower(strings.TrimSpace(provider))
	if k == "" {
		k = KindFilesystem
	}
	switch k {
	case KindFilesystem:
		if p == "" {
			p = ProviderLocal
		}
	case KindSourceCodeRepository:
		if p == "" {
			p = ProviderGitLocal
		}
	case KindCloud:
		if p == "" {
			return k, "", Errorf(ErrInvalidQuery, "cloud storage needs a provider (s3, gcs, azure, oss); none are implemented yet")
		}
	default:
		return "", "", Errorf(ErrInvalidQuery, "kind must be filesystem, source_code_repository, or cloud")
	}
	return k, p, nil
}

func looksRemote(location string) bool {
	s := strings.ToLower(strings.TrimSpace(location))
	return strings.Contains(s, "://") || strings.HasPrefix(s, "git@")
}

type filesystemLocal struct{}

func (filesystemLocal) Kind() string     { return KindFilesystem }
func (filesystemLocal) Provider() string { return ProviderLocal }

func (filesystemLocal) List(_ context.Context, location string, suffixes []string) ([]ListedFile, error) {
	root := strings.TrimSpace(location)
	if looksRemote(root) {
		return nil, Errorf(ErrInvalidQuery, "filesystem/local location must be a path on this host")
	}
	files, err := DocumentFiles(root, suffixes)
	if err != nil {
		return nil, err
	}
	return listed(root, files), nil
}

type gitLocal struct{}

func (gitLocal) Kind() string     { return KindSourceCodeRepository }
func (gitLocal) Provider() string { return ProviderGitLocal }

func (gitLocal) List(ctx context.Context, location string, suffixes []string) ([]ListedFile, error) {
	root := strings.TrimSpace(location)
	if looksRemote(root) {
		return nil, Errorf(ErrInvalidQuery, "source_code_repository/git-local needs a checkout path")
	}
	if err := requireGitCheckout(ctx, root); err != nil {
		return nil, err
	}
	files, err := DocumentFiles(root, suffixes)
	if err != nil {
		return nil, err
	}
	return listed(root, files), nil
}

func requireGitCheckout(ctx context.Context, root string) error {
	if strings.TrimSpace(root) == "" {
		return Errorf(ErrInvalidQuery, "source_code_repository/git-local location is required")
	}
	out, err := exec.CommandContext(ctx, "git", "-C", root, "rev-parse", "--is-inside-work-tree").Output()
	if err != nil || strings.TrimSpace(string(out)) != "true" {
		return Errorf(ErrInvalidQuery, "location %q is not a git checkout", root)
	}
	return nil
}

func listed(root string, files []string) []ListedFile {
	out := make([]ListedFile, 0, len(files))
	for _, rel := range files {
		out = append(out, ListedFile{Name: rel, Location: root, Path: rel})
	}
	return out
}
