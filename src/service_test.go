package kb

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
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
	again, err := svc.Sync(context.Background(), k.ID)
	if err != nil || again.Status != SyncUnchanged || again.Checksum != sync.Checksum {
		t.Fatalf("re-sync = %+v, %v", again, err)
	}

	projA := ContextEntity{Type: EntityProject, ID: "project-a"}
	hits, err := svc.Search(context.Background(), SearchRequest{Entity: projA, Query: "heartbeat"})
	if err != nil || len(hits) != 0 {
		t.Fatalf("unbound search = %+v, %v", hits, err)
	}
	if err := svc.Bind(context.Background(), k.ID, projA); err != nil {
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
	if err := svc.Bind(context.Background(), k.ID, ContextEntity{Type: EntityOrganization, ID: "org-1"}); err != nil {
		t.Fatal(err)
	}
	listed, err = svc.ListForEntity(context.Background(), ContextEntity{Type: EntityOrganization, ID: "org-1"})
	if err != nil || len(listed) != 1 || listed[0].ID != k.ID {
		t.Fatalf("list org = %+v, %v", listed, err)
	}
}

func TestGitLocalRegister(t *testing.T) {
	svc := NewService(NewMemoryStore())
	plain := t.TempDir()
	if err := os.WriteFile(filepath.Join(plain, "readme.md"), []byte("# X\n\nbody\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	k, err := svc.Register(context.Background(), RegisterRequest{
		Kind: KindSourceCodeRepository, Provider: ProviderGitLocal,
		Source: Source{Location: plain},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Sync(context.Background(), k.ID); !IsCode(err, ErrInvalidQuery) {
		t.Fatalf("plain dir sync = %v", err)
	}

	repo := t.TempDir()
	init := exec.Command("git", "-C", repo, "init")
	init.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1")
	if out, err := init.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	if err := os.WriteFile(filepath.Join(repo, "readme.md"), []byte("# X\n\nbody\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	k, err = svc.Register(context.Background(), RegisterRequest{
		Kind: KindSourceCodeRepository, Provider: ProviderGitLocal,
		Source: Source{Location: repo},
	})
	if err != nil {
		t.Fatal(err)
	}
	sync, err := svc.Sync(context.Background(), k.ID)
	if err != nil || sync.Documents != 1 {
		t.Fatalf("git sync = %+v, %v", sync, err)
	}
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
}
