package content

import (
	"fmt"
)

func ToChat(value any) any {
	if value == nil {
		return ""
	}
	switch v := value.(type) {
	case string:
		return v
	case []any:
		parts := make([]map[string]any, 0, len(v))
		hasNonText := false
		for _, item := range v {
			switch it := item.(type) {
			case string:
				parts = append(parts, map[string]any{"type": "text", "text": it})
			case map[string]any:
				itemType, _ := it["type"].(string)
				switch itemType {
				case "input_text", "output_text", "text":
					parts = append(parts, map[string]any{"type": "text", "text": fmt.Sprint(it["text"])})
				case "input_image", "image_url":
					imageURL := imageURLValue(firstNonNil(it["image_url"], it["url"]))
					if imageURL == "" {
						if b64, ok := it["image_base64"].(string); ok && b64 != "" {
							mime, _ := it["media_type"].(string)
							if mime == "" {
								mime = "image/png"
							}
							imageURL = "data:" + mime + ";base64," + b64
						}
					}
					if imageURL != "" {
						parts = append(parts, map[string]any{"type": "image_url", "image_url": map[string]any{"url": imageURL}})
						hasNonText = true
					}
				default:
					if nested, ok := it["content"]; ok {
						converted := ToChat(nested)
						if nestedParts, ok := converted.([]any); ok {
							for _, nestedPart := range nestedParts {
								if part, ok := nestedPart.(map[string]any); ok {
									parts = append(parts, part)
								} else {
									parts = append(parts, map[string]any{"type": "text", "text": fmt.Sprint(nestedPart)})
								}
							}
							hasNonText = true
						} else if s, ok := converted.(string); ok && s != "" {
							parts = append(parts, map[string]any{"type": "text", "text": s})
						}
					} else {
						parts = append(parts, map[string]any{"type": "text", "text": fmt.Sprint(it)})
					}
				}
			default:
				parts = append(parts, map[string]any{"type": "text", "text": fmt.Sprint(item)})
			}
		}
		if hasNonText {
			out := make([]any, 0, len(parts))
			for _, part := range parts {
				out = append(out, part)
			}
			return out
		}
		text := ""
		for _, part := range parts {
			if s, ok := part["text"].(string); ok && s != "" {
				if text != "" {
					text += "\n"
				}
				text += s
			}
		}
		return text
	case map[string]any:
		if text, ok := v["text"]; ok {
			return fmt.Sprint(text)
		}
		if c, ok := v["content"]; ok {
			return ToChat(c)
		}
	}
	return fmt.Sprint(value)
}

func RequestHasImage(request map[string]any) bool {
	return ValueHasImage(request)
}

func ValueHasImage(value any) bool {
	switch v := value.(type) {
	case []any:
		for _, item := range v {
			if ValueHasImage(item) {
				return true
			}
		}
	case map[string]any:
		itemType, _ := v["type"].(string)
		if itemType == "input_image" || itemType == "image_url" {
			if imageURLValue(firstNonNil(v["image_url"], v["url"])) != "" {
				return true
			}
			if b64, ok := v["image_base64"].(string); ok && b64 != "" {
				return true
			}
		}
		for _, child := range v {
			if ValueHasImage(child) {
				return true
			}
		}
	}
	return false
}

func imageURLValue(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case map[string]any:
		if url, ok := v["url"].(string); ok {
			return url
		}
	}
	return ""
}

func firstNonNil(values ...any) any {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}
