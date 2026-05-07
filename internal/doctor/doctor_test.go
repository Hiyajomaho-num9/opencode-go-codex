package doctor

import (
	"path/filepath"
	"testing"
)

func TestParseModelCatalogJSON(t *testing.T) {
	config := `
model = "deepseek-v4-pro"
model_catalog_json = "/tmp/opencode-go-codex/examples/models/deepseek-only.json" # comment

[profiles.deepseek-v4-pro]
model = "deepseek-v4-pro"
`
	got := parseModelCatalogJSON(config)
	want := filepath.Join("/tmp", "opencode-go-codex", "examples", "models", "deepseek-only.json")
	if got != want {
		t.Fatalf("model catalog = %q, want %q", got, want)
	}
}
