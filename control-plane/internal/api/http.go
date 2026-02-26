package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"zsgate/common"
	"zsgate/control-plane/internal/store"
)

type Server struct {
	store *store.Store
}

func NewServer(st *store.Store) *Server {
	return &Server{store: st}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.healthz)
	mux.HandleFunc("/admin/vendors", s.handleVendors)
	mux.HandleFunc("/admin/vendors/", s.handleVendorByID)
	mux.HandleFunc("/admin/model-aliases", s.handleAliases)
	mux.HandleFunc("/admin/routing-policies", s.handlePolicies)
	mux.HandleFunc("/admin/audits", s.handleAudits)
	mux.HandleFunc("/admin/usage", s.handleUsage)
	mux.HandleFunc("/admin/costs/by-dept", s.handleCostsByDept)
	mux.HandleFunc("/admin/costs/by-project", s.handleCostsByProject)
	mux.HandleFunc("/admin/costs/by-user", s.handleCostsByUser)
	mux.HandleFunc("/admin/usage/by-user", s.handleUsageByUser)
	mux.HandleFunc("/internal/events/usage", s.handleUsageEvent)
	mux.HandleFunc("/internal/events/audit", s.handleAuditEvent)
	mux.HandleFunc("/internal/routing", s.handleRouting)
	return mux
}

func (s *Server) healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "control-plane"})
}

func (s *Server) handleVendors(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, s.store.ListVendors())
	case http.MethodPost:
		var v common.Vendor
		if err := json.NewDecoder(r.Body).Decode(&v); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if v.ID == "" || v.Name == "" || v.Endpoint == "" {
			writeError(w, http.StatusBadRequest, fmt.Errorf("id, name, endpoint are required"))
			return
		}
		if v.Status == "" {
			v.Status = common.VendorEnabled
		}
		created := s.store.UpsertVendor(v)
		writeJSON(w, http.StatusCreated, created)
	default:
		methodNotAllowed(w)
	}
}

func (s *Server) handleVendorByID(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/admin/vendors/")
	parts := strings.Split(path, "/")
	id := parts[0]
	if id == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("missing vendor id"))
		return
	}
	if len(parts) == 1 {
		if r.Method != http.MethodPatch {
			methodNotAllowed(w)
			return
		}
		current, err := s.store.GetVendor(id)
		if err != nil {
			writeError(w, http.StatusNotFound, err)
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&current); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		current.ID = id
		writeJSON(w, http.StatusOK, s.store.UpsertVendor(current))
		return
	}
	if len(parts) == 2 && parts[1] == "disable" {
		if r.Method != http.MethodPost {
			methodNotAllowed(w)
			return
		}
		v, err := s.store.DisableVendor(id)
		if err != nil {
			writeError(w, http.StatusNotFound, err)
			return
		}
		writeJSON(w, http.StatusOK, v)
		return
	}
	if len(parts) == 2 && parts[1] == "health-check" {
		if r.Method != http.MethodPost {
			methodNotAllowed(w)
			return
		}
		state := "healthy"
		if r.URL.Query().Get("force") == "fail" {
			state = "unhealthy"
		}
		v, err := s.store.SetVendorHealth(id, state)
		if err != nil {
			writeError(w, http.StatusNotFound, err)
			return
		}
		writeJSON(w, http.StatusOK, v)
		return
	}
	writeError(w, http.StatusNotFound, fmt.Errorf("not found"))
}

func (s *Server) handleAliases(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, s.store.ListAliases())
	case http.MethodPost:
		var a common.ModelAlias
		if err := json.NewDecoder(r.Body).Decode(&a); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if a.ID == "" || a.Alias == "" || a.VendorID == "" || a.VendorModel == "" {
			writeError(w, http.StatusBadRequest, fmt.Errorf("id, alias, vendor_id, vendor_model are required"))
			return
		}
		if !a.Enabled {
			a.Enabled = true
		}
		writeJSON(w, http.StatusCreated, s.store.UpsertAlias(a))
	default:
		methodNotAllowed(w)
	}
}

func (s *Server) handlePolicies(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, s.store.ListPolicies())
	case http.MethodPost:
		var p common.RoutingPolicy
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if p.ID == "" || p.Alias == "" {
			writeError(w, http.StatusBadRequest, fmt.Errorf("id and alias are required"))
			return
		}
		if p.Strategy == "" {
			p.Strategy = "weighted"
		}
		if p.FailoverMode == "" {
			p.FailoverMode = "automatic"
		}
		writeJSON(w, http.StatusCreated, s.store.UpsertPolicy(p))
	default:
		methodNotAllowed(w)
	}
}

func (s *Server) handleUsageEvent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var e common.UsageEvent
	if err := json.NewDecoder(r.Body).Decode(&e); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now().UTC()
	}
	s.store.AddUsage(e)
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "ok"})
}

func (s *Server) handleAuditEvent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	var e common.AuditEvent
	if err := json.NewDecoder(r.Body).Decode(&e); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now().UTC()
	}
	s.store.AddAudit(e)
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "ok"})
}

func (s *Server) handleRouting(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	alias := r.URL.Query().Get("alias")
	if alias == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("alias is required"))
		return
	}
	target, err := s.store.ResolveRoute(alias)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, target)
}

func (s *Server) handleAudits(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	filter := map[string]string{
		"user_id":    r.URL.Query().Get("user_id"),
		"project_id": r.URL.Query().Get("project_id"),
		"trace_id":   r.URL.Query().Get("trace_id"),
	}
	writeJSON(w, http.StatusOK, s.store.Audit(filter))
}

func (s *Server) handleUsage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	writeJSON(w, http.StatusOK, s.store.Usage())
}

func (s *Server) handleCostsByDept(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	resp := map[string]int{}
	for _, e := range s.store.Usage() {
		resp[e.DeptID] += e.CostEstimateCents
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleCostsByProject(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	resp := map[string]int{}
	for _, e := range s.store.Usage() {
		resp[e.ProjectID] += e.CostEstimateCents
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleCostsByUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	resp := map[string]int{}
	for _, e := range s.store.Usage() {
		resp[e.UserID] += e.CostEstimateCents
	}
	writeJSON(w, http.StatusOK, resp)
}

type userUsageSummary struct {
	UserID            string `json:"user_id"`
	Requests          int    `json:"requests"`
	PromptTokens      int    `json:"prompt_tokens"`
	CompletionTokens  int    `json:"completion_tokens"`
	TotalTokens       int    `json:"total_tokens"`
	CostEstimateCents int    `json:"cost_estimate_cents"`
	AverageLatencyMs  int64  `json:"average_latency_ms"`
	LastSeenTimestamp string `json:"last_seen_timestamp"`
}

func (s *Server) handleUsageByUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	type agg struct {
		userUsageSummary
		latencyTotal int64
	}
	data := map[string]*agg{}
	for _, e := range s.store.Usage() {
		if _, ok := data[e.UserID]; !ok {
			data[e.UserID] = &agg{
				userUsageSummary: userUsageSummary{
					UserID: e.UserID,
				},
			}
		}
		a := data[e.UserID]
		a.Requests++
		a.PromptTokens += e.PromptTokens
		a.CompletionTokens += e.CompletionTokens
		a.TotalTokens += e.PromptTokens + e.CompletionTokens
		a.CostEstimateCents += e.CostEstimateCents
		a.latencyTotal += e.LatencyMs
		if a.LastSeenTimestamp == "" || e.Timestamp.Format(time.RFC3339) > a.LastSeenTimestamp {
			a.LastSeenTimestamp = e.Timestamp.Format(time.RFC3339)
		}
	}

	out := make([]userUsageSummary, 0, len(data))
	for _, a := range data {
		if a.Requests > 0 {
			a.AverageLatencyMs = a.latencyTotal / int64(a.Requests)
		}
		out = append(out, a.userUsageSummary)
	}
	writeJSON(w, http.StatusOK, out)
}

func methodNotAllowed(w http.ResponseWriter) {
	writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"))
}

func writeError(w http.ResponseWriter, code int, err error) {
	writeJSON(w, code, map[string]string{"error": err.Error()})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}
