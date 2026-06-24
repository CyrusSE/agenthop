package opencode

import (
	"context"
	"database/sql"
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
	rows, err := db.Query(`SELECT role, content, time_created FROM message WHERE session_id = ? ORDER BY time_created`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var role, content string
		var ts int64
		if rows.Scan(&role, &content, &ts) != nil {
			continue
		}
		mrole := model.RoleUser
		if role == "assistant" {
			mrole = model.RoleAssistant
		}
		conv.Messages = append(conv.Messages, model.Message{
			Role: mrole, Content: content, Timestamp: time.UnixMilli(ts),
		})
	}
	if len(conv.Messages) == 0 {
		return nil, provider.ErrNotFound
	}
	conv.MessageCount = len(conv.Messages)
	return conv, nil
}

func (p *Provider) Write(ctx context.Context, conv *model.Conversation, opts provider.WriteOpts) (*provider.WriteResult, error) {
	if len(conv.Messages) == 0 {
		return nil, provider.ErrEmptySession
	}
	sessionID := "ses_" + strings.ReplaceAll(uuid.New().String(), "-", "")[:20]
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
	if opts.DryRun {
		return &provider.WriteResult{SessionID: sessionID, StoragePath: p.dbPath, ProjectPath: project}, nil
	}
	db, err := sql.Open("sqlite", p.dbPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	_, err = db.Exec(`INSERT INTO session (id, project_id, directory, title, version, time_created, time_updated, slug)
VALUES (?, 'global', ?, ?, '1.0.0', ?, ?, ?)`,
		sessionID, project, title, now, now, util.FirstUserSnippet(title, 20))
	if err != nil {
		return nil, err
	}
	for _, m := range conv.Messages {
		msgID := uuid.New().String()
		ts := m.Timestamp.UnixMilli()
		if ts == 0 {
			ts = now
		}
		_, err = db.Exec(`INSERT INTO message (id, session_id, role, content, time_created, time_updated) VALUES (?, ?, ?, ?, ?, ?)`,
			msgID, sessionID, string(m.Role), m.PlainText(), ts, ts)
		if err != nil {
			return nil, err
		}
	}
	return &provider.WriteResult{SessionID: sessionID, StoragePath: p.dbPath, ProjectPath: project}, nil
}

func (p *Provider) ResumeCommand(r provider.WriteResult) string {
	return "opencode --session " + r.SessionID
}
