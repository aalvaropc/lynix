package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
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
	var report string
	var reportPath string
	var failFast bool
	var only string
	var tags string
	var retries int
	var retryDelayMS int
	var retry5xx bool

	c := &cobra.Command{
		Use:   "run",
		Short: "Run a collection (functional checks) from a Lynix workspace",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := validateReportFlags(report, reportPath); err != nil {
				return err
			}

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

			// Build retry options: CLI flags override workspace config when explicitly set.
			retryOpts := usecase.RunOpts{
				FailFast:   failFast,
				Only:       splitCSV(only),
				Tags:       splitCSV(tags),
				Retries:    ws.cfg.Run.Retries,
				RetryDelay: ws.cfg.Run.RetryDelay,
				Retry5xx:   ws.cfg.Run.Retry5xx,
			}
			if cmd.Flags().Changed("retries") {
				retryOpts.Retries = retries
			}
			if cmd.Flags().Changed("retry-delay") {
				retryOpts.RetryDelay = time.Duration(retryDelayMS) * time.Millisecond
			}
			if cmd.Flags().Changed("retry-5xx") {
				retryOpts.Retry5xx = retry5xx
			}

			uc := usecase.NewRunCollection(ws.collections, ws.envs, ws.runner, store, retryOpts)

			run, runID, err := uc.Execute(cmd.Context(), collectionPath, envArg)
			if err != nil {
				_ = printRun(os.Stdout, run, runID, format)
				return err
			}

			if ws.cfg.Masking.MaskCLIOutput && ws.redactor != nil {
				run = ws.redactor.Redact(run)
			}

			if ws.cfg.Masking.FailOnDetectedSecret && ws.redactor != nil {
				if err := ws.redactor.CheckForSecrets(run); err != nil {
					return err
				}
			}

			if err := printRun(os.Stdout, run, runID, format); err != nil {
				return err
			}

			if report == "junit" {
				if err := writeJUnitReport(reportPath, run, runID); err != nil {
					return err
				}
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
	c.Flags().StringVar(&report, "report", "", "Report type to generate (currently only \"junit\")")
	c.Flags().StringVar(&reportPath, "report-path", "", "File path to write the report to")
	c.Flags().BoolVar(&failFast, "fail-fast", false, "Stop execution on the first failed request")
	c.Flags().StringVar(&only, "only", "", "Run only the named requests (comma-separated)")
	c.Flags().StringVar(&tags, "tags", "", "Run only requests matching any of these tags (comma-separated)")
	c.Flags().IntVar(&retries, "retries", 0, "Number of retries for transient errors (default 0)")
	c.Flags().IntVar(&retryDelayMS, "retry-delay", 0, "Delay between retries in milliseconds (default 0)")
	c.Flags().BoolVar(&retry5xx, "retry-5xx", false, "Retry on HTTP 5xx responses")

	if err := c.MarkFlagRequired("collection"); err != nil {
		panic(fmt.Sprintf("MarkFlagRequired: %v", err))
	}
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

		if r.Attempts > 1 {
			fmt.Fprintf(w, "  attempts: %d\n", r.Attempts)
		}

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
			keys := make([]string, 0, len(r.Extracted))
			for k := range r.Extracted {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				fmt.Fprintf(w, "    - %s = %s\n", k, r.Extracted[k])
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
	return r.Failed()
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

func validateReportFlags(report, reportPath string) error {
	if report == "" && reportPath == "" {
		return nil
	}
	if report != "" && reportPath == "" {
		return fmt.Errorf("--report-path is required when --report is set")
	}
	if report == "" && reportPath != "" {
		return fmt.Errorf("--report is required when --report-path is set")
	}
	if report != "junit" {
		return fmt.Errorf("unsupported report type %q (expected \"junit\")", report)
	}
	return nil
}

func splitCSV(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	var out []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func writeJUnitReport(path string, run domain.RunResult, runID string) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create report file %q: %w", path, err)
	}
	defer f.Close()
	return formatJUnit(f, run, runID)
}
