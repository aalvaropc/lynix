package cli

import (
	"fmt"
	"io"
	"os"
	"sort"
	"text/tabwriter"
	"time"

	"github.com/aalvaropc/lynix/internal/domain"
	"github.com/aalvaropc/lynix/internal/infra/runstore"
	"github.com/aalvaropc/lynix/internal/infra/wiring"
	"github.com/spf13/cobra"
)

func runsCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "runs",
		Short: "Inspect saved run artifacts",
	}

	c.AddCommand(runsListCmd())
	c.AddCommand(runsShowCmd())
	c.AddCommand(runsDiffCmd())
	return c
}

func runsStore(workspaceFlag string) (*runstore.JSONStore, error) {
	ws, err := loadWorkspace(workspaceFlag, wiring.Opts{})
	if err != nil {
		return nil, err
	}
	return runstore.NewJSONStore(ws.root, ws.cfg), nil
}

func runsListCmd() *cobra.Command {
	var workspace string
	var format string
	var limit int

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List saved runs (newest first)",
		RunE: func(_ *cobra.Command, _ []string) error {
			if err := validateListFormat(format); err != nil {
				return err
			}

			store, err := runsStore(workspace)
			if err != nil {
				return err
			}

			summaries, err := store.ListRuns()
			if err != nil {
				return err
			}
			if limit > 0 && len(summaries) > limit {
				summaries = summaries[:limit]
			}

			if format == "json" {
				return printJSONList(os.Stdout, summaries)
			}

			if len(summaries) == 0 {
				fmt.Println("(no saved runs)")
				return nil
			}

			tw := tabwriter.NewWriter(os.Stdout, 2, 4, 2, ' ', 0)
			fmt.Fprintln(tw, "ID\tCOLLECTION\tENV\tSTARTED\tRESULT")
			for _, s := range summaries {
				result := fmt.Sprintf("%d passed", s.Passed)
				if s.Failed > 0 {
					result += fmt.Sprintf(", %d failed", s.Failed)
				}
				if s.Errors > 0 {
					result += fmt.Sprintf(", %d errors", s.Errors)
				}
				started := ""
				if !s.StartedAt.IsZero() {
					started = s.StartedAt.Local().Format(time.RFC3339)
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", s.ID, s.Collection, s.Env, started, result)
			}
			return tw.Flush()
		},
	}

	cmd.Flags().StringVarP(&workspace, "workspace", "w", "", "Workspace root (optional; autodetected if omitted)")
	cmd.Flags().StringVar(&format, "format", "pretty", "Output format: pretty|json")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum runs to list (0 = all)")
	return cmd
}

func runsShowCmd() *cobra.Command {
	var workspace string
	var format string
	var noColor bool

	cmd := &cobra.Command{
		Use:   "show <run-id>",
		Short: "Show a saved run",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			store, err := runsStore(workspace)
			if err != nil {
				return err
			}

			run, err := store.LoadRun(args[0])
			if err != nil {
				return err
			}

			pretty := prettyOpts{colors: newPalette(colorsEnabled(noColor, os.Stdout))}
			return printRun(os.Stdout, run, args[0], format, pretty)
		},
	}

	cmd.Flags().StringVarP(&workspace, "workspace", "w", "", "Workspace root (optional; autodetected if omitted)")
	cmd.Flags().StringVar(&format, "format", "pretty", "Output format: pretty|json")
	cmd.Flags().BoolVar(&noColor, "no-color", false, "Disable colored output")
	return cmd
}

func runsDiffCmd() *cobra.Command {
	var workspace string
	var noColor bool

	cmd := &cobra.Command{
		Use:   "diff <run-id-a> <run-id-b>",
		Short: "Compare two saved runs (status, latency, assertions)",
		Args:  cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			store, err := runsStore(workspace)
			if err != nil {
				return err
			}

			runA, err := store.LoadRun(args[0])
			if err != nil {
				return err
			}
			runB, err := store.LoadRun(args[1])
			if err != nil {
				return err
			}

			c := newPalette(colorsEnabled(noColor, os.Stdout))
			printRunDiff(os.Stdout, args[0], args[1], runA, runB, c)
			return nil
		},
	}

	cmd.Flags().StringVarP(&workspace, "workspace", "w", "", "Workspace root (optional; autodetected if omitted)")
	cmd.Flags().BoolVar(&noColor, "no-color", false, "Disable colored output")
	return cmd
}

// printRunDiff compares two runs request-by-request (matched by name):
// status changes, latency deltas, and assertion regressions/recoveries.
func printRunDiff(w io.Writer, idA, idB string, a, b domain.RunResult, c palette) {
	fmt.Fprintf(w, "Comparing %s → %s\n\n", idA, idB)

	byName := func(run domain.RunResult) map[string]domain.RequestResult {
		out := make(map[string]domain.RequestResult, len(run.Results))
		for _, r := range run.Results {
			out[r.Name] = r
		}
		return out
	}
	mapA, mapB := byName(a), byName(b)

	names := make([]string, 0, len(mapA)+len(mapB))
	seen := map[string]bool{}
	for _, run := range []domain.RunResult{a, b} {
		for _, r := range run.Results {
			if !seen[r.Name] {
				seen[r.Name] = true
				names = append(names, r.Name)
			}
		}
	}
	sort.Strings(names)

	unchanged := 0
	for _, name := range names {
		ra, inA := mapA[name]
		rb, inB := mapB[name]

		switch {
		case !inA:
			fmt.Fprintf(w, "%s+ %s%s (only in %s)\n", c.green, name, c.reset, idB)
		case !inB:
			fmt.Fprintf(w, "%s- %s%s (only in %s)\n", c.red, name, c.reset, idA)
		default:
			changes := diffRequest(ra, rb)
			if len(changes) == 0 {
				unchanged++
				continue
			}
			fmt.Fprintf(w, "%s~ %s%s\n", c.yellow, name, c.reset)
			for _, ch := range changes {
				fmt.Fprintf(w, "    %s\n", ch)
			}
		}
	}

	fmt.Fprintf(w, "\n%d request(s) unchanged\n", unchanged)
}

func diffRequest(a, b domain.RequestResult) []string {
	var out []string

	if a.StatusCode != b.StatusCode {
		out = append(out, fmt.Sprintf("status: %d → %d", a.StatusCode, b.StatusCode))
	}

	// Compare latency only when both requests actually completed: errored
	// requests report 0ms, which would produce meaningless deltas — but
	// legitimate sub-millisecond responses must still compare.
	if delta := b.LatencyMS - a.LatencyMS; delta != 0 && a.Error == nil && b.Error == nil {
		sign := "+"
		if delta < 0 {
			sign = ""
		}
		out = append(out, fmt.Sprintf("latency: %dms → %dms (%s%dms)", a.LatencyMS, b.LatencyMS, sign, delta))
	}

	passA, failA := countAssertionPassFail(a.Assertions)
	passB, failB := countAssertionPassFail(b.Assertions)
	if failA != failB || passA != passB {
		out = append(out, fmt.Sprintf("assertions: %d pass / %d fail → %d pass / %d fail", passA, failA, passB, failB))
		failedIn := func(rs []domain.AssertionResult) map[string]string {
			m := map[string]string{}
			for _, r := range rs {
				if !r.Passed {
					m[r.Name] = r.Message
				}
			}
			return m
		}
		fa, fb := failedIn(a.Assertions), failedIn(b.Assertions)
		for name, msg := range fb {
			if _, was := fa[name]; !was {
				out = append(out, fmt.Sprintf("regressed: %s — %s", name, truncateMessage(msg)))
			}
		}
		for name := range fa {
			if _, still := fb[name]; !still {
				out = append(out, fmt.Sprintf("recovered: %s", name))
			}
		}
	}

	errMsg := func(e *domain.RunError) string {
		if e == nil {
			return ""
		}
		return string(e.Kind)
	}
	if errMsg(a.Error) != errMsg(b.Error) {
		out = append(out, fmt.Sprintf("error: %q → %q", errMsg(a.Error), errMsg(b.Error)))
	}

	return out
}
