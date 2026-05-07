#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")"

if [[ -z "${OPENCODE_GO_API_KEY:-}" && -z "${OPENAI_API_KEY:-}" ]]; then
  echo "Set OPENCODE_GO_API_KEY first." >&2
  exit 1
fi

if [[ -x ./opencode-go-codex ]]; then
  exec ./opencode-go-codex "$@"
fi

exec go run ./cmd/opencode-go-codex "$@"
