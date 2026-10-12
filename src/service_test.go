package kb

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRegisterSyncBindSearch(t *testing.T) {
	svc := NewService(NewMemoryStore())
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "readme.md"), []byte("# Root\n\nhello world\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "docs", "runtime.md"), []byte("# Runtime\n\n## Heartbeat\n\nagents send a heartbeat every tick\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	k, err := svc.Register(context.Background(), RegisterRequest{
		Name: "docs", Kind: KindFilesystem, Provider: ProviderLocal,
		Shape: ShapeDirectory, Source: Source{Location: root},
	})
	if err != nil || k.ID == "" || k.Kind != KindFilesystem {
		t.Fatalf("register = %+v, %v", k, err)
	}

	sync, err := svc.Sync(context.Background(), k.ID)
	if err != nil || sync.Status != SyncIndexed || sync.Documents != 2 || sync.Sections < 1 {
		t.Fatalf("sync = %+v, %v", sync, err)
	}
	if !syncLogHas(sync.Logs, "listed 2 documents") || !syncLogHas(sync.Logs, "done status=indexed") {
		t.Fatalf("sync logs = %+v", sync.Logs)
	}
	again, err := svc.Sync(context.Background(), k.ID)
	if err != nil || again.Status != SyncUnchanged || again.Checksum != sync.Checksum {
		t.Fatalf("re-sync = %+v, %v", again, err)
	}

	projA := ContextEntity{Type: EntityProject, ID: "project-a"}
	hits, err := svc.Search(context.Background(), SearchRequest{Entity: projA, Query: "heartbeat"})
	if err != nil || len(hits) != 0 {
		t.Fatalf("unbound search = %+v, %v", hits, err)
	}
	if _, err := svc.Bind(context.Background(), k.ID, projA); err != nil {
		t.Fatal(err)
	}
	hits, err = svc.Search(context.Background(), SearchRequest{Entity: projA, Query: "heartbeat"})
	if err != nil || len(hits) != 1 || hits[0].KnowledgeID != k.ID {
		t.Fatalf("bound search = %+v, %v", hits, err)
	}
	sec, err := svc.GetSection(context.Background(), hits[0].SectionID)
	if err != nil || sec == nil || sec.KnowledgeID != k.ID {
		t.Fatalf("get section = %+v, %v", sec, err)
	}
	hits, err = svc.Search(context.Background(), SearchRequest{Entity: ContextEntity{Type: EntityProject, ID: "project-b"}, Query: "heartbeat"})
	if err != nil || len(hits) != 0 {
		t.Fatalf("other entity = %+v, %v", hits, err)
	}
	hits, err = svc.Search(context.Background(), SearchRequest{Entity: ContextEntity{Type: EntityOrganization, ID: "project-a"}, Query: "heartbeat"})
	if err != nil || len(hits) != 0 {
		t.Fatalf("same id other type = %+v, %v", hits, err)
	}

	listed, err := svc.ListForEntity(context.Background(), projA)
	if err != nil || len(listed) != 1 || listed[0].ID != k.ID {
		t.Fatalf("list entity = %+v, %v", listed, err)
	}
	if len(listed[0].Bindings) != 1 || listed[0].Bindings[0] != projA {
		t.Fatalf("list bindings = %+v", listed[0].Bindings)
	}
	if _, err := svc.Bind(context.Background(), k.ID, ContextEntity{Type: EntityOrganization, ID: "org-1"}); err != nil {
		t.Fatal(err)
	}
	listed, err = svc.ListForEntity(context.Background(), ContextEntity{Type: EntityOrganization, ID: "org-1"})
	if err != nil || len(listed) != 1 || listed[0].ID != k.ID {
		t.Fatalf("list org = %+v, %v", listed, err)
	}
}

func TestGitCloneOnBind(t *testing.T) {
	t.Setenv(EnvDataDir, t.TempDir())
	svc := NewService(NewMemoryStore())
	origin := initGitOrigin(t, map[string]string{
		"readme.md":      "# Root\n\nhello world\n",
		"docs/runtime.md": "# Runtime\n\n## Heartbeat\n\nagents send a heartbeat every tick\n",
		"main.go":        "package main\n",
	})

	k, err := svc.Register(context.Background(), RegisterRequest{
		Kind: KindSourceCodeRepository, Provider: ProviderGit,
		Shape: ShapeDirectory, Source: Source{Location: origin},
	})
	if err != nil {
		t.Fatal(err)
	}
	if k.Provider != ProviderGit || k.Source.Repository != origin {
		t.Fatalf("register = %+v", k)
	}
	if _, err := os.Stat(workTreeDir(k.ID)); !os.IsNotExist(err) {
		t.Fatalf("clone should wait for bind, stat = %v", err)
	}

	proj := ContextEntity{Type: EntityProject, ID: "project-git"}
	synced, err := svc.Bind(context.Background(), k.ID, proj)
	if err != nil {
		t.Fatal(err)
	}
	if synced == nil || !syncLogHas(synced.Logs, "git clone") || !syncLogHas(synced.Logs, "parse ") {
		t.Fatalf("bind sync logs = %+v", synced)
	}
	if _, err := os.Stat(filepath.Join(workTreeDir(k.ID), ".git")); err != nil {
		t.Fatalf("work tree: %v", err)
	}
	got, err := svc.Get(context.Background(), k.ID)
	if err != nil || got.Documents != 2 || got.Revision == "" {
		t.Fatalf("after bind = %+v, %v", got, err)
	}

	hits, err := svc.Search(context.Background(), SearchRequest{Entity: proj, Query: "heartbeat"})
	if err != nil || len(hits) != 1 || hits[0].KnowledgeID != k.ID {
		t.Fatalf("search = %+v, %v", hits, err)
	}

	if err := os.WriteFile(filepath.Join(workTreeDir(k.ID), "extra.md"), []byte("# Extra\n\nuntracked\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	again, err := svc.Sync(context.Background(), k.ID)
	if err != nil || again.Documents != 2 {
		t.Fatalf("ls-tree should ignore untracked extra.md: %+v, %v", again, err)
	}

	if err := svc.Delete(context.Background(), k.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(workTreeDir(k.ID)); !os.IsNotExist(err) {
		t.Fatalf("delete should remove work tree, stat = %v", err)
	}
}

func TestGitRegisterBindParses(t *testing.T) {
	t.Setenv(EnvDataDir, t.TempDir())
	svc := NewService(NewMemoryStore())
	origin := initGitOrigin(t, map[string]string{
		"docs/runtime.md": "# Runtime\n\n## Heartbeat\n\nagents send a heartbeat every tick\n",
	})
	k, err := svc.Register(context.Background(), RegisterRequest{
		Kind: KindSourceCodeRepository, Source: Source{Location: origin, Path: "docs"},
		Bind: &ContextEntity{Type: EntityProject, ID: "alpha"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if k.Provider != ProviderGit || k.Documents != 1 {
		t.Fatalf("register+bind = %+v", k)
	}
	hits, err := svc.Search(context.Background(), SearchRequest{
		Entity: ContextEntity{Type: EntityProject, ID: "alpha"}, Query: "heartbeat",
	})
	if err != nil || len(hits) != 1 {
		t.Fatalf("search = %+v, %v", hits, err)
	}
}

func TestGitPathFromRepoRoot(t *testing.T) {
	t.Setenv(EnvDataDir, t.TempDir())
	svc := NewService(NewMemoryStore())
	origin := initGitOrigin(t, map[string]string{
		"docs/runtime.md": "# Runtime\n\n## Heartbeat\n\nagents send a heartbeat every tick\n",
	})
	for _, path := range []string{"docs/runtime.md", "/docs/runtime.md", "docs/runtime.md/"} {
		k, err := svc.Register(context.Background(), RegisterRequest{
			ID: "kn-" + strings.ReplaceAll(path, "/", "_"),
			Kind: KindSourceCodeRepository, Shape: ShapeFile,
			Source: Source{Location: origin, Path: path},
			Bind:   &ContextEntity{Type: EntityProject, ID: "p-" + strings.ReplaceAll(path, "/", "_")},
		})
		if err != nil {
			t.Fatalf("path %q: %v", path, err)
		}
		if k.Source.Path != "docs/runtime.md" {
			t.Fatalf("path %q stored as %q", path, k.Source.Path)
		}
		if k.Documents != 1 {
			t.Fatalf("path %q documents = %d", path, k.Documents)
		}
	}
	if _, err := svc.Register(context.Background(), RegisterRequest{
		Kind: KindSourceCodeRepository, Source: Source{Location: origin, Path: "../secret"},
	}); !IsCode(err, ErrInvalidQuery) {
		t.Fatalf("dot-dot path = %v", err)
	}
	if _, err := svc.Register(context.Background(), RegisterRequest{
		Kind: KindSourceCodeRepository,
	}); !IsCode(err, ErrInvalidQuery) || err == nil || !strings.Contains(err.Error(), "git repository") {
		t.Fatalf("missing repo = %v", err)
	}
}

func TestGitFileNeedsPath(t *testing.T) {
	svc := NewService(NewMemoryStore())
	_, err := svc.Register(context.Background(), RegisterRequest{
		Kind: KindSourceCodeRepository, Shape: ShapeFile,
		Source: Source{Location: "git@host:org/repo.git"},
	})
	if !IsCode(err, ErrInvalidQuery) {
		t.Fatalf("missing path = %v", err)
	}
}

func syncLogHas(logs []SyncLog, needle string) bool {
	for _, line := range logs {
		if strings.Contains(line.Message, needle) {
			return true
		}
	}
	return false
}

func initGitOrigin(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-b", "main")
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "test")
	for rel, body := range files {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	run("add", ".")
	run("commit", "-m", "init")
	return root
}

func TestUnimplementedProvider(t *testing.T) {
	svc := NewService(NewMemoryStore())
	if _, err := svc.Register(context.Background(), RegisterRequest{
		Kind: KindCloud, Provider: ProviderS3, Source: Source{Location: "bucket"},
	}); !IsCode(err, ErrInvalidQuery) {
		t.Fatalf("s3 = %v", err)
	}
	if _, err := svc.Register(context.Background(), RegisterRequest{
		Kind: KindSourceCodeRepository, Provider: ProviderGitHub, Source: Source{Location: "owner/repo"},
	}); !IsCode(err, ErrInvalidQuery) {
		t.Fatalf("github = %v", err)
	}
	if _, err := svc.Register(context.Background(), RegisterRequest{
		Kind: KindSourceCodeRepository, Provider: ProviderGitLocal, Source: Source{Location: "/tmp/checkout"},
	}); !IsCode(err, ErrInvalidQuery) {
		t.Fatalf("git-local = %v", err)
	}
}
