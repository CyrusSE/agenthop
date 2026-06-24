package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/CyrusSE/agenthop/internal/index"
	"github.com/CyrusSE/agenthop/internal/model"
	"github.com/CyrusSE/agenthop/internal/provider"
	"github.com/CyrusSE/agenthop/internal/registry"
	"github.com/spf13/cobra"
)

func (a *App) importCmd() *cobra.Command {
	var to, project string
	var dryRun, yes bool
	cmd := &cobra.Command{
		Use:   "import <session.agenthop.json>",
		Short: "Import portable JSON session into a provider",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if to == "" {
				return fmt.Errorf("--to is required")
			}
			f, err := os.Open(args[0])
			if err != nil {
				return err
			}
			defer f.Close()
			var conv model.Conversation
			if err := json.NewDecoder(f).Decode(&conv); err != nil {
				return fmt.Errorf("decode: %w", err)
			}
			dst, err := a.Registry.Get(registry.NormalizeID(to))
			if err != nil {
				return err
			}
			if !dst.Installed() {
				return provider.ErrNotInstalled
			}
			if !yes && !dryRun {
				fmt.Printf("Import %d messages → %s? [y/N] ", len(conv.Messages), to)
				var answer string
				fmt.Scanln(&answer)
				if answer != "y" && answer != "Y" {
					fmt.Println("cancelled")
					return nil
				}
			}
			write, err := dst.Write(cmd.Context(), &conv, provider.WriteOpts{
				ProjectPath: project,
				DryRun:      dryRun,
			})
			if err != nil {
				return err
			}
			if dryRun {
				fmt.Printf("Dry run OK: would write to %s\n", write.StoragePath)
				return nil
			}
			_, _ = index.UpdateIncremental(context.Background(), a.Registry, a.Index, dst.ID())
			fmt.Printf("✅ Imported to %s\n   Session: %s\n   Resume:  %s\n", dst.DisplayName(), write.SessionID, dst.ResumeCommand(*write))
			return nil
		},
	}
	cmd.Flags().StringVar(&to, "to", "", "target provider (required)")
	cmd.Flags().StringVar(&project, "project", "", "target project path")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "validate without writing")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip confirmation")
	return cmd
}
