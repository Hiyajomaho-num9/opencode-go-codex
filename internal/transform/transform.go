package transform

import (
	"github.com/Hiyajomaho-num9/opencode-go-codex/internal/config"
	"github.com/Hiyajomaho-num9/opencode-go-codex/internal/content"
	"github.com/Hiyajomaho-num9/opencode-go-codex/internal/jsonutil"
	"github.com/Hiyajomaho-num9/opencode-go-codex/internal/reasoning"
)

type Defaults struct {
	DefaultReasoningEffort string
	DefaultThinkingType    string
	ReasoningStore         *reasoning.Store
}

func ResponsesInputToMessages(request map[string]any, defaults Defaults) []map[string]any {
	messages := []map[string]any{}
	if instructions, ok := request["instructions"].(string); ok && instructions != "" {
		messages = append(messages, map[string]any{"role": "system", "content": instructions})
	}

	responseInput, ok := request["input"]
	if !ok {
		return messages
	}
	if s, ok := responseInput.(string); ok {
		if s != "" {
			messages = append(messages, map[string]any{"role": "user", "content": s})
		}
		return messages
	}
	items, ok := responseInput.([]any)
	if !ok {
		messages = append(messages, map[string]any{"role": "user", "content": content.ToChat(responseInput)})
		return messages
	}

	var assistant map[string]any
	ensureAssistant := func() map[string]any {
		if assistant == nil {
			assistant = map[string]any{"role": "assistant", "content": ""}
		}
		return assistant
	}
	flushAssistant := func() {
		if assistant == nil {
			return
		}
		if calls, ok := assistant["tool_calls"].([]any); !ok || len(calls) == 0 {
			delete(assistant, "tool_calls")
		}
		if rc, ok := assistant["reasoning_content"].(string); !ok || rc == "" {
			delete(assistant, "reasoning_content")
		}
		messages = append(messages, assistant)
		assistant = nil
	}

	appendMessageItem := func(item map[string]any) {
		role, _ := item["role"].(string)
		if role == "" {
			role = "user"
		}
		if role == "developer" || role == "system" {
			role = "system"
		} else if role != "user" && role != "assistant" && role != "tool" {
			role = "user"
		}
		chatContent := content.ToChat(item["content"])
		if role == "assistant" {
			a := ensureAssistant()
			if s, ok := chatContent.(string); ok && s != "" {
				if existing, _ := a["content"].(string); existing != "" {
					a["content"] = existing + "\n" + s
				} else {
					a["content"] = s
				}
			} else if chatContent != nil {
				a["content"] = chatContent
			}
			itemType, _ := item["type"].(string)
			if itemType == "reasoning" || item["encrypted_content"] != nil || item["reasoning_content"] != nil {
				if rc := reasoning.Extract(item); rc != "" {
					a["reasoning_content"] = rc
				}
			}
			return
		}
		flushAssistant()
		messages = append(messages, map[string]any{"role": role, "content": chatContent})
	}

	for _, raw := range items {
		item, ok := raw.(map[string]any)
		if !ok {
			flushAssistant()
			messages = append(messages, map[string]any{"role": "user", "content": content.ToChat(raw)})
			continue
		}
		itemType, _ := item["type"].(string)
		switch {
		case itemType == "message" || (itemType == "" && (item["role"] != nil || item["content"] != nil)):
			appendMessageItem(item)
		case itemType == "function_call":
			callID := firstString(item["call_id"], item["id"], "call_unknown")
			a := ensureAssistant()
			if _, ok := a["reasoning_content"]; !ok {
				if rc := reasoning.Extract(item); rc != "" {
					a["reasoning_content"] = rc
				} else if defaults.ReasoningStore != nil {
					if rc := defaults.ReasoningStore.Lookup(callID); rc != "" {
						a["reasoning_content"] = rc
					}
				}
			}
			calls, _ := a["tool_calls"].([]any)
			calls = append(calls, map[string]any{
				"id":   callID,
				"type": "function",
				"function": map[string]any{
					"name":      firstString(item["name"], "", ""),
					"arguments": firstString(item["arguments"], "{}", "{}"),
				},
			})
			a["tool_calls"] = calls
		case itemType == "function_call_output":
			flushAssistant()
			callID := firstString(item["call_id"], item["id"], "call_unknown")
			messages = append(messages, map[string]any{"role": "tool", "tool_call_id": callID, "content": content.ToChat(item["output"])})
		case itemType == "reasoning":
			if rc := reasoning.Extract(item); rc != "" {
				ensureAssistant()["reasoning_content"] = rc
			}
		default:
			flushAssistant()
			messages = append(messages, map[string]any{"role": "user", "content": jsonutil.MustMarshalString(item)})
		}
	}
	flushAssistant()
	return messages
}

func ResponsesToolsToChatTools(tools any) []any {
	items, ok := tools.([]any)
	if !ok {
		return nil
	}
	out := []any{}
	for _, raw := range items {
		tool, ok := raw.(map[string]any)
		if !ok || tool["type"] != "function" {
			continue
		}
		name, _ := tool["name"].(string)
		if name == "" {
			continue
		}
		params := tool["parameters"]
		if params == nil {
			params = map[string]any{"type": "object", "properties": map[string]any{}}
		}
		out = append(out, map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        name,
				"description": firstString(tool["description"], "", ""),
				"parameters":  params,
			},
		})
	}
	return out
}

func ResponsesToChatRequest(request map[string]any, model string, defaults Defaults) map[string]any {
	if model == "" {
		model = firstString(request["model"], "", "")
	}
	chat := map[string]any{
		"model":    model,
		"messages": ResponsesInputToMessages(request, defaults),
		"stream":   boolValue(request["stream"], true),
	}
	if tools := ResponsesToolsToChatTools(request["tools"]); len(tools) > 0 {
		chat["tools"] = tools
		chat["tool_choice"] = "auto"
	}
	if tc, ok := request["tool_choice"].(string); ok && (tc == "none" || tc == "auto" || tc == "required") {
		chat["tool_choice"] = tc
	}
	copyFields(chat, request, map[string]string{
		"response_format":   "response_format",
		"stop":              "stop",
		"stream_options":    "stream_options",
		"frequency_penalty": "frequency_penalty",
		"presence_penalty":  "presence_penalty",
	})
	if user, ok := request["user"]; ok {
		chat["user_id"] = user
	}
	if user, ok := request["user_id"]; ok {
		chat["user_id"] = user
	}
	deepseekV4 := config.IsDeepSeekV4Model(model)
	if !deepseekV4 {
		copyFields(chat, request, map[string]string{"temperature": "temperature", "top_p": "top_p"})
	}
	if max, ok := request["max_output_tokens"]; ok {
		chat["max_tokens"] = max
	}
	if max, ok := request["max_tokens"]; ok {
		chat["max_tokens"] = max
	}
	reasoningEffort := ""
	thinkingType := ""
	if r, ok := request["reasoning"].(map[string]any); ok {
		reasoningEffort = firstString(r["effort"], "", "")
		thinkingType = firstString(r["thinking"], "", "")
	}
	if s := firstString(request["reasoning_effort"], "", ""); s != "" {
		reasoningEffort = s
	}
	if s := firstString(request["thinking"], "", ""); s != "" {
		thinkingType = s
	}
	if deepseekV4 {
		if thinkingType == "" {
			thinkingType = defaults.DefaultThinkingType
		}
		chat["thinking"] = map[string]any{"type": config.NormalizeThinkingType(thinkingType)}
		if reasoningEffort == "" {
			reasoningEffort = defaults.DefaultReasoningEffort
		}
	}
	if reasoningEffort != "" && (deepseekV4 || request["reasoning_effort"] != nil) {
		chat["reasoning_effort"] = config.NormalizeReasoningEffort(reasoningEffort)
	}
	return chat
}

func SelectModel(request map[string]any, path, defaultModel, compactModel, visionModel string) string {
	if content.RequestHasImage(request) {
		return visionModel
	}
	if len(path) >= len("/compact") && path[len(path)-len("/compact"):] == "/compact" {
		return compactModel
	}
	return firstString(request["model"], defaultModel, defaultModel)
}

func firstString(a any, b any, fallback string) string {
	for _, v := range []any{a, b} {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	return fallback
}

func boolValue(value any, fallback bool) bool {
	if value == nil {
		return fallback
	}
	if b, ok := value.(bool); ok {
		return b
	}
	return fallback
}

func copyFields(dst, src map[string]any, fields map[string]string) {
	for from, to := range fields {
		if value, ok := src[from]; ok {
			dst[to] = value
		}
	}
}
