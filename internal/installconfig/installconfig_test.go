package installconfig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPatchConfigIsRepeatableAndReplacesTables(t *testing.T) {
	dir := t.TempDir()
	config := filepath.Join(dir, "config.toml")
	models := filepath.Join(dir, "examples", "models", "deepseek-only.json")
	mcp := filepath.Join(dir, "tools", "web_search_mcp.py")
	initial := `model = "old"

[model_providers.OpenCodeGo]
name = "old"
base_url = "http://old"

[profiles.deepseek-v4-pro]
model = "old"

[other]
value = "kept"
`
	if err := os.WriteFile(config, []byte(initial), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Patch(config, models, mcp); err != nil {
		t.Fatal(err)
	}
	if err := Patch(config, models, mcp); err != nil {
		t.Fatal(err)
	}

	b, err := os.ReadFile(config)
	if err != nil {
		t.Fatal(err)
	}
	text := string(b)
	for _, table := range []string{"[model_providers.OpenCodeGo]", "[profiles.deepseek-v4-pro]", "[profiles.deepseek-v4-flash]", "[mcp_servers.web-search]"} {
		if count := strings.Count(text, table); count != 1 {
			t.Fatalf("%s count = %d\n%s", table, count, text)
		}
	}
	if !strings.Contains(text, `[other]`) || !strings.Contains(text, `value = "kept"`) {
		t.Fatalf("unrelated table was not preserved:\n%s", text)
	}
	if strings.Contains(text, `base_url = "http://old"`) {
		t.Fatalf("old table content was not removed:\n%s", text)
	}
	if !strings.Contains(text, `model_context_window = 512000`) || !strings.Contains(text, `model_auto_compact_token_limit = 400000`) {
		t.Fatalf("numeric root config missing:\n%s", text)
	}
}
