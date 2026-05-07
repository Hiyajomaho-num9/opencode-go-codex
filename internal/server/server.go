package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/Hiyajomaho-num9/opencode-go-codex/internal/config"
	"github.com/Hiyajomaho-num9/opencode-go-codex/internal/reasoning"
	"github.com/Hiyajomaho-num9/opencode-go-codex/internal/responses"
	"github.com/Hiyajomaho-num9/opencode-go-codex/internal/sse"
	"github.com/Hiyajomaho-num9/opencode-go-codex/internal/trace"
	"github.com/Hiyajomaho-num9/opencode-go-codex/internal/transform"
)

type Server struct {
	Config         config.Config
	Client         *http.Client
	ReasoningStore *reasoning.Store
}

func New(cfg config.Config) *Server {
	cfg.Upstream = config.NormalizeUpstream(cfg.Upstream)
	return &Server{
		Config:         cfg,
		Client:         &http.Client{Timeout: time.Duration(cfg.TimeoutSeconds) * time.Second},
		ReasoningStore: reasoning.NewStore(),
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("ok\n")) })
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("ready\n")) })
	mux.HandleFunc("/responses", s.handleResponses)
	mux.HandleFunc("/v1/responses", s.handleResponses)
	mux.HandleFunc("/responses/compact", s.handleResponses)
	mux.HandleFunc("/v1/responses/compact", s.handleResponses)
	return mux
}

func (s *Server) ListenAndServe() error {
	addr := s.Config.Host + ":" + s.Config.Port
	return http.ListenAndServe(addr, s.Handler())
}

func (s *Server) handleResponses(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		sendJSONError(w, http.StatusNotFound, "not found")
		return
	}
	tr, err := trace.New(s.Config.TraceDir)
	if err != nil {
		log.Printf("trace init failed: %v", err)
	}
	path := r.URL.Path
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		sendJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	if tr != nil {
		tr.WriteText("incoming_responses_request.raw.json", raw)
	}
	var req map[string]any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&req); err != nil {
		trace.Error(tr, err)
		sendJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	if tr != nil {
		tr.WriteJSON("incoming_responses_request.json", req, true)
	}
	selectedModel := transform.SelectModel(req, path, s.Config.DefaultModel, s.Config.CompactModel, s.Config.VisionModel)
	req["model"] = selectedModel
	chatReq := transform.ResponsesToChatRequest(req, selectedModel, transform.Defaults{DefaultReasoningEffort: s.Config.DefaultReasoningEffort, DefaultThinkingType: s.Config.DefaultThinkingType, ReasoningStore: s.ReasoningStore})
	if hasCompactSuffix(path) {
		delete(chatReq, "tools")
		delete(chatReq, "tool_choice")
	}
	if tr != nil {
		tr.WriteJSON("upstream_chat_request.json", chatReq, true)
		trace.Meta(tr, map[string]any{"path": path, "upstream_url": s.Config.Upstream, "selected_model": selectedModel, "stream": chatReq["stream"]})
	}
	if s.Config.Verbose || s.Config.DebugRouting {
		thinking := "none"
		if m, ok := chatReq["thinking"].(map[string]any); ok {
			thinking = fmt.Sprint(m["type"])
		}
		log.Printf("route path=%s model=%s thinking=%s reasoning_effort=%v", path, chatReq["model"], thinking, chatReq["reasoning_effort"])
	}
	s.forward(w, r, chatReq, tr)
}

func (s *Server) forward(w http.ResponseWriter, r *http.Request, chatReq map[string]any, tr *trace.Trace) {
	body, _ := json.Marshal(chatReq)
	upReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost, s.Config.Upstream, bytes.NewReader(body))
	if err != nil {
		sendJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	auth := ""
	if key := os.Getenv("OPENCODE_GO_API_KEY"); key != "" {
		auth = "Bearer " + key
	} else if header := r.Header.Get("Authorization"); header != "" {
		auth = header
	} else if key := os.Getenv("OPENAI_API_KEY"); key != "" {
		auth = "Bearer " + key
	}
	if auth == "" {
		sendJSONError(w, http.StatusUnauthorized, "missing Authorization header or OPENCODE_GO_API_KEY")
		return
	}
	upReq.Header.Set("Accept", "application/json, text/event-stream")
	upReq.Header.Set("Authorization", auth)
	upReq.Header.Set("Content-Type", "application/json")
	upReq.Header.Set("User-Agent", "opencode-go-codex-go")
	resp, err := s.Client.Do(upReq)
	if err != nil {
		trace.Error(tr, err)
		sendJSONError(w, http.StatusBadGateway, err.Error())
		return
	}
	defer resp.Body.Close()
	if tr != nil {
		tr.WriteText("upstream_status.txt", []byte(fmt.Sprintf("HTTP %d\n", resp.StatusCode)))
	}
	if resp.StatusCode >= 400 {
		detail, _ := io.ReadAll(resp.Body)
		if tr != nil {
			tr.WriteText("upstream_response.raw", detail)
		}
		sendJSONError(w, resp.StatusCode, string(detail))
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "close")
	w.WriteHeader(http.StatusOK)
	writer := sse.Writer{W: w, Trace: tr}
	model, _ := chatReq["model"].(string)
	if stream, _ := chatReq["stream"].(bool); stream {
		if err := sse.StreamChatToResponses(writer, resp.Body, model, s.ReasoningStore); err != nil {
			trace.Error(tr, err)
		}
		return
	}
	raw, _ := io.ReadAll(resp.Body)
	if tr != nil {
		tr.WriteText("upstream_response.raw", raw)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		trace.Error(tr, err)
		_ = sse.EmitTextStream(writer, model, string(raw), nil)
		return
	}
	message := firstChoiceMessage(payload)
	reasoningContent, _ := message["reasoning_content"].(string)
	text, _ := message["content"].(string)
	toolCalls := convertToolCalls(message["tool_calls"])
	callIDs := []string{}
	for _, call := range toolCalls {
		if id, ok := call["call_id"].(string); ok {
			callIDs = append(callIDs, id)
		}
	}
	s.ReasoningStore.Remember(callIDs, reasoningContent)
	usage, _ := payload["usage"].(map[string]any)
	if len(toolCalls) > 0 {
		_ = sse.EmitFinalStream(writer, model, text, toolCalls, reasoningContent, usage)
	} else {
		_ = sse.EmitTextStream(writer, model, text, usage)
	}
}

func sendJSONError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"message": message}})
}

func hasCompactSuffix(path string) bool {
	return len(path) >= len("/compact") && path[len(path)-len("/compact"):] == "/compact"
}

func firstChoiceMessage(payload map[string]any) map[string]any {
	choices, _ := payload["choices"].([]any)
	if len(choices) == 0 {
		return map[string]any{}
	}
	choice, _ := choices[0].(map[string]any)
	message, _ := choice["message"].(map[string]any)
	if message == nil {
		return map[string]any{}
	}
	return message
}

func convertToolCalls(raw any) []map[string]any {
	items, _ := raw.([]any)
	out := []map[string]any{}
	for _, item := range items {
		call, _ := item.(map[string]any)
		fn, _ := call["function"].(map[string]any)
		id, _ := call["id"].(string)
		if id == "" {
			id = responses.NewID("call")
		}
		out = append(out, map[string]any{"item_id": responses.NewID("fc"), "call_id": id, "name": fn["name"], "arguments": fn["arguments"]})
	}
	return out
}
