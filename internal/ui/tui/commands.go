package tui

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/aalvaropc/lynix/internal/domain"
	"github.com/aalvaropc/lynix/internal/infra/wiring"
	"github.com/aalvaropc/lynix/internal/infra/workspacefinder"
	"github.com/aalvaropc/lynix/internal/infra/yamlcollection"
	"github.com/aalvaropc/lynix/internal/infra/yamlenv"
	"github.com/aalvaropc/lynix/internal/usecase"
)

func cmdRefreshWorkspace(deps Deps) tea.Cmd {
	return func() tea.Msg {
		wd, err := os.Getwd()
		if err != nil {
			return workspaceRefreshedMsg{cwd: "", found: false, err: fmt.Errorf("getwd: %w", err)}
		}
		if deps.WorkspaceLocator == nil {
			return workspaceRefreshedMsg{cwd: wd, found: false, err: errors.New("WorkspaceLocator is nil")}
		}

		root, findErr := deps.WorkspaceLocator.FindRoot(context.Background(), wd)
		if findErr != nil {
			return workspaceRefreshedMsg{cwd: wd, found: false, err: findErr}
		}

		return workspaceRefreshedMsg{cwd: wd, found: true, root: root, err: nil}
	}
}

func cmdInitWorkspaceHere(deps Deps, root string, force bool) tea.Cmd {
	return func() tea.Msg {
		if deps.WorkspaceInitializer == nil {
			return initWorkspaceDoneMsg{root: root, err: errors.New("WorkspaceInitializer is nil")}
		}

		err := deps.WorkspaceInitializer.Init(domain.WorkspaceSpec{Root: root}, force)
		return initWorkspaceDoneMsg{root: root, err: err}
	}
}

func cmdLoadCollections(root string) tea.Cmd {
	return func() tea.Msg {
		cfg, err := workspacefinder.LoadConfig(root)
		if err != nil {
			return collectionsLoadedMsg{root: root, err: err}
		}

		loader := yamlcollection.NewLoader(
			yamlcollection.WithCollectionsDir(cfg.Paths.CollectionsDir),
		)

		refs, err := loader.ListCollections(root)
		return collectionsLoadedMsg{root: root, refs: refs, err: err}
	}
}

func cmdLoadEnvironments(root string) tea.Cmd {
	return func() tea.Msg {
		cfg, err := workspacefinder.LoadConfig(root)
		if err != nil {
			return envsLoadedMsg{root: root, err: err}
		}

		loader := yamlenv.NewLoader(
			root,
			yamlenv.WithEnvDir(cfg.Paths.EnvironmentsDir),
		)

		refs, err := loader.ListEnvironments(context.Background(), root)
		return envsLoadedMsg{root: root, refs: refs, err: err}
	}
}

func cmdPreviewCollection(path string) tea.Cmd {
	return func() tea.Msg {
		p := filepath.Clean(path)

		loader := yamlcollection.NewLoader()
		col, err := loader.LoadCollection(p)
		if err != nil {
			return collectionPreviewMsg{path: p, preview: "", err: err}
		}

		var b strings.Builder
		b.WriteString("Collection: ")
		b.WriteString(col.Name)
		b.WriteString("\n\n")

		if len(col.Vars) > 0 {
			b.WriteString("Vars:\n")
			for k, v := range col.Vars {
				b.WriteString("  - ")
				b.WriteString(k)
				b.WriteString(" = ")
				b.WriteString(v)
				b.WriteString("\n")
			}
			b.WriteString("\n")
		}

		b.WriteString("Requests:\n")
		for _, r := range col.Requests {
			b.WriteString("  - ")
			b.WriteString(string(r.Method))
			b.WriteString("  ")
			b.WriteString(r.Name)
			b.WriteString("\n    ")
			b.WriteString(r.URL)
			b.WriteString("\n")
		}

		return collectionPreviewMsg{path: p, preview: b.String(), err: nil}
	}
}

func listenRunner(ch <-chan runnerDoneMsg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return runnerDoneMsg{err: errors.New("runner channel closed")}
		}
		return msg
	}
}

// runParams groups all parameters needed to start a collection run.
type runParams struct {
	workspaceRoot  string
	collectionPath string
	envName        string
	save           bool
	runOpts        usecase.RunOpts
	wiringOpts     wiring.Opts
	log            *slog.Logger
	debug          bool
}

func startRunAsync(p runParams) (chan runnerDoneMsg, context.CancelFunc, tea.Cmd) {
	ch := make(chan runnerDoneMsg, 1)

	log := p.log
	if log == nil {
		log = slog.Default()
	}

	baseCtx, baseCancel := context.WithCancel(context.Background())

	go func() {
		defer baseCancel()
		defer close(ch)

		log.Info("run.start",
			"workspace", p.workspaceRoot,
			"collection_path", p.collectionPath,
			"env", p.envName,
			"debug", p.debug,
		)

		cfg, err := workspacefinder.LoadConfig(p.workspaceRoot)
		if err != nil {
			log.Error("run.load_config.failed", "err", err)
			ch <- runnerDoneMsg{err: err}
			return
		}

		timeout := cfg.Run.Timeout
		if timeout <= 0 {
			timeout = 5 * time.Minute
		}
		ctx, cancel := context.WithTimeout(baseCtx, timeout)
		defer cancel()

		adapters := wiring.NewAdapters(p.workspaceRoot, cfg, p.save, p.wiringOpts)

		// Merge workspace config defaults with user-selected opts.
		opts := p.runOpts
		if opts.Retries == 0 {
			opts.Retries = cfg.Run.Retries
		}
		if opts.RetryDelay == 0 {
			opts.RetryDelay = cfg.Run.RetryDelay
		}
		uc := usecase.NewRunCollection(adapters.Collections, adapters.Envs, adapters.Runner, adapters.Store, opts)

		run, id, execErr := uc.Execute(ctx, p.collectionPath, p.envName)

		if execErr != nil {
			log.Error("run.failed", "err", execErr, "saved_id", id)
		} else {
			log.Info("run.ok", "saved_id", id)
		}

		for _, rr := range run.Results {
			if rr.Error != nil {
				log.Warn("request.error",
					"name", rr.Name,
					"method", string(rr.Method),
					"url", rr.URL,
					"kind", string(rr.Error.Kind),
					"message", rr.Error.Message,
					"status", rr.StatusCode,
					"latency_ms", rr.LatencyMS,
				)
			} else if p.debug {
				log.Debug("request.ok",
					"name", rr.Name,
					"method", string(rr.Method),
					"url", rr.URL,
					"status", rr.StatusCode,
					"latency_ms", rr.LatencyMS,
					"truncated", rr.Response.Truncated,
					"body_bytes", len(rr.Response.Body),
				)
			}
		}

		ch <- runnerDoneMsg{run: run, id: id, err: execErr}
	}()

	return ch, baseCancel, listenRunner(ch)
}
