# Testing

The project intentionally keeps tests small. The goal is to lock the adapter's
compatibility behavior without building a large framework.

## Local Unit Tests

```bash
python3 -m compileall -q adapter opencode_go_codex.py tools/web_search_mcp.py
python3 -m unittest discover -s tests
```

Covered areas:

- Responses input normalization
- image routing
- reasoning effort mapping
- tool call conversion
- SSE event rendering
- trace replay parsing

## Smoke Test

Start the adapter first:

```bash
OPENCODE_GO_API_KEY=... ./start.sh
```

Then run:

```bash
./smoke.sh
```

The smoke script runs compile checks, unit tests, `/healthz`, and one minimal
Responses request through the running adapter.

## Trace-Based Debugging

Enable trace output:

```bash
export OPENCODE_GO_TRACE_DIR=/tmp/opencode-go-codex-trace
./start.sh
```

Each request gets its own directory. Summarize a trace:

```bash
tools/replay_trace.py /tmp/opencode-go-codex-trace/TRACE_ID --summary
```

Replay as SSE:

```bash
tools/replay_trace.py /tmp/opencode-go-codex-trace/TRACE_ID
```

## Doctor

Check local setup:

```bash
python3 opencode_go_codex.py doctor
```

This verifies Python, Codex config hints, model catalog, MCP script, compileall,
API-key environment, and whether the adapter port is reachable.

## Regression Checklist Before Release

```bash
python3 -m compileall -q adapter opencode_go_codex.py tools/web_search_mcp.py
python3 -m unittest discover -s tests
python3 opencode_go_codex.py doctor
```

If the service is running with a valid key:

```bash
./smoke.sh
```
