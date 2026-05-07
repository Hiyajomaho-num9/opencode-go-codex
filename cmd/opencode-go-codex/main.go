package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/Hiyajomaho-num9/opencode-go-codex/internal/config"
	"github.com/Hiyajomaho-num9/opencode-go-codex/internal/doctor"
	"github.com/Hiyajomaho-num9/opencode-go-codex/internal/installconfig"
	"github.com/Hiyajomaho-num9/opencode-go-codex/internal/replay"
	"github.com/Hiyajomaho-num9/opencode-go-codex/internal/server"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "doctor" {
		os.Exit(doctor.Run())
	}
	if len(os.Args) > 1 && os.Args[1] == "replay" {
		os.Exit(replay.Run(os.Args[2:]))
	}
	if len(os.Args) > 1 && os.Args[1] == "install-config" {
		os.Exit(installconfig.Run(os.Args[2:]))
	}
	fs := flag.NewFlagSet("opencode-go-codex", flag.ExitOnError)
	host := fs.String("host", getenv("OPENCODE_GO_CODEX_HOST", "127.0.0.1"), "listen host")
	port := fs.String("port", getenv("OPENCODE_GO_CODEX_PORT", "8768"), "listen port")
	upstream := fs.String("upstream", getenv("OPENCODE_GO_BASE_URL", config.DefaultUpstream), "upstream chat completions URL")
	defaultModel := fs.String("default-model", getenv("OPENCODE_GO_MODEL", config.DefaultModel), "default model")
	compactModel := fs.String("compact-model", getenv("OPENCODE_GO_COMPACT_MODEL", config.DefaultCompactModel), "compact model")
	visionModel := fs.String("vision-model", getenv("OPENCODE_GO_VISION_MODEL", config.DefaultVisionModel), "vision model")
	reasoningEffort := fs.String("reasoning-effort", getenv("OPENCODE_GO_REASONING_EFFORT", config.DefaultReasoningEffort), "default reasoning effort")
	thinking := fs.String("thinking", getenv("OPENCODE_GO_THINKING", config.DefaultThinkingType), "default thinking type")
	timeout := fs.Int("timeout", getenvInt("OPENCODE_GO_TIMEOUT", 900), "upstream timeout seconds")
	traceDir := fs.String("trace-dir", getenv("OPENCODE_GO_TRACE_DIR", ""), "trace directory")
	debugRouting := fs.Bool("debug-routing", getenvBool("OPENCODE_GO_DEBUG_ROUTING", false), "debug routing")
	verbose := fs.Bool("verbose", false, "verbose logs")
	_ = fs.Parse(os.Args[1:])

	cfg := config.Config{
		Host: *host, Port: *port, Upstream: *upstream,
		DefaultModel: *defaultModel, CompactModel: *compactModel, VisionModel: *visionModel,
		DefaultReasoningEffort: config.NormalizeReasoningEffortDefault(*reasoningEffort, config.DefaultReasoningEffort),
		DefaultThinkingType:    config.NormalizeThinkingType(*thinking),
		TimeoutSeconds:         *timeout, TraceDir: *traceDir,
		DebugRouting: *debugRouting, Verbose: *verbose,
	}
	srv := server.New(cfg)
	fmt.Printf("opencode-go-codex listening on http://%s:%s\n", cfg.Host, cfg.Port)
	fmt.Printf("forwarding to %s\n", srv.Config.Upstream)
	fmt.Printf("default model %s\n", cfg.DefaultModel)
	fmt.Printf("compact model %s\n", cfg.CompactModel)
	fmt.Printf("vision model %s\n", cfg.VisionModel)
	fmt.Printf("default thinking %s\n", cfg.DefaultThinkingType)
	fmt.Printf("default reasoning effort %s\n", cfg.DefaultReasoningEffort)
	fmt.Printf("debug routing %v\n", cfg.DebugRouting)
	if cfg.TraceDir != "" {
		fmt.Printf("trace dir %s\n", cfg.TraceDir)
	}
	log.Fatal(srv.ListenAndServe())
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
func getenvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
func getenvBool(key string, fallback bool) bool {
	if v := os.Getenv(key); v != "" {
		return v == "1" || v == "true" || v == "yes"
	}
	return fallback
}
