package kb

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"
)

const (
	EnvProjectsAPIURL      = "PROJECTS_API_URL"
	DefaultProjectsAPIURL  = "http://127.0.0.1:4211"
	defaultCatalogTimeout  = 5 * time.Second
)

// CatalogEntity is one option the dashboard can pick as a CONTEXT entity.
// Core bind/search APIs still take {type, id}; this is only a UI catalogue.
type CatalogEntity struct {
	Type string `json:"type"`
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
}

type CatalogList struct {
	EntityType string          `json:"entity_type"`
	Source     string          `json:"source,omitempty"`
	Available  bool            `json:"available"`
	Entities   []CatalogEntity `json:"entities"`
	Message    string          `json:"message,omitempty"`
}

// Catalog loads entity options from their home registries. Project comes
// from web-cursor GET /api/projects. Knowledge-base does not store projects.
type Catalog struct {
	ProjectsURL string
	Client      *http.Client
}

func NewCatalog() *Catalog {
	base := strings.TrimSpace(os.Getenv(EnvProjectsAPIURL))
	if base == "" {
		base = DefaultProjectsAPIURL
	}
	return &Catalog{
		ProjectsURL: strings.TrimRight(base, "/"),
		Client:      &http.Client{Timeout: defaultCatalogTimeout},
	}
}

func (c *Catalog) List(ctx context.Context, entityType string) CatalogList {
	typ, err := ParseEntity(entityType, "x")
	if err != nil {
		return CatalogList{EntityType: entityType, Entities: []CatalogEntity{}, Message: err.Error()}
	}
	out := CatalogList{EntityType: typ.Type, Entities: []CatalogEntity{}}
	if c == nil {
		out.Message = "catalog is not configured"
		return out
	}
	switch typ.Type {
	case EntityProject:
		return c.listProjects(ctx)
	default:
		out.Message = "no catalogue for this entity type; enter the id"
		return out
	}
}

type webCursorProject struct {
	ID   string `json:"projectId"`
	Name string `json:"name"`
}

func (c *Catalog) listProjects(ctx context.Context) CatalogList {
	out := CatalogList{EntityType: EntityProject, Source: "web-cursor", Entities: []CatalogEntity{}}
	base := strings.TrimRight(strings.TrimSpace(c.ProjectsURL), "/")
	if base == "" {
		base = DefaultProjectsAPIURL
	}
	client := c.Client
	if client == nil {
		client = &http.Client{Timeout: defaultCatalogTimeout}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/api/projects", nil)
	if err != nil {
		out.Message = err.Error()
		return out
	}
	res, err := client.Do(req)
	if err != nil {
		out.Message = "web-cursor projects unavailable: " + err.Error()
		return out
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode >= 300 {
		out.Message = "web-cursor projects returned " + res.Status
		return out
	}
	var rows []webCursorProject
	if err := json.Unmarshal(body, &rows); err != nil {
		out.Message = "web-cursor projects: invalid json"
		return out
	}
	for _, row := range rows {
		id := strings.TrimSpace(row.ID)
		if id == "" {
			continue
		}
		out.Entities = append(out.Entities, CatalogEntity{Type: EntityProject, ID: id, Name: strings.TrimSpace(row.Name)})
	}
	sort.Slice(out.Entities, func(i, j int) bool {
		if out.Entities[i].Name != out.Entities[j].Name {
			return out.Entities[i].Name < out.Entities[j].Name
		}
		return out.Entities[i].ID < out.Entities[j].ID
	})
	out.Available = true
	return out
}
