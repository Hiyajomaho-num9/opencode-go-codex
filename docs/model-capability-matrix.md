# Model Capability Matrix

Default model names are configured by environment variables and Codex profiles.

| Model | Default use | Text | Reasoning | Image input | Web search | Compact |
| --- | --- | --- | --- | --- | --- | --- |
| `deepseek-v4-pro` | main Codex model | yes | `max` | no | MCP fallback | no |
| `deepseek-v4-flash` | `/compact` and cheaper work | yes | `max` | no | MCP fallback | yes |
| `kimi-k2.6` | vision fallback | yes | provider-dependent | yes | not primary | no |

## Routing Priority

```text
image input -> kimi-k2.6
/compact    -> deepseek-v4-flash
default     -> deepseek-v4-pro
```

## Notes

- DeepSeek V4 text models reject OpenAI `image_url` message parts in this setup.
- Codex `xhigh` maps to OpenCode Go / DeepSeek `max`.
- Native Codex web search may not be exposed for custom providers. Use the
  bundled `tools/web_search_mcp.py` fallback.
- Context defaults are intentionally set to `512000` with auto-compact at
  `400000`.
