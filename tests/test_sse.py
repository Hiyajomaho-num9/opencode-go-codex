import io
import json
import unittest

from adapter.sse import stream_chat_to_responses


class FakeServer:
    def __init__(self):
        self.reasoning_by_call_id = {}
        self.reasoning_order = []
        self.reasoning_lock = DummyLock()


class DummyLock:
    def __enter__(self):
        return self

    def __exit__(self, exc_type, exc, tb):
        return False


class FakeHandler:
    def __init__(self):
        self.wfile = io.BytesIO()
        self.server = FakeServer()
        self.trace = None


def parse_sse(raw):
    events = []
    current = {}
    for line in raw.decode("utf-8").splitlines():
        if not line:
            if current:
                events.append(current)
                current = {}
            continue
        key, value = line.split(": ", 1)
        if key == "data":
            current[key] = json.loads(value)
        else:
            current[key] = value
    return events


class SseTests(unittest.TestCase):
    def test_stream_tool_call_fragments_are_joined(self):
        chunks = [
            {"choices": [{"delta": {"reasoning_content": "think "}}]},
            {"choices": [{"delta": {"tool_calls": [{"index": 0, "id": "call_1", "function": {"name": "read_"}}]}}]},
            {"choices": [{"delta": {"tool_calls": [{"index": 0, "function": {"name": "file", "arguments": "{\"path\""}}]}}]},
            {"choices": [{"delta": {"tool_calls": [{"index": 0, "function": {"arguments": ":\"README.md\"}"}}]}}]},
            {"usage": {"prompt_tokens": 1, "completion_tokens": 2, "total_tokens": 3}, "choices": [{"delta": {}}]},
        ]
        upstream = io.BytesIO(b"".join(("data: " + json.dumps(chunk) + "\n\n").encode("utf-8") for chunk in chunks) + b"data: [DONE]\n\n")
        handler = FakeHandler()
        stream_chat_to_responses(handler, upstream, "deepseek-v4-pro")
        events = parse_sse(handler.wfile.getvalue())
        completed = [event for event in events if event["event"] == "response.completed"][-1]
        output = completed["data"]["response"]["output"]
        call = [item for item in output if item["type"] == "function_call"][0]
        self.assertEqual(call["call_id"], "call_1")
        self.assertEqual(call["name"], "read_file")
        self.assertEqual(call["arguments"], "{\"path\":\"README.md\"}")

    def test_stream_text_is_rendered_as_response_events(self):
        chunks = [
            {"choices": [{"delta": {"content": "hel"}}]},
            {"choices": [{"delta": {"content": "lo"}}]},
        ]
        upstream = io.BytesIO(b"".join(("data: " + json.dumps(chunk) + "\n\n").encode("utf-8") for chunk in chunks) + b"data: [DONE]\n\n")
        handler = FakeHandler()
        stream_chat_to_responses(handler, upstream, "deepseek-v4-flash")
        events = parse_sse(handler.wfile.getvalue())
        deltas = [event["data"]["delta"] for event in events if event["event"] == "response.output_text.delta"]
        self.assertEqual("".join(deltas), "hello")


if __name__ == "__main__":
    unittest.main()
