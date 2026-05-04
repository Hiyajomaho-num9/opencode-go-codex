#!/usr/bin/env python3
import html
import json
import re
import sys
import time
import urllib.error
import urllib.parse
import urllib.request


def write_message(payload):
    sys.stdout.write(json.dumps(payload, ensure_ascii=False, separators=(",", ":")) + "\n")
    sys.stdout.flush()


def text_content(text):
    return [{"type": "text", "text": text}]


def request_json(url, timeout=20, retries=2):
    req = urllib.request.Request(
        url,
        headers={
            "accept": "application/json,text/html",
            "user-agent": "Mozilla/5.0 opencode-go-codex-web-search-mcp/0.1",
        },
    )
    last_error = None
    for attempt in range(retries + 1):
        try:
            with urllib.request.urlopen(req, timeout=timeout) as resp:
                return resp.read().decode("utf-8", "replace")
        except urllib.error.HTTPError as exc:
            last_error = exc
            if exc.code != 429 or attempt >= retries:
                raise
            time.sleep(1.5 * (attempt + 1))
        except urllib.error.URLError as exc:
            last_error = exc
            if attempt >= retries:
                raise
            time.sleep(1.0 * (attempt + 1))
    raise last_error


def search_duckduckgo(query, max_results=5):
    url = "https://api.duckduckgo.com/?" + urllib.parse.urlencode(
        {
            "q": query,
            "format": "json",
            "no_html": "1",
            "skip_disambig": "1",
        }
    )
    data = json.loads(request_json(url))
    results = []

    if data.get("AbstractText"):
        results.append(
            {
                "title": data.get("Heading") or "DuckDuckGo Abstract",
                "url": data.get("AbstractURL") or "",
                "snippet": data.get("AbstractText") or "",
            }
        )

    def collect(topic):
        if len(results) >= max_results:
            return
        if not isinstance(topic, dict):
            return
        if "Topics" in topic:
            for child in topic.get("Topics") or []:
                collect(child)
            return
        text = topic.get("Text")
        first_url = topic.get("FirstURL")
        if text or first_url:
            results.append({"title": text or first_url, "url": first_url or "", "snippet": text or ""})

    for topic in data.get("RelatedTopics") or []:
        collect(topic)
        if len(results) >= max_results:
            break

    return results[:max_results]


def search_brave_fallback(query, max_results=5):
    url = "https://search.brave.com/search?q=" + urllib.parse.quote_plus(query)
    body = request_json(url)
    results = []
    for match in re.finditer(r'<a[^>]+href="([^"]+)"[^>]*>(.*?)</a>', body, re.I | re.S):
        href = html.unescape(match.group(1))
        title = re.sub(r"<[^>]+>", "", match.group(2))
        title = html.unescape(re.sub(r"\s+", " ", title)).strip()
        if not href.startswith("http") or not title:
            continue
        if any(item["url"] == href for item in results):
            continue
        results.append({"title": title, "url": href, "snippet": ""})
        if len(results) >= max_results:
            break
    return results


def search_bing_fallback(query, max_results=5):
    url = "https://www.bing.com/search?q=" + urllib.parse.quote_plus(query)
    body = request_json(url)
    results = []
    for match in re.finditer(r'<li class="b_algo".*?<h2[^>]*>.*?<a[^>]+href="([^"]+)"[^>]*>(.*?)</a>.*?(?:<p[^>]*>(.*?)</p>)?', body, re.I | re.S):
        href = html.unescape(match.group(1))
        title = html.unescape(re.sub(r"<[^>]+>", "", match.group(2))).strip()
        snippet = html.unescape(re.sub(r"<[^>]+>", "", match.group(3) or "")).strip()
        title = re.sub(r"\s+", " ", title)
        snippet = re.sub(r"\s+", " ", snippet)
        if not href.startswith("http") or not title:
            continue
        if any(item["url"] == href for item in results):
            continue
        results.append({"title": title, "url": href, "snippet": snippet})
        if len(results) >= max_results:
            break
    return results


def web_search(arguments):
    query = str(arguments.get("query") or "").strip()
    max_results = int(arguments.get("max_results") or 5)
    max_results = max(1, min(max_results, 10))
    if not query:
        return {"error": "query is required"}
    errors = []
    results = []
    for searcher in (search_duckduckgo, search_brave_fallback, search_bing_fallback):
        try:
            results = searcher(query, max_results)
        except Exception as exc:
            errors.append(f"{searcher.__name__}: {exc}")
            continue
        if results:
            break
    if not results and errors:
        return {"query": query, "results": [], "errors": errors}
    return {"query": query, "results": results}


TOOLS = [
    {
        "name": "web_search",
        "description": "Search the web and return concise result titles, URLs, and snippets.",
        "inputSchema": {
            "type": "object",
            "properties": {
                "query": {"type": "string", "description": "Search query."},
                "max_results": {
                    "type": "integer",
                    "description": "Maximum number of results, 1 to 10.",
                    "default": 5,
                },
            },
            "required": ["query"],
            "additionalProperties": False,
        },
    }
]


def handle_request(message):
    method = message.get("method")
    request_id = message.get("id")
    if method == "initialize":
        return {
            "jsonrpc": "2.0",
            "id": request_id,
            "result": {
                "protocolVersion": "2025-06-18",
                "capabilities": {"tools": {}},
                "serverInfo": {"name": "opencode-go-web-search", "version": "0.1.0"},
            },
        }
    if method == "tools/list":
        return {"jsonrpc": "2.0", "id": request_id, "result": {"tools": TOOLS}}
    if method == "resources/list":
        return {"jsonrpc": "2.0", "id": request_id, "result": {"resources": []}}
    if method == "resources/templates/list":
        return {"jsonrpc": "2.0", "id": request_id, "result": {"resourceTemplates": []}}
    if method == "tools/call":
        params = message.get("params") or {}
        name = params.get("name")
        arguments = params.get("arguments") or {}
        try:
            if name == "web_search":
                result = web_search(arguments)
            else:
                raise ValueError(f"unknown tool: {name}")
            return {
                "jsonrpc": "2.0",
                "id": request_id,
                "result": {"content": text_content(json.dumps(result, ensure_ascii=False, indent=2))},
            }
        except Exception as exc:
            return {
                "jsonrpc": "2.0",
                "id": request_id,
                "result": {"isError": True, "content": text_content(str(exc))},
            }
    if method and request_id is not None:
        return {
            "jsonrpc": "2.0",
            "id": request_id,
            "error": {"code": -32601, "message": f"method not found: {method}"},
        }
    return None


def main():
    for line in sys.stdin:
        line = line.strip()
        if not line:
            continue
        try:
            message = json.loads(line)
        except json.JSONDecodeError as exc:
            write_message({"jsonrpc": "2.0", "error": {"code": -32700, "message": str(exc)}})
            continue
        response = handle_request(message)
        if response is not None:
            write_message(response)


if __name__ == "__main__":
    main()
