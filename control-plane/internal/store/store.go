package store

import (
	"errors"
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
}

func New() *Store {
	return &Store{
		vendors:  make(map[string]common.Vendor),
		aliases:  make(map[string]common.ModelAlias),
		policies: make(map[string]common.RoutingPolicy),
		cursor:   make(map[string]int),
	}
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
