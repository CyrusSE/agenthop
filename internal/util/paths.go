package util

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func EncodeClaudeProjectPath(absPath string) string {
	abs, err := filepath.Abs(absPath)
	if err != nil {
		abs = absPath
	}
	return strings.ReplaceAll(abs, string(filepath.Separator), "-")
}

func DecodeClaudeProjectPath(encoded string) string {
	if encoded == "" {
		return ""
	}
	if strings.HasPrefix(encoded, "-") {
		return "/" + strings.ReplaceAll(encoded[1:], "-", "/")
	}
	return strings.ReplaceAll(encoded, "-", string(filepath.Separator))
}

func FileMtime(path string) (time.Time, error) {
	st, err := os.Stat(path)
	if err != nil {
		return time.Time{}, err
	}
	return st.ModTime(), nil
}

func ReadJSONLLines(path string, maxLines int, fn func(line []byte) error) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	n := 0
	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		if err := fn(line); err != nil {
			return err
		}
		n++
		if maxLines > 0 && n >= maxLines {
			break
		}
	}
	return sc.Err()
}

func TailJSONLLines(path string, maxLines int) ([][]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}
	lines := bytes.Split(data, []byte("\n"))
	var out [][]byte
	for i := len(lines) - 1; i >= 0 && len(out) < maxLines; i-- {
		line := bytes.TrimSpace(lines[i])
		if len(line) == 0 {
			continue
		}
		out = append([][]byte{line}, out...)
	}
	return out, nil
}

func ParseTime(s string) time.Time {
	formats := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05.000Z",
		"2006-01-02T15:04:05Z",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

func JSONUnmarshal(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

func JSONMarshal(v any) ([]byte, error) {
	return json.Marshal(v)
}

func FirstUserSnippet(text string, max int) string {
	text = strings.TrimSpace(text)
	text = strings.ReplaceAll(text, "\n", " ")
	if len(text) <= max {
		return text
	}
	return text[:max] + "…"
}

func MatchID(id, query string) bool {
	query = strings.ToLower(strings.TrimSpace(query))
	id = strings.ToLower(id)
	if query == "" {
		return true
	}
	if id == query {
		return true
	}
	if strings.HasSuffix(id, query) {
		return true
	}
	if strings.HasPrefix(id, query) {
		return true
	}
	return false
}
