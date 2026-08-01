package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/aalvaropc/lynix/internal/domain"
	"github.com/aalvaropc/lynix/internal/infra/wiring"
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
	var format string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List collections",
		RunE: func(_ *cobra.Command, _ []string) error {
			if err := validateListFormat(format); err != nil {
				return err
			}

			ws, err := loadWorkspace(workspace, wiring.Opts{})
			if err != nil {
				return err
			}

			refs, err := ws.collections.ListCollections(ws.root)
			if err != nil {
				return err
			}

			if format == "json" {
				return printJSONList(os.Stdout, refListJSON(ws.root, refs))
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
	cmd.Flags().StringVar(&format, "format", "pretty", "Output format: pretty|json")
	return cmd
}

type listEntryJSON struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

func refListJSON(root string, refs []domain.CollectionRef) []listEntryJSON {
	out := make([]listEntryJSON, 0, len(refs))
	for _, r := range refs {
		rel, err := filepath.Rel(root, r.Path)
		if err != nil {
			rel = r.Path
		}
		out = append(out, listEntryJSON{Name: r.Name, Path: rel})
	}
	return out
}

func printJSONList(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
