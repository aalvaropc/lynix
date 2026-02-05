package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/aalvaropc/lynix/internal/buildinfo"
	"github.com/aalvaropc/lynix/internal/infra/fsworkspace"
	"github.com/aalvaropc/lynix/internal/infra/logger"
	"github.com/aalvaropc/lynix/internal/infra/workspacefinder"
	"github.com/aalvaropc/lynix/internal/ui/tui"
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
		Short:        "Lynix — TUI-first API tool",
		SilenceUsage: true,
		RunE: func(_ *cobra.Command, _ []string) error {
			wd, err := os.Getwd()
			if err != nil {
				wd = "."
			}
			wd, _ = filepath.Abs(wd)

			finder := workspacefinder.NewFinder()

			logRoot := wd
			if root, ferr := finder.FindRoot(wd); ferr == nil && root != "" {
				logRoot = root
			}

			cleanup, _ := logger.Setup(logger.Config{
				Root:  logRoot,
				Debug: debug,
			})
			if cleanup != nil {
				defer func() { _ = cleanup() }()
			}

			deps := tui.Deps{
				WorkspaceLocator:     finder,
				WorkspaceInitializer: fsworkspace.NewInitializer(),
				Logger:               logger.L(),
				Debug:                debug,
			}

			return tui.Run(deps)
		},
	}

	cmd.PersistentFlags().BoolVar(&debug, "debug", false, "enable verbose logging to .lynix/logs/lynix.log")

	// Add CLI subcommands for headless usage
	cmd.AddCommand(versionCmd())
	cmd.AddCommand(initCmd())
	cmd.AddCommand(runCmd())
	cmd.AddCommand(collectionsCmd())
	cmd.AddCommand(envsCmd())

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
