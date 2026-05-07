package sse

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/Hiyajomaho-num9/opencode-go-codex/internal/reasoning"
	"github.com/Hiyajomaho-num9/opencode-go-codex/internal/responses"
	"github.com/Hiyajomaho-num9/opencode-go-codex/internal/trace"
)

type Writer struct {
	W     http.ResponseWriter
	Trace *trace.Trace
}

func (w Writer) Event(event string, data any) error {
	if w.Trace != nil {
		w.Trace.AppendEvent(event, data)
	}
	b, err := json.Marshal(data)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w.W, "event: %s\ndata: %s\n\n", event, b); err != nil {
		return err
	}
	if f, ok := w.W.(http.Flusher); ok {
		f.Flush()
	}
	return nil
}

func EmitTextStream(w Writer, model, text string, usage map[string]any) error {
	responseID := responses.NewID("resp")
	messageID := responses.NewID("msg")
	item := map[string]any{"id": messageID, "type": "message", "status": "completed", "role": "assistant", "content": []any{map[string]any{"type": "output_text", "text": text, "annotations": []any{}}}}
	response := responses.MakeResponse(model, []any{item}, usage, "completed")
	response["id"] = responseID
	created := cloneMap(response)
	created["status"] = "in_progress"
	created["output"] = []any{}
	if err := w.Event("response.created", map[string]any{"type": "response.created", "response": created}); err != nil {
		return err
	}
	if err := w.Event("response.output_item.added", map[string]any{"type": "response.output_item.added", "output_index": 0, "item": map[string]any{"id": messageID, "type": "message", "status": "in_progress", "role": "assistant", "content": []any{}}}); err != nil {
		return err
	}
	if err := w.Event("response.content_part.added", map[string]any{"type": "response.content_part.added", "item_id": messageID, "output_index": 0, "content_index": 0, "part": map[string]any{"type": "output_text", "text": "", "annotations": []any{}}}); err != nil {
		return err
	}
	if text != "" {
		if err := w.Event("response.output_text.delta", map[string]any{"type": "response.output_text.delta", "item_id": messageID, "output_index": 0, "content_index": 0, "delta": text}); err != nil {
			return err
		}
	}
	if err := w.Event("response.output_text.done", map[string]any{"type": "response.output_text.done", "item_id": messageID, "output_index": 0, "content_index": 0, "text": text}); err != nil {
		return err
	}
	if err := w.Event("response.content_part.done", map[string]any{"type": "response.content_part.done", "item_id": messageID, "output_index": 0, "content_index": 0, "part": item["content"].([]any)[0]}); err != nil {
		return err
	}
	if err := w.Event("response.output_item.done", map[string]any{"type": "response.output_item.done", "output_index": 0, "item": item}); err != nil {
		return err
	}
	return w.Event("response.completed", map[string]any{"type": "response.completed", "response": response})
}

func EmitFinalStream(w Writer, model, text string, toolCalls []map[string]any, reasoningContent string, usage map[string]any) error {
	output := []any{}
	if reasoningContent != "" {
		output = append(output, reasoning.MakeItem(reasoningContent))
	}
	if text != "" {
		output = append(output, map[string]any{"id": responses.NewID("msg"), "type": "message", "status": "completed", "role": "assistant", "content": []any{map[string]any{"type": "output_text", "text": text, "annotations": []any{}}}})
	}
	for _, call := range toolCalls {
		output = append(output, map[string]any{"id": call["item_id"], "type": "function_call", "status": "completed", "call_id": call["call_id"], "name": call["name"], "arguments": call["arguments"]})
	}
	response := responses.MakeResponse(model, output, usage, "completed")
	return w.Event("response.completed", map[string]any{"type": "response.completed", "response": response})
}

func StreamChatToResponses(w Writer, upstream io.Reader, model string, store *reasoning.Store) error {
	responseID := responses.NewID("resp")
	textItemID := responses.NewID("msg")
	textStarted := false
	text := ""
	reasoningContent := ""
	toolCalls := map[int]map[string]any{}
	outputCount := 0
	var usage map[string]any
	nextOutputIndex := 0
	textOutputIndex := -1
	created := responses.MakeResponse(model, []any{}, nil, "in_progress")
	created["id"] = responseID
	if err := w.Event("response.created", map[string]any{"type": "response.created", "response": created}); err != nil {
		return err
	}

	scanner := bufio.NewScanner(upstream)
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for scanner.Scan() {
		lineBytes := append([]byte{}, scanner.Bytes()...)
		if w.Trace != nil {
			w.Trace.AppendText("upstream_stream.raw", append(lineBytes, '\n'))
		}
		line := strings.TrimSpace(string(lineBytes))
		if line == "" || !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			break
		}
		var chunk map[string]any
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		if u, ok := chunk["usage"].(map[string]any); ok {
			usage = u
		}
		choices, _ := chunk["choices"].([]any)
		for _, rawChoice := range choices {
			choice, _ := rawChoice.(map[string]any)
			delta, _ := choice["delta"].(map[string]any)
			if rd, ok := delta["reasoning_content"].(string); ok && rd != "" {
				reasoningContent += rd
			}
			if c, ok := delta["content"].(string); ok && c != "" {
				if !textStarted {
					textStarted = true
					textOutputIndex = nextOutputIndex
					nextOutputIndex++
					item := map[string]any{"id": textItemID, "type": "message", "status": "in_progress", "role": "assistant", "content": []any{}}
					if err := w.Event("response.output_item.added", map[string]any{"type": "response.output_item.added", "output_index": textOutputIndex, "item": item}); err != nil {
						return err
					}
					if err := w.Event("response.content_part.added", map[string]any{"type": "response.content_part.added", "item_id": textItemID, "output_index": textOutputIndex, "content_index": 0, "part": map[string]any{"type": "output_text", "text": "", "annotations": []any{}}}); err != nil {
						return err
					}
				}
				text += c
				if err := w.Event("response.output_text.delta", map[string]any{"type": "response.output_text.delta", "item_id": textItemID, "output_index": textOutputIndex, "content_index": 0, "delta": c}); err != nil {
					return err
				}
			}
			toolDeltas, _ := delta["tool_calls"].([]any)
			for _, rawTool := range toolDeltas {
				tool, _ := rawTool.(map[string]any)
				idx := intValue(tool["index"], 0)
				fn, _ := tool["function"].(map[string]any)
				call, ok := toolCalls[idx]
				if !ok {
					call = map[string]any{"item_id": responses.NewID("fc"), "call_id": firstString(tool["id"], responses.NewID("call")), "name": "", "arguments": "", "output_index": nil, "added": false}
					toolCalls[idx] = call
				}
				if id, ok := tool["id"].(string); ok && id != "" {
					call["call_id"] = id
				}
				if name, ok := fn["name"].(string); ok && name != "" {
					call["name"] = call["name"].(string) + name
				}
				if call["added"] != true && call["name"].(string) != "" {
					call["output_index"] = nextOutputIndex
					nextOutputIndex++
					item := map[string]any{"id": call["item_id"], "type": "function_call", "status": "in_progress", "call_id": call["call_id"], "name": call["name"], "arguments": ""}
					if err := w.Event("response.output_item.added", map[string]any{"type": "response.output_item.added", "output_index": call["output_index"], "item": item}); err != nil {
						return err
					}
					call["added"] = true
				}
				if args, ok := fn["arguments"].(string); ok && args != "" {
					call["arguments"] = call["arguments"].(string) + args
					if call["added"] == true {
						if err := w.Event("response.function_call_arguments.delta", map[string]any{"type": "response.function_call_arguments.delta", "item_id": call["item_id"], "output_index": call["output_index"], "delta": args}); err != nil {
							return err
						}
					}
				}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}

	completed := []any{}
	if reasoningContent != "" {
		completed = append(completed, reasoning.MakeItem(reasoningContent))
	}
	if textStarted {
		content := map[string]any{"type": "output_text", "text": text, "annotations": []any{}}
		item := map[string]any{"id": textItemID, "type": "message", "status": "completed", "role": "assistant", "content": []any{content}}
		if err := w.Event("response.output_text.done", map[string]any{"type": "response.output_text.done", "item_id": textItemID, "output_index": textOutputIndex, "content_index": 0, "text": text}); err != nil {
			return err
		}
		if err := w.Event("response.content_part.done", map[string]any{"type": "response.content_part.done", "item_id": textItemID, "output_index": textOutputIndex, "content_index": 0, "part": content}); err != nil {
			return err
		}
		if err := w.Event("response.output_item.done", map[string]any{"type": "response.output_item.done", "output_index": textOutputIndex, "item": item}); err != nil {
			return err
		}
		completed = append(completed, map[string]any{"output_index": textOutputIndex, "item": item})
		outputCount++
	}
	callIDs := []string{}
	for _, idx := range sortedKeys(toolCalls) {
		call := toolCalls[idx]
		if call["added"] != true {
			call["output_index"] = nextOutputIndex
			nextOutputIndex++
			item := map[string]any{"id": call["item_id"], "type": "function_call", "status": "in_progress", "call_id": call["call_id"], "name": call["name"], "arguments": ""}
			if err := w.Event("response.output_item.added", map[string]any{"type": "response.output_item.added", "output_index": call["output_index"], "item": item}); err != nil {
				return err
			}
		}
		item := map[string]any{"id": call["item_id"], "type": "function_call", "status": "completed", "call_id": call["call_id"], "name": call["name"], "arguments": call["arguments"]}
		if err := w.Event("response.function_call_arguments.done", map[string]any{"type": "response.function_call_arguments.done", "item_id": call["item_id"], "output_index": call["output_index"], "arguments": call["arguments"]}); err != nil {
			return err
		}
		if err := w.Event("response.output_item.done", map[string]any{"type": "response.output_item.done", "output_index": call["output_index"], "item": item}); err != nil {
			return err
		}
		completed = append(completed, map[string]any{"output_index": call["output_index"], "item": item})
		if id, ok := call["call_id"].(string); ok {
			callIDs = append(callIDs, id)
		}
		outputCount++
	}
	if store != nil {
		store.Remember(callIDs, reasoningContent)
	}
	response := responses.MakeResponse(model, completedItems(completed), usage, "completed")
	response["id"] = responseID
	return w.Event("response.completed", map[string]any{"type": "response.completed", "response": response})
}

func cloneMap(in map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range in {
		out[k] = v
	}
	return out
}
func intValue(v any, fallback int) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case json.Number:
		i, _ := strconv.Atoi(n.String())
		return i
	}
	return fallback
}
func firstString(v any, fallback string) string {
	if s, ok := v.(string); ok && s != "" {
		return s
	}
	return fallback
}
func sortedKeys(m map[int]map[string]any) []int {
	keys := make([]int, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[j] < keys[i] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
	return keys
}
func completedItems(indexed []any) []any {
	prefix := []any{}
	sortable := []any{}
	for _, raw := range indexed {
		wrapper, _ := raw.(map[string]any)
		if _, ok := wrapper["item"]; ok {
			sortable = append(sortable, raw)
		} else {
			prefix = append(prefix, raw)
		}
	}
	for i := 0; i < len(sortable); i++ {
		for j := i + 1; j < len(sortable); j++ {
			if indexedOutputIndex(sortable[j]) < indexedOutputIndex(sortable[i]) {
				sortable[i], sortable[j] = sortable[j], sortable[i]
			}
		}
	}
	out := make([]any, 0, len(indexed))
	out = append(out, prefix...)
	for _, raw := range sortable {
		wrapper, _ := raw.(map[string]any)
		out = append(out, wrapper["item"])
	}
	return out
}
func indexedOutputIndex(raw any) int {
	wrapper, _ := raw.(map[string]any)
	return intValue(wrapper["output_index"], 0)
}
