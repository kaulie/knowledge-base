package kb

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCatalogListsProjectsFromWebCursor(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/projects" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"projectId": "project-b", "name": "zeta"},
			{"projectId": "project-a", "name": "alpha"},
		})
	}))
	defer upstream.Close()

	s := NewServer(NewService(NewMemoryStore()))
	s.catalog = &Catalog{ProjectsURL: upstream.URL, Client: upstream.Client()}

	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/catalog/entities/project", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d %s", rec.Code, rec.Body.String())
	}
	var out CatalogList
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if !out.Available || out.EntityType != EntityProject || len(out.Entities) != 2 {
		t.Fatalf("catalog = %+v", out)
	}
	if out.Entities[0].ID != "project-a" || out.Entities[0].Name != "alpha" || out.Entities[0].Type != EntityProject {
		t.Fatalf("sorted first = %+v", out.Entities[0])
	}
}

func TestCatalogUnknownTypeIsEmpty(t *testing.T) {
	s := NewServer(NewService(nil))
	s.catalog = &Catalog{}
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/catalog/entities/organization", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"available":false`) || !strings.Contains(rec.Body.String(), `"entities":[]`) {
		t.Fatalf("body = %s", rec.Body.String())
	}
}
