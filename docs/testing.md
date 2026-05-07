# Testing

The project intentionally keeps tests small. The goal is to lock the adapter's
compatibility behavior without building a large framework.

## Local Unit Tests

```bash
go test ./...
go build -o opencode-go-codex ./cmd/opencode-go-codex
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
./opencode-go-codex replay /tmp/opencode-go-codex-trace/TRACE_ID --summary
```

Replay as SSE:

```bash
./opencode-go-codex replay /tmp/opencode-go-codex-trace/TRACE_ID
```

## Doctor

Check local setup:

```bash
./opencode-go-codex doctor
```

This verifies the Go runtime, Codex config hints, model catalog, API-key
environment, and whether the adapter port is reachable.

## Regression Checklist Before Release

```bash
go test ./...
go build -o opencode-go-codex ./cmd/opencode-go-codex
./opencode-go-codex doctor
```

If the service is running with a valid key:

```bash
./smoke.sh
```
