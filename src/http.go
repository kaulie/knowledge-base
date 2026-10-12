package kb

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
)

type Server struct {
	svc *Service
}

func NewServer(svc *Service) *Server {
	if svc == nil {
		svc = NewService(nil)
	}
	return &Server{svc: svc}
}

type httpRoute struct {
	Pattern string
	Handler http.HandlerFunc
}

func (s *Server) routes() []httpRoute {
	return []httpRoute{
		{"GET /health", s.handleHealth},
		{"GET /healthz", s.handleHealthAlias},
		{"GET /api/backends", s.handleBackends},
		{"POST /api/knowledge", s.handleRegister},
		{"GET /api/knowledge", s.handleList},
		{"GET /api/knowledge/{id}", s.handleGet},
		{"DELETE /api/knowledge/{id}", s.handleDelete},
		{"POST /api/knowledge/{id}/sync", s.handleSync},
		{"GET /api/knowledge/{id}/documents", s.handleDocuments},
		{"GET /api/knowledge/{id}/bindings", s.handleListBindings},
		{"PUT /api/knowledge/{id}/bindings/{entityType}/{entityID}", s.handleBind},
		{"DELETE /api/knowledge/{id}/bindings/{entityType}/{entityID}", s.handleUnbind},
		{"GET /api/contexts/{entityType}/{entityID}/knowledge", s.handleEntityKnowledge},
		{"GET /api/contexts/{entityType}/{entityID}/search", s.handleSearch},
		{"GET /api/sections/{sectionID}", s.handleGetSection},
		{"GET /dashboard", s.handleDashboard},
		{"GET /", s.handleHome},
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	for _, r := range s.routes() {
		mux.HandleFunc(r.Pattern, r.Handler)
	}
	return mux
}

type errBody struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, err error) {
	code := CodeOf(err)
	status := http.StatusInternalServerError
	switch code {
	case ErrInvalidQuery:
		status = http.StatusBadRequest
	case ErrNotFound:
		status = http.StatusNotFound
	case ErrForbidden:
		status = http.StatusForbidden
	case ErrSourceUnavailable, ErrParseFailed:
		status = http.StatusUnprocessableEntity
	case "":
		code = ErrIndexFailed
	}
	msg := err.Error()
	if prefix := string(code) + ": "; strings.HasPrefix(msg, prefix) {
		msg = strings.TrimPrefix(msg, prefix)
	}
	writeJSON(w, status, errBody{Code: code, Message: msg})
}

// @Summary  健康检查
// @Tags     system
// @Produce  json
// @Success  200  {object}  map[string]string
// @Router   /health [get]
func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// @Summary  健康检查（/health 的别名）
// @Tags     system
// @Produce  json
// @Success  200  {object}  map[string]string
// @Router   /healthz [get]
func (s *Server) handleHealthAlias(w http.ResponseWriter, r *http.Request) {
	s.handleHealth(w, r)
}

// @Summary  已实现的 kind+provider
// @Tags     knowledge
// @Produce  json
// @Success  200  {object}  map[string]any
// @Router   /api/backends [get]
func (s *Server) handleBackends(w http.ResponseWriter, _ *http.Request) {
	type info struct {
		Kind     string `json:"kind"`
		Provider string `json:"provider"`
	}
	var out []info
	for _, b := range ImplementedBackends() {
		out = append(out, info{Kind: b.Kind(), Provider: b.Provider()})
	}
	writeJSON(w, http.StatusOK, map[string]any{"backends": out})
}

// @Summary  登记一条知识源（可选 bind {type,id}）
// @Tags     knowledge
// @Accept   json
// @Produce  json
// @Success  201  {object}  Knowledge
// @Failure  400  {object}  errBody
// @Router   /api/knowledge [post]
func (s *Server) handleRegister(w http.ResponseWriter, req *http.Request) {
	var body RegisterRequest
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, errBody{Code: ErrInvalidQuery, Message: "invalid json: " + err.Error()})
		return
	}
	k, err := s.svc.Register(req.Context(), body)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, k)
}

// @Summary  列出知识源（可按 entity_type + entity_id 过滤）
// @Tags     knowledge
// @Produce  json
// @Param    entity_type  query  string  false  "CONTEXT entity type"
// @Param    entity_id    query  string  false  "CONTEXT entity id"
// @Success  200  {object}  map[string]any
// @Router   /api/knowledge [get]
func (s *Server) handleList(w http.ResponseWriter, req *http.Request) {
	q := req.URL.Query()
	typ := strings.TrimSpace(q.Get("entity_type"))
	id := strings.TrimSpace(q.Get("entity_id"))
	if typ != "" || id != "" {
		entity, err := ParseEntity(typ, id)
		if err != nil {
			writeErr(w, err)
			return
		}
		out, err := s.svc.ListForEntity(req.Context(), entity)
		if err != nil {
			writeErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"knowledge": out, "entity": entity})
		return
	}
	out, err := s.svc.List(req.Context())
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"knowledge": out})
}

// @Summary  一条知识源
// @Tags     knowledge
// @Produce  json
// @Param    id  path  string  true  "knowledge_id"
// @Success  200  {object}  Knowledge
// @Failure  404  {object}  errBody
// @Router   /api/knowledge/{id} [get]
func (s *Server) handleGet(w http.ResponseWriter, req *http.Request) {
	k, err := s.svc.Get(req.Context(), req.PathValue("id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, k)
}

// @Summary  删除一条知识源
// @Tags     knowledge
// @Param    id  path  string  true  "knowledge_id"
// @Success  204
// @Failure  404  {object}  errBody
// @Router   /api/knowledge/{id} [delete]
func (s *Server) handleDelete(w http.ResponseWriter, req *http.Request) {
	if err := s.svc.Delete(req.Context(), req.PathValue("id")); err != nil {
		writeErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// @Summary  同步并重建索引
// @Tags     knowledge
// @Produce  json
// @Param    id  path  string  true  "knowledge_id"
// @Success  200  {object}  SyncResult
// @Router   /api/knowledge/{id}/sync [post]
func (s *Server) handleSync(w http.ResponseWriter, req *http.Request) {
	out, err := s.svc.Sync(req.Context(), req.PathValue("id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// @Summary  一条源下的文档
// @Tags     knowledge
// @Produce  json
// @Param    id  path  string  true  "knowledge_id"
// @Success  200  {object}  map[string]any
// @Router   /api/knowledge/{id}/documents [get]
func (s *Server) handleDocuments(w http.ResponseWriter, req *http.Request) {
	docs, err := s.svc.Documents(req.Context(), req.PathValue("id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"documents": docs})
}

// @Summary  一条源绑到的 CONTEXT entity
// @Tags     knowledge
// @Produce  json
// @Param    id  path  string  true  "knowledge_id"
// @Success  200  {object}  map[string]any
// @Router   /api/knowledge/{id}/bindings [get]
func (s *Server) handleListBindings(w http.ResponseWriter, req *http.Request) {
	entities, err := s.svc.Bindings(req.Context(), req.PathValue("id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	if entities == nil {
		entities = []ContextEntity{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"bindings": entities})
}

// @Summary  把知识源绑到一个 CONTEXT entity
// @Tags     knowledge
// @Produce  json
// @Param    id          path  string  true  "knowledge_id"
// @Param    entityType  path  string  true  "entity type"
// @Param    entityID    path  string  true  "entity id"
// @Success  200  {object}  map[string]any
// @Router   /api/knowledge/{id}/bindings/{entityType}/{entityID} [put]
func (s *Server) handleBind(w http.ResponseWriter, req *http.Request) {
	entity, err := ParseEntity(req.PathValue("entityType"), req.PathValue("entityID"))
	if err != nil {
		writeErr(w, err)
		return
	}
	if err := s.svc.Bind(req.Context(), req.PathValue("id"), entity); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"knowledge_id": req.PathValue("id"), "entity": entity})
}

// @Summary  解绑一个 CONTEXT entity
// @Tags     knowledge
// @Param    id          path  string  true  "knowledge_id"
// @Param    entityType  path  string  true  "entity type"
// @Param    entityID    path  string  true  "entity id"
// @Success  204
// @Router   /api/knowledge/{id}/bindings/{entityType}/{entityID} [delete]
func (s *Server) handleUnbind(w http.ResponseWriter, req *http.Request) {
	entity, err := ParseEntity(req.PathValue("entityType"), req.PathValue("entityID"))
	if err != nil {
		writeErr(w, err)
		return
	}
	if err := s.svc.Unbind(req.Context(), req.PathValue("id"), entity); err != nil {
		writeErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// @Summary  某个 CONTEXT entity 绑定的知识源（context.list）
// @Tags     contexts
// @Produce  json
// @Param    entityType  path  string  true  "entity type"
// @Param    entityID    path  string  true  "entity id"
// @Success  200  {object}  map[string]any
// @Router   /api/contexts/{entityType}/{entityID}/knowledge [get]
func (s *Server) handleEntityKnowledge(w http.ResponseWriter, req *http.Request) {
	entity, err := ParseEntity(req.PathValue("entityType"), req.PathValue("entityID"))
	if err != nil {
		writeErr(w, err)
		return
	}
	out, err := s.svc.ListForEntity(req.Context(), entity)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"knowledge": out, "entity": entity})
}

// @Summary  在某个 CONTEXT entity 的绑定源里检索（context.search）
// @Tags     contexts
// @Produce  json
// @Param    entityType  path   string  true   "entity type"
// @Param    entityID    path   string  true   "entity id"
// @Param    q           query  string  true   "检索词"
// @Param    limit       query  int     false  "条数，默认 10"
// @Success  200  {object}  map[string]any
// @Router   /api/contexts/{entityType}/{entityID}/search [get]
func (s *Server) handleSearch(w http.ResponseWriter, req *http.Request) {
	entity, err := ParseEntity(req.PathValue("entityType"), req.PathValue("entityID"))
	if err != nil {
		writeErr(w, err)
		return
	}
	s.writeSearch(w, req, entity)
}

func (s *Server) writeSearch(w http.ResponseWriter, req *http.Request, entity ContextEntity) {
	q := req.URL.Query()
	search := SearchRequest{Entity: entity, Query: q.Get("q")}
	if raw := strings.TrimSpace(q.Get("limit")); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, errBody{Code: ErrInvalidQuery, Message: "limit is not a number"})
			return
		}
		search.Limit = n
	}
	results, err := s.svc.Search(req.Context(), search)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"results": results, "entity": entity})
}

// @Summary  取一段全文（context.get）
// @Tags     contexts
// @Produce  json
// @Param    sectionID  path  string  true  "section_id"
// @Success  200  {object}  Section
// @Failure  404  {object}  errBody
// @Router   /api/sections/{sectionID} [get]
func (s *Server) handleGetSection(w http.ResponseWriter, req *http.Request) {
	sec, err := s.svc.GetSection(req.Context(), req.PathValue("sectionID"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, sec)
}

// @Summary  维护页（登记 / 绑定 / 同步 / 检索）
// @Tags     system
// @Produce  html
// @Success  200  {string}  string
// @Router   / [get]
func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	s.handleDashboard(w, r)
}

// @Summary  维护页
// @Tags     system
// @Produce  html
// @Success  200  {string}  string
// @Router   /dashboard [get]
func (s *Server) handleDashboard(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = io.WriteString(w, dashboardHTML)
}
