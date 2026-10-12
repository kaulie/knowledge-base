package kb

import (
	"context"
	"sort"
	"sync"
)

type Store interface {
	SaveKnowledge(ctx context.Context, k Knowledge) error
	GetKnowledge(ctx context.Context, id string) (*Knowledge, error)
	ListKnowledge(ctx context.Context) ([]Knowledge, error)
	DeleteKnowledge(ctx context.Context, id string) error

	Bind(ctx context.Context, knowledgeID string, entity ContextEntity) error
	Unbind(ctx context.Context, knowledgeID string, entity ContextEntity) error
	BindingsByKnowledge(ctx context.Context, knowledgeID string) ([]ContextEntity, error)
	KnowledgeIDsByEntity(ctx context.Context, entity ContextEntity) ([]string, error)

	ReplaceDocuments(ctx context.Context, knowledgeID string, docs []Document) error
	ListDocuments(ctx context.Context, knowledgeID string) ([]Document, error)

	ReplaceSections(ctx context.Context, documentID string, sections []Section) error
	GetSection(ctx context.Context, id string) (*Section, error)
	Search(ctx context.Context, req SearchRequest, knowledgeIDs []string) ([]SearchResult, error)
}

type MemoryStore struct {
	mu        sync.RWMutex
	knowledge map[string]Knowledge
	bindings  map[string]map[string]ContextEntity // knowledgeID -> entity.Key()
	documents map[string][]Document      // knowledgeID
	sections  map[string][]Section       // documentID
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		knowledge: make(map[string]Knowledge),
		bindings:  make(map[string]map[string]ContextEntity),
		documents: make(map[string][]Document),
		sections:  make(map[string][]Section),
	}
}

func (m *MemoryStore) SaveKnowledge(_ context.Context, k Knowledge) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.knowledge[k.ID] = k
	return nil
}

func (m *MemoryStore) GetKnowledge(_ context.Context, id string) (*Knowledge, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	k, ok := m.knowledge[id]
	if !ok {
		return nil, nil
	}
	c := k
	return &c, nil
}

func (m *MemoryStore) ListKnowledge(_ context.Context) ([]Knowledge, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]Knowledge, 0, len(m.knowledge))
	for _, k := range m.knowledge {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

func (m *MemoryStore) DeleteKnowledge(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, d := range m.documents[id] {
		delete(m.sections, d.ID)
	}
	delete(m.knowledge, id)
	delete(m.bindings, id)
	delete(m.documents, id)
	return nil
}

func (m *MemoryStore) Bind(_ context.Context, knowledgeID string, entity ContextEntity) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.bindings[knowledgeID] == nil {
		m.bindings[knowledgeID] = map[string]ContextEntity{}
	}
	m.bindings[knowledgeID][entity.Key()] = entity
	return nil
}

func (m *MemoryStore) Unbind(_ context.Context, knowledgeID string, entity ContextEntity) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.bindings[knowledgeID], entity.Key())
	return nil
}

func (m *MemoryStore) BindingsByKnowledge(_ context.Context, knowledgeID string) ([]ContextEntity, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []ContextEntity
	for _, e := range m.bindings[knowledgeID] {
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Type != out[j].Type {
			return out[i].Type < out[j].Type
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

func (m *MemoryStore) KnowledgeIDsByEntity(_ context.Context, entity ContextEntity) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	key := entity.Key()
	var out []string
	for kid, entities := range m.bindings {
		if _, ok := entities[key]; ok {
			out = append(out, kid)
		}
	}
	sort.Strings(out)
	return out, nil
}

func (m *MemoryStore) ReplaceDocuments(_ context.Context, knowledgeID string, docs []Document) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	keep := map[string]bool{}
	for _, d := range docs {
		keep[d.ID] = true
	}
	for _, old := range m.documents[knowledgeID] {
		if !keep[old.ID] {
			delete(m.sections, old.ID)
		}
	}
	cp := make([]Document, len(docs))
	copy(cp, docs)
	m.documents[knowledgeID] = cp
	return nil
}

func (m *MemoryStore) ListDocuments(_ context.Context, knowledgeID string) ([]Document, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	src := m.documents[knowledgeID]
	out := make([]Document, len(src))
	copy(out, src)
	return out, nil
}

func (m *MemoryStore) ReplaceSections(_ context.Context, documentID string, sections []Section) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(sections) == 0 {
		delete(m.sections, documentID)
		return nil
	}
	cp := make([]Section, len(sections))
	copy(cp, sections)
	m.sections[documentID] = cp
	return nil
}

func (m *MemoryStore) GetSection(_ context.Context, id string) (*Section, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, secs := range m.sections {
		for _, s := range secs {
			if s.ID == id {
				c := s
				return &c, nil
			}
		}
	}
	return nil, nil
}

func (m *MemoryStore) Search(_ context.Context, req SearchRequest, knowledgeIDs []string) ([]SearchResult, error) {
	terms := queryTerms(req.Query)
	allow := map[string]bool{}
	for _, id := range knowledgeIDs {
		allow[id] = true
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	var results []SearchResult
	for kid, docs := range m.documents {
		if !allow[kid] {
			continue
		}
		k := m.knowledge[kid]
		for _, d := range docs {
			for _, s := range m.sections[d.ID] {
				score := sectionScore(s, terms)
				if score <= 0 {
					continue
				}
				results = append(results, SearchResult{
					KnowledgeID:  kid,
					DocumentID:   d.ID,
					SectionID:    s.ID,
					DocumentName: d.Name,
					Heading:      s.Heading,
					Snippet:      snippet(s.Content, s.Heading, terms, 160),
					Score:        score,
					Revision:     firstNonEmpty(d.Revision, k.Revision),
				})
			}
		}
	}
	return rankResults(results, req.Limit), nil
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
