# Codex Compatibility

This adapter exposes OpenCode Go Chat Completions as an OpenAI Responses API
surface for Codex CLI.

It is intentionally narrow:

```text
Codex Responses request
  -> opencode-go-codex
  -> OpenCode Go /v1/chat/completions
  -> DeepSeek V4 or Kimi
  -> Responses SSE events back to Codex
```

It is not a general-purpose OpenAI proxy and does not try to support every
Responses field.

## Request Normalization

The main conversion lives in `internal/transform` and `internal/content`.

| Codex / Responses input | Chat Completions output |
| --- | --- |
| `instructions` | first `system` message |
| string `input` | one `user` message |
| `type: message` | message with normalized `role` and converted `content` |
| plain `{role, content}` item | same as `type: message` |
| `input_text`, `output_text`, `text` part | chat text part |
| `input_image`, `image_url` part | chat `image_url` part |
| `image_base64` | `data:<mime>;base64,...` image URL |
| `function_call` | assistant `tool_calls[]` |
| `function_call_output` | `role=tool` message |
| `reasoning` | assistant `reasoning_content` where applicable |

Unknown structured items are serialized as JSON text instead of being dropped.

## Model Routing

Routing is deterministic:

```text
request contains image input -> OPENCODE_GO_VISION_MODEL, default kimi-k2.6
/responses/compact          -> OPENCODE_GO_COMPACT_MODEL, default deepseek-v4-flash
everything else             -> request model or OPENCODE_GO_MODEL, default deepseek-v4-pro
```

Image routing is intentionally checked before compact routing because DeepSeek
V4 text models reject OpenAI-style `image_url` content.

## Reasoning Mapping

Codex uses reasoning levels such as `xhigh`. OpenCode Go / DeepSeek V4 expects
DeepSeek-style values:

```text
Codex low    -> high
Codex medium -> high
Codex high   -> high
Codex xhigh  -> max
max          -> max
```

For `deepseek-v4-*`, the adapter sends:

```json
{
  "thinking": {"type": "enabled"},
  "reasoning_effort": "max"
}
```

unless the request or environment explicitly disables thinking.

## Response Rendering

The response path lives in `internal/sse`.

Non-streaming upstream responses are rendered as Responses SSE events so Codex
sees the same wire API. Streaming upstream responses are parsed from Chat
Completions chunks and converted to Responses events:

- `delta.reasoning_content` is buffered as reasoning.
- `delta.content` becomes `response.output_text.delta`.
- `delta.tool_calls[].function.name` and `.arguments` are joined by index.
- final tool calls become `response.function_call_arguments.done` and
  `response.output_item.done`.

## Unsupported / Not Forwarded

These fields are currently ignored unless explicitly implemented later:

- `parallel_tool_calls`
- `verbosity`
- `service_tier`
- `metadata`
- `store`

Add new passthrough fields deliberately in `internal/transform`; do not copy
the full Codex request blindly.

## Trace Format

Set `OPENCODE_GO_TRACE_DIR` or pass `--trace-dir` to write one directory per
request:

```text
trace-id/
  incoming_responses_request.raw.json
  incoming_responses_request.json
  upstream_chat_request.json
  upstream_status.txt
  upstream_response.raw
  upstream_stream.raw
  response_events.jsonl
  meta.json
  error.txt
```

`response_events.jsonl` can be replayed with:

```bash
./opencode-go-codex replay TRACE_DIR
./opencode-go-codex replay TRACE_DIR --summary
```

Trace JSON is redacted for common key/token/password fields. Raw upstream files
may still contain prompt or output content; do not upload them publicly without
review.
