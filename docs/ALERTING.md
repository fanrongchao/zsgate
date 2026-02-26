# Basic Alerting (OSS)

ZSGate OSS includes baseline threshold alerting with optional webhook delivery.

## Rule API
- `GET /admin/alerts/rules`
- `POST /admin/alerts/rules`

Rule fields:
- `id`, `name`, `metric`, `threshold`
- `window_seconds` (default 300)
- `severity` (`warning`/`critical`)
- `enabled`
- `webhook_url` (optional)
- `cooldown_seconds` (default 120)

## Supported metrics
- `error_rate`: percentage in selected window (0-100)
- `latency_p95`: p95 latency in milliseconds
- `cost_spike`: total estimated cost (cents) in window

## Event API
- `GET /admin/alerts/events?limit=100`
- `POST /admin/alerts/evaluate` (manual evaluation)

Automatic evaluation:
- Triggered whenever new usage events are ingested (`/internal/events/usage`).

## Example rule
```json
{
  "id": "alert-error-rate",
  "name": "Error rate high",
  "metric": "error_rate",
  "threshold": 5,
  "window_seconds": 300,
  "severity": "critical",
  "enabled": true,
  "webhook_url": "https://example.com/hooks/zsgate"
}
```
