package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"zsgate/common"
	"zsgate/control-plane/internal/store"
)

type Server struct {
	store      *store.Store
	httpClient *http.Client
}

func NewServer(st *store.Store) *Server {
	return &Server{
		store:      st,
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}
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
	mux.HandleFunc("/admin/usage/by-scenario", s.handleUsageByScenario)
	mux.HandleFunc("/admin/alerts/rules", s.handleAlertRules)
	mux.HandleFunc("/admin/alerts/events", s.handleAlertEvents)
	mux.HandleFunc("/admin/alerts/evaluate", s.handleAlertEvaluate)
	mux.HandleFunc("/admin/realtime/active-users", s.handleRealtimeActiveUsers)
	mux.HandleFunc("/admin/realtime/active-tasks", s.handleRealtimeActiveTasks)
	mux.HandleFunc("/admin/realtime/stream", s.handleRealtimeStream)
	mux.HandleFunc("/admin/realtime/dialog-stream", s.handleRealtimeDialogStream)
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
	s.notifyTriggers(s.store.EvaluateAlerts(time.Now().UTC()))
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
		"scenario":   r.URL.Query().Get("scenario"),
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

type scenarioUsageSummary struct {
	ScenarioTag       string `json:"scenario_tag"`
	Requests          int    `json:"requests"`
	PromptTokens      int    `json:"prompt_tokens"`
	CompletionTokens  int    `json:"completion_tokens"`
	TotalTokens       int    `json:"total_tokens"`
	CostEstimateCents int    `json:"cost_estimate_cents"`
	AverageLatencyMs  int64  `json:"average_latency_ms"`
}

func (s *Server) handleUsageByScenario(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	type agg struct {
		scenarioUsageSummary
		latencyTotal int64
	}
	data := map[string]*agg{}
	for _, e := range s.store.Usage() {
		scenario := e.ScenarioTag
		if scenario == "" {
			scenario = "general"
		}
		if _, ok := data[scenario]; !ok {
			data[scenario] = &agg{
				scenarioUsageSummary: scenarioUsageSummary{
					ScenarioTag: scenario,
				},
			}
		}
		a := data[scenario]
		a.Requests++
		a.PromptTokens += e.PromptTokens
		a.CompletionTokens += e.CompletionTokens
		a.TotalTokens += e.PromptTokens + e.CompletionTokens
		a.CostEstimateCents += e.CostEstimateCents
		a.latencyTotal += e.LatencyMs
	}
	out := make([]scenarioUsageSummary, 0, len(data))
	for _, a := range data {
		if a.Requests > 0 {
			a.AverageLatencyMs = a.latencyTotal / int64(a.Requests)
		}
		out = append(out, a.scenarioUsageSummary)
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleAlertRules(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, s.store.ListAlertRules())
	case http.MethodPost:
		var rule store.AlertRule
		if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if rule.ID == "" || rule.Name == "" || rule.Metric == "" {
			writeError(w, http.StatusBadRequest, fmt.Errorf("id, name, metric are required"))
			return
		}
		writeJSON(w, http.StatusCreated, s.store.UpsertAlertRule(rule))
	default:
		methodNotAllowed(w)
	}
}

func (s *Server) handleAlertEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	limit := 100
	if v := r.URL.Query().Get("limit"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil {
			limit = parsed
		}
	}
	writeJSON(w, http.StatusOK, s.store.ListAlertEvents(limit))
}

func (s *Server) handleAlertEvaluate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	triggers := s.store.EvaluateAlerts(time.Now().UTC())
	s.notifyTriggers(triggers)
	writeJSON(w, http.StatusOK, map[string]any{"triggered": len(triggers), "events": triggers})
}

type activeUser struct {
	UserID      string `json:"user_id"`
	LastSeen    string `json:"last_seen"`
	Requests1m  int    `json:"requests_1m"`
	Requests5m  int    `json:"requests_5m"`
	ProjectID   string `json:"project_id"`
	ScenarioTag string `json:"scenario_tag"`
}

func (s *Server) handleRealtimeActiveUsers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	now := time.Now().UTC()
	usage := s.store.Usage()
	type agg struct {
		activeUser
		lastTime time.Time
	}
	users := map[string]*agg{}
	for _, e := range usage {
		if now.Sub(e.Timestamp) > 15*time.Minute {
			continue
		}
		if _, ok := users[e.UserID]; !ok {
			users[e.UserID] = &agg{activeUser: activeUser{UserID: e.UserID}}
		}
		u := users[e.UserID]
		if now.Sub(e.Timestamp) <= 5*time.Minute {
			u.Requests5m++
		}
		if now.Sub(e.Timestamp) <= time.Minute {
			u.Requests1m++
		}
		if e.Timestamp.After(u.lastTime) {
			u.lastTime = e.Timestamp
			u.LastSeen = e.Timestamp.Format(time.RFC3339)
			u.ProjectID = e.ProjectID
			u.ScenarioTag = e.ScenarioTag
			if u.ScenarioTag == "" {
				u.ScenarioTag = "general"
			}
		}
	}
	out := make([]activeUser, 0, len(users))
	for _, u := range users {
		out = append(out, u.activeUser)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].LastSeen > out[j].LastSeen })
	writeJSON(w, http.StatusOK, map[string]any{
		"window":       "15m",
		"active_users": out,
		"active_count": len(out),
	})
}

type activeTask struct {
	TaskCategory      string `json:"task_category"`
	ActiveUsers       int    `json:"active_users"`
	Requests1m        int    `json:"requests_1m"`
	Requests5m        int    `json:"requests_5m"`
	CostEstimateCents int    `json:"cost_estimate_cents_5m"`
}

func (s *Server) handleRealtimeActiveTasks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	now := time.Now().UTC()
	usage := s.store.Usage()
	type agg struct {
		activeTask
		userSet map[string]struct{}
	}
	tasks := map[string]*agg{}
	for _, e := range usage {
		if now.Sub(e.Timestamp) > 5*time.Minute {
			continue
		}
		task := e.TaskCategory
		if task == "" {
			task = e.ScenarioTag
		}
		if task == "" {
			task = "general"
		}
		if _, ok := tasks[task]; !ok {
			tasks[task] = &agg{
				activeTask: activeTask{TaskCategory: task},
				userSet:    map[string]struct{}{},
			}
		}
		t := tasks[task]
		t.Requests5m++
		if now.Sub(e.Timestamp) <= time.Minute {
			t.Requests1m++
		}
		t.CostEstimateCents += e.CostEstimateCents
		t.userSet[e.UserID] = struct{}{}
	}
	out := make([]activeTask, 0, len(tasks))
	for _, t := range tasks {
		t.ActiveUsers = len(t.userSet)
		out = append(out, t.activeTask)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Requests5m > out[j].Requests5m })
	writeJSON(w, http.StatusOK, map[string]any{
		"window":       "5m",
		"active_tasks": out,
		"task_count":   len(out),
	})
}

func (s *Server) handleRealtimeStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("stream unsupported"))
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	send := func() {
		now := time.Now().UTC()
		usage := s.store.Usage()
		activeUsers := map[string]struct{}{}
		tasks := map[string]int{}
		for _, e := range usage {
			if now.Sub(e.Timestamp) > 5*time.Minute {
				continue
			}
			activeUsers[e.UserID] = struct{}{}
			task := e.TaskCategory
			if task == "" {
				task = e.ScenarioTag
			}
			if task == "" {
				task = "general"
			}
			tasks[task]++
		}
		payload := map[string]any{
			"timestamp":         now.Format(time.RFC3339),
			"active_user_count": len(activeUsers),
			"active_task_count": len(tasks),
			"task_requests_5m":  tasks,
		}
		b, _ := json.Marshal(payload)
		_, _ = fmt.Fprintf(w, "event: snapshot\ndata: %s\n\n", string(b))
		flusher.Flush()
	}

	send()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			send()
		}
	}
}

func (s *Server) handleRealtimeDialogStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("stream unsupported"))
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	offset := 0
	if r.URL.Query().Get("from") == "tail" {
		_, offset = s.store.AuditSince(0)
	}
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	sendBatch := func() {
		events, next := s.store.AuditSince(offset)
		offset = next
		for _, e := range events {
			if e.Action != "chat.completions" && e.Action != "mcp.call" {
				continue
			}
			payload := map[string]any{
				"timestamp":     e.Timestamp.Format(time.RFC3339),
				"trace_id":      e.TraceID,
				"user_id":       e.UserID,
				"project_id":    e.ProjectID,
				"scenario_tag":  e.ScenarioTag,
				"task_category": e.TaskCategory,
				"action":        e.Action,
				"request_meta":  e.RequestMeta,
				"summary":       e.ContentSummary,
				"retained":      e.ContentRetained,
			}
			b, _ := json.Marshal(payload)
			_, _ = fmt.Fprintf(w, "event: dialog\ndata: %s\n\n", string(b))
		}
		flusher.Flush()
	}

	sendBatch()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			sendBatch()
		}
	}
}

func (s *Server) notifyTriggers(triggers []store.AlertTrigger) {
	for _, trigger := range triggers {
		if trigger.WebhookURL == "" {
			continue
		}
		payload, err := json.Marshal(trigger.Event)
		if err != nil {
			continue
		}
		req, err := http.NewRequest(http.MethodPost, trigger.WebhookURL, bytes.NewReader(payload))
		if err != nil {
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := s.httpClient.Do(req)
		if err != nil {
			continue
		}
		_ = resp.Body.Close()
	}
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
