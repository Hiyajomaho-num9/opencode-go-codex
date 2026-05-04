# opencode-go-codex

Local adapter that lets Codex use an OpenCode Go subscription.

Codex 0.126 only accepts `wire_api = "responses"` for custom providers, while
OpenCode Go exposes OpenAI-compatible Chat Completions. This proxy exposes
`/v1/responses` locally and forwards requests to OpenCode Go
`/v1/chat/completions`.

## Install

Run the installer and enter your OpenCode Go key when prompted:

```bash
/home/kuro/my_code/opencode-go-codex/install.sh
```

It will:

- Write `~/.config/opencode-go-codex/env` with `0600` permissions.
- Install `~/.config/systemd/user/opencode-go-codex.service`.
- Point Codex at `models.deepseek-only.json`.
- Add `deepseek-v4-pro` and `deepseek-v4-flash` profiles.
- Register the bundled web-search MCP fallback and local skill.
- Start the service and run a minimal Codex profile check.

After install:

```bash
codex -p deepseek-v4-pro
codex -p deepseek-v4-flash
```

## Start Manually

```bash
export OPENCODE_GO_API_KEY="..."
/home/kuro/my_code/opencode-go-codex/start.sh
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
```

Routing:

- Normal Codex requests use `OPENCODE_GO_MODEL`, default `deepseek-v4-pro`.
- `/v1/responses/compact` uses `OPENCODE_GO_COMPACT_MODEL`, default `deepseek-v4-flash`.
- Requests containing image input anywhere in the Codex request use `OPENCODE_GO_VISION_MODEL`, default `kimi-k2.6`.
- `deepseek-v4-pro` and `deepseek-v4-flash` default to
  `reasoning_effort: "max"` so they run in the full reasoning tier. Codex
  `model_reasoning_effort = "xhigh"` is also mapped to OpenCode Go's `"max"`
  value.

See `DEEPSEEK_API_NOTES.md` for the exact DeepSeek-style Chat Completions
fields forwarded by the adapter.

## Codex Config

The installer updates `~/.codex/config.toml` automatically. For manual setup,
add the contents of `codex-profile.example.toml`, then run:

```bash
codex -p opencode-go -m deepseek-v4-pro
```

The recommended model catalog is the DeepSeek-only one:

```toml
model_catalog_json = "/home/kuro/my_code/opencode-go-codex/models.deepseek-only.json"
```

It only exposes `deepseek-v4-pro` and `deepseek-v4-flash`, defaults them to
`xhigh`, uses low verbosity, and sets the Codex context knobs for 512k context
with a 400k auto-compact threshold.

If web search is not exposed with `--search`, point Codex at the example model
catalog as well:

```toml
model_catalog_json = "/home/kuro/my_code/opencode-go-codex/models.opencode-go.example.json"
```

The example catalog marks `deepseek-v4-pro` and `deepseek-v4-flash` as
`supports_search_tool: true`, and marks `kimi-k2.6` as text+image capable.

On Codex 0.126, native `--search` may still be withheld from custom providers.
Use the bundled MCP fallback when you need stable web search with OpenCode Go:

```bash
codex mcp add web-search -- /home/kuro/my_code/opencode-go-codex/web_search_mcp.py
```

A matching local skill is installed at `~/.codex/skills/web-search-mcp`. If
Codex does not expose arbitrary MCP tools directly in CLI mode, invoke
`$web-search-mcp`; it calls the same backend through shell JSON-RPC.

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

## Systemd User Service

Copy `opencode-go-codex.service.example` to
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
