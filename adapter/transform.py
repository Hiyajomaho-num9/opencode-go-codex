from .config import DEFAULT_REASONING_EFFORT, DEFAULT_THINKING_TYPE, is_deepseek_v4_model, normalize_reasoning_effort, normalize_thinking_type
from .content import content_to_chat, request_has_image
from .jsonutil import json_dumps
from .reasoning import extract_reasoning_content, lookup_reasoning


def responses_input_to_messages(request, server=None):
    messages = []
    instructions = request.get("instructions")
    if instructions:
        messages.append({"role": "system", "content": str(instructions)})

    response_input = request.get("input", "")
    if isinstance(response_input, str):
        if response_input:
            messages.append({"role": "user", "content": response_input})
        return messages

    if not isinstance(response_input, list):
        messages.append({"role": "user", "content": content_to_chat(response_input)})
        return messages

    assistant_message = None

    def ensure_assistant():
        nonlocal assistant_message
        if assistant_message is None:
            assistant_message = {"role": "assistant", "content": ""}
        return assistant_message

    def flush_assistant():
        nonlocal assistant_message
        if assistant_message is None:
            return
        if not assistant_message.get("tool_calls"):
            assistant_message.pop("tool_calls", None)
        if not assistant_message.get("reasoning_content"):
            assistant_message.pop("reasoning_content", None)
        messages.append(assistant_message)
        assistant_message = None

    def append_message_item(item):
        role = item.get("role", "user")
        if role in ("developer", "system"):
            role = "system"
        elif role not in ("user", "assistant", "tool"):
            role = "user"
        content = content_to_chat(item.get("content"))
        if role == "assistant":
            assistant = ensure_assistant()
            if content:
                assistant["content"] = content if not assistant.get("content") else assistant["content"] + "\n" + content
            reasoning_content = extract_reasoning_content(item)
            if reasoning_content:
                assistant["reasoning_content"] = reasoning_content
        else:
            flush_assistant()
            messages.append({"role": role, "content": content})

    for item in response_input:
        if not isinstance(item, dict):
            flush_assistant()
            messages.append({"role": "user", "content": content_to_chat(item)})
            continue

        item_type = item.get("type")
        if item_type == "message" or (item_type is None and ("role" in item or "content" in item)):
            append_message_item(item)
        elif item_type == "function_call":
            call_id = item.get("call_id") or item.get("id") or "call_unknown"
            assistant = ensure_assistant()
            if not assistant.get("reasoning_content"):
                assistant["reasoning_content"] = extract_reasoning_content(item) or lookup_reasoning(server, call_id)
            assistant.setdefault("tool_calls", []).append(
                {
                    "id": call_id,
                    "type": "function",
                    "function": {
                        "name": item.get("name", ""),
                        "arguments": item.get("arguments", "{}"),
                    },
                }
            )
        elif item_type == "function_call_output":
            flush_assistant()
            call_id = item.get("call_id") or item.get("id") or "call_unknown"
            messages.append({"role": "tool", "tool_call_id": call_id, "content": content_to_chat(item.get("output"))})
        elif item_type == "reasoning":
            reasoning_content = extract_reasoning_content(item)
            if reasoning_content:
                ensure_assistant()["reasoning_content"] = reasoning_content
        else:
            flush_assistant()
            messages.append({"role": "user", "content": json_dumps(item)})

    flush_assistant()
    return messages


def responses_tools_to_chat_tools(tools):
    chat_tools = []
    for tool in tools or []:
        if not isinstance(tool, dict) or tool.get("type") != "function":
            continue
        name = tool.get("name")
        if not name:
            continue
        chat_tools.append(
            {
                "type": "function",
                "function": {
                    "name": name,
                    "description": tool.get("description", ""),
                    "parameters": tool.get("parameters", {"type": "object", "properties": {}}),
                },
            }
        )
    return chat_tools


def responses_to_chat_request(request, default_model, server=None):
    chat_request = {
        "model": request.get("model") or default_model,
        "messages": responses_input_to_messages(request, server),
        "stream": bool(request.get("stream", True)),
    }
    tools = responses_tools_to_chat_tools(request.get("tools"))
    if tools:
        chat_request["tools"] = tools
        chat_request["tool_choice"] = "auto"
    if "tool_choice" in request and request["tool_choice"] in ("none", "auto", "required"):
        chat_request["tool_choice"] = request["tool_choice"]
    if "response_format" in request:
        chat_request["response_format"] = request["response_format"]
    if "stop" in request:
        chat_request["stop"] = request["stop"]
    if "stream_options" in request:
        chat_request["stream_options"] = request["stream_options"]
    if "user" in request:
        chat_request["user_id"] = request["user"]
    if "user_id" in request:
        chat_request["user_id"] = request["user_id"]
    if "frequency_penalty" in request:
        chat_request["frequency_penalty"] = request["frequency_penalty"]
    if "presence_penalty" in request:
        chat_request["presence_penalty"] = request["presence_penalty"]

    deepseek_v4 = is_deepseek_v4_model(chat_request["model"])
    if "temperature" in request and not deepseek_v4:
        chat_request["temperature"] = request["temperature"]
    if "top_p" in request and not deepseek_v4:
        chat_request["top_p"] = request["top_p"]
    if "max_output_tokens" in request:
        chat_request["max_tokens"] = request["max_output_tokens"]
    if "max_tokens" in request:
        chat_request["max_tokens"] = request["max_tokens"]

    reasoning_effort = None
    reasoning = request.get("reasoning")
    if isinstance(reasoning, dict):
        reasoning_effort = reasoning.get("effort")
        thinking_type = reasoning.get("thinking")
    else:
        thinking_type = None
    reasoning_effort = request.get("reasoning_effort") or reasoning_effort
    thinking_type = request.get("thinking") or thinking_type
    if deepseek_v4:
        normalized_thinking = normalize_thinking_type(
            thinking_type,
            default=getattr(server, "default_thinking_type", DEFAULT_THINKING_TYPE),
        )
        chat_request["thinking"] = {"type": normalized_thinking}
    if not reasoning_effort and deepseek_v4:
        reasoning_effort = getattr(server, "default_reasoning_effort", DEFAULT_REASONING_EFFORT)
    if reasoning_effort and (deepseek_v4 or request.get("reasoning_effort")):
        normalized_effort = normalize_reasoning_effort(reasoning_effort, default=None)
        if normalized_effort:
            chat_request["reasoning_effort"] = normalized_effort
    return chat_request


def select_model(request, path, default_model, compact_model, vision_model):
    if request_has_image(request):
        return vision_model
    if path.endswith("/compact"):
        return compact_model
    return request.get("model") or default_model
