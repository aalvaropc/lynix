package cli

import (
	"fmt"
	"os"

	"github.com/aalvaropc/lynix/internal/buildinfo"
	"github.com/aalvaropc/lynix/internal/infra/fsworkspace"
	"github.com/aalvaropc/lynix/internal/infra/workspacefinder"
	"github.com/aalvaropc/lynix/internal/ui/tui"
	"github.com/aalvaropc/lynix/internal/usecase"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "lynix",
	Short: "Lynix - TUI-first API tool for requests, checks, and performance",
	RunE: func(cmd *cobra.Command, args []string) error {
		locator := workspacefinder.NewFinder()
		return tui.Run(tui.Deps{WorkspaceLocator: locator})
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(versionCmd())
	rootCmd.AddCommand(initCmd())
}

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print Lynix version info",
		Run: func(cmd *cobra.Command, args []string) {
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
		RunE: func(cmd *cobra.Command, args []string) error {
			initializer := fsworkspace.NewInitializer()
			uc := usecase.NewInitWorkspace(initializer)
			return uc.Execute(path, force)
		},
	}

	c.Flags().StringVarP(&path, "path", "p", ".", "Target directory")
	c.Flags().BoolVar(&force, "force", false, "Overwrite existing files (where applicable)")
	return c
}
