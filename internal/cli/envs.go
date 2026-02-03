package cli

import (
	"fmt"
	"path/filepath"

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

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List environments",
		RunE: func(_ *cobra.Command, _ []string) error {
			ws, err := loadWorkspace(workspace)
			if err != nil {
				return err
			}

			refs, err := ws.envCatalog.ListEnvironments(ws.root)
			if err != nil {
				return err
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
	return cmd
}
