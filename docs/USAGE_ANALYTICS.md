# Community Usage Analytics

ZSGate OSS includes basic team-level usage analytics APIs.

## Per-user summary
- `GET /admin/usage/by-user`
- Returns per member:
  - request count
  - prompt/completion/total tokens
  - estimated cost (cents)
  - average latency
  - last seen time

## Per-user cost
- `GET /admin/costs/by-user`
- Returns `{ "user_id": cost_cents }`

## Existing breakdowns
- `GET /admin/costs/by-dept`
- `GET /admin/costs/by-project`
- `GET /admin/usage`
- `GET /admin/audits`
