package cli

import (
	"fmt"

	"github.com/aalvaropc/lynix/internal/usecase"
	"github.com/spf13/cobra"
)

func validateCmd() *cobra.Command {
	var workspace string
	var collection string
	var env string

	c := &cobra.Command{
		Use:   "validate",
		Short: "Validate a collection and environment (no HTTP)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ws, err := loadWorkspace(workspace)
			if err != nil {
				return err
			}

			collectionPath, err := resolveCollectionPath(ws, collection)
			if err != nil {
				return err
			}

			envArg, err := resolveEnvironmentArg(ws, env)
			if err != nil {
				return err
			}

			uc := usecase.NewValidateCollection(ws.collections, ws.envs)
			if err := uc.Execute(cmd.Context(), collectionPath, envArg); err != nil {
				return err
			}

			fmt.Println("OK")
			return nil
		},
	}

	c.Flags().StringVarP(&workspace, "workspace", "w", "", "Workspace root (optional; autodetected if omitted)")
	c.Flags().StringVarP(&collection, "collection", "c", "", "Collection name or path (required)")
	c.Flags().StringVarP(&env, "env", "e", "", "Environment name or path (optional; defaults to workspace default env)")

	_ = c.MarkFlagRequired("collection")
	return c
}
