package trace

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Hiyajomaho-num9/opencode-go-codex/internal/responses"
)

type Trace struct {
	Dir string
	ID  string
	mu  sync.Mutex
}

func New(root string) (*Trace, error) {
	if root == "" {
		return nil, nil
	}
	id := time.Now().Format("20060102-150405-") + strings.TrimPrefix(responses.NewID(""), "_")[:8]
	dir := filepath.Join(root, id)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	return &Trace{Dir: dir, ID: id}, nil
}

func (t *Trace) WriteJSON(name string, value any, redact bool) {
	if t == nil {
		return
	}
	if redact {
		value = Redact(value)
	}
	b, err := json.Marshal(value)
	if err != nil {
		b = []byte(`null`)
	}
	_ = os.WriteFile(filepath.Join(t.Dir, name), append(b, '\n'), 0o600)
}

func (t *Trace) WriteText(name string, data []byte) {
	if t == nil {
		return
	}
	_ = os.WriteFile(filepath.Join(t.Dir, name), data, 0o600)
}

func (t *Trace) AppendText(name string, data []byte) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	f, err := os.OpenFile(filepath.Join(t.Dir, name), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.Write(data)
}

func (t *Trace) AppendEvent(event string, data any) {
	if t == nil {
		return
	}
	b, err := json.Marshal(map[string]any{"event": event, "data": data})
	if err != nil {
		return
	}
	t.AppendText("response_events.jsonl", append(b, '\n'))
}

func Redact(value any) any {
	switch v := value.(type) {
	case map[string]any:
		out := map[string]any{}
		for key, child := range v {
			lower := strings.ToLower(key)
			if strings.Contains(lower, "token") || strings.Contains(lower, "key") || strings.Contains(lower, "secret") || strings.Contains(lower, "password") || strings.Contains(lower, "authorization") {
				out[key] = "<redacted>"
			} else {
				out[key] = Redact(child)
			}
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, child := range v {
			out[i] = Redact(child)
		}
		return out
	default:
		return v
	}
}

func Meta(t *Trace, data map[string]any) {
	if t == nil {
		return
	}
	data["trace_id"] = t.ID
	t.WriteJSON("meta.json", data, true)
}

func Error(t *Trace, err error) {
	if t != nil && err != nil {
		t.WriteText("error.txt", []byte(fmt.Sprintf("%v\n", err)))
	}
}
