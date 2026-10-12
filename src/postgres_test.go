package kb

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestResolveDatabaseName(t *testing.T) {
	name, err := resolveDatabaseName("postgres://localhost:5432/knowledge_base?sslmode=disable")
	if err != nil || name != DefaultDatabase {
		t.Fatalf("own name: %s %v", name, err)
	}
	name, err = resolveDatabaseName("postgres://localhost:5432/postgres?sslmode=disable")
	if err != nil || name != DefaultDatabase {
		t.Fatalf("system db remaps to %s: %v", name, err)
	}
	if _, err := resolveDatabaseName("postgres://localhost:5432/autonomy?sslmode=disable"); err == nil {
		t.Fatal("autonomy database must be refused")
	}
}

func TestPostgresStoreRegisterSyncBindSearch(t *testing.T) {
	store := newPostgresStore(t)
	svc := NewService(store)
	root := t.TempDir()
	if err := os.MkdirAll(root+"/docs", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(root+"/docs/runtime.md", []byte("# Runtime\n\n## Heartbeat\n\nagents send a heartbeat every tick\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	k, err := svc.Register(context.Background(), RegisterRequest{
		Name: "docs", Kind: KindFilesystem, Provider: ProviderLocal,
		Shape: ShapeDirectory, Source: Source{Location: root},
	})
	if err != nil {
		t.Fatal(err)
	}
	sync, err := svc.Sync(context.Background(), k.ID)
	if err != nil || sync.Documents < 1 {
		t.Fatalf("sync = %+v %v", sync, err)
	}
	got, err := svc.Get(context.Background(), k.ID)
	if err != nil || got.Documents < 1 || got.Kind != KindFilesystem {
		t.Fatalf("get after restart-shaped read = %+v %v", got, err)
	}
	proj := ContextEntity{Type: EntityProject, ID: "project-pg"}
	if _, err := svc.Bind(context.Background(), k.ID, proj); err != nil {
		t.Fatal(err)
	}
	hits, err := svc.Search(context.Background(), SearchRequest{Entity: proj, Query: "heartbeat"})
	if err != nil || len(hits) == 0 || hits[0].KnowledgeID != k.ID {
		t.Fatalf("search = %+v %v", hits, err)
	}
	sec, err := svc.GetSection(context.Background(), hits[0].SectionID)
	if err != nil || sec == nil || !strings.Contains(sec.Content, "heartbeat") {
		t.Fatalf("section = %+v %v", sec, err)
	}
	if err := svc.Delete(context.Background(), k.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Get(context.Background(), k.ID); !IsCode(err, ErrNotFound) {
		t.Fatalf("deleted still present: %v", err)
	}
}

func TestOpenPostgresStoreCreatesMissingDatabase(t *testing.T) {
	admin, base := pgAdminConn(t)
	name := fmt.Sprintf("kb_create_%d", time.Now().UnixNano()%1e12)
	t.Cleanup(func() { _, _ = admin.Exec(`DROP DATABASE IF EXISTS ` + quoteIdent(name)) })
	store, err := OpenPostgresStore(withDatabase(base, name))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	var exists bool
	if err := admin.QueryRow(`SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)`, name).Scan(&exists); err != nil || !exists {
		t.Fatalf("database %s was not created: exists=%v err=%v", name, exists, err)
	}
}

func TestOpenPostgresStoreRefusesAutonomyDatabase(t *testing.T) {
	if _, err := OpenPostgresStore("postgres://localhost:5432/autonomy?sslmode=disable"); err == nil {
		t.Fatal("expected refusal")
	}
}

func newPostgresStore(t *testing.T) *PostgresStore {
	t.Helper()
	admin, base := pgAdminConn(t)
	name := fmt.Sprintf("kb_test_%d", time.Now().UnixNano()%1e12)
	store, err := OpenPostgresStore(withDatabase(base, name))
	if err != nil {
		t.Fatalf("OpenPostgresStore: %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
		_, _ = admin.Exec(`DROP DATABASE IF EXISTS ` + quoteIdent(name))
	})
	return store
}

func pgAdminConn(t *testing.T) (*sql.DB, string) {
	t.Helper()
	dsn := pgTestDSN()
	admin, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Skipf("postgres is not configured (%s): %v", dsn, err)
	}
	admin.SetMaxOpenConns(2)
	t.Cleanup(func() { _ = admin.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := admin.PingContext(ctx); err != nil {
		t.Skipf("postgres is not reachable at %s: %v", dsn, err)
	}
	return admin, dsn
}

func pgTestDSN() string {
	if dsn := strings.TrimSpace(os.Getenv("KNOWLEDGE_BASE_POSTGRES_TEST_DSN")); dsn != "" {
		return dsn
	}
	if dsn := strings.TrimSpace(os.Getenv("AUTONOMY_POSTGRES_TEST_DSN")); dsn != "" {
		return dsn
	}
	return "postgres://localhost:5432/postgres?sslmode=disable"
}
