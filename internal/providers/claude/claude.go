package claude

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

const ProviderID = "claude-code"

type Provider struct {
	root string
}

func New() *Provider {
	root := config.EnvOrDefault("CLAUDE_CONFIG_DIR", filepath.Join(config.HomeDir(), ".claude"))
	return &Provider{root: root}
}

func (p *Provider) ID() string          { return ProviderID }
func (p *Provider) DisplayName() string { return "Claude Code" }
func (p *Provider) Installed() bool {
	st, err := os.Stat(filepath.Join(p.root, "projects"))
	return err == nil && st.IsDir()
}
func (p *Provider) SupportsResume() bool { return true }

func (p *Provider) DefaultPaths() []provider.PathSpec {
	return []provider.PathSpec{{Label: "projects", Path: filepath.Join(p.root, "projects"), Env: "CLAUDE_CONFIG_DIR"}}
}

func (p *Provider) projectsRoot() string {
	return filepath.Join(p.root, "projects")
}

func (p *Provider) Discover(ctx context.Context, opts provider.DiscoverOpts) ([]model.Summary, error) {
	root := p.projectsRoot()
	var out []model.Summary
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".jsonl") {
			return nil
		}
		if strings.Contains(path, "observer-sessions") && strings.Contains(path, "claude-mem") {
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

type claudeLine struct {
	Type      string `json:"type"`
	SessionID string `json:"sessionId"`
	Timestamp string `json:"timestamp"`
	UUID      string `json:"uuid"`
	IsMeta    bool   `json:"isMeta"`
	Message   *struct {
		Role    string `json:"role"`
		Content any    `json:"content"`
	} `json:"message"`
}

func (p *Provider) summarizeFile(path string) (model.Summary, error) {
	st, err := os.Stat(path)
	if err != nil {
		return model.Summary{}, err
	}
	base := filepath.Base(path)
	id := strings.TrimSuffix(base, ".jsonl")
	encoded := filepath.Base(filepath.Dir(path))
	project := util.DecodeClaudeProjectPath(encoded)
	var title, slashFallback string
	var msgCount int
	var first, last time.Time
	_ = util.ReadJSONLLines(path, 0, func(line []byte) error {
		var row claudeLine
		if json.Unmarshal(line, &row) != nil {
			return nil
		}
		if row.Type != "user" && row.Type != "assistant" {
			if row.SessionID != "" {
				id = row.SessionID
			}
			return nil
		}
		if row.Message == nil {
			return nil
		}
		if row.IsMeta {
			return nil
		}
		msgCount++
		ts := util.ParseTime(row.Timestamp)
		if first.IsZero() {
			first = ts
		}
		last = ts
		if row.Message.Role == "user" {
			text := contentString(row.Message.Content)
			if t, ok := claudeUserTitle(text); ok {
				if strings.HasPrefix(strings.TrimSpace(t), "/") {
					if slashFallback == "" {
						slashFallback = t
					}
				} else if title == "" {
					title = t
				}
			}
		}
		return nil
	})
	if title == "" {
		title = slashFallback
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

func isNoiseTitle(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return true
	}
	if strings.HasPrefix(s, "<local-command-caveat>") {
		return true
	}
	if strings.HasPrefix(s, "<command-message>") || strings.HasPrefix(s, "<command-name>") {
		return true
	}
	if strings.Contains(s, "<command-message>") || strings.Contains(s, "<command-name>") {
		return true
	}
	return false
}

// claudeUserTitle turns a Claude Code user message into a display title.
// Returns ok=false when the message should be skipped (noise / meta injection).
func claudeUserTitle(text string) (string, bool) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", false
	}
	if strings.HasPrefix(text, "<local-command-caveat>") {
		return "", false
	}
	if strings.HasPrefix(text, "<local-command-stdout>") {
		return "", false
	}
	if strings.HasPrefix(text, "<task-notification>") {
		return "", false
	}
	if name := extractXMLTag(text, "command-name"); name != "" {
		name = strings.TrimSpace(name)
		if !strings.HasPrefix(name, "/") {
			name = "/" + name
		}
		if isThinSlashTitle(name) {
			return "", false
		}
		return util.FirstUserSnippet(name, 80), true
	}
	if msg := extractXMLTag(text, "command-message"); msg != "" {
		cmd := "/" + strings.TrimSpace(msg)
		if isThinSlashTitle(cmd) {
			return "", false
		}
		return util.FirstUserSnippet(cmd, 80), true
	}
	return util.FirstUserSnippet(text, 80), true
}

// ponytail: skip Claude UI slash cmds that aren't session topics
func isThinSlashTitle(cmd string) bool {
	cmd = strings.ToLower(strings.Fields(strings.TrimSpace(cmd))[0])
	switch cmd {
	case "/model", "/login", "/effort", "/copy":
		return true
	default:
		return false
	}
}

func extractXMLTag(s, tag string) string {
	open := "<" + tag + ">"
	close := "</" + tag + ">"
	i := strings.Index(s, open)
	if i < 0 {
		return ""
	}
	i += len(open)
	j := strings.Index(s[i:], close)
	if j < 0 {
		return ""
	}
	return s[i : i+j]
}

func contentString(c any) string {
	switch v := c.(type) {
	case string:
		return v
	case []any:
		var parts []string
		for _, item := range v {
			if m, ok := item.(map[string]any); ok {
				if t, _ := m["text"].(string); t != "" {
					parts = append(parts, t)
				}
			}
		}
		return strings.Join(parts, "\n")
	}
	return ""
}

func (p *Provider) Load(ctx context.Context, ref provider.SessionRef) (*model.Conversation, error) {
	path := ref.StoragePath
	if path == "" {
		path = filepath.Join(p.projectsRoot(), util.EncodeClaudeProjectPath(ref.ProjectPath), ref.ID+".jsonl")
	}
	st, err := os.Stat(path)
	if err != nil {
		return nil, provider.ErrNotFound
	}
	encoded := filepath.Base(filepath.Dir(path))
	project := util.DecodeClaudeProjectPath(encoded)
	conv := &model.Conversation{
		ID: ref.ID, Provider: ProviderID, ProjectPath: project, StoragePath: path,
	}
	_ = util.ReadJSONLLines(path, 0, func(line []byte) error {
		var row claudeLine
		if json.Unmarshal(line, &row) != nil {
			return nil
		}
		if row.SessionID != "" {
			conv.ID = row.SessionID
		}
		if row.Type != "user" && row.Type != "assistant" {
			return nil
		}
		if row.Message == nil {
			return nil
		}
		ts := util.ParseTime(row.Timestamp)
		role := model.RoleUser
		if row.Message.Role == "assistant" || row.Type == "assistant" {
			role = model.RoleAssistant
		}
		conv.Messages = append(conv.Messages, model.Message{
			Role: role, Content: contentString(row.Message.Content), Timestamp: ts,
		})
		return nil
	})
	if len(conv.Messages) == 0 {
		return nil, provider.ErrNotFound
	}
	conv.MessageCount = len(conv.Messages)
	conv.CreatedAt = conv.Messages[0].Timestamp
	conv.UpdatedAt = conv.Messages[len(conv.Messages)-1].Timestamp
	if conv.Title == "" {
		var slashFallback string
		for _, m := range conv.Messages {
			if m.Role != model.RoleUser {
				continue
			}
			if t, ok := claudeUserTitle(m.PlainText()); ok {
				if strings.HasPrefix(strings.TrimSpace(t), "/") {
					if slashFallback == "" {
						slashFallback = t
					}
				} else {
					conv.Title = t
					break
				}
			}
		}
		if conv.Title == "" {
			conv.Title = slashFallback
		}
	}
	_ = st
	return conv, nil
}

func (p *Provider) Write(ctx context.Context, conv *model.Conversation, opts provider.WriteOpts) (*provider.WriteResult, error) {
	if len(conv.Messages) == 0 {
		return nil, provider.ErrEmptySession
	}
	project := opts.ProjectPath
	if project == "" {
		project = conv.ProjectPath
	}
	if project == "" {
		project, _ = os.Getwd()
	}
	dir := filepath.Join(p.projectsRoot(), util.EncodeClaudeProjectPath(project))
	sessionID := uuid.New().String()
	path := filepath.Join(dir, sessionID+".jsonl")
	if opts.DryRun {
		return &provider.WriteResult{SessionID: sessionID, StoragePath: path, ProjectPath: project}, nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	meta := model.NewMigrationMeta(conv)
	var lines []string
	progress := map[string]any{
		"type": "progress", "sessionId": sessionID,
		"timestamp": time.Now().UTC().Format(time.RFC3339Nano),
		"uuid": uuid.New().String(),
		"data": meta,
	}
	if b, err := json.Marshal(progress); err == nil {
		lines = append(lines, string(b))
	}
	var parent string
	for _, m := range conv.Messages {
		u := uuid.New().String()
		entryType := "user"
		role := "user"
		content := m.PlainText()
		if m.Role == model.RoleAssistant {
			entryType = "assistant"
			role = "assistant"
		}
		row := map[string]any{
			"type": entryType, "sessionId": sessionID,
			"timestamp": m.Timestamp.UTC().Format(time.RFC3339Nano),
			"uuid": u, "parentUuid": parent,
			"message": map[string]any{"role": role, "content": content},
		}
		if parent == "" {
			delete(row, "parentUuid")
		}
		b, _ := json.Marshal(row)
		lines = append(lines, string(b))
		parent = u
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		return nil, err
	}
	return &provider.WriteResult{SessionID: sessionID, StoragePath: path, ProjectPath: project}, nil
}

func (p *Provider) ResumeCommand(r provider.WriteResult) string {
	if r.ProjectPath != "" {
		return "cd " + r.ProjectPath + " && claude --resume " + r.SessionID
	}
	return "claude --resume " + r.SessionID
}
