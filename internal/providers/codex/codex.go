package codex

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/CyrusSE/agenthop/internal/config"
	"github.com/CyrusSE/agenthop/internal/model"
	"github.com/CyrusSE/agenthop/internal/provider"
	"github.com/CyrusSE/agenthop/internal/util"
	"github.com/google/uuid"
)

const ProviderID = "codex"

type Provider struct {
	sessionsRoot string
}

func New() *Provider {
	return &Provider{sessionsRoot: resolveSessionsRoot()}
}

func resolveSessionsRoot() string {
	if home := config.EnvOrDefault("CODEX_HOME", ""); home != "" {
		return filepath.Join(home, "sessions")
	}
	snap := filepath.Join(config.HomeDir(), "snap", "codex", "current", "sessions")
	if st, err := os.Stat(snap); err == nil && st.IsDir() {
		return snap
	}
	return filepath.Join(config.HomeDir(), ".codex", "sessions")
}

func (p *Provider) ID() string          { return ProviderID }
func (p *Provider) DisplayName() string { return "Codex" }
func (p *Provider) Installed() bool {
	st, err := os.Stat(p.sessionsRoot)
	return err == nil && st.IsDir()
}
func (p *Provider) SupportsResume() bool { return true }

func (p *Provider) DefaultPaths() []provider.PathSpec {
	return []provider.PathSpec{{Label: "sessions", Path: p.sessionsRoot, Env: "CODEX_HOME"}}
}

func (p *Provider) Discover(ctx context.Context, opts provider.DiscoverOpts) ([]model.Summary, error) {
	var out []model.Summary
	_ = filepath.WalkDir(p.sessionsRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasPrefix(filepath.Base(path), "rollout-") || !strings.HasSuffix(path, ".jsonl") {
			return nil
		}
		sm, err := p.summarizeFile(path)
		if err != nil || sm.ID == "" {
			return nil
		}
		if opts.ProjectFilter != "" && !strings.Contains(sm.ProjectPath, opts.ProjectFilter) {
			return nil
		}
		out = append(out, sm)
		if opts.Limit > 0 && len(out) >= opts.Limit {
			return filepath.SkipAll
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		return nil
	})
	return out, nil
}

func sessionIDFromRollout(path string) string {
	base := filepath.Base(path)
	base = strings.TrimSuffix(base, ".jsonl")
	parts := strings.Split(base, "-")
	if len(parts) >= 2 {
		return parts[len(parts)-1]
	}
	return base
}

func (p *Provider) summarizeFile(path string) (model.Summary, error) {
	st, err := os.Stat(path)
	if err != nil {
		return model.Summary{}, err
	}
	id := sessionIDFromRollout(path)
	var title string
	var msgCount int
	var first, last time.Time
	var project string
	_ = util.ReadJSONLLines(path, 0, func(line []byte) error {
		var row map[string]any
		if json.Unmarshal(line, &row) != nil {
			return nil
		}
		if t, _ := row["type"].(string); t == "session_meta" {
			if sid, _ := row["session_id"].(string); sid != "" {
				id = sid
			}
			if cwd, _ := row["cwd"].(string); cwd != "" {
				project = cwd
			}
			return nil
		}
		if em, ok := row["event_msg"].(map[string]any); ok {
			role, _ := em["role"].(string)
			if role != "user" && role != "assistant" {
				return nil
			}
			msgCount++
			if text, _ := em["message"].(string); text != "" && title == "" && role == "user" {
				title = util.FirstUserSnippet(text, 80)
			}
		}
		return nil
	})
	tail, _ := util.TailJSONLLines(path, 5)
	for _, line := range tail {
		var row map[string]any
		if json.Unmarshal(line, &row) != nil {
			continue
		}
		if ts, _ := row["timestamp"].(string); ts != "" {
			last = util.ParseTime(ts)
		}
	}
	if first.IsZero() {
		first = st.ModTime()
	}
	if last.IsZero() {
		last = st.ModTime()
	}
	if title == "" {
		title = "(no title)"
	}
	return model.Summary{
		ID: id, Provider: ProviderID, ProjectPath: project, Title: title,
		CreatedAt: first, UpdatedAt: last, MessageCount: msgCount,
		StoragePath: path, SourceMtime: st.ModTime().Unix(),
	}, nil
}

func (p *Provider) Load(ctx context.Context, ref provider.SessionRef) (*model.Conversation, error) {
	path := ref.StoragePath
	if path == "" {
		return nil, provider.ErrNotFound
	}
	st, err := os.Stat(path)
	if err != nil {
		return nil, provider.ErrNotFound
	}
	conv := &model.Conversation{ID: ref.ID, Provider: ProviderID, StoragePath: path}
	seen := map[string]bool{}
	_ = util.ReadJSONLLines(path, 0, func(line []byte) error {
		var row map[string]any
		if json.Unmarshal(line, &row) != nil {
			return nil
		}
		if t, _ := row["type"].(string); t == "session_meta" {
			if sid, _ := row["session_id"].(string); sid != "" {
				conv.ID = sid
			}
			if cwd, _ := row["cwd"].(string); cwd != "" {
				conv.ProjectPath = cwd
			}
			return nil
		}
		ts := util.ParseTime(stringField(row, "timestamp"))
		if em, ok := row["event_msg"].(map[string]any); ok {
			role, _ := em["role"].(string)
			text, _ := em["message"].(string)
			if role == "" || text == "" {
				return nil
			}
			key := role + "|" + text
			if seen[key] {
				return nil
			}
			seen[key] = true
			mrole := model.RoleUser
			if role == "assistant" {
				mrole = model.RoleAssistant
			}
			conv.Messages = append(conv.Messages, model.Message{Role: mrole, Content: text, Timestamp: ts})
		}
		return nil
	})
	if len(conv.Messages) == 0 {
		return nil, provider.ErrNotFound
	}
	conv.MessageCount = len(conv.Messages)
	conv.CreatedAt = conv.Messages[0].Timestamp
	conv.UpdatedAt = conv.Messages[len(conv.Messages)-1].Timestamp
	for _, m := range conv.Messages {
		if m.Role == model.RoleUser && m.PlainText() != "" {
			conv.Title = util.FirstUserSnippet(m.PlainText(), 80)
			break
		}
	}
	_ = st
	return conv, nil
}

func stringField(m map[string]any, k string) string {
	if v, ok := m[k].(string); ok {
		return v
	}
	return ""
}

func (p *Provider) Write(ctx context.Context, conv *model.Conversation, opts provider.WriteOpts) (*provider.WriteResult, error) {
	if len(conv.Messages) == 0 {
		return nil, provider.ErrEmptySession
	}
	now := time.Now().UTC()
	sessionID := uuid.New().String()
	parts := now.Format("2006-01-02T15-04-05")
	dir := filepath.Join(p.sessionsRoot, now.Format("2006"), now.Format("01"), now.Format("02"))
	path := filepath.Join(dir, "rollout-"+parts+"-"+sessionID+".jsonl")
	project := opts.ProjectPath
	if project == "" {
		project = conv.ProjectPath
	}
	if opts.DryRun {
		return &provider.WriteResult{SessionID: sessionID, StoragePath: path, ProjectPath: project}, nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	meta := model.NewMigrationMeta(conv)
	var lines []string
	sessionMeta := map[string]any{
		"type": "session_meta", "session_id": sessionID, "cwd": project,
		"timestamp": now.Format(time.RFC3339Nano),
	}
	if b, err := json.Marshal(sessionMeta); err == nil {
		lines = append(lines, string(b))
	}
	for _, m := range conv.Messages {
		role := string(m.Role)
		row := map[string]any{
			"type": "event_msg", "timestamp": m.Timestamp.UTC().Format(time.RFC3339Nano),
			"event_msg": map[string]any{"role": role, "message": m.PlainText()},
		}
		if b, err := json.Marshal(row); err == nil {
			lines = append(lines, string(b))
		}
	}
	metaLine := map[string]any{"type": model.MigrationType, "data": meta}
	if b, err := json.Marshal(metaLine); err == nil {
		lines = append(lines, string(b))
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		return nil, err
	}
	return &provider.WriteResult{SessionID: sessionID, StoragePath: path, ProjectPath: project}, nil
}

func (p *Provider) ResumeCommand(r provider.WriteResult) string {
	return "codex resume " + r.SessionID
}
