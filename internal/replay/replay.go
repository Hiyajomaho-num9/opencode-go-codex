package replay

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

type Event struct {
	Event string         `json:"event"`
	Data  map[string]any `json:"data"`
}

func Run(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: opencode-go-codex replay TRACE_DIR [--summary]")
		return 2
	}
	traceDir := args[0]
	summary := false
	for _, arg := range args[1:] {
		if arg == "--summary" {
			summary = true
		}
	}
	events, err := Load(filepath.Join(traceDir, "response_events.jsonl"))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if summary {
		PrintSummary(events)
		return 0
	}
	PrintSSE(events)
	return 0
}

func Load(path string) ([]Event, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("missing trace events: %s", path)
	}
	defer f.Close()
	events := []Event{}
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var event Event
		if err := json.Unmarshal(line, &event); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, scanner.Err()
}

func PrintSSE(events []Event) {
	for _, event := range events {
		b, _ := json.Marshal(event.Data)
		name := event.Event
		if name == "" {
			name = "message"
		}
		fmt.Printf("event: %s\n", name)
		fmt.Printf("data: %s\n\n", b)
	}
}

func PrintSummary(events []Event) {
	counts := map[string]int{}
	for _, event := range events {
		name := event.Event
		if name == "" {
			name = "message"
		}
		counts[name]++
	}
	keys := make([]string, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	fmt.Println("events:")
	for _, key := range keys {
		fmt.Printf("  %s: %d\n", key, counts[key])
	}
	for i := len(events) - 1; i >= 0; i-- {
		if events[i].Event != "response.completed" {
			continue
		}
		response, _ := events[i].Data["response"].(map[string]any)
		fmt.Println("completed:")
		fmt.Printf("  id: %v\n", response["id"])
		fmt.Printf("  model: %v\n", response["model"])
		fmt.Printf("  status: %v\n", response["status"])
		if output, ok := response["output"].([]any); ok {
			fmt.Printf("  output_items: %d\n", len(output))
		}
		return
	}
}
