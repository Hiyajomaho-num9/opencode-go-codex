import json


def json_dumps(value):
    return json.dumps(value, ensure_ascii=False, separators=(",", ":"))
