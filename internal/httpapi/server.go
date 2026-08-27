package httpapi

import (
	"biocuration/internal/application"
	"biocuration/internal/domain"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Server struct{ App *application.Service }

func New(app *application.Service) *Server { return &Server{App: app} }
func (s *Server) Handler() http.Handler {
	m := http.NewServeMux()
	m.HandleFunc("/healthz", s.health)
	m.HandleFunc("/selfcheck", s.selfcheck)
	m.HandleFunc("/v1/trees", s.trees)
	m.HandleFunc("/v1/trees/", s.treeBatches)
	m.HandleFunc("/v1/batches/", s.batchRoutes)
	m.HandleFunc("/v1/credentials/", s.verify)
	return m
}
func write(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
func errResp(w http.ResponseWriter, e error) {
	status := http.StatusInternalServerError
	code := "internal_error"
	var d *domain.DomainError
	if errors.As(e, &d) {
		code = string(d.Code)
		switch d.Code {
		case domain.ErrInvalidInput:
			status = 400
		case domain.ErrNotFound:
			status = 404
		case domain.ErrConflict, domain.ErrVersionMismatch:
			status = 409
		case domain.ErrInvalidState:
			status = 422
		case domain.ErrIdempotency:
			status = 409
		}
	}
	write(w, status, map[string]string{"error": code, "message": e.Error()})
}
func decode(r *http.Request, v any) error {
	if r.Body == nil {
		return &domain.DomainError{Code: domain.ErrInvalidInput, Message: "请求体不能为空"}
	}
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if e := d.Decode(v); e != nil {
		return &domain.DomainError{Code: domain.ErrInvalidInput, Message: "请求 JSON 无效"}
	}
	return nil
}
func key(r *http.Request) string { return strings.TrimSpace(r.Header.Get("Idempotency-Key")) }
func requireKey(w http.ResponseWriter, r *http.Request) bool {
	if key(r) == "" {
		errResp(w, &domain.DomainError{Code: domain.ErrInvalidInput, Message: "缺少 Idempotency-Key"})
		return false
	}
	return true
}
func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		w.WriteHeader(405)
		return
	}
	write(w, 200, map[string]string{"status": "ok"})
}
func (s *Server) selfcheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		w.WriteHeader(405)
		return
	}
	write(w, 200, map[string]string{"status": "ready"})
}

type treeReq struct {
	TreeID              string `json:"treeID"`
	Species             string `json:"species"`
	LocationDescription string `json:"locationDescription"`
	ProtectedStatus     bool   `json:"protectedStatus"`
}

func (s *Server) trees(w http.ResponseWriter, r *http.Request) {
	if r.Method == "PATCH" {
		s.updateTree(w, r)
		return
	}
	if r.Method != "POST" {
		w.WriteHeader(405)
		return
	}
	if !requireKey(w, r) {
		return
	}
	var q treeReq
	if e := decode(r, &q); e != nil {
		errResp(w, e)
		return
	}
	t, e := s.App.CreateTree(r.Context(), q.TreeID, q.Species, q.LocationDescription, q.ProtectedStatus, key(r))
	if e != nil {
		errResp(w, e)
		return
	}
	write(w, 201, t)
}

type treeUpdateReq struct {
	TreeID                  string `json:"treeID"`
	Species                 string `json:"species"`
	LocationDescription     string `json:"locationDescription"`
	ProtectedStatus         bool   `json:"protectedStatus"`
	ExpectedBaselineVersion int    `json:"expectedBaselineVersion"`
}

func (s *Server) updateTree(w http.ResponseWriter, r *http.Request) {
	if !requireKey(w, r) {
		return
	}
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) != 3 || parts[0] != "v1" || parts[1] != "trees" {
		http.NotFound(w, r)
		return
	}
	var q treeUpdateReq
	if e := decode(r, &q); e != nil {
		errResp(w, e)
		return
	}
	t, e := s.App.UpdateTree(r.Context(), parts[2], q.TreeID, q.Species, q.LocationDescription, q.ProtectedStatus, q.ExpectedBaselineVersion, key(r))
	if e != nil {
		errResp(w, e)
		return
	}
	write(w, 200, t)
}

type batchReq struct {
	Collector      string   `json:"collector"`
	CollectedAt    string   `json:"collectedAt"`
	TargetTissues  []string `json:"targetTissues"`
	TargetQuantity int      `json:"targetQuantity"`
}

func (s *Server) treeBatches(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if r.Method == "PATCH" && len(parts) == 3 && parts[0] == "v1" && parts[1] == "trees" {
		s.updateTree(w, r)
		return
	}
	if len(parts) != 4 || parts[0] != "v1" || parts[1] != "trees" || parts[3] != "batches" {
		http.NotFound(w, r)
		return
	}
	if r.Method != "POST" || !requireKey(w, r) {
		if r.Method != "POST" {
			w.WriteHeader(405)
		}
		return
	}
	var q batchReq
	if e := decode(r, &q); e != nil {
		errResp(w, e)
		return
	}
	at, e := time.Parse(time.RFC3339, q.CollectedAt)
	if e != nil {
		errResp(w, &domain.DomainError{Code: domain.ErrInvalidInput, Message: "collectedAt 必须为 RFC3339"})
		return
	}
	b, e := s.App.CreateBatch(r.Context(), parts[2], q.Collector, at, q.TargetTissues, q.TargetQuantity, key(r))
	if e != nil {
		errResp(w, e)
		return
	}
	write(w, 201, b)
}
func (s *Server) batchRoutes(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 4 || parts[0] != "v1" || parts[1] != "batches" {
		http.NotFound(w, r)
		return
	}
	id := parts[2]
	action := parts[3]
	if action == "resampling" && r.Method == "GET" {
		s.listResampling(w, r, id)
		return
	}
	if action == "freeze" && r.Method == "GET" {
		s.getFreeze(w, r, id)
		return
	}
	if r.Method != "POST" {
		w.WriteHeader(405)
		return
	}
	if !requireKey(w, r) {
		return
	}
	switch action {
	case "inspection":
		s.inspection(w, r, id)
	case "resampling":
		if len(parts) != 5 || (parts[4] != "resolve" && parts[4] != "resolve-all") {
			http.NotFound(w, r)
			return
		}
		if parts[4] == "resolve-all" {
			s.resolveAll(w, r, id)
		} else {
			s.resolve(w, r, id)
		}
	case "freeze":
		s.freeze(w, r, id)
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) listResampling(w http.ResponseWriter, r *http.Request, id string) {
	status := domain.ResamplingStatus(r.URL.Query().Get("status"))
	limit := 20
	if raw := r.URL.Query().Get("limit"); raw != "" {
		n, e := strconv.Atoi(raw)
		if e != nil {
			errResp(w, &domain.DomainError{Code: domain.ErrInvalidInput, Message: "limit 无效"})
			return
		}
		limit = n
	}
	out, e := s.App.ListResampling(r.Context(), id, status, limit)
	if e != nil {
		errResp(w, e)
		return
	}
	write(w, 200, out)
}

func (s *Server) getFreeze(w http.ResponseWriter, r *http.Request, id string) {
	f, e := s.App.Store.FreezeByBatch(r.Context(), id)
	if e != nil {
		errResp(w, e)
		return
	}
	write(w, 200, f)
}

type inspectionReq struct {
	SampleID           string `json:"sampleID"`
	Label              string `json:"label"`
	Quantity           int    `json:"quantity"`
	ContainerCondition string `json:"containerCondition"`
	ChainNotes         string `json:"chainNotes"`
	ExpectedVersion    int    `json:"expectedVersion"`
}

func (s *Server) inspection(w http.ResponseWriter, r *http.Request, id string) {
	var q inspectionReq
	if e := decode(r, &q); e != nil {
		errResp(w, e)
		return
	}
	i, t, b, e := s.App.Inspect(r.Context(), id, q.SampleID, q.Label, q.Quantity, q.ContainerCondition, q.ChainNotes, key(r), q.ExpectedVersion)
	if e != nil {
		errResp(w, e)
		return
	}
	write(w, 200, map[string]any{"inspection": i, "tasks": t, "batch": b})
}

type resolveReq struct {
	TaskID          string `json:"taskID"`
	ExpectedVersion int    `json:"expectedVersion"`
}

func (s *Server) resolve(w http.ResponseWriter, r *http.Request, id string) {
	var q resolveReq
	if e := decode(r, &q); e != nil {
		errResp(w, e)
		return
	}
	t, b, e := s.App.Resolve(r.Context(), id, q.TaskID, q.ExpectedVersion, key(r))
	if e != nil {
		errResp(w, e)
		return
	}
	write(w, 200, map[string]any{"task": t, "batch": b})
}

type resolveAllReq struct {
	TaskIDs         []string `json:"taskIDs"`
	ExpectedVersion int      `json:"expectedVersion"`
}

func (s *Server) resolveAll(w http.ResponseWriter, r *http.Request, id string) {
	var q resolveAllReq
	if e := decode(r, &q); e != nil {
		errResp(w, e)
		return
	}
	t, b, e := s.App.ResolveAll(r.Context(), id, q.TaskIDs, q.ExpectedVersion, key(r))
	if e != nil {
		errResp(w, e)
		return
	}
	write(w, 200, map[string]any{"tasks": t, "batch": b})
}

type freezeReq struct {
	FrozenBy        string `json:"frozenBy"`
	ExpectedVersion int    `json:"expectedVersion"`
}

func (s *Server) freeze(w http.ResponseWriter, r *http.Request, id string) {
	var q freezeReq
	if e := decode(r, &q); e != nil {
		errResp(w, e)
		return
	}
	f, c, e := s.App.Freeze(r.Context(), id, q.FrozenBy, q.ExpectedVersion, key(r))
	if e != nil {
		errResp(w, e)
		return
	}
	write(w, 201, map[string]any{"freeze": f, "credential": c})
}
func (s *Server) verify(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		w.WriteHeader(405)
		return
	}
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) != 4 || parts[0] != "v1" || parts[1] != "credentials" || parts[3] != "verify" {
		http.NotFound(w, r)
		return
	}
	q := r.URL.Query()
	history := false
	if raw := q.Get("history"); raw != "" {
		switch raw {
		case "true", "1":
			history = true
		case "false", "0":
		default:
			errResp(w, &domain.DomainError{Code: domain.ErrInvalidInput, Message: "history 无效"})
			return
		}
	}
	limit := 20
	if raw := q.Get("limit"); raw != "" {
		n, e := strconv.Atoi(raw)
		if e != nil {
			errResp(w, &domain.DomainError{Code: domain.ErrInvalidInput, Message: "limit 无效"})
			return
		}
		limit = n
	}
	var from, to *time.Time
	for name, dst := range map[string]**time.Time{"from": &from, "to": &to} {
		if raw := q.Get(name); raw != "" {
			t, e := time.Parse(time.RFC3339, raw)
			if e != nil {
				errResp(w, &domain.DomainError{Code: domain.ErrInvalidInput, Message: name + " 无效"})
				return
			}
			*dst = &t
		}
	}
	if from != nil && to != nil && from.After(*to) {
		errResp(w, &domain.DomainError{Code: domain.ErrInvalidInput, Message: "时间范围无效"})
		return
	}
	out, e := s.App.VerifyDetails(r.Context(), parts[2], history, from, to, limit)
	if e != nil {
		errResp(w, e)
		return
	}
	write(w, 200, out)
}
