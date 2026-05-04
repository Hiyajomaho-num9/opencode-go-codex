#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")"

if [[ -z "${OPENCODE_GO_API_KEY:-}" && -z "${OPENAI_API_KEY:-}" ]]; then
  echo "Set OPENCODE_GO_API_KEY first." >&2
  exit 1
fi

exec python3 ./opencode_go_codex.py "$@"
