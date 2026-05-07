package installconfig

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

func Run(args []string) int {
	if len(args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: opencode-go-codex install-config CONFIG MODEL_CATALOG MCP_SCRIPT")
		return 2
	}
	if err := Patch(args[0], args[1], args[2]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func Patch(configPath, modelCatalog, mcpScript string) error {
	b, err := os.ReadFile(configPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	text := string(b)
	text = setRoot(text, "model_catalog_json", quote(modelCatalog))
	text = setRoot(text, "model_provider", quote("OpenCodeGo"))
	text = setRoot(text, "model", quote("deepseek-v4-pro"))
	text = setRoot(text, "review_model", quote("deepseek-v4-pro"))
	text = setRoot(text, "model_reasoning_effort", quote("xhigh"))
	text = setRoot(text, "model_verbosity", quote("low"))
	text = setRoot(text, "model_context_window", "512000")
	text = setRoot(text, "model_auto_compact_token_limit", "400000")
	text = appendTable(text, "model_providers.OpenCodeGo", `name = "OpenCodeGo"
base_url = "http://127.0.0.1:8768/v1"
wire_api = "responses"`)
	text = appendTable(text, "profiles.deepseek-v4-pro", `model_provider = "OpenCodeGo"
model = "deepseek-v4-pro"
model_reasoning_effort = "xhigh"
model_verbosity = "low"
model_context_window = 512000
model_auto_compact_token_limit = 400000`)
	text = appendTable(text, "profiles.deepseek-v4-flash", `model_provider = "OpenCodeGo"
model = "deepseek-v4-flash"
model_reasoning_effort = "xhigh"
model_verbosity = "low"
model_context_window = 512000
model_auto_compact_token_limit = 400000`)
	text = appendTable(text, "mcp_servers.web-search", `command = "`+escape(mcpScript)+`"`)
	return os.WriteFile(configPath, []byte(strings.TrimSpace(text)+"\n"), 0o600)
}

func setRoot(text, key, value string) string {
	line := key + " = " + value
	re := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(key) + `\s*=.*$`)
	if re.MatchString(text) {
		return re.ReplaceAllString(text, line)
	}
	firstTable := regexp.MustCompile(`(?m)^\[`).FindStringIndex(text)
	if firstTable != nil {
		return text[:firstTable[0]] + line + "\n" + text[firstTable[0]:]
	}
	return strings.TrimRight(text, "\n") + "\n" + line + "\n"
}

func appendTable(text, name, body string) string {
	text = removeTable(text, name)
	return strings.TrimRight(text, "\n") + "\n\n[" + name + "]\n" + strings.TrimSpace(body) + "\n"
}

func removeTable(text, name string) string {
	lines := strings.SplitAfter(text, "\n")
	out := strings.Builder{}
	inTarget := false
	targetHeader := "[" + name + "]"
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") {
			inTarget = trimmed == targetHeader
		}
		if !inTarget {
			out.WriteString(line)
		}
	}
	return out.String()
}

func quote(value string) string { return `"` + escape(value) + `"` }
func escape(value string) string {
	return strings.ReplaceAll(strings.ReplaceAll(value, `\`, `/`), `"`, `\"`)
}
