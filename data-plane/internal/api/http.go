package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
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

	audit := common.AuditEvent{
		TraceID:        ctxMeta.TraceID,
		Timestamp:      time.Now().UTC(),
		Action:         "chat.completions",
		RequestMeta:    fmt.Sprintf("model=%s provider=%s", req.Model, target.VendorID),
		ResponseMeta:   fmt.Sprintf("completion_tokens=%d", out.CompletionTokens),
		ContentHash:    hash(prompt),
		RedactionLevel: "metadata-only",
		RiskFlags:      nil,
		RequestContext: ctxMeta,
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
	s.cp.EmitAudit(r.Context(), common.AuditEvent{
		TraceID:        ctxMeta.TraceID,
		Timestamp:      time.Now().UTC(),
		Action:         "mcp.call",
		RequestMeta:    "mcp_request",
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
