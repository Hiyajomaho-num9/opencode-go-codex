package transform

import "testing"

func TestPlainRoleContentMessageIsPreserved(t *testing.T) {
	request := map[string]any{"input": []any{
		map[string]any{"role": "user", "content": []any{map[string]any{"type": "input_text", "text": "hello"}}},
		map[string]any{"role": "assistant", "content": []any{map[string]any{"type": "output_text", "text": "world"}}},
	}}
	messages := ResponsesInputToMessages(request, Defaults{})
	if len(messages) != 2 || messages[0]["content"] != "hello" || messages[1]["content"] != "world" {
		t.Fatalf("unexpected messages: %#v", messages)
	}
	if _, ok := messages[1]["reasoning_content"]; ok {
		t.Fatalf("plain assistant content must not become reasoning_content: %#v", messages[1])
	}
}

func TestImageBase64RoutesToVision(t *testing.T) {
	request := map[string]any{"input": []any{map[string]any{"role": "user", "content": []any{map[string]any{"type": "image_url", "image_base64": "AAAA", "media_type": "image/png"}}}}}
	model := SelectModel(request, "/v1/responses", "deepseek-v4-pro", "deepseek-v4-flash", "kimi-k2.6")
	if model != "kimi-k2.6" {
		t.Fatalf("model = %q", model)
	}
	messages := ResponsesInputToMessages(request, Defaults{})
	parts := messages[0]["content"].([]any)
	image := parts[0].(map[string]any)["image_url"].(map[string]any)["url"]
	if image != "data:image/png;base64,AAAA" {
		t.Fatalf("image url = %#v", image)
	}
}

func TestOpenAIImageURLObjectIsNotDoubleWrapped(t *testing.T) {
	request := map[string]any{"input": []any{map[string]any{"type": "message", "role": "user", "content": []any{map[string]any{"type": "image_url", "image_url": map[string]any{"url": "data:image/png;base64,BBBB"}}}}}}
	messages := ResponsesInputToMessages(request, Defaults{})
	parts := messages[0]["content"].([]any)
	imageURL := parts[0].(map[string]any)["image_url"].(map[string]any)
	if imageURL["url"] != "data:image/png;base64,BBBB" || len(imageURL) != 1 {
		t.Fatalf("bad image_url: %#v", imageURL)
	}
}

func TestNestedContentKeepsImageParts(t *testing.T) {
	request := map[string]any{"input": []any{map[string]any{"type": "message", "role": "user", "content": []any{
		map[string]any{"type": "wrapper", "content": []any{map[string]any{"type": "image_url", "image_url": map[string]any{"url": "data:image/png;base64,CCCC"}}}},
	}}}}
	model := SelectModel(request, "/v1/responses", "deepseek-v4-pro", "deepseek-v4-flash", "kimi-k2.6")
	if model != "kimi-k2.6" {
		t.Fatalf("model = %q", model)
	}
	messages := ResponsesInputToMessages(request, Defaults{})
	parts := messages[0]["content"].([]any)
	imageURL := parts[0].(map[string]any)["image_url"].(map[string]any)
	if imageURL["url"] != "data:image/png;base64,CCCC" {
		t.Fatalf("bad nested image_url: %#v", imageURL)
	}
}

func TestXHighMapsToMax(t *testing.T) {
	request := map[string]any{"model": "deepseek-v4-pro", "input": "hello", "reasoning_effort": "xhigh"}
	chat := ResponsesToChatRequest(request, "deepseek-v4-pro", Defaults{DefaultReasoningEffort: "max", DefaultThinkingType: "enabled"})
	thinking := chat["thinking"].(map[string]any)
	if thinking["type"] != "enabled" || chat["reasoning_effort"] != "max" {
		t.Fatalf("bad thinking/reasoning: %#v", chat)
	}
}

func TestCompactUsesFlashUnlessImagePresent(t *testing.T) {
	text := map[string]any{"input": "compact me"}
	if SelectModel(text, "/v1/responses/compact", "deepseek-v4-pro", "deepseek-v4-flash", "kimi-k2.6") != "deepseek-v4-flash" {
		t.Fatal("text compact did not route to flash")
	}
	image := map[string]any{"input": []any{map[string]any{"content": []any{map[string]any{"type": "input_image", "image_url": "data:image/png;base64,AAAA"}}}}}
	if SelectModel(image, "/v1/responses/compact", "deepseek-v4-pro", "deepseek-v4-flash", "kimi-k2.6") != "kimi-k2.6" {
		t.Fatal("image compact did not route to vision")
	}
}
