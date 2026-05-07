import json
import os
import socket
import subprocess
import sys
from pathlib import Path


def ok(name, detail=""):
    print(f"[OK]   {name}{': ' + detail if detail else ''}")


def warn(name, detail=""):
    print(f"[WARN] {name}{': ' + detail if detail else ''}")


def fail(name, detail=""):
    print(f"[FAIL] {name}{': ' + detail if detail else ''}")


def port_open(host, port, timeout=1.0):
    try:
        with socket.create_connection((host, port), timeout=timeout):
            return True
    except OSError:
        return False


def run_doctor(args):
    root = Path(__file__).resolve().parents[1]
    codex_home = Path(os.environ.get("CODEX_HOME", Path.home() / ".codex"))
    config = codex_home / "config.toml"
    models = root / "examples" / "models" / "deepseek-only.json"
    mcp = root / "tools" / "web_search_mcp.py"

    exit_code = 0
    if sys.version_info >= (3, 10):
        ok("python", sys.version.split()[0])
    else:
        fail("python", "Python 3.10+ is recommended")
        exit_code = 1

    if config.exists():
        text = config.read_text(encoding="utf-8", errors="replace")
        ok("codex config", str(config))
        for needle in [
            "model_providers.OpenCodeGo",
            "wire_api = \"responses\"",
            "deepseek-v4-pro",
            "deepseek-v4-flash",
        ]:
            if needle in text:
                ok("config contains", needle)
            else:
                warn("config missing", needle)
    else:
        warn("codex config missing", str(config))

    if models.exists():
        try:
            payload = json.loads(models.read_text(encoding="utf-8"))
            slugs = [item.get("slug") for item in payload.get("models", [])]
            if "deepseek-v4-pro" in slugs and "deepseek-v4-flash" in slugs:
                ok("model catalog", ", ".join(slugs))
            else:
                warn("model catalog lacks expected DeepSeek slugs", str(models))
        except Exception as exc:
            fail("model catalog invalid", str(exc))
            exit_code = 1
    else:
        fail("model catalog missing", str(models))
        exit_code = 1

    if mcp.exists() and os.access(mcp, os.X_OK):
        ok("web-search MCP", str(mcp))
    elif mcp.exists():
        warn("web-search MCP is not executable", str(mcp))
    else:
        warn("web-search MCP missing", str(mcp))

    if os.environ.get("OPENCODE_GO_API_KEY") or os.environ.get("OPENAI_API_KEY"):
        ok("API key env", "present")
    else:
        warn("API key env", "set OPENCODE_GO_API_KEY before starting")

    host = os.environ.get("OPENCODE_GO_CODEX_HOST", "127.0.0.1")
    port = int(os.environ.get("OPENCODE_GO_CODEX_PORT", "8768"))
    if port_open(host, port):
        ok("adapter port", f"{host}:{port} is reachable")
    else:
        warn("adapter port", f"{host}:{port} is not listening")

    try:
        subprocess.run(
            [sys.executable, "-m", "compileall", "-q", "adapter", "opencode_go_codex.py", "tools/web_search_mcp.py"],
            cwd=root,
            check=True,
        )
        ok("compileall")
    except subprocess.CalledProcessError as exc:
        fail("compileall", f"exit {exc.returncode}")
        exit_code = 1

    return exit_code
