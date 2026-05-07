package doctor

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

func Run() int {
	exitCode := 0
	ok("go runtime", runtime.Version()+" "+runtime.GOOS+"/"+runtime.GOARCH)
	codexHome := os.Getenv("CODEX_HOME")
	if codexHome == "" {
		codexHome = filepath.Join(homeDir(), ".codex")
	}
	configPath := filepath.Join(codexHome, "config.toml")
	configText := ""
	if text, err := os.ReadFile(configPath); err == nil {
		ok("codex config", configPath)
		configText = string(text)
		for _, needle := range []string{"model_providers.OpenCodeGo", "wire_api = \"responses\"", "deepseek-v4-pro", "deepseek-v4-flash"} {
			if strings.Contains(configText, needle) {
				ok("config contains", needle)
			} else {
				warn("config missing", needle)
			}
		}
	} else {
		warn("codex config missing", configPath)
	}
	models := resolveModelCatalog(configText)
	if b, err := os.ReadFile(models); err == nil {
		var payload map[string]any
		if err := json.Unmarshal(b, &payload); err == nil {
			ok("model catalog", models)
		} else {
			fail("model catalog invalid", err.Error())
			exitCode = 1
		}
	} else {
		fail("model catalog missing", models)
		exitCode = 1
	}
	if os.Getenv("OPENCODE_GO_API_KEY") != "" || os.Getenv("OPENAI_API_KEY") != "" {
		ok("API key env", "present")
	} else {
		warn("API key env", "set OPENCODE_GO_API_KEY before starting")
	}
	host := getenv("OPENCODE_GO_CODEX_HOST", "127.0.0.1")
	port := getenv("OPENCODE_GO_CODEX_PORT", "8768")
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), time.Second)
	if err == nil {
		_ = conn.Close()
		ok("adapter port", host+":"+port+" is reachable")
	} else {
		warn("adapter port", host+":"+port+" is not listening")
	}
	return exitCode
}

func ok(name, detail string)   { fmt.Printf("[OK]   %s: %s\n", name, detail) }
func warn(name, detail string) { fmt.Printf("[WARN] %s: %s\n", name, detail) }
func fail(name, detail string) { fmt.Printf("[FAIL] %s: %s\n", name, detail) }
func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
func homeDir() string {
	if h, err := os.UserHomeDir(); err == nil {
		return h
	}
	return "."
}

func resolveModelCatalog(configText string) string {
	for _, candidate := range modelCatalogCandidates(configText) {
		if candidate == "" {
			continue
		}
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	candidates := modelCatalogCandidates(configText)
	if len(candidates) > 0 {
		return candidates[0]
	}
	return defaultModelCatalog()
}

func modelCatalogCandidates(configText string) []string {
	candidates := []string{}
	if configured := parseModelCatalogJSON(configText); configured != "" {
		candidates = append(candidates, configured)
	}
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), defaultModelCatalog()))
	}
	candidates = append(candidates, defaultModelCatalog())
	return candidates
}

func parseModelCatalogJSON(configText string) string {
	for _, line := range strings.Split(configText, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "[") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok || strings.TrimSpace(key) != "model_catalog_json" {
			continue
		}
		value = strings.TrimSpace(strings.SplitN(value, "#", 2)[0])
		value = strings.Trim(value, `"'`)
		if value != "" {
			return value
		}
	}
	return ""
}

func defaultModelCatalog() string {
	return filepath.Join("examples", "models", "deepseek-only.json")
}
