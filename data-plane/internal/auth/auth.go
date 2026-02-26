package auth

import (
	"errors"
	"net/http"

	"zsgate/common"
)

var ErrUnauthorized = errors.New("missing X-ZS-API-Key")

func RequestContextFromHeaders(r *http.Request) (common.RequestContext, error) {
	if r.Header.Get("X-ZS-API-Key") == "" {
		return common.RequestContext{}, ErrUnauthorized
	}
	ctx := common.RequestContext{
		TraceID:     headerOr(r, "X-ZS-Trace-Id", ""),
		OrgID:       headerOr(r, "X-ZS-Org-Id", "org-default"),
		DeptID:      headerOr(r, "X-ZS-Dept-Id", "dept-default"),
		ProjectID:   headerOr(r, "X-ZS-Project-Id", "project-default"),
		UserID:      headerOr(r, "X-ZS-User-Id", "user-default"),
		ClientApp:   headerOr(r, "X-ZS-Client-App", "unknown-client"),
		ScenarioTag: headerOr(r, "X-ZS-Scenario", "general"),
	}
	if ctx.TraceID == "" {
		ctx.TraceID = r.Header.Get("X-Request-Id")
	}
	if ctx.TraceID == "" {
		ctx.TraceID = "trace-auto"
	}
	return ctx, nil
}

func headerOr(r *http.Request, key, fallback string) string {
	if v := r.Header.Get(key); v != "" {
		return v
	}
	return fallback
}
