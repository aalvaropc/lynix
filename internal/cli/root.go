package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/aalvaropc/lynix/internal/buildinfo"
	"github.com/aalvaropc/lynix/internal/infra/fsworkspace"
	"github.com/aalvaropc/lynix/internal/usecase"
)

func Execute() {
	cmd := newRootCmd()
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	var debug bool

	cmd := &cobra.Command{
		Use:          "lynix",
		Short:        "Lynix — API testing for CI/CD pipelines",
		SilenceUsage: true,
	}

	cmd.PersistentFlags().BoolVar(&debug, "debug", false, "enable verbose logging to .lynix/logs/lynix.log")

	cmd.AddCommand(versionCmd())
	cmd.AddCommand(initCmd())
	cmd.AddCommand(runCmd())
	cmd.AddCommand(validateCmd())
	cmd.AddCommand(collectionsCmd())
	cmd.AddCommand(envsCmd())
	cmd.AddCommand(importCmd())

	return cmd
}

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print Lynix version info",
		Run: func(_ *cobra.Command, _ []string) {
			fmt.Println(buildinfo.String())
		},
	}
}

func initCmd() *cobra.Command {
	var path string
	var force bool

	c := &cobra.Command{
		Use:   "init",
		Short: "Create a Lynix workspace (collections, envs, templates)",
		RunE: func(_ *cobra.Command, _ []string) error {
			initializer := fsworkspace.NewInitializer()
			uc := usecase.NewInitWorkspace(initializer)
			return uc.Execute(path, force)
		},
	}

	c.Flags().StringVarP(&path, "path", "p", ".", "Target directory")
	c.Flags().BoolVar(&force, "force", false, "Overwrite existing files (where applicable)")
	return c
}
