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
	"github.com/aalvaropc/lynix/internal/infra/httpclient"
	"github.com/aalvaropc/lynix/internal/infra/httprunner"
	"github.com/aalvaropc/lynix/internal/infra/runstore"
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

		root, findErr := deps.WorkspaceLocator.FindRoot(wd)
		if findErr != nil {
			return workspaceRefreshedMsg{cwd: wd, found: false, err: findErr}
		}

		return workspaceRefreshedMsg{cwd: wd, found: true, root: root, err: nil}
	}
}

func cmdInitWorkspaceHere(deps Deps, root string) tea.Cmd {
	return func() tea.Msg {
		if deps.WorkspaceInitializer == nil {
			return initWorkspaceDoneMsg{root: root, err: errors.New("WorkspaceInitializer is nil")}
		}

		err := deps.WorkspaceInitializer.Init(domain.WorkspaceSpec{Root: root}, true)
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

		refs, err := loader.ListEnvironments(root)
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

func startRunAsync(
	workspaceRoot, collectionPath, envName string,
	log *slog.Logger,
	debug bool,
) (chan runnerDoneMsg, tea.Cmd) {
	ch := make(chan runnerDoneMsg, 1)

	if log == nil {
		log = slog.Default()
	}

	go func() {
		defer close(ch)

		log.Info("run.start",
			"workspace", workspaceRoot,
			"collection_path", collectionPath,
			"env", envName,
			"debug", debug,
		)

		cfg, err := workspacefinder.LoadConfig(workspaceRoot)
		if err != nil {
			log.Error("run.load_config.failed", "err", err)
			ch <- runnerDoneMsg{err: err}
			return
		}

		colLoader := yamlcollection.NewLoader(
			yamlcollection.WithCollectionsDir(cfg.Paths.CollectionsDir),
		)
		envLoader := yamlenv.NewLoader(
			workspaceRoot,
			yamlenv.WithEnvDir(cfg.Paths.EnvironmentsDir),
		)

		client := httpclient.New(httpclient.DefaultConfig())
		runner := httprunner.New(client)
		store := runstore.NewJSONStore(workspaceRoot, cfg, runstore.WithIndex(true))

		uc := usecase.NewRunCollection(colLoader, envLoader, runner, store)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		run, id, execErr := uc.Execute(ctx, collectionPath, envName)

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
			} else if debug {
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

	return ch, listenRunner(ch)
}
