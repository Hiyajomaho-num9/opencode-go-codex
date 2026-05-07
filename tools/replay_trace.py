#!/usr/bin/env python3
import argparse
import json
import sys
from pathlib import Path


def load_jsonl_events(path):
    events = []
    with path.open("r", encoding="utf-8") as f:
        for line in f:
            line = line.strip()
            if not line:
                continue
            events.append(json.loads(line))
    return events


def print_sse(events):
    for item in events:
        event = item.get("event", "message")
        data = item.get("data", {})
        print(f"event: {event}")
        print("data: " + json.dumps(data, ensure_ascii=False, separators=(",", ":")))
        print()


def summarize(events):
    counts = {}
    for item in events:
        event = item.get("event", "message")
        counts[event] = counts.get(event, 0) + 1
    print("events:")
    for event, count in sorted(counts.items()):
        print(f"  {event}: {count}")
    completed = [item for item in events if item.get("event") == "response.completed"]
    if completed:
        response = completed[-1].get("data", {}).get("response", {})
        print("completed:")
        print(f"  id: {response.get('id')}")
        print(f"  model: {response.get('model')}")
        print(f"  status: {response.get('status')}")
        print(f"  output_items: {len(response.get('output') or [])}")


def main():
    parser = argparse.ArgumentParser(description="Replay or summarize an opencode-go-codex trace directory.")
    parser.add_argument("trace_dir", help="trace directory containing response_events.jsonl")
    parser.add_argument("--summary", action="store_true", help="print a compact event summary instead of SSE")
    args = parser.parse_args()

    trace_dir = Path(args.trace_dir).expanduser()
    events_path = trace_dir / "response_events.jsonl"
    if not events_path.exists():
        print(f"missing trace events: {events_path}", file=sys.stderr)
        return 1

    events = load_jsonl_events(events_path)
    if args.summary:
        summarize(events)
    else:
        print_sse(events)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
