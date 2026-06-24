package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/CyrusSE/agenthop/internal/index"
	"github.com/CyrusSE/agenthop/internal/migrate"
	"github.com/CyrusSE/agenthop/internal/provider"
	"github.com/CyrusSE/agenthop/internal/registry"
	"github.com/CyrusSE/agenthop/internal/util"
	"github.com/CyrusSE/agenthop/internal/tui"
	"github.com/spf13/cobra"
)

var version = "dev"

type App struct {
	Registry *registry.Registry
	Index    *index.Store
	Migrate  *migrate.Engine
	Verbose  bool
}

func NewApp() (*App, error) {
	reg := registry.New()
	idx, err := index.Open("")
	if err != nil {
		return nil, err
	}
	return &App{
		Registry: reg,
		Index:    idx,
		Migrate:  &migrate.Engine{Registry: reg, Index: idx},
	}, nil
}

func (a *App) Root() *cobra.Command {
	root := &cobra.Command{
		Use:           "agenthop",
		Short:         "Hop AI coding sessions between agents",
		Long:          "List, show, and migrate conversation sessions across Claude Code, Codex, Cursor, OpenCode, CommandCode, Hermes, and more.",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       version,
		RunE: func(cmd *cobra.Command, args []string) error {
			return tui.Run(a.Registry, a.Index, a.Migrate)
		},
	}
	root.PersistentFlags().BoolVarP(&a.Verbose, "verbose", "v", false, "verbose output")

	root.AddCommand(a.listCmd())
	root.AddCommand(a.showCmd())
	root.AddCommand(a.migrateCmd())
	root.AddCommand(a.indexCmd())
	root.AddCommand(a.providersCmd())
	root.AddCommand(a.exportCmd())
	root.AddCommand(a.importCmd())
	root.AddCommand(a.resumeCmd())
	root.AddCommand(a.tuiCmd())
	return root
}

func (a *App) ensureIndex(ctx context.Context, providerFilter string, refresh bool) error {
	if !refresh {
		counts, _ := a.Index.CountByProvider()
		if providerFilter != "" {
			if counts[registry.NormalizeID(providerFilter)] > 0 {
				return nil
			}
		} else {
			total := 0
			for _, n := range counts {
				total += n
			}
			if total > 0 {
				return nil
			}
		}
	}
	_, err := index.UpdateIncremental(ctx, a.Registry, a.Index, registry.NormalizeID(providerFilter))
	return err
}

func (a *App) listCmd() *cobra.Command {
	var providerID, project string
	var limit int
	var asJSON, refresh, cwdOnly bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List indexed sessions",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if err := a.ensureIndex(ctx, providerID, refresh); err != nil {
				return err
			}
			opts := index.ListOpts{
				Provider: registry.NormalizeID(providerID),
				Limit:    limit,
			}
			if cwdOnly {
				wd, err := os.Getwd()
				if err != nil {
					return err
				}
				opts.ProjectCWD = util.NormalizeProjectPath(wd)
			} else if project != "" {
				opts.ProjectFilter = project
			}
			items, err := a.Index.List(opts)
			if err != nil {
				return err
			}
			if asJSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(items)
			}
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tPROVIDER\tUPDATED\tMSGS\tTITLE")
			for _, s := range items {
				fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\n",
					s.ShortID(), registry.DisplayName(a.Registry, s.Provider),
					util.FormatRelative(s.UpdatedAt), s.MessageCount, util.TruncateRunes(s.Title, 50))
			}
			return w.Flush()
		},
	}
	cmd.Flags().StringVar(&providerID, "provider", "", "filter by provider")
	cmd.Flags().StringVar(&project, "project", "", "filter by project path substring")
	cmd.Flags().BoolVar(&cwdOnly, "cwd", false, "only sessions for the current working directory")
	cmd.Flags().IntVar(&limit, "limit", 0, "max results (0 = unlimited)")
	cmd.Flags().BoolVar(&asJSON, "json", false, "JSON output")
	cmd.Flags().BoolVar(&refresh, "refresh", false, "refresh index before listing")
	return cmd
}

func (a *App) showCmd() *cobra.Command {
	var from string
	var limit int
	var raw, refresh bool
	cmd := &cobra.Command{
		Use:   "show <session-id>",
		Short: "Show session messages",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if err := a.ensureIndex(ctx, from, refresh); err != nil {
				return err
			}
			sm, p, err := migrate.ResolveSession(ctx, a.Registry, a.Index, args[0], from)
			if err != nil {
				return err
			}
			conv, err := p.Load(ctx, provider.SessionRef{
				ID: sm.ID, StoragePath: sm.StoragePath, ProjectPath: sm.ProjectPath,
			})
			if err != nil {
				return err
			}
			fmt.Printf("Session %s (%s)\nProject: %s\nMessages: %d\n\n", conv.ID, conv.Provider, conv.ProjectPath, len(conv.Messages))
			msgs := conv.Messages
			if limit > 0 && len(msgs) > limit {
				msgs = msgs[len(msgs)-limit:]
			}
			for _, m := range msgs {
				text := m.PlainText()
				if !raw {
					text = util.TruncateRunes(text, 2000)
				}
				fmt.Printf("[%s]\n%s\n\n", m.Role, text)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&from, "provider", "", "source provider")
	cmd.Flags().IntVar(&limit, "limit", 0, "show last N messages")
	cmd.Flags().BoolVar(&raw, "raw", false, "do not truncate")
	cmd.Flags().BoolVar(&refresh, "refresh", false, "refresh index before show")
	return cmd
}

func (a *App) migrateCmd() *cobra.Command {
	var to, from, project string
	var dryRun, yes, refresh bool
	cmd := &cobra.Command{
		Use:   "migrate <session-id>",
		Short: "Migrate a session to another provider",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if to == "" {
				return fmt.Errorf("--to is required")
			}
			ctx := cmd.Context()
			if err := a.ensureIndex(ctx, from, refresh); err != nil {
				return err
			}
			if !yes && !dryRun {
				if !confirmAction(fmt.Sprintf("Migrate %s → %s? [y/N] ", args[0], to)) {
					fmt.Println("cancelled")
					return nil
				}
			}
			res, err := a.Migrate.Run(ctx, migrate.Options{
				SessionID: args[0], FromProvider: from, ToProvider: to,
				ProjectPath: project, DryRun: dryRun,
			})
			if err != nil {
				return err
			}
			if dryRun {
				if res.AlreadyExists {
					fmt.Printf("Dry run OK: already migrated to %s\n   Path: %s\n", res.TargetName, res.Write.StoragePath)
					return nil
				}
				fmt.Printf("Dry run OK: would write to %s\n", res.Write.StoragePath)
				return nil
			}
			if res.AlreadyExists {
				fmt.Printf("ℹ️  Already migrated to %s\n", res.TargetName)
			} else {
				fmt.Printf("✅ Migrated to %s\n", res.TargetName)
			}
			fmt.Printf("   Session: %s\n", res.Write.SessionID)
			fmt.Printf("   Path:    %s\n", res.Write.StoragePath)
			fmt.Printf("   Resume:  %s\n", res.Resume)
			return nil
		},
	}
	cmd.Flags().StringVar(&to, "to", "", "target provider (required)")
	cmd.Flags().StringVar(&from, "from", "", "source provider")
	cmd.Flags().StringVar(&project, "project", "", "target project path")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "validate without writing")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip confirmation")
	cmd.Flags().BoolVar(&refresh, "refresh", false, "refresh index before migrate")
	return cmd
}

func (a *App) indexCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "index", Short: "Manage session index"}
	var providerID string
	status := &cobra.Command{
		Use:   "status",
		Short: "Show index status",
		RunE: func(cmd *cobra.Command, args []string) error {
			counts, err := a.Index.CountByProvider()
			if err != nil {
				return err
			}
			last, _ := a.Index.GetMeta("last_update")
			rebuild, _ := a.Index.GetMeta("last_rebuild")
			fmt.Printf("Last update: %s\n", last)
			fmt.Printf("Last rebuild: %s\n", rebuild)
			for p, n := range counts {
				fmt.Printf("  %s: %d\n", p, n)
			}
			return nil
		},
	}
	rebuild := &cobra.Command{
		Use:   "rebuild",
		Short: "Rebuild session index",
		RunE: func(cmd *cobra.Command, args []string) error {
			n, err := index.Rebuild(cmd.Context(), a.Registry, a.Index, registry.NormalizeID(providerID))
			if err != nil {
				return err
			}
			fmt.Printf("Indexed %d sessions\n", n)
			return nil
		},
	}
	rebuild.Flags().StringVar(&providerID, "provider", "", "single provider")
	update := &cobra.Command{
		Use:   "update",
		Short: "Incremental index update",
		RunE: func(cmd *cobra.Command, args []string) error {
			n, err := index.UpdateIncremental(cmd.Context(), a.Registry, a.Index, registry.NormalizeID(providerID))
			if err != nil {
				return err
			}
			fmt.Printf("Updated %d sessions\n", n)
			return nil
		},
	}
	update.Flags().StringVar(&providerID, "provider", "", "single provider")
	cmd.AddCommand(status, rebuild, update)
	return cmd
}

func (a *App) providersCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "providers", Short: "List providers"}
	doctor := &cobra.Command{
		Use:   "doctor",
		Short: "Check provider paths",
		RunE: func(cmd *cobra.Command, args []string) error {
			for _, p := range a.Registry.All() {
				status := "missing"
				if p.Installed() {
					status = "ok"
				}
				fmt.Printf("%-14s %-12s %s\n", p.ID(), status, p.DisplayName())
				for _, ps := range p.DefaultPaths() {
					fmt.Printf("    %s: %s\n", ps.Label, util.TildePath(ps.Path))
				}
			}
			return nil
		},
	}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		counts, _ := a.Index.CountByProvider()
		for _, p := range a.Registry.All() {
			inst := "stub"
			if p.Installed() {
				inst = "installed"
			}
			fmt.Printf("%-14s %-10s %-8s sessions=%d\n", p.ID(), inst, p.DisplayName(), counts[p.ID()])
		}
		return nil
	}
	cmd.AddCommand(doctor)
	return cmd
}

func (a *App) exportCmd() *cobra.Command {
	var from, out string
	var refresh bool
	cmd := &cobra.Command{
		Use:   "export <session-id>",
		Short: "Export session to portable JSON",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if err := a.ensureIndex(ctx, from, refresh); err != nil {
				return err
			}
			sm, p, err := migrate.ResolveSession(ctx, a.Registry, a.Index, args[0], from)
			if err != nil {
				return err
			}
			conv, err := p.Load(ctx, provider.SessionRef{ID: sm.ID, StoragePath: sm.StoragePath, ProjectPath: sm.ProjectPath})
			if err != nil {
				return err
			}
			if out == "" {
				out = "session.agenthop.json"
			}
			f, err := os.Create(out)
			if err != nil {
				return err
			}
			defer f.Close()
			enc := json.NewEncoder(f)
			enc.SetIndent("", "  ")
			return enc.Encode(conv)
		},
	}
	cmd.Flags().StringVar(&from, "provider", "", "source provider")
	cmd.Flags().StringVarP(&out, "output", "o", "", "output file")
	cmd.Flags().BoolVar(&refresh, "refresh", false, "refresh index before export")
	return cmd
}

func (a *App) tuiCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "tui",
		Short: "Interactive terminal UI",
		RunE: func(cmd *cobra.Command, args []string) error {
			return tui.Run(a.Registry, a.Index, a.Migrate)
		},
	}
}

func confirmAction(prompt string) bool {
	fmt.Print(prompt)
	var answer string
	fmt.Scanln(&answer)
	return strings.ToLower(strings.TrimSpace(answer)) == "y"
}

