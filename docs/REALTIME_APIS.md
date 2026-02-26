# Realtime APIs (OSS)

## Active users
- `GET /admin/realtime/active-users`
- Window: 15 minutes
- Includes per-user request counts in 1m/5m and latest project/scenario

## Active tasks
- `GET /admin/realtime/active-tasks`
- Window: 5 minutes
- Aggregated by `task_category` / `scenario_tag`

## Snapshot stream
- `GET /admin/realtime/stream`
- SSE event `snapshot` every 2 seconds

## Dialog stream
- `GET /admin/realtime/dialog-stream`
- SSE event `dialog` for new conversation events
- Supports `?from=tail` to only stream new events from connection time
- Default payload is metadata + sanitized summary
