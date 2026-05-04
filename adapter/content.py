from .jsonutil import json_dumps


def content_to_chat(content):
    if content is None:
        return ""
    if isinstance(content, str):
        return content
    if isinstance(content, list):
        parts = []
        has_non_text = False
        for item in content:
            if isinstance(item, str):
                parts.append({"type": "text", "text": item})
                continue
            if not isinstance(item, dict):
                parts.append({"type": "text", "text": str(item)})
                continue
            item_type = item.get("type")
            if item_type in ("input_text", "output_text", "text"):
                parts.append({"type": "text", "text": str(item.get("text", ""))})
            elif item_type in ("input_image", "image_url"):
                image_url = item.get("image_url") or item.get("url")
                if not image_url and item.get("image_base64"):
                    mime = item.get("media_type") or "image/png"
                    image_url = f"data:{mime};base64,{item['image_base64']}"
                if image_url:
                    parts.append({"type": "image_url", "image_url": {"url": image_url}})
                    has_non_text = True
            elif "content" in item:
                nested = content_to_chat(item["content"])
                if isinstance(nested, list):
                    parts.extend(nested)
                    has_non_text = True
                elif nested:
                    parts.append({"type": "text", "text": nested})
            else:
                parts.append({"type": "text", "text": json_dumps(item)})
        if has_non_text:
            return parts
        return "\n".join(part["text"] for part in parts if part.get("text"))
    if isinstance(content, dict):
        if "text" in content:
            return str(content["text"])
        if "content" in content:
            return content_to_chat(content["content"])
    return str(content)


IMAGE_TYPE_MARKERS = {"input_image", "image_url"}
IMAGE_KEY_MARKERS = {"image_url", "image_base64"}


def value_has_image(value):
    if value is None:
        return False
    if isinstance(value, list):
        return any(value_has_image(item) for item in value)
    if isinstance(value, dict):
        item_type = value.get("type")
        if item_type in IMAGE_TYPE_MARKERS:
            return True
        if any(key in value and value.get(key) for key in IMAGE_KEY_MARKERS):
            return True
        return any(value_has_image(child) for child in value.values())
    return False


def request_has_image(request):
    return value_has_image(request)
