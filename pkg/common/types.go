package common

import "time"

type VendorType string

const (
	VendorOpenAI VendorType = "openai"
	VendorClaude VendorType = "claude"
	VendorGemini VendorType = "gemini"
	VendorVLLM   VendorType = "vllm"
)

type VendorStatus string

const (
	VendorEnabled  VendorStatus = "enabled"
	VendorDisabled VendorStatus = "disabled"
)

type RequestContext struct {
	TraceID     string `json:"trace_id"`
	OrgID       string `json:"org_id"`
	DeptID      string `json:"dept_id"`
	ProjectID   string `json:"project_id"`
	UserID      string `json:"user_id"`
	ClientApp   string `json:"client_app"`
	ScenarioTag string `json:"scenario_tag"`
}

type UsageEvent struct {
	TraceID           string    `json:"trace_id"`
	Timestamp         time.Time `json:"timestamp"`
	Provider          string    `json:"provider"`
	Model             string    `json:"model"`
	PromptTokens      int       `json:"prompt_tokens"`
	CompletionTokens  int       `json:"completion_tokens"`
	CostEstimateCents int       `json:"cost_estimate_cents"`
	LatencyMs         int64     `json:"latency_ms"`
	Status            string    `json:"status"`
	RequestContext
}

type AuditEvent struct {
	TraceID        string    `json:"trace_id"`
	Timestamp      time.Time `json:"timestamp"`
	Action         string    `json:"action"`
	RequestMeta    string    `json:"request_meta"`
	ResponseMeta   string    `json:"response_meta"`
	ContentHash    string    `json:"content_hash"`
	RedactionLevel string    `json:"redaction_level"`
	RiskFlags      []string  `json:"risk_flags"`
	RequestContext
}

type Vendor struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Type        VendorType   `json:"type"`
	Region      string       `json:"region"`
	Endpoint    string       `json:"endpoint"`
	Status      VendorStatus `json:"status"`
	Credential  string       `json:"credential,omitempty"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
	HealthState string       `json:"health_state"`
}

type ModelAlias struct {
	ID          string    `json:"id"`
	Alias       string    `json:"alias"`
	VendorID    string    `json:"vendor_id"`
	VendorModel string    `json:"vendor_model"`
	Weight      int       `json:"weight"`
	Priority    int       `json:"priority"`
	Enabled     bool      `json:"enabled"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type RoutingPolicy struct {
	ID           string    `json:"id"`
	Alias        string    `json:"alias"`
	Strategy     string    `json:"strategy"`
	FailoverMode string    `json:"failover_mode"`
	UpdatedAt    time.Time `json:"updated_at"`
}
