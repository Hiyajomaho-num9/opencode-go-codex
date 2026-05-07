import os
import time
import uuid
from pathlib import Path

from .jsonutil import json_dumps


SENSITIVE_KEYS = {
    "api_key",
    "apikey",
    "authorization",
    "auth",
    "key",
    "password",
    "secret",
    "token",
}


def make_trace(root, enabled=True):
    if not root or not enabled:
        return None
    trace_id = time.strftime("%Y%m%d-%H%M%S-") + uuid.uuid4().hex[:8]
    trace_dir = Path(root).expanduser() / trace_id
    trace_dir.mkdir(parents=True, exist_ok=True)
    return Trace(trace_dir, trace_id)


def redact(value):
    if isinstance(value, dict):
        redacted = {}
        for key, child in value.items():
            lower = str(key).lower()
            if any(marker in lower for marker in SENSITIVE_KEYS):
                redacted[key] = "<redacted>"
            else:
                redacted[key] = redact(child)
        return redacted
    if isinstance(value, list):
        return [redact(item) for item in value]
    return value


class Trace:
    def __init__(self, path, trace_id):
        self.path = path
        self.trace_id = trace_id
        self._events_file = None

    def write_json(self, name, data, redact_secrets=True):
        payload = redact(data) if redact_secrets else data
        (self.path / name).write_text(json_dumps(payload) + "\n", encoding="utf-8")

    def write_text(self, name, data):
        if isinstance(data, bytes):
            data = data.decode("utf-8", "replace")
        (self.path / name).write_text(str(data), encoding="utf-8")

    def append_response_event(self, event, data):
        if self._events_file is None:
            self._events_file = (self.path / "response_events.jsonl").open("a", encoding="utf-8")
        self._events_file.write(json_dumps({"event": event, "data": data}) + "\n")
        self._events_file.flush()

    def close(self):
        if self._events_file is not None:
            self._events_file.close()
            self._events_file = None


def write_meta(trace, **data):
    if trace:
        trace.write_json("meta.json", {"trace_id": trace.trace_id, **data})
