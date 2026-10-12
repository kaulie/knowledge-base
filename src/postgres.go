package kb

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

const (
	// EnvPostgresDSN is the connection string. Same Postgres instance as
	// autonomy is fine; the database name must not be autonomy.
	EnvPostgresDSN = "KNOWLEDGE_BASE_POSTGRES_DSN"
	// EnvDatabaseURL is the conventional fallback DSN variable.
	EnvDatabaseURL = "DATABASE_URL"
	// DefaultDatabase is the dedicated database this service creates and uses.
	DefaultDatabase = "knowledge_base"
	// DefaultPostgresDSN talks to the local instance autonomy tests also use.
	DefaultPostgresDSN = "postgres://localhost:5432/knowledge_base?sslmode=disable"
)

// PostgresStore is Store over PostgreSQL. Schema is created on open.
type PostgresStore struct {
	db *sql.DB
}

// OpenConfiguredStore opens the postgres store the process runs on:
// KNOWLEDGE_BASE_POSTGRES_DSN, else DATABASE_URL, else DefaultPostgresDSN.
// The named database is created on the instance if it does not exist.
func OpenConfiguredStore() (*PostgresStore, error) {
	return OpenPostgresStore(configuredPostgresDSN())
}

func configuredPostgresDSN() string {
	for _, env := range []string{EnvPostgresDSN, EnvDatabaseURL} {
		if v := strings.TrimSpace(os.Getenv(env)); v != "" {
			return v
		}
	}
	return DefaultPostgresDSN
}

// OpenPostgresStore connects to dsn, creates the target database when missing,
// and migrates the schema. It refuses to use autonomy's database.
func OpenPostgresStore(dsn string) (*PostgresStore, error) {
	dsn = strings.TrimSpace(dsn)
	if dsn == "" {
		return nil, fmt.Errorf("open postgres: empty dsn")
	}
	name, err := resolveDatabaseName(dsn)
	if err != nil {
		return nil, err
	}
	dsn = withDatabase(dsn, name)
	if err := ensureDatabase(dsn, name); err != nil {
		return nil, err
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}
	db.SetMaxOpenConns(8)
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("open postgres: %w", err)
	}
	s := &PostgresStore{db: db}
	if err := s.migrate(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *PostgresStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func resolveDatabaseName(dsn string) (string, error) {
	name := databaseFromDSN(dsn)
	switch strings.ToLower(name) {
	case "autonomy":
		return "", fmt.Errorf("knowledge-base must use its own database, not autonomy")
	case "", "postgres", "template0", "template1":
		name = DefaultDatabase
	}
	if !validDatabaseName(name) {
		return "", fmt.Errorf("postgres database name %q is not a simple identifier", name)
	}
	return name, nil
}

func validDatabaseName(name string) bool {
	if name == "" || len(name) > 63 {
		return false
	}
	for i, r := range name {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r == '_' {
			continue
		}
		if i > 0 && r >= '0' && r <= '9' {
			continue
		}
		return false
	}
	return true
}

func databaseFromDSN(dsn string) string {
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		u, err := url.Parse(dsn)
		if err == nil {
			return strings.TrimPrefix(u.Path, "/")
		}
	}
	if m := regexpDbname.FindStringSubmatch(dsn); len(m) == 2 {
		return m[1]
	}
	return ""
}

func withDatabase(dsn, name string) string {
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		u, err := url.Parse(dsn)
		if err == nil {
			u.Path = "/" + name
			return u.String()
		}
	}
	if strings.Contains(dsn, "dbname=") {
		return regexpDbnameReplace.ReplaceAllString(dsn, "dbname="+name)
	}
	return strings.TrimSpace(dsn) + " dbname=" + name
}

var (
	regexpDbname        = regexp.MustCompile(`(?:^|[?&\s])dbname=([^\s&]+)`)
	regexpDbnameReplace = regexp.MustCompile(`dbname=\S+`)
)

func ensureDatabase(appDSN, name string) error {
	adminDSN := withDatabase(appDSN, "postgres")
	db, err := sql.Open("pgx", adminDSN)
	if err != nil {
		return fmt.Errorf("create database %s: %w", name, err)
	}
	defer func() { _ = db.Close() }()
	db.SetMaxOpenConns(1)
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("create database %s: %w", name, err)
	}
	var exists bool
	if err := db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)`, name).Scan(&exists); err != nil {
		return fmt.Errorf("create database %s: %w", name, err)
	}
	if exists {
		return nil
	}
	if _, err := db.ExecContext(ctx, `CREATE DATABASE `+quoteIdent(name)); err != nil {
		return fmt.Errorf("create database %s: %w", name, err)
	}
	return nil
}

func quoteIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

const postgresSchema = `
CREATE TABLE IF NOT EXISTS knowledge (
  id         TEXT PRIMARY KEY,
  name       TEXT NOT NULL DEFAULT '',
  kind       TEXT NOT NULL,
  provider   TEXT NOT NULL DEFAULT '',
  shape      TEXT NOT NULL DEFAULT '',
  source     TEXT NOT NULL DEFAULT '{}',
  suffixes   TEXT NOT NULL DEFAULT '[]',
  access     TEXT NOT NULL DEFAULT '{}',
  metadata   TEXT NOT NULL DEFAULT '{}',
  revision   TEXT NOT NULL DEFAULT '',
  checksum   TEXT NOT NULL DEFAULT '',
  documents  INTEGER NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS bindings (
  knowledge_id TEXT NOT NULL,
  entity_type  TEXT NOT NULL,
  entity_id    TEXT NOT NULL,
  PRIMARY KEY (knowledge_id, entity_type, entity_id)
);
CREATE INDEX IF NOT EXISTS idx_bindings_entity ON bindings(entity_type, entity_id);

CREATE TABLE IF NOT EXISTS documents (
  id           TEXT PRIMARY KEY,
  knowledge_id TEXT NOT NULL,
  name         TEXT NOT NULL DEFAULT '',
  path         TEXT NOT NULL DEFAULT '',
  revision     TEXT NOT NULL DEFAULT '',
  checksum     TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_documents_knowledge ON documents(knowledge_id);

CREATE TABLE IF NOT EXISTS sections (
  id           TEXT PRIMARY KEY,
  document_id  TEXT NOT NULL,
  knowledge_id TEXT NOT NULL,
  heading      TEXT NOT NULL DEFAULT '',
  path         TEXT NOT NULL DEFAULT '',
  content      TEXT NOT NULL DEFAULT '',
  ord          INTEGER NOT NULL DEFAULT 0,
  start_line   INTEGER NOT NULL DEFAULT 0,
  end_line     INTEGER NOT NULL DEFAULT 0,
  search       tsvector GENERATED ALWAYS AS (
    setweight(to_tsvector('simple', coalesce(heading, '')), 'A') ||
    setweight(to_tsvector('simple', coalesce(path, '')), 'B') ||
    setweight(to_tsvector('simple', coalesce(content, '')), 'C')
  ) STORED
);
CREATE INDEX IF NOT EXISTS idx_sections_document ON sections(document_id);
CREATE INDEX IF NOT EXISTS idx_sections_knowledge ON sections(knowledge_id);
CREATE INDEX IF NOT EXISTS idx_sections_search ON sections USING GIN(search);
`

func (s *PostgresStore) migrate(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, postgresSchema); err != nil {
		return fmt.Errorf("migrate postgres schema: %w", err)
	}
	return nil
}

func (s *PostgresStore) SaveKnowledge(ctx context.Context, k Knowledge) error {
	source, err := json.Marshal(k.Source)
	if err != nil {
		return err
	}
	suffixes, err := json.Marshal(k.Suffixes)
	if err != nil {
		return err
	}
	if suffixes == nil {
		suffixes = []byte("[]")
	}
	access, err := json.Marshal(k.Access)
	if err != nil {
		return err
	}
	metadata, err := json.Marshal(k.Metadata)
	if err != nil {
		return err
	}
	if metadata == nil {
		metadata = []byte("{}")
	}
	_, err = s.db.ExecContext(ctx, `
INSERT INTO knowledge (id, name, kind, provider, shape, source, suffixes, access, metadata, revision, checksum, documents, created_at, updated_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
ON CONFLICT (id) DO UPDATE SET
  name = EXCLUDED.name, kind = EXCLUDED.kind, provider = EXCLUDED.provider,
  shape = EXCLUDED.shape, source = EXCLUDED.source, suffixes = EXCLUDED.suffixes,
  access = EXCLUDED.access, metadata = EXCLUDED.metadata, revision = EXCLUDED.revision,
  checksum = EXCLUDED.checksum, documents = EXCLUDED.documents, updated_at = EXCLUDED.updated_at
`, k.ID, k.Name, k.Kind, k.Provider, k.Shape, string(source), string(suffixes), string(access),
		string(metadata), k.Revision, k.Checksum, k.Documents, k.CreatedAt, k.UpdatedAt)
	return err
}

func (s *PostgresStore) GetKnowledge(ctx context.Context, id string) (*Knowledge, error) {
	k, err := scanKnowledge(s.db.QueryRowContext(ctx, knowledgeSelect+` WHERE id = $1`, id))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &k, nil
}

func (s *PostgresStore) ListKnowledge(ctx context.Context) ([]Knowledge, error) {
	rows, err := s.db.QueryContext(ctx, knowledgeSelect+` ORDER BY name, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Knowledge{}
	for rows.Next() {
		k, err := scanKnowledge(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

const knowledgeSelect = `
SELECT id, name, kind, provider, shape, source, suffixes, access, metadata, revision, checksum, documents, created_at, updated_at
FROM knowledge`

type rowScanner interface {
	Scan(dest ...any) error
}

func scanKnowledge(row rowScanner) (Knowledge, error) {
	var (
		k                          Knowledge
		source, suffixes, access, metadata string
	)
	err := row.Scan(&k.ID, &k.Name, &k.Kind, &k.Provider, &k.Shape, &source, &suffixes, &access, &metadata,
		&k.Revision, &k.Checksum, &k.Documents, &k.CreatedAt, &k.UpdatedAt)
	if err != nil {
		return Knowledge{}, err
	}
	_ = json.Unmarshal([]byte(source), &k.Source)
	_ = json.Unmarshal([]byte(suffixes), &k.Suffixes)
	_ = json.Unmarshal([]byte(access), &k.Access)
	_ = json.Unmarshal([]byte(metadata), &k.Metadata)
	return k, nil
}

func (s *PostgresStore) DeleteKnowledge(ctx context.Context, id string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `DELETE FROM sections WHERE knowledge_id = $1`, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM documents WHERE knowledge_id = $1`, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM bindings WHERE knowledge_id = $1`, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM knowledge WHERE id = $1`, id); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *PostgresStore) Bind(ctx context.Context, knowledgeID string, entity ContextEntity) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO bindings (knowledge_id, entity_type, entity_id) VALUES ($1,$2,$3)
ON CONFLICT (knowledge_id, entity_type, entity_id) DO NOTHING
`, knowledgeID, entity.Type, entity.ID)
	return err
}

func (s *PostgresStore) Unbind(ctx context.Context, knowledgeID string, entity ContextEntity) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM bindings WHERE knowledge_id = $1 AND entity_type = $2 AND entity_id = $3`,
		knowledgeID, entity.Type, entity.ID)
	return err
}

func (s *PostgresStore) BindingsByKnowledge(ctx context.Context, knowledgeID string) ([]ContextEntity, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT entity_type, entity_id FROM bindings WHERE knowledge_id = $1 ORDER BY entity_type, entity_id
`, knowledgeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ContextEntity
	for rows.Next() {
		var e ContextEntity
		if err := rows.Scan(&e.Type, &e.ID); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *PostgresStore) KnowledgeIDsByEntity(ctx context.Context, entity ContextEntity) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT knowledge_id FROM bindings WHERE entity_type = $1 AND entity_id = $2 ORDER BY knowledge_id
`, entity.Type, entity.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

func (s *PostgresStore) ListBoundEntities(ctx context.Context) ([]ContextEntity, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT DISTINCT entity_type, entity_id FROM bindings ORDER BY entity_type, entity_id
`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ContextEntity
	for rows.Next() {
		var e ContextEntity
		if err := rows.Scan(&e.Type, &e.ID); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *PostgresStore) ReplaceDocuments(ctx context.Context, knowledgeID string, docs []Document) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	keep := make([]string, 0, len(docs))
	for _, d := range docs {
		keep = append(keep, d.ID)
	}
	if len(keep) == 0 {
		if _, err := tx.ExecContext(ctx, `DELETE FROM sections WHERE knowledge_id = $1`, knowledgeID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM documents WHERE knowledge_id = $1`, knowledgeID); err != nil {
			return err
		}
		return tx.Commit()
	}
	if _, err := tx.ExecContext(ctx, `
DELETE FROM sections WHERE document_id IN (
  SELECT id FROM documents WHERE knowledge_id = $1 AND NOT (id = ANY($2))
)`, knowledgeID, keep); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM documents WHERE knowledge_id = $1 AND NOT (id = ANY($2))`, knowledgeID, keep); err != nil {
		return err
	}
	for _, d := range docs {
		if _, err := tx.ExecContext(ctx, `
INSERT INTO documents (id, knowledge_id, name, path, revision, checksum)
VALUES ($1,$2,$3,$4,$5,$6)
ON CONFLICT (id) DO UPDATE SET
  knowledge_id = EXCLUDED.knowledge_id, name = EXCLUDED.name, path = EXCLUDED.path,
  revision = EXCLUDED.revision, checksum = EXCLUDED.checksum
`, d.ID, knowledgeID, d.Name, d.Path, d.Revision, d.Checksum); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *PostgresStore) ListDocuments(ctx context.Context, knowledgeID string) ([]Document, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, knowledge_id, name, path, revision, checksum FROM documents WHERE knowledge_id = $1 ORDER BY path, id
`, knowledgeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Document{}
	for rows.Next() {
		var d Document
		if err := rows.Scan(&d.ID, &d.KnowledgeID, &d.Name, &d.Path, &d.Revision, &d.Checksum); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *PostgresStore) ReplaceSections(ctx context.Context, documentID string, sections []Section) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `DELETE FROM sections WHERE document_id = $1`, documentID); err != nil {
		return err
	}
	for _, sec := range sections {
		if _, err := tx.ExecContext(ctx, `
INSERT INTO sections (id, document_id, knowledge_id, heading, path, content, ord, start_line, end_line)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
`, sec.ID, documentID, sec.KnowledgeID, sec.Heading, sec.Path, sec.Content, sec.Order, sec.StartLine, sec.EndLine); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *PostgresStore) ListSections(ctx context.Context, documentID string) ([]Section, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, document_id, knowledge_id, heading, path, content, ord, start_line, end_line
FROM sections WHERE document_id = $1 ORDER BY ord, id
`, documentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Section{}
	for rows.Next() {
		sct, err := scanSection(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, sct)
	}
	return out, rows.Err()
}

func (s *PostgresStore) GetSection(ctx context.Context, id string) (*Section, error) {
	sct, err := scanSection(s.db.QueryRowContext(ctx, `
SELECT id, document_id, knowledge_id, heading, path, content, ord, start_line, end_line
FROM sections WHERE id = $1
`, id))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &sct, nil
}

func scanSection(row rowScanner) (Section, error) {
	var s Section
	err := row.Scan(&s.ID, &s.DocumentID, &s.KnowledgeID, &s.Heading, &s.Path, &s.Content, &s.Order, &s.StartLine, &s.EndLine)
	return s, err
}

func (s *PostgresStore) Search(ctx context.Context, req SearchRequest, knowledgeIDs []string) ([]SearchResult, error) {
	if len(knowledgeIDs) == 0 {
		return []SearchResult{}, nil
	}
	q := parseQuery(req.Query)
	if q.empty() {
		return []SearchResult{}, nil
	}
	limit := req.Limit
	if limit <= 0 {
		limit = DefaultSearchLimit
	}
	if limit > MaxSearchLimit {
		limit = MaxSearchLimit
	}

	args := make([]any, 0, 6)
	n := 1
	scoreParts := []string{"0::float8"}
	var filters []string
	if len(q.Words) > 0 {
		tsq := strings.Join(q.Words, " | ")
		args = append(args, tsq)
		w := n
		n++
		filters = append(filters, fmt.Sprintf("s.search @@ to_tsquery('simple', $%d)", w))
		scoreParts = append(scoreParts,
			fmt.Sprintf("ts_rank(s.search, to_tsquery('simple', $%d))", w),
			fmt.Sprintf("(CASE WHEN to_tsvector('simple', s.heading) @@ to_tsquery('simple', $%d) THEN 1 ELSE 0 END)", w),
		)
	}
	if len(q.Files) > 0 {
		args = append(args, q.Files)
		f := n
		n++
		filters = append(filters, fmt.Sprintf(
			`EXISTS (SELECT 1 FROM unnest($%d::text[]) AS f(name) WHERE strpos(lower(d.name), f.name) > 0 OR strpos(lower(coalesce(d.path, '')), f.name) > 0)`,
			f))
		scoreParts = append(scoreParts, fmt.Sprintf(
			`(CASE WHEN lower(regexp_replace(d.name, '.*/', '')) = ANY($%d) OR lower(regexp_replace(coalesce(d.path, ''), '.*/', '')) = ANY($%d) THEN 12 + (CASE WHEN s.ord <= 1 THEN 2 ELSE 0 END) WHEN EXISTS (SELECT 1 FROM unnest($%d::text[]) AS f(name) WHERE strpos(lower(d.name), f.name) > 0 OR strpos(lower(coalesce(d.path, '')), f.name) > 0) THEN 8 ELSE 0 END)`,
			f, f, f))
	}
	args = append(args, knowledgeIDs)
	kid := n
	n++
	args = append(args, limit)
	lim := n
	sqlText := fmt.Sprintf(`
SELECT s.id, s.document_id, s.knowledge_id, s.heading, s.content,
       d.name, COALESCE(NULLIF(d.revision, ''), k.revision),
       %s
FROM sections s
JOIN documents d ON d.id = s.document_id
JOIN knowledge k ON k.id = s.knowledge_id
WHERE s.knowledge_id = ANY($%d)
  AND (%s)
ORDER BY 8 DESC, d.name ASC, s.id ASC
LIMIT $%d
`, strings.Join(scoreParts, " + "), kid, strings.Join(filters, " OR "), lim)
	rows, err := s.db.QueryContext(ctx, sqlText, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SearchResult{}
	terms := q.terms()
	for rows.Next() {
		var r SearchResult
		if err := rows.Scan(&r.SectionID, &r.DocumentID, &r.KnowledgeID, &r.Heading, &r.Snippet, &r.DocumentName, &r.Revision, &r.Score); err != nil {
			return nil, err
		}
		r.Snippet = snippet(r.Snippet, r.Heading, terms, 160)
		out = append(out, r)
	}
	return out, rows.Err()
}

var _ Store = (*PostgresStore)(nil)
