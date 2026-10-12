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

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.handleHealth)
	mux.HandleFunc("GET /healthz", s.handleHealth)
	mux.HandleFunc("GET /api/backends", s.handleBackends)
	mux.HandleFunc("POST /api/knowledge", s.handleRegister)
	mux.HandleFunc("GET /api/knowledge", s.handleList)
	mux.HandleFunc("GET /api/knowledge/{id}", s.handleGet)
	mux.HandleFunc("DELETE /api/knowledge/{id}", s.handleDelete)
	mux.HandleFunc("POST /api/knowledge/{id}/sync", s.handleSync)
	mux.HandleFunc("GET /api/knowledge/{id}/documents", s.handleDocuments)
	mux.HandleFunc("GET /api/knowledge/{id}/bindings", s.handleListBindings)
	mux.HandleFunc("PUT /api/knowledge/{id}/bindings/{entityType}/{entityID}", s.handleBind)
	mux.HandleFunc("DELETE /api/knowledge/{id}/bindings/{entityType}/{entityID}", s.handleUnbind)
	mux.HandleFunc("GET /api/contexts/{entityType}/{entityID}/knowledge", s.handleEntityKnowledge)
	mux.HandleFunc("GET /api/contexts/{entityType}/{entityID}/search", s.handleSearch)
	mux.HandleFunc("GET /api/sections/{sectionID}", s.handleGetSection)
	mux.HandleFunc("GET /", s.handleHome)
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

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

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

func (s *Server) handleGet(w http.ResponseWriter, req *http.Request) {
	k, err := s.svc.Get(req.Context(), req.PathValue("id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, k)
}

func (s *Server) handleDelete(w http.ResponseWriter, req *http.Request) {
	if err := s.svc.Delete(req.Context(), req.PathValue("id")); err != nil {
		writeErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleSync(w http.ResponseWriter, req *http.Request) {
	out, err := s.svc.Sync(req.Context(), req.PathValue("id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleDocuments(w http.ResponseWriter, req *http.Request) {
	docs, err := s.svc.Documents(req.Context(), req.PathValue("id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"documents": docs})
}

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

func (s *Server) handleGetSection(w http.ResponseWriter, req *http.Request) {
	sec, err := s.svc.GetSection(req.Context(), req.PathValue("sectionID"))
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, sec)
}

func (s *Server) handleHome(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = io.WriteString(w, "knowledge-base\nGET /health  GET /api/backends  GET /api/knowledge\nAutonomy: GET /api/contexts/{type}/{id}/knowledge  GET /api/contexts/{type}/{id}/search?q=\n")
}
