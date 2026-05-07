import argparse
import os

from .doctor import run_doctor
from .config import (
    DEFAULT_COMPACT_MODEL,
    DEFAULT_MODEL,
    DEFAULT_REASONING_EFFORT,
    DEFAULT_THINKING_TYPE,
    DEFAULT_UPSTREAM,
    DEFAULT_VISION_MODEL,
)
from .server import create_server


def build_parser():
    parser = argparse.ArgumentParser(description="Expose OpenCode Go Chat Completions as OpenAI Responses for Codex.")
    parser.add_argument("command", nargs="?", choices=("serve", "doctor"), default="serve")
    parser.add_argument("--host", default=os.environ.get("OPENCODE_GO_CODEX_HOST", "127.0.0.1"))
    parser.add_argument("--port", type=int, default=int(os.environ.get("OPENCODE_GO_CODEX_PORT", "8768")))
    parser.add_argument("--upstream", default=os.environ.get("OPENCODE_GO_BASE_URL", DEFAULT_UPSTREAM))
    parser.add_argument("--default-model", default=os.environ.get("OPENCODE_GO_MODEL", DEFAULT_MODEL))
    parser.add_argument("--compact-model", default=os.environ.get("OPENCODE_GO_COMPACT_MODEL", DEFAULT_COMPACT_MODEL))
    parser.add_argument("--vision-model", default=os.environ.get("OPENCODE_GO_VISION_MODEL", DEFAULT_VISION_MODEL))
    parser.add_argument("--reasoning-effort", default=os.environ.get("OPENCODE_GO_REASONING_EFFORT", DEFAULT_REASONING_EFFORT))
    parser.add_argument("--thinking", default=os.environ.get("OPENCODE_GO_THINKING", DEFAULT_THINKING_TYPE))
    parser.add_argument("--timeout", type=float, default=float(os.environ.get("OPENCODE_GO_TIMEOUT", "900")))
    parser.add_argument("--trace-dir", default=os.environ.get("OPENCODE_GO_TRACE_DIR", ""))
    parser.add_argument("--debug-routing", action="store_true", default=os.environ.get("OPENCODE_GO_DEBUG_ROUTING", "0") in ("1", "true", "yes"))
    parser.add_argument("--verbose", action="store_true")
    return parser


def main():
    args = build_parser().parse_args()
    if args.command == "doctor":
        raise SystemExit(run_doctor(args))
    server = create_server(
        args.host,
        args.port,
        args.upstream,
        args.default_model,
        args.compact_model,
        args.vision_model,
        args.reasoning_effort,
        args.thinking,
        args.timeout,
        args.verbose,
        args.debug_routing,
        args.trace_dir,
    )
    print(f"opencode-go-codex listening on http://{args.host}:{args.port}", flush=True)
    print(f"forwarding to {server.upstream_url}", flush=True)
    print(f"default model {server.default_model}", flush=True)
    print(f"compact model {server.compact_model}", flush=True)
    print(f"vision model {server.vision_model}", flush=True)
    print(f"default thinking {server.default_thinking_type}", flush=True)
    print(f"default reasoning effort {server.default_reasoning_effort}", flush=True)
    print(f"debug routing {str(server.debug_routing).lower()}", flush=True)
    if server.trace_dir:
        print(f"trace dir {server.trace_dir}", flush=True)
    server.serve_forever()
