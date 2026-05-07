package config

import "strings"

const (
	DefaultUpstream        = "https://opencode.ai/zen/go/v1/chat/completions"
	DefaultModel           = "deepseek-v4-pro"
	DefaultCompactModel    = "deepseek-v4-flash"
	DefaultVisionModel     = "kimi-k2.6"
	DefaultReasoningEffort = "max"
	DefaultThinkingType    = "enabled"
)

type Config struct {
	Host                   string
	Port                   string
	Upstream               string
	DefaultModel           string
	CompactModel           string
	VisionModel            string
	DefaultReasoningEffort string
	DefaultThinkingType    string
	TimeoutSeconds         int
	Verbose                bool
	DebugRouting           bool
	TraceDir               string
}

func NormalizeUpstream(url string) string {
	url = strings.TrimRight(url, "/")
	if strings.HasSuffix(url, "/chat/completions") {
		return url
	}
	return url + "/chat/completions"
}

func NormalizeReasoningEffort(value string) string {
	switch strings.ToLower(value) {
	case "max", "xhigh":
		return "max"
	case "", "low", "medium", "high":
		return "high"
	default:
		return DefaultReasoningEffort
	}
}

func NormalizeReasoningEffortDefault(value, fallback string) string {
	if value == "" {
		value = fallback
	}
	return NormalizeReasoningEffort(value)
}

func NormalizeThinkingType(value string) string {
	switch strings.ToLower(value) {
	case "enabled", "enable", "true", "1":
		return "enabled"
	case "disabled", "disable", "false", "0":
		return "disabled"
	case "":
		return DefaultThinkingType
	default:
		return DefaultThinkingType
	}
}

func IsDeepSeekV4Model(model string) bool {
	return strings.HasPrefix(model, "deepseek-v4")
}
