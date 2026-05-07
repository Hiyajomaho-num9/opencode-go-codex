import json
import tempfile
import unittest
from pathlib import Path

from adapter.trace import make_trace, redact
from tools.replay_trace import load_jsonl_events


class TraceReplayTests(unittest.TestCase):
    def test_trace_writes_replayable_events(self):
        with tempfile.TemporaryDirectory() as td:
            trace = make_trace(td)
            trace.write_json("incoming.json", {"Authorization": "Bearer secret", "input": "hello"})
            trace.append_response_event("response.created", {"type": "response.created"})
            trace.append_response_event("response.completed", {"type": "response.completed"})
            trace.close()

            events = load_jsonl_events(Path(trace.path) / "response_events.jsonl")
            self.assertEqual([event["event"] for event in events], ["response.created", "response.completed"])
            incoming = json.loads((Path(trace.path) / "incoming.json").read_text(encoding="utf-8"))
            self.assertEqual(incoming["Authorization"], "<redacted>")

    def test_redact_nested_secrets(self):
        self.assertEqual(
            redact({"headers": {"authorization": "Bearer x"}, "ok": [{"api_key": "y"}]}),
            {"headers": {"authorization": "<redacted>"}, "ok": [{"api_key": "<redacted>"}]},
        )


if __name__ == "__main__":
    unittest.main()
