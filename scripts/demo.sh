#!/usr/bin/env bash
set -euo pipefail

BASE=${BASE:-http://localhost:8080}
ADMIN=${ADMIN:-http://localhost:8081}

echo "== Send chat request"
curl -sS -X POST "$BASE/v1/chat/completions" \
  -H 'Content-Type: application/json' \
  -H 'X-ZS-API-Key: demo-key' \
  -H 'X-ZS-Trace-Id: trace-demo-1' \
  -H 'X-ZS-User-Id: alice' \
  -H 'X-ZS-Dept-Id: engineering' \
  -H 'X-ZS-Project-Id: zsgate' \
  -H 'X-ZS-Scenario: coding' \
  -d '{"model":"gpt-4o-prod","messages":[{"role":"user","content":"hello from zsgate"}]}' | sed 's/.*/&\n/'

echo "== Team usage by user"
curl -sS "$ADMIN/admin/usage/by-user" | sed 's/.*/&\n/'

echo "== Cost by user"
curl -sS "$ADMIN/admin/costs/by-user" | sed 's/.*/&\n/'

echo "== Audits"
curl -sS "$ADMIN/admin/audits" | sed 's/.*/&\n/'
