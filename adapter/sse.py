import json
import uuid

from .jsonutil import json_dumps
from .reasoning import make_reasoning_item, remember_reasoning
from .responses import make_response


def write_sse(handler, event, data):
    trace = getattr(handler, "trace", None)
    if trace:
        trace.append_response_event(event, data)
    handler.wfile.write(("event: " + event + "\n").encode("utf-8"))
    handler.wfile.write(("data: " + json_dumps(data) + "\n\n").encode("utf-8"))
    handler.wfile.flush()


def emit_text_stream(handler, model, text, usage=None):
    response_id = "resp_" + uuid.uuid4().hex
    message_id = "msg_" + uuid.uuid4().hex
    item = {
        "id": message_id,
        "type": "message",
        "status": "completed",
        "role": "assistant",
        "content": [{"type": "output_text", "text": text, "annotations": []}],
    }
    response = make_response(model, [item], usage)
    response["id"] = response_id
    write_sse(handler, "response.created", {"type": "response.created", "response": {**response, "status": "in_progress", "output": []}})
    write_sse(handler, "response.output_item.added", {"type": "response.output_item.added", "output_index": 0, "item": {**item, "status": "in_progress", "content": []}})
    write_sse(handler, "response.content_part.added", {"type": "response.content_part.added", "item_id": message_id, "output_index": 0, "content_index": 0, "part": {"type": "output_text", "text": "", "annotations": []}})
    if text:
        write_sse(handler, "response.output_text.delta", {"type": "response.output_text.delta", "item_id": message_id, "output_index": 0, "content_index": 0, "delta": text})
    write_sse(handler, "response.output_text.done", {"type": "response.output_text.done", "item_id": message_id, "output_index": 0, "content_index": 0, "text": text})
    write_sse(handler, "response.content_part.done", {"type": "response.content_part.done", "item_id": message_id, "output_index": 0, "content_index": 0, "part": item["content"][0]})
    write_sse(handler, "response.output_item.done", {"type": "response.output_item.done", "output_index": 0, "item": item})
    write_sse(handler, "response.completed", {"type": "response.completed", "response": response})


def emit_final_stream(handler, model, text, tool_calls, reasoning_content="", usage=None):
    output = []
    if reasoning_content:
        output.append(make_reasoning_item(reasoning_content))
    if text:
        output.append(
            {
                "id": "msg_" + uuid.uuid4().hex,
                "type": "message",
                "status": "completed",
                "role": "assistant",
                "content": [{"type": "output_text", "text": text, "annotations": []}],
            }
        )
    for call in tool_calls:
        output.append(
            {
                "id": call["item_id"],
                "type": "function_call",
                "status": "completed",
                "call_id": call["call_id"],
                "name": call.get("name", ""),
                "arguments": call.get("arguments", ""),
            }
        )
    response = make_response(model, output, usage)
    write_sse(handler, "response.completed", {"type": "response.completed", "response": response})


def stream_chat_to_responses(handler, upstream_response, model):
    response_id = "resp_" + uuid.uuid4().hex
    text_item_id = "msg_" + uuid.uuid4().hex
    text_started = False
    text = ""
    reasoning_content = ""
    tool_calls = {}
    output_count = 0
    usage = None
    created = make_response(model, [], status="in_progress")
    created["id"] = response_id
    write_sse(handler, "response.created", {"type": "response.created", "response": created})

    for raw_line in upstream_response:
        trace = getattr(handler, "trace", None)
        if trace:
            with (trace.path / "upstream_stream.raw").open("ab") as f:
                f.write(raw_line)
        line = raw_line.decode("utf-8", "replace").strip()
        if not line or not line.startswith("data:"):
            continue
        data = line[5:].strip()
        if data == "[DONE]":
            break
        try:
            chunk = json.loads(data)
        except json.JSONDecodeError:
            continue
        if chunk.get("usage"):
            usage = chunk["usage"]
        for choice in chunk.get("choices", []):
            delta = choice.get("delta") or {}
            reasoning_delta = delta.get("reasoning_content")
            if reasoning_delta:
                reasoning_content += reasoning_delta
            content = delta.get("content")
            if content:
                if not text_started:
                    text_started = True
                    item = {"id": text_item_id, "type": "message", "status": "in_progress", "role": "assistant", "content": []}
                    write_sse(handler, "response.output_item.added", {"type": "response.output_item.added", "output_index": output_count, "item": item})
                    write_sse(handler, "response.content_part.added", {"type": "response.content_part.added", "item_id": text_item_id, "output_index": output_count, "content_index": 0, "part": {"type": "output_text", "text": "", "annotations": []}})
                text += content
                write_sse(handler, "response.output_text.delta", {"type": "response.output_text.delta", "item_id": text_item_id, "output_index": output_count, "content_index": 0, "delta": content})
            for tool_delta in delta.get("tool_calls") or []:
                index = int(tool_delta.get("index", 0))
                function = tool_delta.get("function") or {}
                call = tool_calls.setdefault(
                    index,
                    {
                        "item_id": "fc_" + uuid.uuid4().hex,
                        "call_id": tool_delta.get("id") or "call_" + uuid.uuid4().hex,
                        "name": "",
                        "arguments": "",
                        "output_index": None,
                        "added": False,
                    },
                )
                if tool_delta.get("id"):
                    call["call_id"] = tool_delta["id"]
                if function.get("name"):
                    call["name"] += function["name"]
                if not call["added"] and call["name"]:
                    call["output_index"] = output_count + len([entry for entry in tool_calls.values() if entry.get("added")])
                    item = {
                        "id": call["item_id"],
                        "type": "function_call",
                        "status": "in_progress",
                        "call_id": call["call_id"],
                        "name": call["name"],
                        "arguments": "",
                    }
                    write_sse(handler, "response.output_item.added", {"type": "response.output_item.added", "output_index": call["output_index"], "item": item})
                    call["added"] = True
                if function.get("arguments"):
                    call["arguments"] += function["arguments"]
                    if call["added"]:
                        write_sse(handler, "response.function_call_arguments.delta", {"type": "response.function_call_arguments.delta", "item_id": call["item_id"], "output_index": call["output_index"], "delta": function["arguments"]})

    completed_output = []
    if reasoning_content:
        completed_output.append(make_reasoning_item(reasoning_content))
    if text_started:
        content = {"type": "output_text", "text": text, "annotations": []}
        item = {"id": text_item_id, "type": "message", "status": "completed", "role": "assistant", "content": [content]}
        write_sse(handler, "response.output_text.done", {"type": "response.output_text.done", "item_id": text_item_id, "output_index": output_count, "content_index": 0, "text": text})
        write_sse(handler, "response.content_part.done", {"type": "response.content_part.done", "item_id": text_item_id, "output_index": output_count, "content_index": 0, "part": content})
        write_sse(handler, "response.output_item.done", {"type": "response.output_item.done", "output_index": output_count, "item": item})
        completed_output.append(item)
        output_count += 1

    for call in sorted(tool_calls.values(), key=lambda item: item["output_index"] if item["output_index"] is not None else 999999):
        if not call["added"]:
            call["output_index"] = output_count
            write_sse(
                handler,
                "response.output_item.added",
                {
                    "type": "response.output_item.added",
                    "output_index": call["output_index"],
                    "item": {
                        "id": call["item_id"],
                        "type": "function_call",
                        "status": "in_progress",
                        "call_id": call["call_id"],
                        "name": call.get("name", ""),
                        "arguments": "",
                    },
                },
            )
        item = {
            "id": call["item_id"],
            "type": "function_call",
            "status": "completed",
            "call_id": call["call_id"],
            "name": call.get("name", ""),
            "arguments": call.get("arguments", ""),
        }
        write_sse(handler, "response.function_call_arguments.done", {"type": "response.function_call_arguments.done", "item_id": call["item_id"], "output_index": call["output_index"], "arguments": call.get("arguments", "")})
        write_sse(handler, "response.output_item.done", {"type": "response.output_item.done", "output_index": call["output_index"], "item": item})
        completed_output.append(item)
        output_count += 1

    remember_reasoning(
        handler.server,
        [call.get("call_id") for call in tool_calls.values()],
        reasoning_content,
    )
    response = make_response(model, completed_output, usage)
    response["id"] = response_id
    write_sse(handler, "response.completed", {"type": "response.completed", "response": response})
