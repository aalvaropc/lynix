package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/aalvaropc/lynix/internal/domain"
	"github.com/aalvaropc/lynix/internal/infra/wiring"
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
	var insecure bool
	var noRedirects bool
	var dryRun bool
	var parallel bool
	var varFlags []string
	var quiet bool
	var noColor bool

	c := &cobra.Command{
		Use:   "run",
		Short: "Run a collection (functional checks) from a Lynix workspace",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := validateReportFlags(report, reportPath); err != nil {
				return err
			}

			cliVars, err := parseVarFlags(varFlags)
			if err != nil {
				return err
			}

			wiringOpts := wiring.Opts{
				Insecure:          insecure,
				NoFollowRedirects: noRedirects,
			}
			ws, err := loadWorkspaceOrStandalone(cmd.Flags().Changed("workspace"), workspace, wiringOpts)
			if err != nil {
				return err
			}

			if ws.cfg.Run.Insecure || insecure {
				fmt.Fprintln(os.Stderr, "Warning: TLS certificate verification is DISABLED for every request in this run")
			}

			collectionPath, err := resolveCollectionPath(ws, collection)
			if err != nil {
				return err
			}

			envArg, err := resolveEnvironmentArg(ws, env)
			if err != nil {
				return err
			}

			// Feed known secret values (secrets file + sensitive env vars +
			// sensitive --var overrides) to the redactor so they are
			// scrubbed from every output surface.
			if ws.redactor != nil {
				if secretsEnv, envErr := ws.envs.LoadEnvironment(envArg); envErr == nil {
					ws.redactor.AddSecretsFromEnv(secretsEnv)
				}
				ws.redactor.AddSecretsFromVars(cliVars)
			}

			var store = ws.store
			if noSave || dryRun {
				store = nil
			}

			retryOpts := usecase.RunOpts{
				FailFast:   failFast,
				Only:       splitCSV(only),
				Tags:       splitCSV(tags),
				Retries:    ws.cfg.Run.Retries,
				RetryDelay: ws.cfg.Run.RetryDelay,
				Retry5xx:   ws.cfg.Run.Retry5xx,
				DryRun:     dryRun,
				Parallel:   parallel,
				Vars:       cliVars,
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

			// run.timeout_seconds bounds the whole run (parity with the
			// documented behavior; it was previously ignored by the CLI).
			ctx := cmd.Context()
			if ws.cfg.Run.Timeout > 0 {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, ws.cfg.Run.Timeout)
				defer cancel()
			}

			pretty := prettyOpts{
				quiet:  quiet,
				colors: newPalette(colorsEnabled(noColor, os.Stdout)),
			}

			run, runID, err := uc.Execute(ctx, collectionPath, envArg)
			if err != nil {
				if dryRun {
					_ = printDryRun(os.Stdout, run)
				} else {
					_ = printRun(os.Stdout, run, runID, format, pretty)
				}
				return err
			}

			if dryRun {
				return printDryRun(os.Stdout, run)
			}

			if ws.cfg.Masking.MaskCLIOutput && ws.redactor != nil {
				run = ws.redactor.Redact(run)
			}

			if ws.cfg.Masking.FailOnDetectedSecret && ws.redactor != nil {
				if err := ws.redactor.CheckForSecrets(run); err != nil {
					return err
				}
			}

			if err := printRun(os.Stdout, run, runID, format, pretty); err != nil {
				return err
			}

			// When stdout is redirected (e.g. `> report.txt`), echo the
			// one-line summary to stderr so the terminal still shows it.
			if !isTerminal(os.Stdout) && format != "json" {
				total := run.EndedAt.Sub(run.StartedAt)
				fmt.Fprint(os.Stderr, summaryLine(run, total, palette{}))
			}

			if report == "junit" {
				if err := writeJUnitReport(reportPath, run, runID); err != nil {
					return err
				}
			}

			fails := countFailures(run)
			if fails > 0 {
				// Execution errors (network, timeouts) outrank assertion
				// failures: "couldn't reach the API" is a different incident
				// than "the API misbehaved".
				code := exitAssertFailed
				for _, rr := range run.Results {
					if rr.Error != nil {
						code = exitExecution
						break
					}
				}
				return &codedError{code: code, err: fmt.Errorf("run failed (%d failed request(s))", fails)}
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
	c.Flags().IntVar(&retries, "retries", 0, "Number of retries for transient errors (default: run.retries from lynix.yaml)")
	c.Flags().IntVar(&retryDelayMS, "retry-delay", 0, "Delay between retries in milliseconds (default: run.retry_delay_ms from lynix.yaml)")
	c.Flags().BoolVar(&retry5xx, "retry-5xx", false, "Retry on HTTP 5xx responses")
	c.Flags().BoolVar(&insecure, "insecure", false, "Skip TLS certificate verification")
	c.Flags().BoolVar(&noRedirects, "no-redirects", false, "Do not follow HTTP redirects")
	c.Flags().BoolVar(&dryRun, "dry-run", false, "Resolve variables and show requests without executing")
	c.Flags().BoolVar(&parallel, "parallel", false, "Execute independent requests in parallel")
	c.Flags().StringArrayVar(&varFlags, "var", nil, "Override a variable (key=value, repeatable; wins over env and collection vars)")
	c.Flags().BoolVarP(&quiet, "quiet", "q", false, "Show only failed requests in pretty output")
	c.Flags().BoolVar(&noColor, "no-color", false, "Disable colored output (NO_COLOR is also honored)")

	if err := c.MarkFlagRequired("collection"); err != nil {
		panic(fmt.Sprintf("MarkFlagRequired: %v", err))
	}
	return c
}

func printDryRun(w io.Writer, run domain.RunResult) error {
	fmt.Fprintf(w, "Collection: %s\n", run.CollectionName)
	fmt.Fprintf(w, "Env:        %s\n", run.EnvironmentName)
	fmt.Fprintln(w)

	for _, r := range run.Results {
		fmt.Fprintf(w, "--- %s ---\n", r.Name)

		if r.Error != nil {
			fmt.Fprintf(w, "  error: %s\n\n", r.Error.Message)
			continue
		}

		fmt.Fprintf(w, "%s %s\n", r.Method, r.ResolvedURL)

		if len(r.RequestHeaders) > 0 {
			fmt.Fprintln(w, "Headers:")
			keys := make([]string, 0, len(r.RequestHeaders))
			for k := range r.RequestHeaders {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				fmt.Fprintf(w, "  %s: %s\n", k, r.RequestHeaders[k])
			}
		}

		if len(r.RequestBody) > 0 {
			fmt.Fprintln(w, "Body:")
			var buf json.RawMessage
			if json.Unmarshal(r.RequestBody, &buf) == nil {
				var pretty []byte
				if p, err := json.MarshalIndent(buf, "  ", "  "); err == nil {
					pretty = p
				} else {
					pretty = r.RequestBody
				}
				fmt.Fprintf(w, "  %s\n", pretty)
			} else {
				fmt.Fprintf(w, "  %s\n", r.RequestBody)
			}
		}

		fmt.Fprintln(w)
	}

	resolved := 0
	for _, r := range run.Results {
		if r.Error == nil {
			resolved++
		}
	}
	fmt.Fprintln(w, "───────────────────────────────────")
	fmt.Fprintf(w, "Dry run: %d request(s) resolved", resolved)
	if errs := len(run.Results) - resolved; errs > 0 {
		fmt.Fprintf(w, ", %d error(s)", errs)
	}
	fmt.Fprintln(w)

	return nil
}

// prettyOpts controls the human-readable output.
type prettyOpts struct {
	quiet  bool // only failed requests
	colors palette
}

func printRun(w io.Writer, run domain.RunResult, runID string, format string, opts prettyOpts) error {
	switch format {
	case "json":
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")

		duration := run.EndedAt.Sub(run.StartedAt)
		if run.StartedAt.IsZero() || run.EndedAt.IsZero() {
			duration = 0
		}
		passed, failed, errs := summarizeResults(run)

		payload := map[string]any{
			"run_id": runID,
			"run":    run,
			"summary": map[string]any{
				"total":       len(run.Results),
				"passed":      passed,
				"failed":      failed,
				"errors":      errs,
				"duration_ms": duration.Milliseconds(),
			},
		}
		return enc.Encode(payload)
	case "pretty", "":
		printPrettyRun(w, run, runID, opts)
		return nil
	default:
		return fmt.Errorf("unsupported format %q (expected pretty|json)", format)
	}
}

const maxMessageLen = 300
const maxBodyExcerptLen = 200

func printPrettyRun(w io.Writer, run domain.RunResult, runID string, opts prettyOpts) {
	c := opts.colors
	total := run.EndedAt.Sub(run.StartedAt)
	if run.StartedAt.IsZero() || run.EndedAt.IsZero() {
		total = 0
	}

	fmt.Fprintf(w, "Collection: %s\n", run.CollectionName)
	fmt.Fprintf(w, "Env:        %s\n", run.EnvironmentName)
	fmt.Fprintf(w, "Duration:   %s\n", total)
	if runID != "" {
		fmt.Fprintf(w, "Run ID:     %s\n", runID)
	}
	fmt.Fprintln(w)

	for _, r := range run.Results {
		failed := isRequestFailed(r)
		if opts.quiet && !failed {
			continue
		}

		status := c.green + "OK" + c.reset
		if failed {
			status = c.red + c.bold + "FAIL" + c.reset
		}

		fmt.Fprintf(w, "- [%s] %s (%s) %dms\n", status, r.Name, r.Method, r.LatencyMS)

		if r.Attempts > 1 {
			fmt.Fprintf(w, "  attempts: %d\n", r.Attempts)
		}

		if r.Error != nil {
			fmt.Fprintf(w, "  %serror:%s %s (%s)\n", c.red, c.reset, truncateMessage(r.Error.Message), r.Error.Kind)
		} else {
			fmt.Fprintf(w, "  status: %d\n", r.StatusCode)
		}

		// Detail lines only for failures — a 30-request run with 5 checks
		// each should not print 150 lines of passing noise.
		if len(r.Assertions) > 0 {
			pass, fail := countAssertionPassFail(r.Assertions)
			fmt.Fprintf(w, "  assertions: %d pass / %d fail\n", pass, fail)
			for _, a := range r.Assertions {
				if a.Passed {
					continue
				}
				fmt.Fprintf(w, "    %s✗ %s — %s%s\n", c.red, a.Name, truncateMessage(a.Message), c.reset)
			}
		}

		if len(r.Extracts) > 0 {
			ok, bad := countExtractPassFail(r.Extracts)
			if bad > 0 {
				fmt.Fprintf(w, "  extracts: %d ok / %d fail\n", ok, bad)
				for _, e := range r.Extracts {
					if e.Success {
						continue
					}
					fmt.Fprintf(w, "    %s✗ %s — %s%s\n", c.red, e.Name, truncateMessage(e.Message), c.reset)
				}
			}
		}

		// A failing request prints a response excerpt: the assertion message
		// alone rarely explains what the server actually said.
		if failed && r.Error == nil && len(r.Response.Body) > 0 {
			fmt.Fprintf(w, "  %sresponse:%s %s\n", c.dim, c.reset, excerpt(r.Response.Body, maxBodyExcerptLen))
		}

		if len(r.Extracted) > 0 && !opts.quiet {
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

	passed, failed, errs := summarizeResults(run)
	if opts.quiet && failed == 0 && errs == 0 {
		fmt.Fprintf(w, "%sAll %d request(s) passed.%s (%s)\n", c.green, passed, c.reset, total)
		return
	}
	fmt.Fprintln(w, "───────────────────────────────────")
	fmt.Fprint(w, summaryLine(run, total, c))
}

func summaryLine(run domain.RunResult, total time.Duration, c palette) string {
	passed, failed, errs := summarizeResults(run)
	fc := ""
	if failed > 0 || errs > 0 {
		fc = c.red
	} else {
		fc = c.green
	}
	plural := "s"
	if errs == 1 {
		plural = ""
	}
	return fmt.Sprintf("%sResults: %d passed, %d failed, %d error%s%s (%s)\n",
		fc, passed, failed, errs, plural, c.reset, total)
}

func truncateMessage(s string) string {
	return excerptString(s, maxMessageLen)
}

func excerpt(b []byte, maxRunes int) string {
	return excerptString(string(b), maxRunes)
}

func excerptString(s string, maxRunes int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	return string(runes[:maxRunes]) + "…"
}

func summarizeResults(run domain.RunResult) (passed, failed, errors int) {
	for _, r := range run.Results {
		switch {
		case r.Error != nil:
			errors++
		case r.Failed():
			failed++
		default:
			passed++
		}
	}
	return
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
