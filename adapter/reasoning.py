import base64
import json
import uuid

from .config import REASONING_CACHE_LIMIT, REASONING_ENCRYPTED_PREFIX
from .jsonutil import json_dumps


def encode_reasoning_content(reasoning_content):
    payload = {
        "format": REASONING_ENCRYPTED_PREFIX,
        "reasoning_content": reasoning_content,
    }
    return base64.b64encode(json_dumps(payload).encode("utf-8")).decode("ascii")


def decode_reasoning_content(encrypted_content):
    if not encrypted_content:
        return ""
    try:
        raw = base64.b64decode(str(encrypted_content), validate=True).decode("utf-8")
        payload = json.loads(raw)
    except Exception:
        return ""
    if payload.get("format") != REASONING_ENCRYPTED_PREFIX:
        return ""
    return str(payload.get("reasoning_content") or "")


def extract_reasoning_content(item):
    if not isinstance(item, dict):
        return ""

    decoded = decode_reasoning_content(item.get("encrypted_content"))
    if decoded:
        return decoded

    content = item.get("content")
    if isinstance(content, str):
        return content
    if isinstance(content, list):
        parts = []
        for part in content:
            if isinstance(part, str):
                parts.append(part)
            elif isinstance(part, dict):
                text = part.get("text") or part.get("reasoning_text")
                if text:
                    parts.append(str(text))
        return "\n".join(parts)
    return ""


def make_reasoning_item(reasoning_content):
    return {
        "id": "rs_" + uuid.uuid4().hex,
        "type": "reasoning",
        "summary": [],
        "content": None,
        "encrypted_content": encode_reasoning_content(reasoning_content),
    }


def remember_reasoning(server, call_ids, reasoning_content):
    if not reasoning_content or not call_ids:
        return
    with server.reasoning_lock:
        for call_id in call_ids:
            if not call_id:
                continue
            if call_id not in server.reasoning_by_call_id:
                server.reasoning_order.append(call_id)
            server.reasoning_by_call_id[call_id] = reasoning_content
        while len(server.reasoning_order) > REASONING_CACHE_LIMIT:
            old_call_id = server.reasoning_order.pop(0)
            server.reasoning_by_call_id.pop(old_call_id, None)


def lookup_reasoning(server, call_id):
    if not server or not call_id:
        return ""
    with server.reasoning_lock:
        return server.reasoning_by_call_id.get(call_id, "")
