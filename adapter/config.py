DEFAULT_UPSTREAM = "https://opencode.ai/zen/go/v1/chat/completions"
DEFAULT_MODEL = "deepseek-v4-pro"
DEFAULT_COMPACT_MODEL = "deepseek-v4-flash"
DEFAULT_VISION_MODEL = "kimi-k2.6"
DEFAULT_REASONING_EFFORT = "max"
DEFAULT_THINKING_TYPE = "enabled"

REASONING_CACHE_LIMIT = 4096
REASONING_ENCRYPTED_PREFIX = "opencode-go-codex/deepseek-reasoning/v1"

REASONING_EFFORT_ALIASES = {
    "low": "high",
    "medium": "high",
    "high": "high",
    "xhigh": "max",
    "max": "max",
}

THINKING_TYPE_ALIASES = {
    "enabled": "enabled",
    "enable": "enabled",
    "true": "enabled",
    "1": "enabled",
    "disabled": "disabled",
    "disable": "disabled",
    "false": "disabled",
    "0": "disabled",
}


def normalize_upstream(url):
    url = url.rstrip("/")
    if url.endswith("/chat/completions"):
        return url
    return url + "/chat/completions"


def normalize_reasoning_effort(value, default=DEFAULT_REASONING_EFFORT):
    if value is None:
        return default
    return REASONING_EFFORT_ALIASES.get(str(value).lower(), default)


def normalize_thinking_type(value, default=DEFAULT_THINKING_TYPE):
    if value is None:
        return default
    return THINKING_TYPE_ALIASES.get(str(value).lower(), default)


def is_deepseek_v4_model(model):
    return str(model or "").startswith("deepseek-v4")
