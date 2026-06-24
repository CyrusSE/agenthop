package debuglog

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const logPath = "/home/cyrus/Documents/test/miggrate/.cursor/debug-da41ea.log"

type Entry struct {
	SessionID    string         `json:"sessionId,omitempty"`
	RunID        string         `json:"runId,omitempty"`
	HypothesisID string         `json:"hypothesisId,omitempty"`
	Location     string         `json:"location"`
	Message      string         `json:"message"`
	Data         map[string]any `json:"data,omitempty"`
	Timestamp    int64          `json:"timestamp"`
}

func sanitize(s string) string {
	if s == "" {
		return s
	}
	home := os.Getenv("HOME")
	if home != "" {
		s = strings.ReplaceAll(s, home, "~")
	}
	return s
}

func SanitizeMap(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		switch t := v.(type) {
		case string:
			out[k] = sanitize(t)
		default:
			out[k] = v
		}
	}
	return out
}

// #region agent log
func Log(hypothesisID, location, message, runID string, data map[string]any) {
	if os.Getenv("AGENTHOP_DEBUG") == "" {
		return
	}
	e := Entry{
		SessionID:    "da41ea",
		RunID:        runID,
		HypothesisID: hypothesisID,
		Location:     location,
		Message:      message,
		Data:         SanitizeMap(data),
		Timestamp:    time.Now().UnixMilli(),
	}
	b, err := json.Marshal(e)
	if err != nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(logPath), 0o755)
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	_, _ = f.Write(append(b, '\n'))
	_ = f.Close()
}

// #endregion
