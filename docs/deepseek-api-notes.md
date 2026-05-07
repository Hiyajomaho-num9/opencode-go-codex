# DeepSeek API Notes for OpenCode Go Codex

This project uses OpenCode Go as the upstream gateway, but the request body is
shaped after DeepSeek's OpenAI-compatible Chat Completions API.

Reference pages:

- https://api-docs.deepseek.com/zh-cn/api/create-chat-completion
- https://api-docs.deepseek.com/zh-cn/guides/deepseek-v3-2
- https://api-docs.deepseek.com/zh-cn/guides/function_calling

## Model Routing

Codex sends OpenAI Responses requests to this adapter.

The adapter forwards Chat Completions requests to OpenCode Go:

```text
Codex Responses -> opencode-go-codex -> OpenCode Go /v1/chat/completions
```

Default routing:

```text
normal text request -> deepseek-v4-pro
/v1/responses/compact -> deepseek-v4-flash
request containing input_image -> kimi-k2.6
```

Kimi is only used as a vision fallback because DeepSeek V4 text models do not
handle image input in this setup.

## Reasoning And Thinking

DeepSeek V4 thinking mode uses two relevant fields:

```json
{
  "thinking": {
    "type": "enabled"
  },
  "reasoning_effort": "max"
}
```

DeepSeek's documented `reasoning_effort` values are:

```text
high
max
```

Codex uses different effort labels, so the adapter maps them:

```text
Codex low    -> DeepSeek high
Codex medium -> DeepSeek high
Codex high   -> DeepSeek high
Codex xhigh  -> DeepSeek max
max          -> DeepSeek max
```

For `deepseek-v4-pro` and `deepseek-v4-flash`, the adapter defaults to:

```json
{
  "thinking": {
    "type": "enabled"
  },
  "reasoning_effort": "max"
}
```

This is intentional: the Codex profiles are configured for the full reasoning
tier by default.

## Parameters Forwarded

The adapter forwards these Codex/Responses fields to Chat Completions:

```text
model -> model
input/instructions -> messages
tools -> tools
tool_choice -> tool_choice
max_output_tokens -> max_tokens
max_tokens -> max_tokens
response_format -> response_format
stop -> stop
stream_options -> stream_options
user/user_id -> user_id
frequency_penalty -> frequency_penalty
presence_penalty -> presence_penalty
```

For non-DeepSeek-V4 models, these sampling fields are also forwarded:

```text
temperature
top_p
```

For DeepSeek V4 thinking mode, `temperature` and `top_p` are intentionally not
forwarded because DeepSeek's guide says sampling parameters are not effective in
thinking mode.

## Tool Calls

Codex Responses function tools are converted to Chat Completions tools:

```text
Responses function tool -> Chat Completions tools[].function
Responses function_call -> assistant.tool_calls[]
Responses function_call_output -> role=tool message
```

DeepSeek thinking mode requires previous assistant `reasoning_content` to be
passed back when continuing after tool calls. The adapter stores the upstream
`reasoning_content` and restores it on the next tool-result turn.

This prevents upstream errors like:

```text
The reasoning_content in the thinking mode must be passed back to the API.
```

## Not Forwarded

The adapter does not blindly forward every Responses field. In particular:

```text
parallel_tool_calls
verbosity
service_tier
metadata
store
```

Those are either OpenAI Responses-specific, unsupported by the documented
DeepSeek Chat Completions schema, or not known to be accepted by OpenCode Go.

If OpenCode Go later documents extra supported fields, add them explicitly in
`internal/transform` instead of blindly copying the full Codex request.
