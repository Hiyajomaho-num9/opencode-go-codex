#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CODEX_HOME="${CODEX_HOME:-$HOME/.codex}"
CONFIG="$CODEX_HOME/config.toml"
MODEL_CATALOG="$ROOT/examples/models/deepseek-only.json"
ENV_DIR="$HOME/.config/opencode-go-codex"
ENV_FILE="$ENV_DIR/env"
UNIT_DIR="$HOME/.config/systemd/user"
UNIT_FILE="$UNIT_DIR/opencode-go-codex.service"
MCP_SCRIPT="$ROOT/tools/web_search_mcp.py"

say() {
  printf '%s\n' "$*"
}

need_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    say "Missing required command: $1" >&2
    exit 1
  fi
}

read_key() {
  if [[ -n "${OPENCODE_GO_API_KEY:-}" ]]; then
    return
  fi
  printf 'OpenCode Go API key: '
  stty -echo
  IFS= read -r OPENCODE_GO_API_KEY
  stty echo
  printf '\n'
  if [[ -z "$OPENCODE_GO_API_KEY" ]]; then
    say "API key is required." >&2
    exit 1
  fi
  export OPENCODE_GO_API_KEY
}

write_env() {
  say "Writing service environment: $ENV_FILE"
  mkdir -p "$ENV_DIR"
  umask 077
  {
    printf 'OPENCODE_GO_API_KEY=%s\n' "$OPENCODE_GO_API_KEY"
    printf 'OPENCODE_GO_CODEX_HOST=127.0.0.1\n'
    printf 'OPENCODE_GO_CODEX_PORT=8768\n'
    printf 'OPENCODE_GO_BASE_URL=https://opencode.ai/zen/go/v1/chat/completions\n'
    printf 'OPENCODE_GO_MODEL=deepseek-v4-pro\n'
    printf 'OPENCODE_GO_COMPACT_MODEL=deepseek-v4-flash\n'
    printf 'OPENCODE_GO_VISION_MODEL=kimi-k2.6\n'
    printf 'OPENCODE_GO_REASONING_EFFORT=max\n'
    printf 'OPENCODE_GO_THINKING=enabled\n'
    printf 'OPENCODE_GO_TIMEOUT=900\n'
    printf 'OPENCODE_GO_DEBUG_ROUTING=1\n'
  } > "$ENV_FILE"
  chmod 600 "$ENV_FILE"
}

write_service() {
  say "Installing systemd user service: $UNIT_FILE"
  mkdir -p "$UNIT_DIR"
  cat > "$UNIT_FILE" <<EOF
[Unit]
Description=OpenCode Go adapter for Codex
After=network-online.target

[Service]
Type=simple
WorkingDirectory=$ROOT
EnvironmentFile=$ENV_FILE
ExecStart=$ROOT/start.sh
Restart=on-failure
RestartSec=3

[Install]
WantedBy=default.target
EOF
  chmod 644 "$UNIT_FILE"
  systemd-analyze --user verify "$UNIT_FILE"
  systemctl --user daemon-reload
}

patch_config() {
  say "Updating Codex config: $CONFIG"
  mkdir -p "$CODEX_HOME"
  if [[ -f "$CONFIG" ]]; then
    cp "$CONFIG" "$CONFIG.bak.$(date +%Y%m%d%H%M%S)"
  else
    : > "$CONFIG"
  fi

  python3 - "$CONFIG" "$MODEL_CATALOG" "$MCP_SCRIPT" <<'PY'
import re
import sys
from pathlib import Path

config = Path(sys.argv[1])
model_catalog = sys.argv[2]
mcp_script = sys.argv[3]
text = config.read_text(encoding="utf-8")

def set_root(key, value):
    global text
    line = f'{key} = "{value}"'
    pattern = re.compile(rf'^{re.escape(key)}\s*=.*$', re.M)
    if pattern.search(text):
        text = pattern.sub(line, text, count=1)
        return
    first_table = re.search(r'^\[', text, re.M)
    if first_table:
        text = text[:first_table.start()] + line + "\n" + text[first_table.start():]
    else:
        text = text.rstrip() + "\n" + line + "\n"

def set_root_raw(key, value):
    global text
    line = f'{key} = {value}'
    pattern = re.compile(rf'^{re.escape(key)}\s*=.*$', re.M)
    if pattern.search(text):
        text = pattern.sub(line, text, count=1)
        return
    first_table = re.search(r'^\[', text, re.M)
    if first_table:
        text = text[:first_table.start()] + line + "\n" + text[first_table.start():]
    else:
        text = text.rstrip() + "\n" + line + "\n"

def remove_table(name):
    global text
    pattern = re.compile(rf'^\[{re.escape(name)}\]\n.*?(?=^\[|\Z)', re.M | re.S)
    text = pattern.sub("", text)

def append_table(name, body):
    global text
    remove_table(name)
    text = text.rstrip() + f"\n\n[{name}]\n{body.rstrip()}\n"

set_root("model_catalog_json", model_catalog)
set_root("model_reasoning_effort", "xhigh")
set_root("model_verbosity", "low")
set_root_raw("model_context_window", "512000")
set_root_raw("model_auto_compact_token_limit", "400000")

append_table("model_providers.OpenCodeGo", '''
name = "OpenCodeGo"
base_url = "http://127.0.0.1:8768/v1"
wire_api = "responses"
''')

append_table("profiles.deepseek-v4-pro", '''
model_provider = "OpenCodeGo"
model = "deepseek-v4-pro"
model_reasoning_effort = "xhigh"
model_verbosity = "low"
model_context_window = 512000
model_auto_compact_token_limit = 400000
''')

append_table("profiles.deepseek-v4-flash", '''
model_provider = "OpenCodeGo"
model = "deepseek-v4-flash"
model_reasoning_effort = "xhigh"
model_verbosity = "low"
model_context_window = 512000
model_auto_compact_token_limit = 400000
''')

append_table("mcp_servers.web-search", f'''
command = "{mcp_script}"
''')

config.write_text(text.strip() + "\n", encoding="utf-8")
PY
}

install_skill() {
  local skill_dir="$CODEX_HOME/skills/web-search-mcp"
  say "Installing web-search fallback skill: $skill_dir"
  mkdir -p "$skill_dir"
  cat > "$skill_dir/SKILL.md" <<EOF
---
name: web-search-mcp
description: Use when native Codex web_search is unavailable with the OpenCode Go custom provider. Calls the local web_search MCP-compatible script through shell JSON-RPC.
---

Use this shell JSON-RPC command to search the web:

\`\`\`bash
printf '%s\n' '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"web_search","arguments":{"query":"QUERY HERE","max_results":5}}}' | "$MCP_SCRIPT"
\`\`\`

Retry once after a short sleep if the backend returns a transient network or rate-limit error.
EOF
}

start_service() {
  say "Starting service"
  systemctl --user enable --now opencode-go-codex.service
  sleep 1
  systemctl --user --no-pager --full status opencode-go-codex.service || true
  curl -fsS http://127.0.0.1:8768/healthz >/dev/null
}

verify_codex() {
  if ! command -v codex >/dev/null 2>&1; then
    say "Codex CLI not found; skipping Codex profile verification."
    return
  fi
  say "Verifying Codex profiles"
  OPENAI_API_KEY=dummy-local-key codex exec --skip-git-repo-check --ephemeral -p deepseek-v4-pro --json 'Reply exactly INSTALL_PRO_OK.' >/tmp/opencode-go-codex-pro.jsonl
  OPENAI_API_KEY=dummy-local-key codex exec --skip-git-repo-check --ephemeral -p deepseek-v4-flash --json 'Reply exactly INSTALL_FLASH_OK.' >/tmp/opencode-go-codex-flash.jsonl
  grep -q 'INSTALL_PRO_OK' /tmp/opencode-go-codex-pro.jsonl
  grep -q 'INSTALL_FLASH_OK' /tmp/opencode-go-codex-flash.jsonl
}

main() {
  need_cmd python3
  need_cmd systemctl
  need_cmd systemd-analyze
  need_cmd curl

  say "OpenCode Go Codex installer"
  say "Project: $ROOT"
  say "This will write a user service, service env, Codex profiles, and a DeepSeek-only model catalog."

  read_key
  write_env
  write_service
  patch_config
  install_skill
  start_service
  verify_codex

  say "Install complete."
  say "Use: codex -p deepseek-v4-pro"
  say "Use: codex -p deepseek-v4-flash"
}

main "$@"
