package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/CyrusSE/agenthop/internal/debuglog"
	"github.com/CyrusSE/agenthop/internal/index"
	"github.com/CyrusSE/agenthop/internal/migrate"
	"github.com/CyrusSE/agenthop/internal/model"
	"github.com/CyrusSE/agenthop/internal/provider"
	"github.com/CyrusSE/agenthop/internal/registry"
)

var (
	accentStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("141"))
	titleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	mutedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	okStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	errStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	paneStyle   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("238")).Padding(0, 1)
	footerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Background(lipgloss.Color("235")).Padding(0, 1)
)

const (
	stageProviders = iota
	stageSessions
	stagePreview
	stageMigrate
)

type sessionItem struct{ summary model.Summary }

func (i sessionItem) Title() string {
	return fmt.Sprintf("%s  %s", accentStyle.Render(i.summary.ShortID()), truncate(i.summary.Title, 36))
}
func (i sessionItem) Description() string { return truncate(i.summary.ProjectPath, 48) }
func (i sessionItem) FilterValue() string {
	return i.summary.ID + " " + i.summary.Title + " " + i.summary.ProjectPath
}

type providerItem struct{ id, name string; count int }

func (i providerItem) Title() string {
	return fmt.Sprintf("%s %s", i.name, mutedStyle.Render(fmt.Sprintf("(%d)", i.count)))
}
func (i providerItem) Description() string { return i.id }
func (i providerItem) FilterValue() string { return i.id + " " + i.name }

type targetItem struct{ id, name string }

func (i targetItem) Title() string       { return i.name }
func (i targetItem) Description() string { return mutedStyle.Render(i.id) }
func (i targetItem) FilterValue() string { return i.id + " " + i.name }

type sessionsLoadedMsg struct {
	provider string
	items    []list.Item
	err      error
}
type previewLoadedMsg struct {
	content string
	err     error
}
type migrateDoneMsg struct {
	res *migrate.Result
	err error
}
type indexRefreshedMsg struct {
	counts   map[string]int
	err      error
	provider string
}

type modelState struct {
	reg       *registry.Registry
	idx       *index.Store
	engine    *migrate.Engine
	providers list.Model
	sessions  list.Model
	targets   list.Model
	preview   viewport.Model
	spinner   spinner.Model
	stage     int
	selected  *sessionItem
	selectedP string
	loading   bool
	lastResume string
	err       string
	status    string
	width     int
	height    int
}

func Run(reg *registry.Registry, idx *index.Store, engine *migrate.Engine) error {
	counts, _ := idx.CountByProvider()
	pitems := providerItems(reg, counts)
	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = true
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.Foreground(lipgloss.Color("212"))
	provList := list.New(pitems, delegate, 42, 22)
	provList.Title = "Agents"
	provList.SetShowStatusBar(false)
	provList.DisableQuitKeybindings()
	sessList := list.New([]list.Item{}, delegate, 52, 22)
	sessList.Title = "Sessions"
	sessList.SetFilteringEnabled(true)
	sessList.SetShowStatusBar(true)
	sessList.DisableQuitKeybindings()
	targetList := list.New([]list.Item{}, delegate, 36, 14)
	targetList.Title = "Migrate to"
	targetList.DisableQuitKeybindings()
	vp := viewport.New(64, 20)
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = accentStyle
	m := modelState{
		reg: reg, idx: idx, engine: engine,
		providers: provList, sessions: sessList, targets: targetList,
		preview: vp, spinner: sp,
	}
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func providerItems(reg *registry.Registry, counts map[string]int) []list.Item {
	var pitems []list.Item
	for _, p := range reg.All() {
		if !p.Installed() {
			continue
		}
		pitems = append(pitems, providerItem{id: p.ID(), name: p.DisplayName(), count: counts[p.ID()]})
	}
	return pitems
}

func targetItems(reg *registry.Registry, exclude string) []list.Item {
	var items []list.Item
	for _, p := range reg.All() {
		if !p.Installed() || p.ID() == exclude {
			continue
		}
		items = append(items, targetItem{id: p.ID(), name: p.DisplayName()})
	}
	return items
}

func (m modelState) Init() tea.Cmd {
	return nil
}

func loadSessionsCmd(reg *registry.Registry, idx *index.Store, providerID string) tea.Cmd {
	return func() tea.Msg {
		start := time.Now()
		n, err := index.UpdateIncremental(context.Background(), reg, idx, providerID)
		debuglog.Log("H1", "tui.loadSessions", "index update", "run1", map[string]any{
			"provider": providerID, "updated": n, "ms": time.Since(start).Milliseconds(),
		})
		items, lerr := idx.List(index.ListOpts{Provider: providerID, Limit: 500})
		if err == nil {
			err = lerr
		}
		var sitems []list.Item
		for _, s := range items {
			sitems = append(sitems, sessionItem{summary: s})
		}
		debuglog.Log("H2", "tui.loadSessions", "sessions listed", "run1", map[string]any{
			"provider": providerID, "count": len(sitems),
		})
		return sessionsLoadedMsg{provider: providerID, items: sitems, err: err}
	}
}

func loadPreviewCmd(reg *registry.Registry, sm model.Summary) tea.Cmd {
	return func() tea.Msg {
		p, err := reg.Get(sm.Provider)
		if err != nil {
			return previewLoadedMsg{err: err}
		}
		conv, err := p.Load(context.Background(), provider.SessionRef{
			ID: sm.ID, StoragePath: sm.StoragePath, ProjectPath: sm.ProjectPath,
		})
		if err != nil {
			return previewLoadedMsg{err: err}
		}
		var b strings.Builder
		b.WriteString(titleStyle.Render(truncate(conv.Title, 60)) + "\n")
		b.WriteString(mutedStyle.Render(fmt.Sprintf("%s · %d messages\n\n", conv.Provider, len(conv.Messages))))
		msgs := conv.Messages
		if len(msgs) > 40 {
			b.WriteString(mutedStyle.Render(fmt.Sprintf("… last 40 of %d\n\n", len(msgs))))
			msgs = msgs[len(msgs)-40:]
		}
		for _, msg := range msgs {
			role := accentStyle.Render(string(msg.Role))
			b.WriteString(role + "\n" + truncate(msg.PlainText(), 500) + "\n\n")
		}
		return previewLoadedMsg{content: b.String()}
	}
}

func migrateCmd(engine *migrate.Engine, sm model.Summary, to string) tea.Cmd {
	return func() tea.Msg {
		res, err := engine.Run(context.Background(), migrate.Options{
			SessionID: sm.ID, FromProvider: sm.Provider, ToProvider: to,
		})
		debuglog.Log("H3", "tui.migrate", "migrate finished", "run1", map[string]any{
			"to": to, "ok": err == nil, "already": res != nil && res.AlreadyExists,
		})
		return migrateDoneMsg{res: res, err: err}
	}
}

func refreshIndexCmd(reg *registry.Registry, idx *index.Store, providerFilter string) tea.Cmd {
	return func() tea.Msg {
		_, err := index.UpdateIncremental(context.Background(), reg, idx, providerFilter)
		counts, _ := idx.CountByProvider()
		return indexRefreshedMsg{counts: counts, err: err, provider: providerFilter}
	}
}

func (m modelState) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.layout()
		return m, nil
	case sessionsLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err.Error()
			return m, nil
		}
		m.sessions.SetItems(msg.items)
		m.selectedP = msg.provider
		m.stage = stageSessions
		m.status = fmt.Sprintf("Loaded %d sessions", len(msg.items))
		m.layout()
		return m, nil
	case previewLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err.Error()
			return m, nil
		}
		m.preview.SetContent(msg.content)
		m.stage = stagePreview
		return m, nil
	case migrateDoneMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err.Error()
			return m, nil
		}
		m.lastResume = msg.res.Resume
		if msg.res.AlreadyExists {
			m.status = okStyle.Render("Already migrated — resume command ready (press c to copy)")
		} else {
			m.status = okStyle.Render("Migration complete — resume command ready (press c to copy)")
		}
		m.stage = stagePreview
		return m, nil
	case indexRefreshedMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err.Error()
			return m, nil
		}
		m.providers.SetItems(providerItems(m.reg, msg.counts))
		if msg.provider != "" {
			m.status = fmt.Sprintf("Refreshed %s index", msg.provider)
		} else {
			m.status = "Index refreshed"
		}
		return m, nil
	case tea.KeyMsg:
		if m.loading {
			if msg.String() == "ctrl+c" || msg.String() == "q" {
				return m, tea.Quit
			}
			return m, nil
		}
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "esc":
			m.err, m.status = "", ""
			switch m.stage {
			case stageMigrate:
				m.stage = stagePreview
			case stagePreview:
				m.stage = stageSessions
			case stageSessions:
				m.stage = stageProviders
			}
			return m, nil
		case "c":
			if m.lastResume != "" {
				_ = clipboard.WriteAll(m.lastResume)
				m.status = okStyle.Render("Copied resume command to clipboard")
			}
			return m, nil
		case "enter":
			switch m.stage {
			case stageProviders:
				if it, ok := m.providers.SelectedItem().(providerItem); ok {
					m.loading = true
					m.err = ""
					return m, tea.Batch(m.spinner.Tick, loadSessionsCmd(m.reg, m.idx, it.id))
				}
			case stageSessions:
				if it, ok := m.sessions.SelectedItem().(sessionItem); ok {
					m.selected = &it
					m.loading = true
					return m, tea.Batch(m.spinner.Tick, loadPreviewCmd(m.reg, it.summary))
				}
			case stageMigrate:
				if m.selected == nil {
					return m, nil
				}
				if tgt, ok := m.targets.SelectedItem().(targetItem); ok {
					m.loading = true
					m.err = ""
					debuglog.Log("H3", "tui.Update", "migrate start", "run1", map[string]any{"to": tgt.id})
					return m, tea.Batch(m.spinner.Tick, migrateCmd(m.engine, m.selected.summary, tgt.id))
				}
			}
			return m, nil
		case "m":
			if m.stage >= stageSessions && m.selected != nil {
				items := targetItems(m.reg, m.selected.summary.Provider)
				debuglog.Log("H4", "tui.Update", "targets built", "run1", map[string]any{"count": len(items)})
				m.targets.SetItems(items)
				m.stage = stageMigrate
				m.err = ""
				m.layout()
			} else if m.stage == stageSessions {
				if it, ok := m.sessions.SelectedItem().(sessionItem); ok {
					m.selected = &it
					m.targets.SetItems(targetItems(m.reg, it.summary.Provider))
					m.stage = stageMigrate
					m.layout()
				}
			}
			return m, nil
		case "r":
			m.loading = true
			filter := ""
			if m.selectedP != "" {
				filter = m.selectedP
			}
			return m, tea.Batch(m.spinner.Tick, refreshIndexCmd(m.reg, m.idx, filter))
		}
	}
	var cmd tea.Cmd
	if m.loading {
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
	switch m.stage {
	case stageProviders:
		m.providers, cmd = m.providers.Update(msg)
	case stageSessions:
		m.sessions, cmd = m.sessions.Update(msg)
	case stageMigrate:
		m.targets, cmd = m.targets.Update(msg)
	default:
		m.preview, cmd = m.preview.Update(msg)
	}
	return m, cmd
}

func (m *modelState) layout() {
	if m.width < 40 {
		return
	}
	h := m.height - 5
	if h < 8 {
		h = 8
	}
	w := m.width / 3
	if w < 24 {
		w = 24
	}
	m.providers.SetSize(w, h)
	m.sessions.SetSize(w, h)
	m.targets.SetSize(w, min(16, h))
	pw := m.width - 2*w - 6
	if pw < 28 {
		pw = 28
	}
	m.preview.Width = pw
	m.preview.Height = h
}

func (m modelState) View() string {
	var b strings.Builder
	logo := accentStyle.Render("◆ agenthop") + mutedStyle.Render("  session migrator")
	b.WriteString(logo + "\n")
	if m.loading {
		b.WriteString(m.spinner.View() + mutedStyle.Render(" working…") + "\n")
	}
	if m.status != "" {
		b.WriteString(m.status + "\n")
	}
	if m.err != "" {
		b.WriteString(errStyle.Render("✗ "+m.err) + "\n")
	}
	b.WriteString("\n")
	switch m.stage {
	case stageProviders:
		b.WriteString(paneStyle.Width(m.width - 4).Render(m.providers.View()))
	case stageSessions:
		b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top,
			paneStyle.Width(m.width/2-2).Render(m.providers.View()),
			paneStyle.Width(m.width/2-2).Render(m.sessions.View()),
		))
	case stageMigrate:
		b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top,
			paneStyle.Width(m.width/2-2).Render(m.sessions.View()),
			paneStyle.Width(m.width/2-2).Render(m.targets.View()),
		))
	default:
		b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top,
			paneStyle.Width(m.width/3-2).Render(m.sessions.View()),
			paneStyle.Width(m.width*2/3-4).Render(m.preview.View()),
		))
	}
	help := "↑↓ navigate · enter open · m migrate · / filter · r refresh · c copy resume · esc back · q quit"
	b.WriteString("\n" + footerStyle.Width(m.width).Render(help))
	return b.String()
}

func truncate(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "…"
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
