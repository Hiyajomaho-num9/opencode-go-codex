package sse

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/Hiyajomaho-num9/opencode-go-codex/internal/reasoning"
)

type mockWriter struct {
	bytes.Buffer
	header http.Header
}

func (m *mockWriter) Header() http.Header {
	if m.header == nil {
		m.header = http.Header{}
	}
	return m.header
}
func (m *mockWriter) WriteHeader(statusCode int) {}
func (m *mockWriter) Flush()                     {}

func TestStreamToolCallFragmentsAreJoined(t *testing.T) {
	body := sseLines(
		`{"choices":[{"delta":{"reasoning_content":"think "}}]}`,
		`{"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_1","function":{"name":"read_"}}]}}]}`,
		`{"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"name":"file","arguments":"{\"path\""}}]}}]}`,
		`{"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":":\"README.md\"}"}}]}}]}`,
	)
	w := &mockWriter{}
	if err := StreamChatToResponses(Writer{W: w}, strings.NewReader(body), "deepseek-v4-pro", reasoning.NewStore()); err != nil {
		t.Fatal(err)
	}
	events := parseEvents(t, w.String())
	completed := events[len(events)-1]["data"].(map[string]any)["response"].(map[string]any)
	output := completed["output"].([]any)
	var call map[string]any
	for _, item := range output {
		m := item.(map[string]any)
		if m["type"] == "function_call" {
			call = m
		}
	}
	if call == nil || call["call_id"] != "call_1" || call["name"] != "read_file" || call["arguments"] != "{\"path\":\"README.md\"}" {
		t.Fatalf("bad call: %#v", call)
	}
}

func TestStreamTextDeltas(t *testing.T) {
	body := sseLines(
		`{"choices":[{"delta":{"content":"hel"}}]}`,
		`{"choices":[{"delta":{"content":"lo"}}]}`,
	)
	w := &mockWriter{}
	if err := StreamChatToResponses(Writer{W: w}, strings.NewReader(body), "deepseek-v4-flash", reasoning.NewStore()); err != nil {
		t.Fatal(err)
	}
	events := parseEvents(t, w.String())
	text := ""
	for _, event := range events {
		if event["event"] == "response.output_text.delta" {
			text += event["data"].(map[string]any)["delta"].(string)
		}
	}
	if text != "hello" {
		t.Fatalf("text = %q", text)
	}
}

func TestStreamToolBeforeTextUsesUniqueOutputIndexes(t *testing.T) {
	body := sseLines(
		`{"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_1","function":{"name":"read_file","arguments":"{\"path\":\"README.md\"}"}}]}}]}`,
		`{"choices":[{"delta":{"content":"done"}}]}`,
	)
	w := &mockWriter{}
	if err := StreamChatToResponses(Writer{W: w}, strings.NewReader(body), "deepseek-v4-pro", reasoning.NewStore()); err != nil {
		t.Fatal(err)
	}
	events := parseEvents(t, w.String())
	seen := map[int]string{}
	for _, event := range events {
		if event["event"] != "response.output_item.added" {
			continue
		}
		data := event["data"].(map[string]any)
		index := int(data["output_index"].(float64))
		item := data["item"].(map[string]any)
		if previous, ok := seen[index]; ok {
			t.Fatalf("duplicate output_index %d for %s and %s", index, previous, item["type"])
		}
		seen[index] = item["type"].(string)
	}
	if seen[0] != "function_call" || seen[1] != "message" {
		t.Fatalf("bad output indexes: %#v", seen)
	}
	completed := events[len(events)-1]["data"].(map[string]any)["response"].(map[string]any)
	output := completed["output"].([]any)
	if output[0].(map[string]any)["type"] != "function_call" || output[1].(map[string]any)["type"] != "message" {
		t.Fatalf("completed output order mismatch: %#v", output)
	}
}

func sseLines(chunks ...string) string {
	out := ""
	for _, chunk := range chunks {
		out += "data: " + chunk + "\n\n"
	}
	return out + "data: [DONE]\n\n"
}

func parseEvents(t *testing.T, raw string) []map[string]any {
	t.Helper()
	blocks := strings.Split(strings.TrimSpace(raw), "\n\n")
	events := []map[string]any{}
	for _, block := range blocks {
		event := map[string]any{}
		for _, line := range strings.Split(block, "\n") {
			if strings.HasPrefix(line, "event: ") {
				event["event"] = strings.TrimPrefix(line, "event: ")
			}
			if strings.HasPrefix(line, "data: ") {
				var data map[string]any
				if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &data); err != nil {
					t.Fatal(err)
				}
				event["data"] = data
			}
		}
		events = append(events, event)
	}
	return events
}
