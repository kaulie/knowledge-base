package kb

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHTTPAutonomyReadPath(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "notes.md"), []byte("# Notes\n\n## Tick\n\nheartbeat from the runtime\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	h := NewServer(NewService(NewMemoryStore())).Handler()
	post := func(path, body string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, path, strings.NewReader(body)))
		return rec
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/backends", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "filesystem") || !strings.Contains(rec.Body.String(), `"provider":"git"`) {
		t.Fatalf("backends: %d %s", rec.Code, rec.Body.String())
	}

	rec = post("/api/knowledge", `{"name":"notes","kind":"filesystem","provider":"local","shape":"directory","source":{"location":"`+root+`"},"bind":{"type":"project","id":"alpha"}}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("register: %d %s", rec.Code, rec.Body.String())
	}
	var created Knowledge
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil || created.ID == "" {
		t.Fatalf("created = %s, %v", rec.Body.String(), err)
	}

	rec = post("/api/knowledge/"+created.ID+"/sync", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("sync: %d %s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/contexts/project/alpha/search?q=heartbeat", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "heartbeat") {
		t.Fatalf("search: %d %s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/contexts/project/beta/search?q=heartbeat", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"results":[]`) {
		t.Fatalf("other entity: %d %s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/contexts/project/alpha/knowledge", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), created.ID) {
		t.Fatalf("list: %d %s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	put := httptest.NewRequest(http.MethodPut, "/api/knowledge/"+created.ID+"/bindings/organization/org-1", nil)
	h.ServeHTTP(httptest.NewRecorder(), put)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/contexts/organization/org-1/search?q=heartbeat", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "heartbeat") {
		t.Fatalf("org search: %d %s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/contexts/not-a-type/x/search?q=heartbeat", nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("bad entity type: %d %s", rec.Code, rec.Body.String())
	}
}

func TestHTTPHealth(t *testing.T) {
	h := NewServer(NewService(nil)).Handler()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("health = %d", rec.Code)
	}
}
