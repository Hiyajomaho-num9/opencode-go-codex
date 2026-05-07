# opencode-go-codex

A local 2api tool that translates OpenCode Go Chat Completions into the
Responses API.

This tool currently targets DeepSeek V4 models only. It uses Kimi 2.6 and the
custom `web_search_mcp` tool to fill the image recognition and web search gaps.
GLM 5.1 support may be considered later if needed, mainly because the GLM 5.1
API interface is much more complex.

## Install

Linux:

```bash
git clone https://github.com/YOUR_NAME/opencode-go-codex.git
cd opencode-go-codex
./install.sh
```

Windows PowerShell:

```powershell
git clone https://github.com/YOUR_NAME/opencode-go-codex.git
cd opencode-go-codex
.\install.ps1
```

The Linux installer will:

- Write `~/.config/opencode-go-codex/env` with `0600` permissions.
- Install `~/.config/systemd/user/opencode-go-codex.service`.
- Point Codex at `examples/models/deepseek-only.json`.
- Add `deepseek-v4-pro` and `deepseek-v4-flash` profiles.
- Register the bundled web-search MCP fallback and local skill.
- Start the service and run a minimal Codex profile check.

The Windows installer will:

- Reuse `opencode-go-codex.exe` from the release package, or build it with Go if
  the binary is missing.
- Write user-level `OPENCODE_GO_*` environment variables.
- Point Codex at `examples/models/deepseek-only.json`.
- Add `deepseek-v4-pro` and `deepseek-v4-flash` profiles.
- Register the bundled web-search MCP fallback and local skill.

After installation:

```bash
codex -p deepseek-v4-pro
codex -p deepseek-v4-flash
```

## Build

The Go adapter is the primary implementation. Build a single local binary:

```bash
go build -o opencode-go-codex ./cmd/opencode-go-codex
```

Cross-compile for Windows:

```bash
GOOS=windows GOARCH=amd64 go build -o opencode-go-codex.exe ./cmd/opencode-go-codex
```

The Go adapter is the only service implementation. Python is only used by the
optional bundled `tools/web_search_mcp.py` helper.

## Start Manually

```bash
export OPENCODE_GO_API_KEY="..."
./start.sh
```

Optional environment variables:

```bash
export OPENCODE_GO_BASE_URL="https://opencode.ai/zen/go/v1/chat/completions"
export OPENCODE_GO_MODEL="deepseek-v4-pro"
export OPENCODE_GO_COMPACT_MODEL="deepseek-v4-flash"
export OPENCODE_GO_VISION_MODEL="kimi-k2.6"
export OPENCODE_GO_REASONING_EFFORT="max"
export OPENCODE_GO_CODEX_PORT="8768"
export OPENCODE_GO_TIMEOUT="900"
export OPENCODE_GO_DEBUG_ROUTING="1"
export OPENCODE_GO_TRACE_DIR="/tmp/opencode-go-codex-trace"
```

Routing rules:

- Normal Codex requests use `OPENCODE_GO_MODEL`, default `deepseek-v4-pro`.
- `/compact` uses `OPENCODE_GO_COMPACT_MODEL`, default `deepseek-v4-flash`.
- Requests containing image input anywhere in the Codex request use `OPENCODE_GO_VISION_MODEL`, default `kimi-k2.6`.
- `deepseek-v4-pro` and `deepseek-v4-flash` default to
  `reasoning_effort: "max"`, the full reasoning tier. Codex
  `model_reasoning_effort = "xhigh"` is also mapped to OpenCode Go's `"max"`.

See `docs/deepseek-api-notes.md` for the exact DeepSeek-style Chat Completions
fields forwarded by the adapter.

More compatibility notes:

- `docs/codex-compatibility.md`
- `docs/tool-call-semantics.md`
- `docs/model-capability-matrix.md`
- `docs/troubleshooting.md`
- `docs/testing.md`

## Codex Config

The installer updates `~/.codex/config.toml` automatically. For manual setup,
add the contents of `examples/codex/profile.example.toml` to your config, then run:

```bash
codex -p opencode-go -m deepseek-v4-pro
```

The recommended model catalog is the DeepSeek-only catalog:

```toml
model_catalog_json = "/path/to/opencode-go-codex/examples/models/deepseek-only.json"
```

It only exposes `deepseek-v4-pro` and `deepseek-v4-flash`, defaults them to
`xhigh`, uses low verbosity, and sets Codex context knobs to 512k context with
a 400k auto-compact threshold.

The reason this does not use 1M context by default is that DeepSeek's
hallucination rate becomes too high after roughly 800k tokens. 512k is enough
for current usage. If you need to search PDF contents, set context to 800k and
auto-compact to 720k.

If `--search` does not expose web search, you can also point Codex at the
example model catalog:

```toml
model_catalog_json = "/path/to/opencode-go-codex/examples/models/opencode-go.example.json"
```

The example catalog marks `deepseek-v4-pro` and `deepseek-v4-flash` as
`supports_search_tool: true`, and marks `kimi-k2.6` as text+image capable.

On Codex 0.126, native `--search` may still be withheld from custom providers.
Use the bundled MCP fallback if you need stable web search with OpenCode Go:

```bash
codex mcp add web-search -- /path/to/opencode-go-codex/tools/web_search_mcp.py
```

The installer also installs a matching local skill at
`~/.codex/skills/web-search-mcp`. If Codex CLI mode does not expose arbitrary
MCP tools directly, invoke `$web-search-mcp`; it calls the same backend through
shell JSON-RPC.

For one-off testing without editing `~/.codex/config.toml`:

```bash
export OPENAI_API_KEY="$OPENCODE_GO_API_KEY"
codex exec --skip-git-repo-check --ephemeral \
  -c 'model_provider="OpenCodeGo"' \
  -c 'model="deepseek-v4-flash"' \
  -c 'model_providers.OpenCodeGo.name="OpenCodeGo"' \
  -c 'model_providers.OpenCodeGo.base_url="http://127.0.0.1:8768/v1"' \
  -c 'model_providers.OpenCodeGo.wire_api="responses"' \
  'Reply exactly OK.'
```

## systemd User Service

Copy `examples/systemd/opencode-go-codex.service` to
`~/.config/systemd/user/opencode-go-codex.service`, replace the key line or use
an `EnvironmentFile`, then run:

```bash
systemctl --user daemon-reload
systemctl --user enable --now opencode-go-codex.service
```

## Verify

```bash
curl http://127.0.0.1:8768/healthz
```

```bash
curl -N http://127.0.0.1:8768/v1/responses \
  -H "Authorization: Bearer $OPENCODE_GO_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"model":"deepseek-v4-flash","input":"Reply exactly OK.","stream":false,"max_output_tokens":32}'
```

## Debug And Test

Check local setup:

```bash
./opencode-go-codex doctor
```

Run local regression tests:

```bash
go test ./...
go build -o opencode-go-codex ./cmd/opencode-go-codex
```

Run a smoke test against a running adapter:

```bash
./smoke.sh
```

Replay or summarize a trace:

```bash
./opencode-go-codex replay /tmp/opencode-go-codex-trace/TRACE_ID --summary
./opencode-go-codex replay /tmp/opencode-go-codex-trace/TRACE_ID
```
