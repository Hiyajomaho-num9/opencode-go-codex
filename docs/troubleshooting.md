# Troubleshooting

## `unknown variant image_url`

Example:

```text
Failed to deserialize the JSON body into the target type:
messages[7]: unknown variant `image_url`, expected `text`
```

Cause: an image request was routed to a DeepSeek V4 text model.

Fix:

1. Confirm `OPENCODE_GO_VISION_MODEL=kimi-k2.6`.
2. Enable routing logs with `OPENCODE_GO_DEBUG_ROUTING=1`.
3. Set `OPENCODE_GO_TRACE_DIR=/tmp/opencode-go-codex-trace` and inspect
   `meta.json` plus `upstream_chat_request.json`.

Expected routing:

```text
has_image=true model=kimi-k2.6
```

## Thinking Is Not Enabled

Expected DeepSeek V4 request shape:

```json
{
  "thinking": {"type": "enabled"},
  "reasoning_effort": "max"
}
```

If the model says it is not thinking:

1. Check `upstream_chat_request.json` in a trace.
2. Confirm the selected model starts with `deepseek-v4`.
3. Confirm Codex profile uses `model_reasoning_effort = "xhigh"`.

The adapter maps `xhigh` to `max`.

## `We're currently experiencing high demand`

This is an upstream capacity or gateway error. The adapter cannot fix it.

Practical options:

- retry
- switch to `deepseek-v4-flash`
- reduce request size
- inspect `upstream_status.txt` and `upstream_response.raw`

## Web Search Not Available

Codex may withhold native `--search` from custom providers.

Use the bundled MCP fallback:

```bash
codex mcp add web-search -- /path/to/opencode-go-codex/tools/web_search_mcp.py
```

Or invoke the bundled skill if your Codex CLI exposes skills but not MCP tools:

```text
$web-search-mcp
```

## `/compact` Uses The Wrong Model

Requests to `/v1/responses/compact` should use `deepseek-v4-flash` by default.

Check:

```bash
./opencode-go-codex doctor
```

and confirm:

```bash
export OPENCODE_GO_COMPACT_MODEL=deepseek-v4-flash
```

Image requests still route to the vision model first.

## Tool Calls Do Not Execute

This adapter only emits function-call events. Codex executes tools.

Debug steps:

1. Enable `OPENCODE_GO_TRACE_DIR`.
2. Check `response_events.jsonl`.
3. Look for:
   - `response.output_item.added`
   - `response.function_call_arguments.delta`
   - `response.function_call_arguments.done`
   - `response.output_item.done`
4. Replay the trace:

```bash
./opencode-go-codex replay TRACE_DIR --summary
```

If these events are present, the adapter produced tool calls and the failure is
on the Codex/tool-execution side.

## What To Include In Bug Reports

Do not send screenshots-only logs.

Include:

- opencode-go-codex commit or version
- Codex CLI version
- OS and architecture
- exact command
- redacted `~/.codex/config.toml`
- text logs
- trace directory after removing secrets
- minimal reproduction prompt
