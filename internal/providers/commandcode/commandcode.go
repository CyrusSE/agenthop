package commandcode

import (
	"context"
	"os"
	"path/filepath"

	"github.com/CyrusSE/agenthop/internal/config"
	"github.com/CyrusSE/agenthop/internal/model"
	"github.com/CyrusSE/agenthop/internal/provider"
)

const ProviderID = "commandcode"

// CommandCode uses a Claude Code-like JSONL layout under ~/.commandcode/projects.
type Provider struct {
	root string
}

func New() *Provider {
	root := config.EnvOrDefault("COMMANDCODE_HOME", filepath.Join(config.HomeDir(), ".commandcode"))
	return &Provider{root: root}
}

func (p *Provider) projectsRoot() string {
	return filepath.Join(p.root, "projects")
}

func (p *Provider) ID() string          { return ProviderID }
func (p *Provider) DisplayName() string { return "CommandCode" }
func (p *Provider) Installed() bool {
	st, err := os.Stat(p.projectsRoot())
	return err == nil && st.IsDir()
}
func (p *Provider) SupportsResume() bool { return true }

func (p *Provider) DefaultPaths() []provider.PathSpec {
	return []provider.PathSpec{{Label: "projects", Path: p.projectsRoot(), Env: "COMMANDCODE_HOME"}}
}

func (p *Provider) Discover(ctx context.Context, opts provider.DiscoverOpts) ([]model.Summary, error) {
	return discoverWithRoot(ctx, p.projectsRoot(), ProviderID, opts)
}

func (p *Provider) Load(ctx context.Context, ref provider.SessionRef) (*model.Conversation, error) {
	conv, err := loadWithRoot(ref, p.projectsRoot())
	if err != nil {
		return nil, err
	}
	conv.Provider = ProviderID
	return conv, nil
}

func (p *Provider) Write(ctx context.Context, conv *model.Conversation, opts provider.WriteOpts) (*provider.WriteResult, error) {
	conv.Provider = ProviderID
	return writeWithRoot(ctx, conv, opts, p.projectsRoot())
}

func (p *Provider) ResumeCommand(r provider.WriteResult) string {
	if r.ProjectPath != "" {
		return "cd " + r.ProjectPath + " && commandcode --resume " + r.SessionID
	}
	return "commandcode --resume " + r.SessionID
}
