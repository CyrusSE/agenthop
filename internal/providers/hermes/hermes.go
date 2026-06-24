package hermes

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"time"

	"github.com/CyrusSE/agenthop/internal/config"
	"github.com/CyrusSE/agenthop/internal/model"
	"github.com/CyrusSE/agenthop/internal/provider"
	"github.com/CyrusSE/agenthop/internal/util"
	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

const ProviderID = "hermes"

type Provider struct {
	dbPath string
}

func New() *Provider {
	root := config.EnvOrDefault("HERMES_HOME", filepath.Join(config.HomeDir(), ".hermes"))
	return &Provider{dbPath: filepath.Join(root, "state.db")}
}

func (p *Provider) ID() string          { return ProviderID }
func (p *Provider) DisplayName() string { return "Hermes" }
func (p *Provider) Installed() bool {
	_, err := os.Stat(p.dbPath)
	return err == nil
}
func (p *Provider) SupportsResume() bool { return true }

func (p *Provider) DefaultPaths() []provider.PathSpec {
	return []provider.PathSpec{{Label: "state.db", Path: p.dbPath, Env: "HERMES_HOME"}}
}

func (p *Provider) Discover(ctx context.Context, opts provider.DiscoverOpts) ([]model.Summary, error) {
	db, err := sql.Open("sqlite", p.dbPath+"?mode=ro")
	if err != nil {
		return nil, err
	}
	defer db.Close()
	rows, err := db.Query(`SELECT id, title, started_at, message_count FROM sessions ORDER BY started_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	st, _ := os.Stat(p.dbPath)
	mtime := st.ModTime().Unix()
	var out []model.Summary
	for rows.Next() {
		var id, title string
		var started float64
		var msgCount int
		if rows.Scan(&id, &title, &started, &msgCount) != nil {
			continue
		}
		if title == "" {
			title = "(hermes session)"
		}
		ts := time.Unix(int64(started), 0)
		out = append(out, model.Summary{
			ID: id, Provider: ProviderID, Title: title,
			CreatedAt: ts, UpdatedAt: ts, MessageCount: msgCount,
			StoragePath: p.dbPath + "#" + id, SourceMtime: mtime,
		})
		if opts.Limit > 0 && len(out) >= opts.Limit {
			break
		}
	}
	return out, nil
}

func (p *Provider) Load(ctx context.Context, ref provider.SessionRef) (*model.Conversation, error) {
	db, err := sql.Open("sqlite", p.dbPath+"?mode=ro")
	if err != nil {
		return nil, err
	}
	defer db.Close()
	var title string
	var started float64
	err = db.QueryRow(`SELECT title, started_at FROM sessions WHERE id = ?`, ref.ID).Scan(&title, &started)
	if err != nil {
		return nil, provider.ErrNotFound
	}
	conv := &model.Conversation{
		ID: ref.ID, Provider: ProviderID, Title: title,
		CreatedAt: time.Unix(int64(started), 0), StoragePath: p.dbPath + "#" + ref.ID,
	}
	rows, err := db.Query(`SELECT role, content FROM messages WHERE session_id = ? ORDER BY id`, ref.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var role, content string
		if rows.Scan(&role, &content) != nil {
			continue
		}
		mrole := model.RoleUser
		if role == "assistant" {
			mrole = model.RoleAssistant
		}
		conv.Messages = append(conv.Messages, model.Message{Role: mrole, Content: content})
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
	sessionID := uuid.New().String()
	now := float64(time.Now().Unix())
	title := conv.Title
	if title == "" {
		title = util.FirstUserSnippet(conv.Messages[0].PlainText(), 60)
	}
	if opts.DryRun {
		return &provider.WriteResult{SessionID: sessionID, StoragePath: p.dbPath + "#" + sessionID, ProjectPath: conv.ProjectPath}, nil
	}
	db, err := sql.Open("sqlite", p.dbPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	_, err = db.Exec(`INSERT INTO sessions (id, source, started_at, message_count, title) VALUES (?, 'cli', ?, ?, ?)`,
		sessionID, now, len(conv.Messages), title)
	if err != nil {
		return nil, err
	}
	for _, m := range conv.Messages {
		_, err = db.Exec(`INSERT INTO messages (session_id, role, content) VALUES (?, ?, ?)`,
			sessionID, string(m.Role), m.PlainText())
		if err != nil {
			return nil, err
		}
	}
	return &provider.WriteResult{SessionID: sessionID, StoragePath: p.dbPath + "#" + sessionID, ProjectPath: conv.ProjectPath}, nil
}

func (p *Provider) ResumeCommand(r provider.WriteResult) string {
	return "hermes --session " + r.SessionID
}
