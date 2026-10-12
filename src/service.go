package kb

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"strings"
	"time"
)

const (
	DefaultSearchLimit = 10
	MaxSearchLimit     = 50
)

type Service struct {
	store Store
	now   func() time.Time
	newID func() string
}

func NewService(store Store) *Service {
	if store == nil {
		store = NewMemoryStore()
	}
	return &Service{store: store, now: time.Now, newID: mintID}
}

func mintID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "kn-" + time.Now().UTC().Format("20060102150405.000000000")
	}
	return "kn-" + hex.EncodeToString(b[:])
}

type RegisterRequest struct {
	ID            string            `json:"id,omitempty"`
	Name          string            `json:"name,omitempty"`
	Kind          string            `json:"kind"`
	Provider      string            `json:"provider,omitempty"`
	Shape         string            `json:"shape,omitempty"`
	Source        Source            `json:"source"`
	Suffixes      []string          `json:"suffixes,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
	BindProjectID string            `json:"bind_project_id,omitempty"`
}

func (s *Service) Register(ctx context.Context, req RegisterRequest) (Knowledge, error) {
	backend, err := ResolveBackend(req.Kind, req.Provider)
	if err != nil {
		return Knowledge{}, err
	}
	shape := strings.ToLower(strings.TrimSpace(req.Shape))
	if shape == "" {
		shape = ShapeDirectory
	}
	if shape != ShapeFile && shape != ShapeDirectory {
		return Knowledge{}, Errorf(ErrInvalidQuery, "shape must be file or directory")
	}
	loc := strings.TrimSpace(req.Source.Location)
	if loc == "" {
		return Knowledge{}, Errorf(ErrInvalidQuery, "source.location is required")
	}
	now := s.now()
	k := Knowledge{
		ID:        strings.TrimSpace(req.ID),
		Name:      strings.TrimSpace(req.Name),
		Kind:      backend.Kind(),
		Provider:  backend.Provider(),
		Shape:     shape,
		Source:    req.Source,
		Suffixes:  req.Suffixes,
		Access:    Access{Mode: "bound"},
		Metadata:  req.Metadata,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if k.ID == "" {
		k.ID = s.newID()
	}
	if k.Name == "" {
		k.Name = filepath.Base(loc)
	}
	if existing, _ := s.store.GetKnowledge(ctx, k.ID); existing != nil {
		k.CreatedAt = existing.CreatedAt
		k.Revision = existing.Revision
		k.Checksum = existing.Checksum
		k.Documents = existing.Documents
	}
	if err := s.store.SaveKnowledge(ctx, k); err != nil {
		return Knowledge{}, Wrap(ErrIndexFailed, err, "save knowledge %q", k.ID)
	}
	if pid := strings.TrimSpace(req.BindProjectID); pid != "" {
		if err := s.store.Bind(ctx, k.ID, pid); err != nil {
			return Knowledge{}, Wrap(ErrIndexFailed, err, "bind %q to %q", k.ID, pid)
		}
	}
	return k, nil
}

func (s *Service) Get(ctx context.Context, id string) (*Knowledge, error) {
	k, err := s.store.GetKnowledge(ctx, strings.TrimSpace(id))
	if err != nil {
		return nil, Wrap(ErrIndexFailed, err, "read knowledge %q", id)
	}
	if k == nil {
		return nil, Errorf(ErrNotFound, "knowledge %q is not registered", id)
	}
	return k, nil
}

func (s *Service) List(ctx context.Context) ([]Knowledge, error) {
	out, err := s.store.ListKnowledge(ctx)
	if err != nil {
		return nil, Wrap(ErrIndexFailed, err, "list knowledge")
	}
	if out == nil {
		out = []Knowledge{}
	}
	return out, nil
}

func (s *Service) Delete(ctx context.Context, id string) error {
	if _, err := s.Get(ctx, id); err != nil {
		return err
	}
	if err := s.store.DeleteKnowledge(ctx, strings.TrimSpace(id)); err != nil {
		return Wrap(ErrIndexFailed, err, "delete knowledge %q", id)
	}
	return nil
}

func (s *Service) Bind(ctx context.Context, knowledgeID, projectID string) error {
	if _, err := s.Get(ctx, knowledgeID); err != nil {
		return err
	}
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return Errorf(ErrInvalidQuery, "project_id is required to bind")
	}
	return s.store.Bind(ctx, strings.TrimSpace(knowledgeID), projectID)
}

func (s *Service) Unbind(ctx context.Context, knowledgeID, projectID string) error {
	if _, err := s.Get(ctx, knowledgeID); err != nil {
		return err
	}
	return s.store.Unbind(ctx, strings.TrimSpace(knowledgeID), strings.TrimSpace(projectID))
}

func (s *Service) Bindings(ctx context.Context, knowledgeID string) ([]string, error) {
	if _, err := s.Get(ctx, knowledgeID); err != nil {
		return nil, err
	}
	return s.store.BindingsByKnowledge(ctx, strings.TrimSpace(knowledgeID))
}

func (s *Service) ListForProject(ctx context.Context, projectID string) ([]Knowledge, error) {
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return nil, Errorf(ErrInvalidQuery, "project_id is required")
	}
	ids, err := s.store.KnowledgeIDsByProject(ctx, projectID)
	if err != nil {
		return nil, Wrap(ErrIndexFailed, err, "bindings for project %q", projectID)
	}
	out := make([]Knowledge, 0, len(ids))
	for _, id := range ids {
		k, err := s.store.GetKnowledge(ctx, id)
		if err != nil {
			return nil, err
		}
		if k != nil {
			out = append(out, *k)
		}
	}
	return out, nil
}

func (s *Service) Documents(ctx context.Context, knowledgeID string) ([]Document, error) {
	if _, err := s.Get(ctx, knowledgeID); err != nil {
		return nil, err
	}
	docs, err := s.store.ListDocuments(ctx, strings.TrimSpace(knowledgeID))
	if err != nil {
		return nil, Wrap(ErrIndexFailed, err, "list documents for %q", knowledgeID)
	}
	if docs == nil {
		docs = []Document{}
	}
	return docs, nil
}

func (s *Service) Sync(ctx context.Context, knowledgeID string) (SyncResult, error) {
	k, err := s.Get(ctx, knowledgeID)
	if err != nil {
		return SyncResult{}, err
	}
	backend, err := ResolveBackend(k.Kind, k.Provider)
	if err != nil {
		return SyncResult{}, err
	}
	var files []ListedFile
	root := strings.TrimSpace(k.Source.Location)
	if k.Shape == ShapeFile {
		files = []ListedFile{{Name: filepath.Base(root), Location: root, Path: strings.TrimSpace(k.Source.Path)}}
	} else {
		files, err = backend.List(ctx, root, k.Suffixes)
		if err != nil {
			return SyncResult{}, err
		}
		if len(files) == 0 {
			return SyncResult{}, Errorf(ErrInvalidQuery, "no documents under %q", root)
		}
	}
	docs := make([]Document, 0, len(files))
	var sectionsN int
	var digest strings.Builder
	var revision string
	for _, f := range files {
		rel := f.Path
		if rel == "" {
			rel = f.Name
		}
		content, rev, err := loadFile(ctx, *k, rel)
		if err != nil {
			return SyncResult{}, err
		}
		if rev != "" {
			revision = rev
		}
		sum := sha256.Sum256(content)
		checksum := hex.EncodeToString(sum[:])
		digest.WriteString(checksum)
		docID := k.ID + ":" + rel
		doc := Document{ID: docID, KnowledgeID: k.ID, Name: f.Name, Path: rel, Revision: rev, Checksum: checksum}
		sections, err := ParseMarkdown(docID, content)
		if err != nil {
			return SyncResult{}, err
		}
		for i := range sections {
			sections[i].KnowledgeID = k.ID
		}
		if err := s.store.ReplaceSections(ctx, docID, sections); err != nil {
			return SyncResult{}, Wrap(ErrIndexFailed, err, "index %q", docID)
		}
		sectionsN += len(sections)
		docs = append(docs, doc)
	}
	if err := s.store.ReplaceDocuments(ctx, k.ID, docs); err != nil {
		return SyncResult{}, Wrap(ErrIndexFailed, err, "replace documents for %q", k.ID)
	}
	all := sha256.Sum256([]byte(digest.String()))
	checksum := hex.EncodeToString(all[:])
	status := SyncIndexed
	if k.Checksum == checksum && (revision == "" || k.Revision == revision) && k.Documents == len(docs) {
		status = SyncUnchanged
	}
	k.Checksum = checksum
	if revision != "" {
		k.Revision = revision
	} else {
		k.Revision = checksum[:12]
	}
	k.Documents = len(docs)
	k.UpdatedAt = s.now()
	if err := s.store.SaveKnowledge(ctx, *k); err != nil {
		return SyncResult{}, Wrap(ErrIndexFailed, err, "record revision for %q", k.ID)
	}
	return SyncResult{
		KnowledgeID: k.ID, Status: status, Revision: k.Revision, Checksum: checksum,
		Documents: len(docs), Sections: sectionsN,
	}, nil
}

func (s *Service) Search(ctx context.Context, req SearchRequest) ([]SearchResult, error) {
	if strings.TrimSpace(req.ProjectID) == "" {
		return nil, Errorf(ErrInvalidQuery, "search requires a project_id")
	}
	if len(queryTerms(req.Query)) == 0 {
		return nil, Errorf(ErrInvalidQuery, "search query %q has no searchable terms", req.Query)
	}
	if req.Limit <= 0 {
		req.Limit = DefaultSearchLimit
	}
	if req.Limit > MaxSearchLimit {
		req.Limit = MaxSearchLimit
	}
	ids, err := s.store.KnowledgeIDsByProject(ctx, strings.TrimSpace(req.ProjectID))
	if err != nil {
		return nil, Wrap(ErrIndexFailed, err, "bindings for project %q", req.ProjectID)
	}
	results, err := s.store.Search(ctx, req, ids)
	if err != nil {
		return nil, Wrap(ErrIndexFailed, err, "search project %q", req.ProjectID)
	}
	if results == nil {
		results = []SearchResult{}
	}
	return results, nil
}

func (s *Service) GetSection(ctx context.Context, sectionID string) (*Section, error) {
	sec, err := s.store.GetSection(ctx, strings.TrimSpace(sectionID))
	if err != nil {
		return nil, Wrap(ErrIndexFailed, err, "read section %q", sectionID)
	}
	if sec == nil {
		return nil, Errorf(ErrNotFound, "section %q is not indexed", sectionID)
	}
	if k, err := s.store.GetKnowledge(ctx, sec.KnowledgeID); err == nil && k != nil {
		sec.Revision = k.Revision
		sec.Source = k.Source
	}
	return sec, nil
}
