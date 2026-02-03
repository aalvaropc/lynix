package cli

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"
)

func collectionsCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "collections",
		Short: "Manage collections in a workspace",
	}

	c.AddCommand(collectionsListCmd())
	return c
}

func collectionsListCmd() *cobra.Command {
	var workspace string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List collections",
		RunE: func(_ *cobra.Command, _ []string) error {
			ws, err := loadWorkspace(workspace)
			if err != nil {
				return err
			}

			refs, err := ws.collections.ListCollections(ws.root)
			if err != nil {
				return err
			}

			if len(refs) == 0 {
				fmt.Println("(no collections found)")
				return nil
			}

			fmt.Printf("Workspace: %s\n\n", ws.root)
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
