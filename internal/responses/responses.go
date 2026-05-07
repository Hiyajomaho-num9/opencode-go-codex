package responses

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

type Usage map[string]any

func NewID(prefix string) string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return prefix + "_" + hex.EncodeToString([]byte(time.Now().Format("20060102150405.000000000")))
	}
	return prefix + "_" + hex.EncodeToString(buf)
}

func NormalizeUsage(usage map[string]any) map[string]any {
	if usage == nil {
		return map[string]any{"input_tokens": 0, "output_tokens": 0, "total_tokens": 0}
	}
	input := numberValue(usage["input_tokens"], numberValue(usage["prompt_tokens"], 0))
	output := numberValue(usage["output_tokens"], numberValue(usage["completion_tokens"], 0))
	total := numberValue(usage["total_tokens"], input+output)
	return map[string]any{"input_tokens": input, "output_tokens": output, "total_tokens": total}
}

func MakeResponse(model string, output []any, usage map[string]any, status string) map[string]any {
	if status == "" {
		status = "completed"
	}
	return map[string]any{
		"id":         NewID("resp"),
		"object":     "response",
		"created_at": time.Now().Unix(),
		"status":     status,
		"model":      model,
		"output":     output,
		"usage":      NormalizeUsage(usage),
	}
}

func numberValue(value any, fallback float64) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case jsonNumber:
		f, err := v.Float64()
		if err == nil {
			return f
		}
	}
	return fallback
}

type jsonNumber interface {
	Float64() (float64, error)
}
