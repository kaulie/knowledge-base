package kb

import (
	"context"
	"strings"
)

// Materialized is a local work tree the indexer can read.
type Materialized struct {
	Root     string
	Revision string
}

// Backend is one (kind, provider) implementation.
type Backend interface {
	Kind() string
	Provider() string
	Materialize(ctx context.Context, k Knowledge) (Materialized, error)
	List(ctx context.Context, k Knowledge, root string) ([]ListedFile, error)
}

type backendKey struct{ kind, provider string }

var implemented = map[backendKey]Backend{
	{KindFilesystem, ProviderLocal}:         filesystemLocal{},
	{KindSourceCodeRepository, ProviderGit}: gitRepo{},
}

func ImplementedBackends() []Backend {
	return []Backend{filesystemLocal{}, gitRepo{}}
}

func ResolveBackend(kind, provider string) (Backend, error) {
	k, p, err := normalizeKindProvider(kind, provider)
	if err != nil {
		return nil, err
	}
	if b, ok := implemented[backendKey{k, p}]; ok {
		return b, nil
	}
	return nil, Errorf(ErrInvalidQuery, "knowledge %s/%s is not implemented (have filesystem/local, source_code_repository/git)", k, p)
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
			p = ProviderGit
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

func (filesystemLocal) Materialize(ctx context.Context, k Knowledge) (Materialized, error) {
	root := strings.TrimSpace(k.Source.Location)
	if looksRemote(root) {
		return Materialized{}, Errorf(ErrInvalidQuery, "filesystem/local location must be a path on this host")
	}
	if root == "" {
		return Materialized{}, Errorf(ErrInvalidQuery, "source.location is required")
	}
	note(ctx, "info", "open host path %s", root)
	return Materialized{Root: root}, nil
}

func (filesystemLocal) List(_ context.Context, k Knowledge, root string) ([]ListedFile, error) {
	walk := strings.TrimSpace(root)
	if k.Shape != ShapeFile {
		if sub := strings.TrimSpace(k.Source.Path); sub != "" {
			walk = joinSource(walk, sub)
		}
	}
	if looksRemote(walk) {
		return nil, Errorf(ErrInvalidQuery, "filesystem/local location must be a path on this host")
	}
	files, err := DocumentFiles(walk, k.Suffixes)
	if err != nil {
		return nil, err
	}
	return listed(walk, files), nil
}

type gitRepo struct{}

func (gitRepo) Kind() string     { return KindSourceCodeRepository }
func (gitRepo) Provider() string { return ProviderGit }

func (gitRepo) Materialize(ctx context.Context, k Knowledge) (Materialized, error) {
	remote := gitRemote(k.Source)
	if remote == "" {
		return Materialized{}, Errorf(ErrInvalidQuery, "source_code_repository/git needs a clone URL in source.location or source.repository")
	}
	dest := workTreeDir(k.ID)
	rev, err := ensureClone(ctx, remote, k.Source.Ref, dest)
	if err != nil {
		return Materialized{}, err
	}
	return Materialized{Root: dest, Revision: rev}, nil
}

func (gitRepo) List(ctx context.Context, k Knowledge, root string) ([]ListedFile, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil, Errorf(ErrInvalidQuery, "source_code_repository/git has no work tree")
	}
	tracked, err := gitTrackedFiles(ctx, root)
	if err != nil {
		return nil, err
	}
	want, err := documentSuffixSet(k.Suffixes)
	if err != nil {
		return nil, err
	}
	prefix := gitPathPrefix(k.Source.Path)
	var files []string
	for _, rel := range tracked {
		if prefix != "" && rel != prefix && !strings.HasPrefix(rel, prefix+"/") {
			continue
		}
		if !want[suffixOf(rel)] {
			continue
		}
		files = append(files, rel)
	}
	return listed(root, files), nil
}

func gitRemote(src Source) string {
	if r := strings.TrimSpace(src.Repository); r != "" {
		return r
	}
	return strings.TrimSpace(src.Location)
}

// gitPathPrefix is a path from the repository root. Leading and trailing
// slashes are optional: "docs/x.md" and "/docs/x.md" are the same.
func gitPathPrefix(path string) string {
	p := strings.TrimSpace(path)
	p = strings.ReplaceAll(p, "\\", "/")
	p = strings.Trim(p, "/")
	for strings.Contains(p, "//") {
		p = strings.ReplaceAll(p, "//", "/")
	}
	return p
}

func validRepoRelPath(p string) bool {
	if p == "" {
		return true
	}
	for _, part := range strings.Split(p, "/") {
		if part == "" || part == "." || part == ".." {
			return false
		}
	}
	return true
}

func suffixOf(rel string) string {
	i := strings.LastIndex(rel, ".")
	if i < 0 || i == len(rel)-1 {
		return ""
	}
	if strings.Contains(rel[i+1:], "/") {
		return ""
	}
	return strings.ToLower(rel[i:])
}

func listed(root string, files []string) []ListedFile {
	out := make([]ListedFile, 0, len(files))
	for _, rel := range files {
		out = append(out, ListedFile{Name: rel, Location: root, Path: rel})
	}
	return out
}
