#!/usr/bin/env bash
set -euo pipefail

BASE="${BASE:-http://localhost:8080}"

post() {
  curl -sS -X POST "$BASE$1" -H 'Content-Type: application/json' -d "$2"
  echo
}

echo "== health"
curl -sS "$BASE/healthz"
echo

echo "== renewal"
post /v1/recommendations '{"client_id":"smb-001"}'

echo "== cross-sell with promo"
post /v1/recommendations '{"client_id":"smb-002"}'

echo "== upsell with manager approval"
post /v1/recommendations '{"client_id":"smb-005"}'

echo "== ask: why"
post /v1/ask '{"client_id":"smb-002","question":"Почему векторный редактор?"}'

echo "== ask: off-topic refusal"
post /v1/ask '{"client_id":"smb-001","question":"Какая завтра погода?"}'

echo "== fallback"
post /v1/recommendations '{"client_id":"smb-006"}'
