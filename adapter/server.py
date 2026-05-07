import json
import os
import threading
import traceback
import urllib.error
import urllib.request
import uuid
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

from .config import normalize_reasoning_effort, normalize_thinking_type, normalize_upstream
from .jsonutil import json_dumps
from .reasoning import remember_reasoning
from .sse import emit_final_stream, emit_text_stream, stream_chat_to_responses
from .content import request_has_image
from .trace import make_trace, write_meta
from .transform import responses_to_chat_request, select_model


class ProxyHandler(BaseHTTPRequestHandler):
    server_version = "OpenCodeGoCodex/0.2"
    protocol_version = "HTTP/1.1"

    def do_GET(self):
        if self.path == "/healthz":
            body = b"ok\n"
            self.send_response(200)
            self.send_header("content-type", "text/plain")
            self.send_header("content-length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)
            return
        if self.path == "/readyz":
            body = b"ready\n"
            self.send_response(200)
            self.send_header("content-type", "text/plain")
            self.send_header("content-length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)
            return
        self.send_error(404)

    def do_POST(self):
        path = self.path.split("?", 1)[0]
        if path not in ("/responses", "/v1/responses", "/responses/compact", "/v1/responses/compact"):
            self.send_error(404)
            return
        try:
            self.trace = make_trace(self.server.trace_dir)
            length = int(self.headers.get("content-length", "0") or 0)
            raw_body = self.rfile.read(length)
            if self.trace:
                self.trace.write_text("incoming_responses_request.raw.json", raw_body)
            request = json.loads(raw_body)
            if self.trace:
                self.trace.write_json("incoming_responses_request.json", request)
            has_image = request_has_image(request)
            selected_model = select_model(
                request,
                path,
                self.server.default_model,
                self.server.compact_model,
                self.server.vision_model,
            )
            request["model"] = selected_model
            chat_request = responses_to_chat_request(request, selected_model, self.server)
            if path.endswith("/compact"):
                chat_request.pop("tools", None)
                chat_request.pop("tool_choice", None)
            if self.trace:
                self.trace.write_json("upstream_chat_request.json", chat_request)
                write_meta(
                    self.trace,
                    path=path,
                    upstream_url=self.server.upstream_url,
                    has_image=has_image,
                    selected_model=selected_model,
                    stream=bool(chat_request.get("stream")),
                )
            self.log_route(path, has_image, chat_request)
            self.forward_request(chat_request)
        except Exception as exc:
            print("request failed:", repr(exc), flush=True)
            traceback.print_exc()
            trace = getattr(self, "trace", None)
            if trace:
                trace.write_text("error.txt", repr(exc) + "\n" + traceback.format_exc())
            self.send_json_error(500, str(exc))
        finally:
            trace = getattr(self, "trace", None)
            if trace:
                trace.close()

    def log_route(self, path, has_image, chat_request):
        if not (self.server.verbose or self.server.debug_routing):
            return
        thinking = chat_request.get("thinking") or {}
        print(
            "route "
            f"path={path} "
            f"has_image={str(has_image).lower()} "
            f"model={chat_request.get('model')} "
            f"thinking={thinking.get('type', 'none')} "
            f"reasoning_effort={chat_request.get('reasoning_effort', 'none')}",
            flush=True,
        )

    def forward_request(self, chat_request):
        api_key = os.environ.get("OPENCODE_GO_API_KEY")
        if api_key:
            auth = "Bearer " + api_key
        else:
            auth = self.headers.get("authorization")
            if not auth and os.environ.get("OPENAI_API_KEY"):
                auth = "Bearer " + os.environ["OPENAI_API_KEY"]
        if not auth:
            self.send_json_error(401, "missing Authorization header or OPENCODE_GO_API_KEY")
            return

        upstream_request = urllib.request.Request(
            self.server.upstream_url,
            data=json_dumps(chat_request).encode("utf-8"),
            headers={
                "accept": "application/json, text/event-stream",
                "authorization": auth,
                "content-type": "application/json",
                "user-agent": "curl/8.5.0",
            },
            method="POST",
        )
        try:
            upstream = urllib.request.urlopen(upstream_request, timeout=self.server.timeout)
        except urllib.error.HTTPError as exc:
            detail = exc.read().decode("utf-8", "replace")
            trace = getattr(self, "trace", None)
            if trace:
                trace.write_text("upstream_status.txt", f"HTTP {exc.code}\n")
                trace.write_text("upstream_response.raw", detail)
            self.send_json_error(exc.code, detail)
            return
        trace = getattr(self, "trace", None)
        if trace:
            trace.write_text("upstream_status.txt", f"HTTP {getattr(upstream, 'status', 200)}\n")

        self.send_response(200)
        self.send_header("content-type", "text/event-stream")
        self.send_header("cache-control", "no-cache")
        self.send_header("connection", "close")
        self.end_headers()

        if chat_request.get("stream"):
            stream_chat_to_responses(self, upstream, chat_request["model"])
            return

        raw_payload = upstream.read()
        if trace:
            trace.write_text("upstream_response.raw", raw_payload)
        payload = json.loads(raw_payload)
        message = payload.get("choices", [{}])[0].get("message", {})
        reasoning_content = message.get("reasoning_content") or ""
        text = message.get("content") or ""
        tool_calls = []
        for tool_call in message.get("tool_calls") or []:
            function = tool_call.get("function") or {}
            tool_calls.append(
                {
                    "item_id": "fc_" + uuid.uuid4().hex,
                    "call_id": tool_call.get("id") or "call_" + uuid.uuid4().hex,
                    "name": function.get("name", ""),
                    "arguments": function.get("arguments", ""),
                }
            )
        remember_reasoning(self.server, [call.get("call_id") for call in tool_calls], reasoning_content)
        if tool_calls:
            emit_final_stream(self, chat_request["model"], text, tool_calls, reasoning_content, payload.get("usage"))
        else:
            emit_text_stream(self, chat_request["model"], text, payload.get("usage"))

    def send_json_error(self, status, message):
        body = json_dumps({"error": {"message": message}}).encode("utf-8")
        self.send_response(status)
        self.send_header("content-type", "application/json")
        self.send_header("content-length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, fmt, *args):
        if self.server.verbose:
            super().log_message(fmt, *args)


def create_server(host, port, upstream, default_model, compact_model, vision_model, reasoning_effort, thinking_type, timeout, verbose=False, debug_routing=False, trace_dir=""):
    server = ThreadingHTTPServer((host, port), ProxyHandler)
    server.upstream_url = normalize_upstream(upstream)
    server.default_model = default_model
    server.compact_model = compact_model
    server.vision_model = vision_model
    server.default_reasoning_effort = normalize_reasoning_effort(reasoning_effort)
    server.default_thinking_type = normalize_thinking_type(thinking_type)
    server.timeout = timeout
    server.verbose = verbose
    server.debug_routing = debug_routing
    server.trace_dir = trace_dir or os.environ.get("OPENCODE_GO_TRACE_DIR", "")
    server.reasoning_by_call_id = {}
    server.reasoning_order = []
    server.reasoning_lock = threading.Lock()
    return server
