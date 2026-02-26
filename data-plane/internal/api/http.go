package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"zsgate/common"
	"zsgate/data-plane/internal/auth"
	"zsgate/data-plane/internal/provider"
	"zsgate/data-plane/internal/router"
)

type Server struct {
	cp       *router.ControlPlaneClient
	provider *provider.StubProvider
}

type chatRequest struct {
	Model    string `json:"model"`
	Messages []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"messages"`
}

type embedRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

func NewServer(cp *router.ControlPlaneClient, pr *provider.StubProvider) *Server {
	return &Server{cp: cp, provider: pr}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.healthz)
	mux.HandleFunc("/v1/models", s.models)
	mux.HandleFunc("/v1/chat/completions", s.chatCompletions)
	mux.HandleFunc("/v1/embeddings", s.embeddings)
	mux.HandleFunc("/mcp", s.mcp)
	return mux
}

func (s *Server) healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "data-plane"})
}

func (s *Server) models(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": []map[string]string{{"id": "gpt-4o-prod"}, {"id": "code-assistant"}}})
}

func (s *Server) chatCompletions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"))
		return
	}
	ctxMeta, err := auth.RequestContextFromHeaders(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err)
		return
	}
	var req chatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if req.Model == "" || len(req.Messages) == 0 {
		writeError(w, http.StatusBadRequest, fmt.Errorf("model and messages are required"))
		return
	}

	target, err := s.cp.ResolveRoute(r.Context(), req.Model)
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}

	prompt := req.Messages[len(req.Messages)-1].Content
	ctxMeta = enrichTaskContext(ctxMeta, prompt)
	start := time.Now()
	out, err := s.provider.Generate(r.Context(), target.VendorType, target.VendorModel, provider.CompletionRequest{ModelAlias: req.Model, Prompt: prompt})
	latency := time.Since(start).Milliseconds()
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}

	usage := common.UsageEvent{
		TraceID:           ctxMeta.TraceID,
		Timestamp:         time.Now().UTC(),
		Provider:          target.VendorID,
		Model:             target.VendorModel,
		PromptTokens:      out.PromptTokens,
		CompletionTokens:  out.CompletionTokens,
		CostEstimateCents: estimateCost(target.VendorType, out.PromptTokens, out.CompletionTokens),
		LatencyMs:         latency,
		Status:            "ok",
		RequestContext:    ctxMeta,
	}
	s.cp.EmitUsage(r.Context(), usage)

	retained, redactionLevel, summary := contentRetention(prompt)
	if summary == "" {
		summary = fmt.Sprintf("task=%s metadata-only", ctxMeta.TaskCategory)
	}

	audit := common.AuditEvent{
		TraceID:         ctxMeta.TraceID,
		Timestamp:       time.Now().UTC(),
		Action:          "chat.completions",
		RequestMeta:     fmt.Sprintf("model=%s provider=%s task=%s", req.Model, target.VendorID, ctxMeta.TaskCategory),
		ResponseMeta:    fmt.Sprintf("completion_tokens=%d", out.CompletionTokens),
		ContentHash:     hash(prompt),
		RedactionLevel:  redactionLevel,
		ContentRetained: retained,
		ContentSummary:  summary,
		RiskFlags:       nil,
		RequestContext:  ctxMeta,
	}
	s.cp.EmitAudit(r.Context(), audit)

	writeJSON(w, http.StatusOK, map[string]any{
		"id":      "chatcmpl-zsgate",
		"object":  "chat.completion",
		"created": time.Now().Unix(),
		"model":   req.Model,
		"choices": []map[string]any{{"index": 0, "message": map[string]string{"role": "assistant", "content": out.Content}, "finish_reason": "stop"}},
		"usage": map[string]int{
			"prompt_tokens":     out.PromptTokens,
			"completion_tokens": out.CompletionTokens,
			"total_tokens":      out.PromptTokens + out.CompletionTokens,
		},
		"zsgate": map[string]string{
			"vendor_id":    target.VendorID,
			"vendor_type":  target.VendorType,
			"vendor_model": target.VendorModel,
		},
	})
}

func (s *Server) embeddings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"))
		return
	}
	ctxMeta, err := auth.RequestContextFromHeaders(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err)
		return
	}
	var req embedRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if req.Model == "" || req.Input == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("model and input are required"))
		return
	}
	ctxMeta = enrichTaskContext(ctxMeta, req.Input)
	target, err := s.cp.ResolveRoute(r.Context(), req.Model)
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}

	usage := common.UsageEvent{
		TraceID:           ctxMeta.TraceID,
		Timestamp:         time.Now().UTC(),
		Provider:          target.VendorID,
		Model:             target.VendorModel,
		PromptTokens:      len(req.Input) / 4,
		CompletionTokens:  0,
		CostEstimateCents: estimateCost(target.VendorType, len(req.Input)/4, 0),
		LatencyMs:         20,
		Status:            "ok",
		RequestContext:    ctxMeta,
	}
	s.cp.EmitUsage(r.Context(), usage)

	writeJSON(w, http.StatusOK, map[string]any{
		"object": "list",
		"data":   []map[string]any{{"object": "embedding", "index": 0, "embedding": []float64{0.01, 0.02, 0.03}}},
		"model":  req.Model,
	})
}

func (s *Server) mcp(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"))
		return
	}
	ctxMeta, err := auth.RequestContextFromHeaders(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err)
		return
	}
	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	ctxMeta = enrichTaskContext(ctxMeta, "mcp")
	s.cp.EmitAudit(r.Context(), common.AuditEvent{
		TraceID:        ctxMeta.TraceID,
		Timestamp:      time.Now().UTC(),
		Action:         "mcp.call",
		RequestMeta:    fmt.Sprintf("mcp_request task=%s", ctxMeta.TaskCategory),
		ResponseMeta:   "accepted",
		ContentHash:    hash("mcp"),
		RedactionLevel: "metadata-only",
		RequestContext: ctxMeta,
	})
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "echo": body})
}

func estimateCost(vendor string, promptTokens, completionTokens int) int {
	rate := 2
	switch vendor {
	case "openai", "claude", "gemini":
		rate = 5
	case "vllm":
		rate = 1
	}
	return (promptTokens + completionTokens) * rate
}

func hash(v string) string {
	sum := sha256.Sum256([]byte(v))
	return hex.EncodeToString(sum[:])
}

func writeError(w http.ResponseWriter, code int, err error) {
	writeJSON(w, code, map[string]string{"error": err.Error()})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func controlPlaneURL() string {
	if v := os.Getenv("CONTROL_PLANE_URL"); v != "" {
		return v
	}
	return "http://localhost:8081"
}

func enrichTaskContext(ctx common.RequestContext, input string) common.RequestContext {
	if ctx.ScenarioTag == "" || ctx.ScenarioTag == "general" {
		ctx.ScenarioTag = classifyTask(input)
	}
	ctx.TaskCategory = ctx.ScenarioTag
	return ctx
}

func classifyTask(text string) string {
	v := strings.ToLower(text)
	switch {
	case strings.Contains(v, "code"), strings.Contains(v, "bug"), strings.Contains(v, "sql"), strings.Contains(v, "api"), strings.Contains(v, "golang"), strings.Contains(v, "rust"):
		return "coding"
	case strings.Contains(v, "translate"), strings.Contains(v, "翻译"), strings.Contains(v, "润色"):
		return "translation"
	case strings.Contains(v, "summary"), strings.Contains(v, "总结"), strings.Contains(v, "分析"), strings.Contains(v, "report"):
		return "analysis"
	case strings.Contains(v, "runbook"), strings.Contains(v, "运维"), strings.Contains(v, "报警"), strings.Contains(v, "incident"):
		return "ops"
	default:
		return "general"
	}
}

func contentRetention(prompt string) (bool, string, string) {
	mode := strings.ToLower(getEnv("ZS_AUDIT_CONTENT_MODE", "metadata"))
	switch mode {
	case "full":
		return true, "summary-only", truncateForSummary(prompt)
	case "sampled":
		sampleRate := parseFloat(getEnv("ZS_AUDIT_SAMPLE_RATE", "0.1"), 0.1)
		if rand.Float64() <= sampleRate {
			return true, "summary-only", truncateForSummary(prompt)
		}
		return false, "metadata-only", ""
	default:
		return false, "metadata-only", ""
	}
}

func truncateForSummary(v string) string {
	v = strings.TrimSpace(v)
	if len(v) <= 140 {
		return v
	}
	return v[:140] + "..."
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func parseFloat(v string, fallback float64) float64 {
	out, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return fallback
	}
	if out < 0 {
		return 0
	}
	if out > 1 {
		return 1
	}
	return out
}
