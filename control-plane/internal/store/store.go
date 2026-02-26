package store

import (
	"errors"
	"fmt"
	"math"
	"math/rand"
	"sort"
	"sync"
	"time"

	"zsgate/common"
)

var ErrNotFound = errors.New("not found")

type Store struct {
	mu sync.RWMutex

	vendors  map[string]common.Vendor
	aliases  map[string]common.ModelAlias
	policies map[string]common.RoutingPolicy

	usage  []common.UsageEvent
	audit  []common.AuditEvent
	cursor map[string]int

	alertRules  map[string]AlertRule
	alertEvents []AlertEvent
	lastAlertAt map[string]time.Time
}

func New() *Store {
	return &Store{
		vendors:     make(map[string]common.Vendor),
		aliases:     make(map[string]common.ModelAlias),
		policies:    make(map[string]common.RoutingPolicy),
		cursor:      make(map[string]int),
		alertRules:  make(map[string]AlertRule),
		lastAlertAt: make(map[string]time.Time),
	}
}

type AlertRule struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	Metric          string    `json:"metric"`
	Threshold       float64   `json:"threshold"`
	WindowSeconds   int       `json:"window_seconds"`
	Severity        string    `json:"severity"`
	Enabled         bool      `json:"enabled"`
	WebhookURL      string    `json:"webhook_url,omitempty"`
	CooldownSeconds int       `json:"cooldown_seconds"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type AlertEvent struct {
	ID        string    `json:"id"`
	RuleID    string    `json:"rule_id"`
	RuleName  string    `json:"rule_name"`
	Metric    string    `json:"metric"`
	Value     float64   `json:"value"`
	Threshold float64   `json:"threshold"`
	Severity  string    `json:"severity"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

type AlertTrigger struct {
	Event      AlertEvent `json:"event"`
	WebhookURL string     `json:"-"`
}

func (s *Store) UpsertVendor(v common.Vendor) common.Vendor {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	if old, ok := s.vendors[v.ID]; ok {
		v.CreatedAt = old.CreatedAt
	} else {
		v.CreatedAt = now
	}
	v.UpdatedAt = now
	if v.HealthState == "" {
		v.HealthState = "unknown"
	}
	s.vendors[v.ID] = v
	return v
}

func (s *Store) ListVendors() []common.Vendor {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]common.Vendor, 0, len(s.vendors))
	for _, v := range s.vendors {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (s *Store) GetVendor(id string) (common.Vendor, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.vendors[id]
	if !ok {
		return common.Vendor{}, ErrNotFound
	}
	return v, nil
}

func (s *Store) DisableVendor(id string) (common.Vendor, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.vendors[id]
	if !ok {
		return common.Vendor{}, ErrNotFound
	}
	v.Status = common.VendorDisabled
	v.UpdatedAt = time.Now().UTC()
	s.vendors[id] = v
	return v, nil
}

func (s *Store) SetVendorHealth(id, state string) (common.Vendor, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.vendors[id]
	if !ok {
		return common.Vendor{}, ErrNotFound
	}
	v.HealthState = state
	v.UpdatedAt = time.Now().UTC()
	s.vendors[id] = v
	return v, nil
}

func (s *Store) UpsertAlias(a common.ModelAlias) common.ModelAlias {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	if old, ok := s.aliases[a.ID]; ok {
		a.CreatedAt = old.CreatedAt
	} else {
		a.CreatedAt = now
	}
	a.UpdatedAt = now
	s.aliases[a.ID] = a
	return a
}

func (s *Store) ListAliases() []common.ModelAlias {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]common.ModelAlias, 0, len(s.aliases))
	for _, a := range s.aliases {
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Alias == out[j].Alias {
			return out[i].Priority < out[j].Priority
		}
		return out[i].Alias < out[j].Alias
	})
	return out
}

func (s *Store) UpsertPolicy(p common.RoutingPolicy) common.RoutingPolicy {
	s.mu.Lock()
	defer s.mu.Unlock()
	p.UpdatedAt = time.Now().UTC()
	s.policies[p.ID] = p
	return p
}

func (s *Store) ListPolicies() []common.RoutingPolicy {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]common.RoutingPolicy, 0, len(s.policies))
	for _, p := range s.policies {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (s *Store) AddUsage(e common.UsageEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.usage = append(s.usage, e)
}

func (s *Store) AddAudit(e common.AuditEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.audit = append(s.audit, e)
}

func (s *Store) UpsertAlertRule(rule AlertRule) AlertRule {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !rule.Enabled {
		rule.Enabled = true
	}
	if rule.WindowSeconds <= 0 {
		rule.WindowSeconds = 300
	}
	if rule.Severity == "" {
		rule.Severity = "warning"
	}
	if rule.CooldownSeconds <= 0 {
		rule.CooldownSeconds = 120
	}
	rule.UpdatedAt = time.Now().UTC()
	s.alertRules[rule.ID] = rule
	return rule
}

func (s *Store) ListAlertRules() []AlertRule {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]AlertRule, 0, len(s.alertRules))
	for _, r := range s.alertRules {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (s *Store) ListAlertEvents(limit int) []AlertEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if limit <= 0 {
		limit = 100
	}
	if limit > len(s.alertEvents) {
		limit = len(s.alertEvents)
	}
	out := make([]AlertEvent, 0, limit)
	for i := len(s.alertEvents) - 1; i >= 0 && len(out) < limit; i-- {
		out = append(out, s.alertEvents[i])
	}
	return out
}

func (s *Store) EvaluateAlerts(now time.Time) []AlertTrigger {
	s.mu.Lock()
	defer s.mu.Unlock()

	triggers := make([]AlertTrigger, 0)
	for _, rule := range s.alertRules {
		if !rule.Enabled {
			continue
		}
		windowStart := now.Add(-time.Duration(rule.WindowSeconds) * time.Second)
		window := make([]common.UsageEvent, 0)
		for _, e := range s.usage {
			if e.Timestamp.After(windowStart) {
				window = append(window, e)
			}
		}
		if len(window) == 0 {
			continue
		}

		value, ok := evaluateMetric(rule.Metric, window)
		if !ok || value <= rule.Threshold {
			continue
		}

		lastAt := s.lastAlertAt[rule.ID]
		if !lastAt.IsZero() && now.Sub(lastAt) < time.Duration(rule.CooldownSeconds)*time.Second {
			continue
		}

		event := AlertEvent{
			ID:        fmt.Sprintf("%s-%d", rule.ID, now.UnixNano()),
			RuleID:    rule.ID,
			RuleName:  rule.Name,
			Metric:    rule.Metric,
			Value:     value,
			Threshold: rule.Threshold,
			Severity:  rule.Severity,
			Message:   fmt.Sprintf("%s triggered: %.2f > %.2f", rule.Metric, value, rule.Threshold),
			Timestamp: now,
		}
		s.alertEvents = append(s.alertEvents, event)
		if len(s.alertEvents) > 2000 {
			s.alertEvents = s.alertEvents[len(s.alertEvents)-2000:]
		}
		s.lastAlertAt[rule.ID] = now
		triggers = append(triggers, AlertTrigger{Event: event, WebhookURL: rule.WebhookURL})
	}
	return triggers
}

func evaluateMetric(metric string, usage []common.UsageEvent) (float64, bool) {
	switch metric {
	case "error_rate":
		total := len(usage)
		if total == 0 {
			return 0, false
		}
		failed := 0
		for _, e := range usage {
			if e.Status != "ok" {
				failed++
			}
		}
		return (float64(failed) / float64(total)) * 100, true
	case "cost_spike":
		sum := 0
		for _, e := range usage {
			sum += e.CostEstimateCents
		}
		return float64(sum), true
	case "latency_p95":
		latencies := make([]int64, 0, len(usage))
		for _, e := range usage {
			latencies = append(latencies, e.LatencyMs)
		}
		if len(latencies) == 0 {
			return 0, false
		}
		sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
		idx := int(math.Ceil(float64(len(latencies))*0.95)) - 1
		if idx < 0 {
			idx = 0
		}
		if idx >= len(latencies) {
			idx = len(latencies) - 1
		}
		return float64(latencies[idx]), true
	default:
		return 0, false
	}
}

func (s *Store) Usage() []common.UsageEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]common.UsageEvent, len(s.usage))
	copy(out, s.usage)
	return out
}

func (s *Store) Audit(filter map[string]string) []common.AuditEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]common.AuditEvent, 0, len(s.audit))
	for _, e := range s.audit {
		if filter["user_id"] != "" && e.UserID != filter["user_id"] {
			continue
		}
		if filter["project_id"] != "" && e.ProjectID != filter["project_id"] {
			continue
		}
		if filter["trace_id"] != "" && e.TraceID != filter["trace_id"] {
			continue
		}
		if filter["scenario"] != "" && e.ScenarioTag != filter["scenario"] {
			continue
		}
		out = append(out, e)
	}
	return out
}

func (s *Store) AuditSince(offset int) ([]common.AuditEvent, int) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if offset < 0 {
		offset = 0
	}
	if offset > len(s.audit) {
		offset = len(s.audit)
	}
	out := make([]common.AuditEvent, len(s.audit[offset:]))
	copy(out, s.audit[offset:])
	return out, len(s.audit)
}

type RouteTarget struct {
	Alias       string `json:"alias"`
	VendorID    string `json:"vendor_id"`
	VendorType  string `json:"vendor_type"`
	Endpoint    string `json:"endpoint"`
	VendorModel string `json:"vendor_model"`
}

func (s *Store) ResolveRoute(alias string) (RouteTarget, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	candidates := make([]common.ModelAlias, 0)
	for _, a := range s.aliases {
		if a.Alias != alias || !a.Enabled {
			continue
		}
		v, ok := s.vendors[a.VendorID]
		if !ok || v.Status != common.VendorEnabled {
			continue
		}
		candidates = append(candidates, a)
	}
	if len(candidates) == 0 {
		return RouteTarget{}, ErrNotFound
	}

	weighted := make([]common.ModelAlias, 0)
	for _, c := range candidates {
		w := c.Weight
		if w <= 0 {
			w = 1
		}
		for i := 0; i < w; i++ {
			weighted = append(weighted, c)
		}
	}
	if len(weighted) == 0 {
		return RouteTarget{}, ErrNotFound
	}

	idx := s.cursor[alias]
	if idx >= len(weighted) {
		idx = rand.Intn(len(weighted))
	}
	pick := weighted[idx]
	s.cursor[alias] = (idx + 1) % len(weighted)
	vendor := s.vendors[pick.VendorID]

	return RouteTarget{
		Alias:       alias,
		VendorID:    vendor.ID,
		VendorType:  string(vendor.Type),
		Endpoint:    vendor.Endpoint,
		VendorModel: pick.VendorModel,
	}, nil
}
