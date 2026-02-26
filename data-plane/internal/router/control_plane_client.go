package router

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"zsgate/common"
)

type RouteTarget struct {
	Alias       string `json:"alias"`
	VendorID    string `json:"vendor_id"`
	VendorType  string `json:"vendor_type"`
	Endpoint    string `json:"endpoint"`
	VendorModel string `json:"vendor_model"`
}

type ControlPlaneClient struct {
	baseURL string
	http    *http.Client
}

func NewControlPlaneClient(baseURL string) *ControlPlaneClient {
	return &ControlPlaneClient{
		baseURL: baseURL,
		http:    &http.Client{Timeout: 8 * time.Second},
	}
}

func (c *ControlPlaneClient) ResolveRoute(ctx context.Context, alias string) (RouteTarget, error) {
	u, err := url.Parse(c.baseURL + "/internal/routing")
	if err != nil {
		return RouteTarget{}, err
	}
	q := u.Query()
	q.Set("alias", alias)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return RouteTarget{}, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return RouteTarget{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return RouteTarget{}, fmt.Errorf("route resolve failed: %s", resp.Status)
	}
	var target RouteTarget
	if err := json.NewDecoder(resp.Body).Decode(&target); err != nil {
		return RouteTarget{}, err
	}
	return target, nil
}

func (c *ControlPlaneClient) EmitUsage(ctx context.Context, e common.UsageEvent) {
	_ = c.postJSON(ctx, "/internal/events/usage", e)
}

func (c *ControlPlaneClient) EmitAudit(ctx context.Context, e common.AuditEvent) {
	_ = c.postJSON(ctx, "/internal/events/audit", e)
}

func (c *ControlPlaneClient) postJSON(ctx context.Context, path string, payload any) error {
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("emit failed: %s", resp.Status)
	}
	return nil
}
