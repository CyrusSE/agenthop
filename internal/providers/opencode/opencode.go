package opencode

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/CyrusSE/agenthop/internal/config"
	"github.com/CyrusSE/agenthop/internal/model"
	"github.com/CyrusSE/agenthop/internal/provider"
	"github.com/CyrusSE/agenthop/internal/util"
	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

const ProviderID = "opencode"

type Provider struct {
	dbPath string
}

func New() *Provider {
	root := config.EnvOrDefault("XDG_DATA_HOME", filepath.Join(config.HomeDir(), ".local", "share"))
	return &Provider{dbPath: filepath.Join(root, "opencode", "opencode.db")}
}

func (p *Provider) ID() string          { return ProviderID }
func (p *Provider) DisplayName() string { return "OpenCode" }
func (p *Provider) Installed() bool {
	_, err := os.Stat(p.dbPath)
	return err == nil
}
func (p *Provider) SupportsResume() bool { return true }

func (p *Provider) DefaultPaths() []provider.PathSpec {
	return []provider.PathSpec{{Label: "database", Path: p.dbPath}}
}

func (p *Provider) openRO() (*sql.DB, error) {
	return sql.Open("sqlite", p.dbPath+"?mode=ro")
}

func (p *Provider) Discover(ctx context.Context, opts provider.DiscoverOpts) ([]model.Summary, error) {
	db, err := p.openRO()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	rows, err := db.Query(`SELECT id, directory, title, time_created, time_updated FROM session ORDER BY time_updated DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	st, _ := os.Stat(p.dbPath)
	mtime := st.ModTime().Unix()
	var out []model.Summary
	for rows.Next() {
		var id, dir, title string
		var created, updated int64
		if err := rows.Scan(&id, &dir, &title, &created, &updated); err != nil {
			continue
		}
		if opts.ProjectFilter != "" && !strings.Contains(dir, opts.ProjectFilter) {
			continue
		}
		var msgCount int
		_ = db.QueryRow(`SELECT COUNT(*) FROM message WHERE session_id = ?`, id).Scan(&msgCount)
		if t := util.PickStoredOrMessages(title, p.opencodeUserLines(db, id)); t != "" {
			title = t
		}
		if title == "" {
			title = "(opencode session)"
		}
		out = append(out, model.Summary{
			ID: id, Provider: ProviderID, ProjectPath: dir, Title: title,
			CreatedAt: time.UnixMilli(created), UpdatedAt: time.UnixMilli(updated),
			MessageCount: msgCount, StoragePath: p.dbPath + "#" + id, SourceMtime: mtime,
		})
		if opts.Limit > 0 && len(out) >= opts.Limit {
			break
		}
	}
	return out, nil
}

type ocMessageData struct {
	Role string `json:"role"`
	Time struct {
		Created int64 `json:"created"`
	} `json:"time"`
}

type ocPartData struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func (p *Provider) Load(ctx context.Context, ref provider.SessionRef) (*model.Conversation, error) {
	db, err := p.openRO()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	id := ref.ID
	var dir, title string
	var created, updated int64
	err = db.QueryRow(`SELECT directory, title, time_created, time_updated FROM session WHERE id = ?`, id).
		Scan(&dir, &title, &created, &updated)
	if err != nil {
		return nil, provider.ErrNotFound
	}
	conv := &model.Conversation{
		ID: id, Provider: ProviderID, ProjectPath: dir, Title: title,
		CreatedAt: time.UnixMilli(created), UpdatedAt: time.UnixMilli(updated),
		StoragePath: p.dbPath + "#" + id,
	}
	rows, err := db.Query(`SELECT id, data, time_created FROM message WHERE session_id = ? ORDER BY time_created`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var msgID, data string
		var ts int64
		if rows.Scan(&msgID, &data, &ts) != nil {
			continue
		}
		var md ocMessageData
		if json.Unmarshal([]byte(data), &md) != nil || md.Role == "" {
			continue
		}
		content := p.messageText(db, msgID)
		if content == "" {
			continue
		}
		mrole := model.RoleUser
		if md.Role == "assistant" {
			mrole = model.RoleAssistant
		}
		msgTS := ts
		if md.Time.Created > 0 {
			msgTS = md.Time.Created
		}
		conv.Messages = append(conv.Messages, model.Message{
			Role: mrole, Content: content, Timestamp: time.UnixMilli(msgTS),
		})
	}
	if len(conv.Messages) == 0 {
		return nil, provider.ErrNotFound
	}
	conv.MessageCount = len(conv.Messages)
	return conv, nil
}

func (p *Provider) opencodeUserLines(db *sql.DB, sessionID string) []string {
	rows, err := db.Query(`SELECT id, data FROM message WHERE session_id = ? ORDER BY time_created LIMIT 40`, sessionID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var lines []string
	for rows.Next() {
		var msgID, data string
		if rows.Scan(&msgID, &data) != nil {
			continue
		}
		var md ocMessageData
		if json.Unmarshal([]byte(data), &md) != nil || md.Role != "user" {
			continue
		}
		if text := p.messageText(db, msgID); text != "" {
			lines = append(lines, text)
		}
	}
	return lines
}

func (p *Provider) messageText(db *sql.DB, messageID string) string {
	rows, err := db.Query(`SELECT data FROM part WHERE message_id = ? ORDER BY time_created`, messageID)
	if err != nil {
		return ""
	}
	defer rows.Close()
	var parts []string
	for rows.Next() {
		var data string
		if rows.Scan(&data) != nil {
			continue
		}
		var pd ocPartData
		if json.Unmarshal([]byte(data), &pd) != nil || pd.Text == "" {
			continue
		}
		if pd.Type != "text" && pd.Type != "reasoning" {
			continue
		}
		parts = append(parts, pd.Text)
	}
	return strings.Join(parts, "\n")
}

func (p *Provider) Write(ctx context.Context, conv *model.Conversation, opts provider.WriteOpts) (*provider.WriteResult, error) {
	if len(conv.Messages) == 0 {
		return nil, provider.ErrEmptySession
	}
	sessionID := ocID("ses_")
	project := opts.ProjectPath
	if project == "" {
		project = conv.ProjectPath
	}
	if project == "" {
		project, _ = os.Getwd()
	}
	now := time.Now().UnixMilli()
	title := conv.Title
	if title == "" {
		title = "Migrated session"
	}
	storagePath := p.dbPath + "#" + sessionID
	if opts.DryRun {
		return &provider.WriteResult{SessionID: sessionID, StoragePath: storagePath, ProjectPath: project}, nil
	}
	db, err := sql.Open("sqlite", p.dbPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	if err := p.ensureGlobalProject(db, now); err != nil {
		return nil, err
	}
	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	meta := model.NewMigrationMeta(conv)
	metaJSON, _ := json.Marshal(map[string]any{"agenthop_migration": meta})
	slug := util.FirstUserSnippet(title, 20)
	if slug == "" {
		slug = "migrated"
	}
	_, err = tx.Exec(`INSERT INTO session (
  id, project_id, directory, title, version, slug, time_created, time_updated, metadata, agent, model, cost,
  tokens_input, tokens_output, tokens_reasoning, tokens_cache_read, tokens_cache_write
) VALUES (?, 'global', ?, ?, '1.15.13', ?, ?, ?, ?, 'build', '{"id":"claude-sonnet-4.6","providerID":"github-copilot"}', 0, 0, 0, 0, 0, 0)`,
		sessionID, project, title, slug, now, now, string(metaJSON))
	if err != nil {
		return nil, fmt.Errorf("insert session: %w", err)
	}
	for _, m := range conv.Messages {
		msgID := ocID("msg_")
		ts := m.Timestamp.UnixMilli()
		if ts == 0 {
			ts = now
		}
		msgData, _ := json.Marshal(map[string]any{
			"role":    string(m.Role),
			"time":    map[string]any{"created": ts},
			"summary": map[string]any{"diffs": []any{}},
		})
		_, err = tx.Exec(`INSERT INTO message (id, session_id, time_created, time_updated, data) VALUES (?, ?, ?, ?, ?)`,
			msgID, sessionID, ts, ts, string(msgData))
		if err != nil {
			return nil, fmt.Errorf("insert message: %w", err)
		}
		partID := ocID("prt_")
		partData, _ := json.Marshal(ocPartData{Type: "text", Text: m.PlainText()})
		_, err = tx.Exec(`INSERT INTO part (id, message_id, session_id, time_created, time_updated, data) VALUES (?, ?, ?, ?, ?, ?)`,
			partID, msgID, sessionID, ts, ts, string(partData))
		if err != nil {
			return nil, fmt.Errorf("insert part: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit session: %w", err)
	}
	return &provider.WriteResult{SessionID: sessionID, StoragePath: storagePath, ProjectPath: project}, nil
}

func (p *Provider) ensureGlobalProject(db *sql.DB, now int64) error {
	var n int
	_ = db.QueryRow(`SELECT COUNT(*) FROM project WHERE id = 'global'`).Scan(&n)
	if n > 0 {
		return nil
	}
	_, err := db.Exec(`INSERT OR IGNORE INTO project (id, worktree, time_created, time_updated, sandboxes) VALUES ('global', '/', ?, ?, '[]')`, now, now)
	return err
}

func ocID(prefix string) string {
	raw := strings.ReplaceAll(uuid.New().String(), "-", "")
	if len(raw) > 20 {
		raw = raw[:20]
	}
	return prefix + raw
}

func (p *Provider) ResumeCommand(r provider.WriteResult) string {
	return "opencode --session " + r.SessionID
}
