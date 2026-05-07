# Tool Call Semantics

Codex talks to this adapter through the Responses API. OpenCode Go expects
OpenAI-compatible Chat Completions tool calls. The adapter only translates
between those two shapes; it does not execute tools.

## Incoming Tool Definitions

Responses function tools:

```json
{
  "type": "function",
  "name": "read_file",
  "description": "Read a file",
  "parameters": {"type": "object", "properties": {"path": {"type": "string"}}}
}
```

become Chat Completions tools:

```json
{
  "type": "function",
  "function": {
    "name": "read_file",
    "description": "Read a file",
    "parameters": {"type": "object", "properties": {"path": {"type": "string"}}}
  }
}
```

Only `type: function` tools are forwarded.

## Assistant Tool Calls

Responses history item:

```json
{
  "type": "function_call",
  "call_id": "call_1",
  "name": "read_file",
  "arguments": "{\"path\":\"README.md\"}"
}
```

becomes:

```json
{
  "role": "assistant",
  "content": "",
  "tool_calls": [
    {
      "id": "call_1",
      "type": "function",
      "function": {
        "name": "read_file",
        "arguments": "{\"path\":\"README.md\"}"
      }
    }
  ]
}
```

Adjacent assistant function calls are kept on the same assistant message until a
tool output or user message flushes the assistant turn.

## Tool Results

Responses item:

```json
{
  "type": "function_call_output",
  "call_id": "call_1",
  "output": "file text"
}
```

becomes:

```json
{
  "role": "tool",
  "tool_call_id": "call_1",
  "content": "file text"
}
```

## Reasoning Preservation

DeepSeek thinking mode requires prior assistant `reasoning_content` to be passed
back after a tool call. The adapter records upstream reasoning by `call_id` and
restores it when the next request returns the tool result.

This avoids upstream errors like:

```text
The reasoning_content in the thinking mode must be passed back to the API.
```

## Streaming Tool Calls

Streaming Chat Completions chunks can split tool calls across multiple chunks:

```json
{"index":0,"id":"call_1","function":{"name":"read_"}}
{"index":0,"function":{"name":"file","arguments":"{\"path\""}}
{"index":0,"function":{"arguments":":\"README.md\"}"}}
```

The adapter joins by `index`:

- `id` becomes `call_id`
- `function.name` fragments are appended
- `function.arguments` fragments are appended
- the item is emitted once a name exists
- final arguments are sent in `response.function_call_arguments.done`

## Failure Policy

Malformed tool deltas are handled conservatively:

- missing `index` defaults to `0`
- missing `id` gets a generated `call_*`
- missing `name` delays `response.output_item.added` until the name appears
- missing or empty `arguments` becomes an empty string

The adapter does not validate schema or execute the tool. Codex remains the
tool executor and must validate tool names and arguments on its side.
