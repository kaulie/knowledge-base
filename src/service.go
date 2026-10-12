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
	Metadata map[string]string `json:"metadata,omitempty"`
	Bind     *ContextEntity    `json:"bind,omitempty"`
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
	src := req.Source
	loc := strings.TrimSpace(src.Location)
	if loc == "" {
		loc = strings.TrimSpace(src.Repository)
	}
	if loc == "" {
		if backend.Kind() == KindSourceCodeRepository {
			return Knowledge{}, Errorf(ErrInvalidQuery, "git repository is required (source.repository or source.location)")
		}
		return Knowledge{}, Errorf(ErrInvalidQuery, "source.location is required")
	}
	src.Location = loc
	if backend.Kind() == KindSourceCodeRepository {
		if strings.TrimSpace(src.Repository) == "" {
			src.Repository = loc
		}
		cleaned, asDir := normalizeGitPath(src.Path)
		src.Path = cleaned
		if asDir {
			shape = ShapeDirectory
		}
		if !validRepoRelPath(src.Path) {
			return Knowledge{}, Errorf(ErrInvalidQuery, "source.path is from the repository root (docs, docs/*, docs/x.md); it must stay inside the tree")
		}
		if shape == ShapeFile && src.Path == "" {
			return Knowledge{}, Errorf(ErrInvalidQuery, "source.path from the repository root is required for shape file")
		}
	}
	now := s.now()
	k := Knowledge{
		ID:        strings.TrimSpace(req.ID),
		Name:      strings.TrimSpace(req.Name),
		Kind:      backend.Kind(),
		Provider:  backend.Provider(),
		Shape:     shape,
		Source:    src,
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
		if backend.Kind() == KindSourceCodeRepository {
			k.Name = repoDisplayName(loc)
		} else {
			k.Name = filepath.Base(loc)
		}
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
	if entity, ok, err := entityFromBind(req.Bind); err != nil {
		return Knowledge{}, err
	} else if ok {
		synced, err := s.Bind(ctx, k.ID, entity)
		if err != nil {
			return Knowledge{}, err
		}
		fresh, err := s.Get(ctx, k.ID)
		if err != nil {
			return Knowledge{}, err
		}
		fresh.LastSync = synced
		return *fresh, nil
	}
	s.attachBindings(ctx, &k)
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
	s.attachBindings(ctx, k)
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
	s.attachBindingsAll(ctx, out)
	return out, nil
}

func (s *Service) attachBindings(ctx context.Context, k *Knowledge) {
	if k == nil {
		return
	}
	ents, err := s.store.BindingsByKnowledge(ctx, k.ID)
	if err != nil || ents == nil {
		k.Bindings = []ContextEntity{}
		return
	}
	k.Bindings = ents
}

func (s *Service) attachBindingsAll(ctx context.Context, items []Knowledge) {
	for i := range items {
		s.attachBindings(ctx, &items[i])
	}
}

func (s *Service) Delete(ctx context.Context, id string) error {
	k, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	if k.Kind == KindSourceCodeRepository {
		removeWorkTree(k.ID)
	}
	if err := s.store.DeleteKnowledge(ctx, strings.TrimSpace(id)); err != nil {
		return Wrap(ErrIndexFailed, err, "delete knowledge %q", id)
	}
	return nil
}

func (s *Service) Bind(ctx context.Context, knowledgeID string, entity ContextEntity) (*SyncResult, error) {
	k, err := s.Get(ctx, knowledgeID)
	if err != nil {
		return nil, err
	}
	e, err := ParseEntity(entity.Type, entity.ID)
	if err != nil {
		return nil, err
	}
	var synced *SyncResult
	if k.Kind == KindSourceCodeRepository {
		out, err := s.Sync(ctx, k.ID)
		if err != nil {
			return &out, err
		}
		synced = &out
	}
	if err := s.store.Bind(ctx, strings.TrimSpace(knowledgeID), e); err != nil {
		return synced, err
	}
	return synced, nil
}

func (s *Service) Unbind(ctx context.Context, knowledgeID string, entity ContextEntity) error {
	if _, err := s.Get(ctx, knowledgeID); err != nil {
		return err
	}
	e, err := ParseEntity(entity.Type, entity.ID)
	if err != nil {
		return err
	}
	return s.store.Unbind(ctx, strings.TrimSpace(knowledgeID), e)
}

func (s *Service) Bindings(ctx context.Context, knowledgeID string) ([]ContextEntity, error) {
	if _, err := s.Get(ctx, knowledgeID); err != nil {
		return nil, err
	}
	return s.store.BindingsByKnowledge(ctx, strings.TrimSpace(knowledgeID))
}

func (s *Service) ListForEntity(ctx context.Context, entity ContextEntity) ([]Knowledge, error) {
	e, err := ParseEntity(entity.Type, entity.ID)
	if err != nil {
		return nil, err
	}
	ids, err := s.store.KnowledgeIDsByEntity(ctx, e)
	if err != nil {
		return nil, Wrap(ErrIndexFailed, err, "bindings for %s", e.Key())
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
	s.attachBindingsAll(ctx, out)
	s.attachFiles(ctx, out)
	return out, nil
}

const maxListedFiles = 100

func (s *Service) attachFiles(ctx context.Context, items []Knowledge) {
	for i := range items {
		docs, err := s.store.ListDocuments(ctx, items[i].ID)
		if err != nil || len(docs) == 0 {
			continue
		}
		n := len(docs)
		if n > maxListedFiles {
			n = maxListedFiles
		}
		files := make([]string, 0, n)
		for _, d := range docs[:n] {
			p := strings.TrimSpace(d.Path)
			if p == "" {
				p = d.Name
			}
			if p != "" {
				files = append(files, p)
			}
		}
		items[i].Files = files
	}
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

func (s *Service) ListContexts(ctx context.Context) ([]ContextSummary, error) {
	ents, err := s.store.ListBoundEntities(ctx)
	if err != nil {
		return nil, Wrap(ErrIndexFailed, err, "list bound contexts")
	}
	out := make([]ContextSummary, 0, len(ents))
	for _, e := range ents {
		ids, err := s.store.KnowledgeIDsByEntity(ctx, e)
		if err != nil {
			return nil, err
		}
		var docs, secs int
		for _, id := range ids {
			dlist, err := s.store.ListDocuments(ctx, id)
			if err != nil {
				return nil, err
			}
			docs += len(dlist)
			for _, d := range dlist {
				slist, err := s.store.ListSections(ctx, d.ID)
				if err != nil {
					return nil, err
				}
				secs += len(slist)
			}
		}
		out = append(out, ContextSummary{
			Type: e.Type, ID: e.ID, Knowledge: len(ids), Documents: docs, Sections: secs,
		})
	}
	return out, nil
}

func (s *Service) Browse(ctx context.Context, entity ContextEntity) (BrowseResult, error) {
	items, err := s.ListForEntity(ctx, entity)
	if err != nil {
		return BrowseResult{}, err
	}
	e, err := ParseEntity(entity.Type, entity.ID)
	if err != nil {
		return BrowseResult{}, err
	}
	out := BrowseResult{Entity: e, Knowledge: []BrowseKnowledge{}}
	for _, k := range items {
		docs, err := s.Documents(ctx, k.ID)
		if err != nil {
			return BrowseResult{}, err
		}
		files := make([]BrowseFile, 0, len(docs))
		for _, d := range docs {
			secs, err := s.store.ListSections(ctx, d.ID)
			if err != nil {
				return BrowseResult{}, err
			}
			heads := make([]BrowseSection, 0, len(secs))
			for _, sec := range secs {
				heads = append(heads, BrowseSection{ID: sec.ID, Heading: sec.Heading})
			}
			files = append(files, BrowseFile{ID: d.ID, Name: d.Name, Path: d.Path, Sections: heads})
		}
		out.Knowledge = append(out.Knowledge, BrowseKnowledge{
			ID: k.ID, Name: k.Name, Kind: k.Kind, Provider: k.Provider,
			Revision: k.Revision, Documents: len(docs), Files: files,
		})
	}
	return out, nil
}

func (s *Service) Sync(ctx context.Context, knowledgeID string) (SyncResult, error) {
	lg := newSyncLogger(s.now)
	ctx = withSyncLog(ctx, lg)
	fail := func(id string, err error) (SyncResult, error) {
		note(ctx, "error", "%s", err.Error())
		return SyncResult{KnowledgeID: id, Logs: lg.entries()}, err
	}

	k, err := s.Get(ctx, knowledgeID)
	if err != nil {
		return fail(knowledgeID, err)
	}
	note(ctx, "info", "start %s kind=%s provider=%s shape=%s", k.ID, k.Kind, k.Provider, k.Shape)
	if k.Kind == KindSourceCodeRepository {
		note(ctx, "info", "git repository %s", gitRemote(k.Source))
		if k.Source.Ref != "" {
			note(ctx, "info", "ref %s", k.Source.Ref)
		}
		if k.Source.Path != "" {
			note(ctx, "info", "path from repository root: %s", k.Source.Path)
		}
	} else if loc := strings.TrimSpace(k.Source.Location); loc != "" {
		note(ctx, "info", "location %s", loc)
	}

	backend, err := ResolveBackend(k.Kind, k.Provider)
	if err != nil {
		return fail(k.ID, err)
	}
	mat, err := backend.Materialize(ctx, *k)
	if err != nil {
		return fail(k.ID, err)
	}
	if mat.Revision != "" {
		note(ctx, "info", "work tree ready revision %s", mat.Revision)
	}

	var files []ListedFile
	root := strings.TrimSpace(mat.Root)
	single := k.Shape == ShapeFile
	if k.Kind == KindSourceCodeRepository {
		spec, asDir := normalizeGitPath(k.Source.Path)
		if asDir || strings.ContainsAny(spec, "*?[") {
			single = false
			if spec != strings.TrimSpace(k.Source.Path) {
				note(ctx, "info", "path %s is a directory glob, using %s", k.Source.Path, spec)
			}
		}
	}
	if single {
		rel := strings.TrimSpace(k.Source.Path)
		if k.Kind == KindSourceCodeRepository {
			rel, _ = normalizeGitPath(rel)
		}
		if rel == "" {
			rel = filepath.Base(strings.TrimSpace(k.Source.Location))
		}
		files = []ListedFile{{Name: filepath.Base(rel), Location: root, Path: rel}}
		note(ctx, "info", "single file %s", rel)
	} else {
		files, err = backend.List(ctx, *k, root)
		if err != nil {
			return fail(k.ID, err)
		}
		if len(files) == 0 {
			return fail(k.ID, Errorf(ErrInvalidQuery, "no documents under %q", root))
		}
		note(ctx, "info", "listed %d documents", len(files))
	}
	docs := make([]Document, 0, len(files))
	var sectionsN int
	var digest strings.Builder
	var revision string
	if mat.Revision != "" {
		revision = mat.Revision
	}
	for _, f := range files {
		rel := f.Path
		if rel == "" {
			rel = f.Name
		}
		content, rev, err := loadFile(ctx, *k, root, rel)
		if err != nil {
			return fail(k.ID, err)
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
			return fail(k.ID, err)
		}
		for i := range sections {
			sections[i].KnowledgeID = k.ID
		}
		if err := s.store.ReplaceSections(ctx, docID, sections); err != nil {
			return fail(k.ID, Wrap(ErrIndexFailed, err, "index %q", docID))
		}
		sectionsN += len(sections)
		docs = append(docs, doc)
		note(ctx, "info", "parse %s (%d sections, %d bytes)", rel, len(sections), len(content))
	}
	if err := s.store.ReplaceDocuments(ctx, k.ID, docs); err != nil {
		return fail(k.ID, Wrap(ErrIndexFailed, err, "replace documents for %q", k.ID))
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
		return fail(k.ID, Wrap(ErrIndexFailed, err, "record revision for %q", k.ID))
	}
	note(ctx, "info", "done status=%s documents=%d sections=%d revision=%s", status, len(docs), sectionsN, k.Revision)
	return SyncResult{
		KnowledgeID: k.ID, Status: status, Revision: k.Revision, Checksum: checksum,
		Documents: len(docs), Sections: sectionsN, Logs: lg.entries(),
	}, nil
}

func (s *Service) Search(ctx context.Context, req SearchRequest) ([]SearchResult, error) {
	entity, err := ParseEntity(req.Entity.Type, req.Entity.ID)
	if err != nil {
		return nil, err
	}
	req.Entity = entity
	if len(queryTerms(req.Query)) == 0 {
		return nil, Errorf(ErrInvalidQuery, "search query %q has no searchable terms", req.Query)
	}
	if req.Limit <= 0 {
		req.Limit = DefaultSearchLimit
	}
	if req.Limit > MaxSearchLimit {
		req.Limit = MaxSearchLimit
	}
	ids, err := s.store.KnowledgeIDsByEntity(ctx, entity)
	if err != nil {
		return nil, Wrap(ErrIndexFailed, err, "bindings for %s", entity.Key())
	}
	results, err := s.store.Search(ctx, req, ids)
	if err != nil {
		return nil, Wrap(ErrIndexFailed, err, "search %s", entity.Key())
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
