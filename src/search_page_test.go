package kb

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSearchPageIsServed(t *testing.T) {
	h := NewServer(NewService(nil)).Handler()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/search", nil))
	if rec.Code != http.StatusOK || !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/html") {
		t.Fatalf("GET /search = %d %q", rec.Code, rec.Header().Get("Content-Type"))
	}
	if rec.Body.String() != searchHTML {
		t.Fatal("GET /search should serve the search page")
	}
}

func TestSearchPageListsBoundContexts(t *testing.T) {
	for _, want := range []string{
		`href="/"`, `href="/search"`,
		`id="ctx-list"`, `id="ctx-main"`, `id="tree"`, `id="f-query"`, `id="results"`,
		`"/api/contexts"`, `"/browse"`, `"/search?q="`, `"/api/sections/"`,
		`function loadContexts()`, `Bound contexts`, `Indexed data`,
	} {
		if !strings.Contains(searchHTML, want) {
			t.Errorf("search page should contain %q", want)
		}
	}
	if strings.Contains(searchHTML, "project_id") {
		t.Error("search page must not use project_id")
	}
	if strings.Contains(searchHTML, "http://") || strings.Contains(searchHTML, "https://") {
		t.Error("search page should only call relative paths")
	}
}
