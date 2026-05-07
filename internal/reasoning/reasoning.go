package reasoning

import (
	"encoding/base64"
	"encoding/json"
	"sync"

	"github.com/Hiyajomaho-num9/opencode-go-codex/internal/responses"
)

const encryptedPrefix = "opencode-go-codex/deepseek-reasoning/v1"
const cacheLimit = 4096

type Store struct {
	mu    sync.Mutex
	data  map[string]string
	order []string
}

func NewStore() *Store {
	return &Store{data: map[string]string{}}
}

func (s *Store) Remember(callIDs []string, content string) {
	if s == nil || content == "" || len(callIDs) == 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, id := range callIDs {
		if id == "" {
			continue
		}
		if _, ok := s.data[id]; !ok {
			s.order = append(s.order, id)
		}
		s.data[id] = content
	}
	for len(s.order) > cacheLimit {
		old := s.order[0]
		s.order = s.order[1:]
		delete(s.data, old)
	}
}

func (s *Store) Lookup(callID string) string {
	if s == nil || callID == "" {
		return ""
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.data[callID]
}

func Encode(content string) string {
	payload := map[string]string{"format": encryptedPrefix, "reasoning_content": content}
	b, _ := json.Marshal(payload)
	return base64.StdEncoding.EncodeToString(b)
}

func Decode(encrypted string) string {
	if encrypted == "" {
		return ""
	}
	raw, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		return ""
	}
	var payload map[string]string
	if err := json.Unmarshal(raw, &payload); err != nil {
		return ""
	}
	if payload["format"] != encryptedPrefix {
		return ""
	}
	return payload["reasoning_content"]
}

func Extract(item map[string]any) string {
	if item == nil {
		return ""
	}
	if encrypted, ok := item["encrypted_content"].(string); ok {
		if decoded := Decode(encrypted); decoded != "" {
			return decoded
		}
	}
	if explicit, ok := item["reasoning_content"].(string); ok {
		return explicit
	}
	content := item["content"]
	switch v := content.(type) {
	case string:
		return v
	case []any:
		out := ""
		for _, part := range v {
			s := ""
			switch p := part.(type) {
			case string:
				s = p
			case map[string]any:
				if text, ok := p["text"].(string); ok {
					s = text
				} else if text, ok := p["reasoning_text"].(string); ok {
					s = text
				}
			}
			if s != "" {
				if out != "" {
					out += "\n"
				}
				out += s
			}
		}
		return out
	}
	return ""
}

func MakeItem(content string) map[string]any {
	return map[string]any{
		"id":                responses.NewID("rs"),
		"type":              "reasoning",
		"summary":           []any{},
		"content":           nil,
		"encrypted_content": Encode(content),
	}
}
