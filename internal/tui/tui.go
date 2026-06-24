package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/CyrusSE/agenthop/internal/index"
	"github.com/CyrusSE/agenthop/internal/migrate"
	"github.com/CyrusSE/agenthop/internal/model"
	"github.com/CyrusSE/agenthop/internal/provider"
	"github.com/CyrusSE/agenthop/internal/registry"
)

var (
	titleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	okStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	paneStyle   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
)

const (
	stageProviders = iota
	stageSessions
	stagePreview
	stageMigrate
)

type sessionItem struct {
	summary model.Summary
}

func (i sessionItem) Title() string       { return i.summary.ShortID() + "  " + truncate(i.summary.Title, 40) }
func (i sessionItem) Description() string { return i.summary.ProjectPath }
func (i sessionItem) FilterValue() string {
	return i.summary.ID + " " + i.summary.Title + " " + i.summary.ProjectPath
}

type providerItem struct {
	id    string
	name  string
	count int
}

func (i providerItem) Title() string       { return fmt.Sprintf("%s (%d)", i.name, i.count) }
func (i providerItem) Description() string { return i.id }
func (i providerItem) FilterValue() string { return i.id + " " + i.name }

type targetItem struct {
	id   string
	name string
}

func (i targetItem) Title() string       { return i.name }
func (i targetItem) Description() string { return i.id }
func (i targetItem) FilterValue() string { return i.id + " " + i.name }

type modelState struct {
	reg       *registry.Registry
	idx       *index.Store
	engine    *migrate.Engine
	providers list.Model
	sessions  list.Model
	targets   list.Model
	preview   viewport.Model
	stage     int
	selected  *sessionItem
	err       string
	status    string
	width     int
	height    int
}

func Run(reg *registry.Registry, idx *index.Store, engine *migrate.Engine) error {
	ctx := context.Background()
	_, _ = index.UpdateIncremental(ctx, reg, idx, "")
	counts, _ := idx.CountByProvider()
	var pitems []list.Item
	for _, p := range reg.All() {
		if !p.Installed() {
			continue
		}
		pitems = append(pitems, providerItem{id: p.ID(), name: p.DisplayName(), count: counts[p.ID()]})
	}
	delegate := list.NewDefaultDelegate()
	provList := list.New(pitems, delegate, 40, 20)
	provList.Title = "Providers"
	provList.SetShowStatusBar(false)
	sessList := list.New([]list.Item{}, delegate, 50, 20)
	sessList.Title = "Sessions  / filter"
	sessList.SetFilteringEnabled(true)
	targetList := list.New([]list.Item{}, delegate, 40, 12)
	targetList.Title = "Migrate to"
	vp := viewport.New(60, 18)
	m := modelState{
		reg: reg, idx: idx, engine: engine,
		providers: provList, sessions: sessList, targets: targetList, preview: vp,
	}
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func (m modelState) Init() tea.Cmd { return nil }

func (m modelState) loadTargets(exclude string) {
	var items []list.Item
	for _, p := range m.reg.All() {
		if !p.Installed() || p.ID() == exclude {
			continue
		}
		items = append(items, targetItem{id: p.ID(), name: p.DisplayName()})
	}
	m.targets.SetItems(items)
}

func (m modelState) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		w := msg.Width / 3
		if w < 20 {
			w = 20
		}
		m.providers.SetSize(w, msg.Height-4)
		m.sessions.SetSize(w, msg.Height-4)
		m.targets.SetSize(w, 12)
		m.preview.Width = msg.Width - 2*w - 4
		if m.preview.Width < 20 {
			m.preview.Width = 20
		}
		m.preview.Height = msg.Height - 6
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "esc":
			m.err = ""
			m.status = ""
			if m.stage == stageMigrate {
				m.stage = stageSessions
			} else if m.stage > stageProviders {
				m.stage--
			}
			return m, nil
		case "enter":
			switch m.stage {
			case stageProviders:
				if it, ok := m.providers.SelectedItem().(providerItem); ok {
					items, err := m.idx.List(index.ListOpts{Provider: it.id, Limit: 300})
					if err != nil {
						m.err = err.Error()
						return m, nil
					}
					var sitems []list.Item
					for _, s := range items {
						sitems = append(sitems, sessionItem{summary: s})
					}
					m.sessions.SetItems(sitems)
					m.stage = stageSessions
				}
			case stageSessions:
				if it, ok := m.sessions.SelectedItem().(sessionItem); ok {
					m.selected = &it
					p, _ := m.reg.Get(it.summary.Provider)
					conv, err := p.Load(context.Background(), provider.SessionRef{
						ID: it.summary.ID, StoragePath: it.summary.StoragePath, ProjectPath: it.summary.ProjectPath,
					})
					if err != nil {
						m.err = err.Error()
						return m, nil
					}
					var b strings.Builder
					limit := len(conv.Messages)
					if limit > 30 {
						limit = 30
						b.WriteString(fmt.Sprintf("… showing last %d of %d messages\n\n", limit, len(conv.Messages)))
						conv.Messages = conv.Messages[len(conv.Messages)-limit:]
					}
					for _, msg := range conv.Messages {
						b.WriteString(fmt.Sprintf("[%s]\n%s\n\n", msg.Role, truncate(msg.PlainText(), 400)))
					}
					m.preview.SetContent(b.String())
					m.stage = stagePreview
				}
			case stageMigrate:
				if m.selected == nil {
					return m, nil
				}
				if tgt, ok := m.targets.SelectedItem().(targetItem); ok {
					res, err := m.engine.Run(context.Background(), migrate.Options{
						SessionID:    m.selected.summary.ID,
						FromProvider: m.selected.summary.Provider,
						ToProvider:   tgt.id,
					})
					if err != nil {
						m.err = err.Error()
						return m, nil
					}
					if res.AlreadyExists {
						m.status = okStyle.Render(fmt.Sprintf("Already migrated → %s\n%s", res.TargetName, res.Resume))
					} else {
						m.status = okStyle.Render(fmt.Sprintf("Migrated → %s\n%s", res.TargetName, res.Resume))
					}
					m.stage = stagePreview
				}
			}
			return m, nil
		case "m":
			if m.stage >= stageSessions {
				if it, ok := m.sessions.SelectedItem().(sessionItem); ok {
					m.selected = &it
					m.loadTargets(it.summary.Provider)
					m.stage = stageMigrate
					m.err = ""
				}
			}
			return m, nil
		case "r":
			_, _ = index.UpdateIncremental(context.Background(), m.reg, m.idx, "")
			counts, _ := m.idx.CountByProvider()
			var pitems []list.Item
			for _, p := range m.reg.All() {
				if !p.Installed() {
					continue
				}
				pitems = append(pitems, providerItem{id: p.ID(), name: p.DisplayName(), count: counts[p.ID()]})
			}
			m.providers.SetItems(pitems)
			m.status = "Index refreshed"
			return m, nil
		}
	}
	var cmd tea.Cmd
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

func (m modelState) View() string {
	help := statusStyle.Render("↑↓ navigate · enter select · m migrate · r refresh · esc back · q quit")
	header := titleStyle.Render("agenthop") + "  " + help
	if m.status != "" {
		header += "\n" + m.status
	}
	if m.err != "" {
		header += "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render(m.err)
	}
	switch m.stage {
	case stageProviders:
		return header + "\n" + paneStyle.Render(m.providers.View())
	case stageSessions:
		return header + "\n" + lipgloss.JoinHorizontal(lipgloss.Top,
			paneStyle.Render(m.providers.View()),
			paneStyle.Render(m.sessions.View()),
		)
	case stageMigrate:
		return header + "\n" + lipgloss.JoinHorizontal(lipgloss.Top,
			paneStyle.Render(m.sessions.View()),
			paneStyle.Render(m.targets.View()),
		)
	default:
		return header + "\n" + lipgloss.JoinHorizontal(lipgloss.Top,
			paneStyle.Width(m.width/3).Render(m.providers.View()),
			paneStyle.Width(m.width/3).Render(m.sessions.View()),
			paneStyle.Width(m.width-m.width*2/3-4).Render(m.preview.View()),
		)
	}
}

func truncate(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
