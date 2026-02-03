package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/aalvaropc/lynix/internal/domain"
	"github.com/aalvaropc/lynix/internal/usecase"
	"github.com/spf13/cobra"
)

func runCmd() *cobra.Command {
	var workspace string
	var collection string
	var env string
	var noSave bool
	var format string

	c := &cobra.Command{
		Use:   "run",
		Short: "Run a collection (functional checks) from a Lynix workspace",
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

			var store = ws.store
			if noSave {
				store = nil
			}

			uc := usecase.NewRunCollection(ws.collections, ws.envs, ws.runner, store)

			run, runID, err := uc.Execute(cmd.Context(), collectionPath, envArg)
			if err != nil {
				// If save fails, we still may want to print something when format=json/pretty.
				// Here we print what we can and return error.
				_ = printRun(os.Stdout, run, runID, format)
				return err
			}

			if err := printRun(os.Stdout, run, runID, format); err != nil {
				return err
			}

			fails := countFailures(run)
			if fails > 0 {
				return fmt.Errorf("run failed (%d failed request(s))", fails)
			}
			return nil
		},
	}

	c.Flags().StringVarP(&workspace, "workspace", "w", "", "Workspace root (optional; autodetected if omitted)")
	c.Flags().StringVarP(&collection, "collection", "c", "", "Collection name or path (required)")
	c.Flags().StringVarP(&env, "env", "e", "", "Environment name or path (optional; defaults to workspace default env)")
	c.Flags().BoolVar(&noSave, "no-save", false, "Do not save run artifact under runs/")
	c.Flags().StringVar(&format, "format", "pretty", "Output format: pretty|json")

	_ = c.MarkFlagRequired("collection")
	return c
}

func printRun(w io.Writer, run domain.RunResult, runID string, format string) error {
	switch format {
	case "json":
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		// Include runID (optional) as a wrapper to avoid changing domain model.
		payload := map[string]any{
			"run_id": runID,
			"run":    run,
		}
		return enc.Encode(payload)
	case "pretty", "":
		printPrettyRun(w, run, runID)
		return nil
	default:
		return fmt.Errorf("unsupported format %q (expected pretty|json)", format)
	}
}

func printPrettyRun(w io.Writer, run domain.RunResult, runID string) {
	total := run.EndedAt.Sub(run.StartedAt)
	if run.StartedAt.IsZero() || run.EndedAt.IsZero() {
		total = 0
	}

	fmt.Fprintf(w, "Collection: %s\n", run.CollectionName)
	fmt.Fprintf(w, "Env:        %s\n", run.EnvironmentName)
	fmt.Fprintf(w, "Started:    %s\n", run.StartedAt.Format(time.RFC3339))
	fmt.Fprintf(w, "Ended:      %s\n", run.EndedAt.Format(time.RFC3339))
	fmt.Fprintf(w, "Duration:   %s\n", total)
	if runID != "" {
		fmt.Fprintf(w, "Run ID:     %s\n", runID)
	}
	fmt.Fprintln(w)

	for _, r := range run.Results {
		status := "OK"
		if isRequestFailed(r) {
			status = "FAIL"
		}

		fmt.Fprintf(w, "- [%s] %s (%s) %dms\n", status, r.Name, r.Method, r.LatencyMS)

		if r.Error != nil {
			fmt.Fprintf(w, "  error: %s (%s)\n", r.Error.Message, r.Error.Kind)
		} else {
			fmt.Fprintf(w, "  status: %d\n", r.StatusCode)
		}

		if len(r.Assertions) > 0 {
			pass, fail := countAssertionPassFail(r.Assertions)
			fmt.Fprintf(w, "  assertions: %d pass / %d fail\n", pass, fail)
			for _, a := range r.Assertions {
				mark := "✓"
				if !a.Passed {
					mark = "✗"
				}
				fmt.Fprintf(w, "    %s %s — %s\n", mark, a.Name, a.Message)
			}
		}

		if len(r.Extracts) > 0 {
			ok, bad := countExtractPassFail(r.Extracts)
			fmt.Fprintf(w, "  extracts: %d ok / %d fail\n", ok, bad)
			for _, e := range r.Extracts {
				mark := "✓"
				if !e.Success {
					mark = "✗"
				}
				fmt.Fprintf(w, "    %s %s — %s\n", mark, e.Name, e.Message)
			}
		}

		if len(r.Extracted) > 0 {
			fmt.Fprintf(w, "  extracted vars:\n")
			for k, v := range r.Extracted {
				fmt.Fprintf(w, "    - %s = %s\n", k, v)
			}
		}

		fmt.Fprintln(w)
	}
}

func countFailures(run domain.RunResult) int {
	n := 0
	for _, r := range run.Results {
		if isRequestFailed(r) {
			n++
		}
	}
	return n
}

func isRequestFailed(r domain.RequestResult) bool {
	if r.Error != nil {
		return true
	}
	for _, a := range r.Assertions {
		if !a.Passed {
			return true
		}
	}
	for _, e := range r.Extracts {
		if !e.Success {
			return true
		}
	}
	return false
}

func countAssertionPassFail(in []domain.AssertionResult) (pass int, fail int) {
	for _, a := range in {
		if a.Passed {
			pass++
		} else {
			fail++
		}
	}
	return pass, fail
}

func countExtractPassFail(in []domain.ExtractResult) (ok int, bad int) {
	for _, e := range in {
		if e.Success {
			ok++
		} else {
			bad++
		}
	}
	return ok, bad
}
