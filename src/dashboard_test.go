package kb

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDashboardIsServed(t *testing.T) {
	h := NewServer(NewService(nil)).Handler()
	for _, path := range []string{"/", "/dashboard"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK || !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/html") {
			t.Fatalf("GET %s = %d %q", path, rec.Code, rec.Header().Get("Content-Type"))
		}
		if rec.Body.String() != dashboardHTML {
			t.Fatalf("GET %s should serve the dashboard", path)
		}
	}
}

func TestDashboardIsBuiltOnEntityAPIs(t *testing.T) {
	for _, want := range []string{
		`id="f-type"`, `id="f-id"`, `id="f-id-select"`, `id="f-type-custom"`,
		`function loadCatalog()`, `"/api/catalog/entities/project"`,
		`value="project"`, `value="organization"`, `value="service"`, `value="environment"`, `value="task"`,
		`id="f-scope"`, `value="all"`, `value="entity"`,
		`id="rows"`, `id="f-filter"`, `tr class="group"`,
		`id="register"`, `id="f-kind"`, `id="f-provider"`,
		`value="filesystem"`, `value="source_code_repository"`, `value="cloud"`,
		`id="f-shape"`, `value="file"`, `value="directory"`, `recursive collection`,
		`id="f-name"`, `id="f-location"`, `id="f-suffixes"`, `id="f-path"`, `id="f-ref"`, `id="f-sync"`, `id="f-bind"`,
		`git@host:org/repo.git`, `Git repository address is required`,
		`path from repository root`, `Leading slash is optional`,
		`body.bind = ent`, `body.source.repository = location`,
		`id="search"`, `id="f-query"`, `id="results"`, `id="section"`,
		`async function api(method, path, body)`,
		`api("GET", "/api/backends")`,
		`"/api/knowledge?entity_type="`,
		`api("POST", "/api/knowledge", body)`,
		`"/api/knowledge/" + encodeURIComponent(syncBtn.dataset.sync) + "/sync"`,
		`"/api/knowledge/" + encodeURIComponent(id) + "/bindings/"`,
		`api("DELETE", "/api/knowledge/" + encodeURIComponent(id))`,
		`"/api/contexts/" + encodeURIComponent(ent.type) + "/"`,
		`"/api/sections/" + encodeURIComponent(hit.dataset.section)`,
		`confirm("Delete source "`, `The remote is not touched.`,
		`function currentEntity()`,
	} {
		if !strings.Contains(dashboardHTML, want) {
			t.Errorf("dashboard should contain %q", want)
		}
	}
	if strings.Contains(dashboardHTML, "project_id") {
		t.Error("dashboard must not use project_id; project is one entity type")
	}
	if strings.Contains(dashboardHTML, "http://") || strings.Contains(dashboardHTML, "https://") {
		t.Error("dashboard should only call relative paths and load no external asset")
	}
	if strings.Contains(dashboardHTML, "git-local") {
		t.Error("source_code_repository must clone via git, not git-local")
	}
}
