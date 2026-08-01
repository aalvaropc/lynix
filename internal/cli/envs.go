package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/aalvaropc/lynix/internal/infra/wiring"
	"github.com/spf13/cobra"
)

func envsCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "envs",
		Short: "Manage environments in a workspace",
	}

	c.AddCommand(envsListCmd())
	return c
}

func envsListCmd() *cobra.Command {
	var workspace string
	var format string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List environments",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ws, err := loadWorkspace(workspace, wiring.Opts{})
			if err != nil {
				return err
			}

			refs, err := ws.envCatalog.ListEnvironments(cmd.Context(), ws.root)
			if err != nil {
				return err
			}

			if format == "json" {
				entries := make([]listEntryJSON, 0, len(refs))
				for _, r := range refs {
					rel, relErr := filepath.Rel(ws.root, r.Path)
					if relErr != nil {
						rel = r.Path
					}
					entries = append(entries, listEntryJSON{Name: r.Name, Path: rel})
				}
				return printJSONList(os.Stdout, map[string]any{
					"default":      ws.cfg.Defaults.Environment,
					"environments": entries,
				})
			}

			if len(refs) == 0 {
				fmt.Println("(no environments found)")
				return nil
			}

			fmt.Printf("Workspace: %s\n", ws.root)
			fmt.Printf("Default:   %s\n\n", ws.cfg.Defaults.Environment)

			for _, r := range refs {
				rel, _ := filepath.Rel(ws.root, r.Path)
				fmt.Printf("- %s  (%s)\n", r.Name, rel)
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&workspace, "workspace", "w", "", "Workspace root (optional; autodetected if omitted)")
	cmd.Flags().StringVar(&format, "format", "pretty", "Output format: pretty|json")
	return cmd
}
