#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
HOST="${OPENCODE_GO_CODEX_HOST:-127.0.0.1}"
PORT="${OPENCODE_GO_CODEX_PORT:-8768}"
BASE="http://$HOST:$PORT"
AUTH="${OPENCODE_GO_API_KEY:-${OPENAI_API_KEY:-dummy-local-key}}"

python3 -m compileall -q "$ROOT/adapter" "$ROOT/opencode_go_codex.py" "$ROOT/tools/web_search_mcp.py"
python3 -m unittest discover -s "$ROOT/tests"

if ! curl -fsS "$BASE/healthz" >/dev/null; then
  echo "adapter is not reachable at $BASE" >&2
  echo "start it first: OPENCODE_GO_API_KEY=... ./start.sh" >&2
  exit 1
fi

curl -fsS -N "$BASE/v1/responses" \
  -H "Authorization: Bearer $AUTH" \
  -H "Content-Type: application/json" \
  -d '{"model":"deepseek-v4-flash","input":"Reply exactly SMOKE_OK.","stream":false,"max_output_tokens":32}' \
  | grep -q 'response.completed'

echo "smoke ok"
