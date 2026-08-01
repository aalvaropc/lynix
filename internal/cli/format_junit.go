package cli

import (
	"encoding/xml"
	"fmt"
	"io"
	"strings"

	"github.com/aalvaropc/lynix/internal/domain"
)

// JUnit XML structs follow the Jenkins-compatible schema (most widely supported).

type junitTestSuites struct {
	XMLName    xml.Name         `xml:"testsuites"`
	Tests      int              `xml:"tests,attr"`
	Failures   int              `xml:"failures,attr"`
	Errors     int              `xml:"errors,attr"`
	Time       string           `xml:"time,attr"`
	TestSuites []junitTestSuite `xml:"testsuite"`
}

type junitTestSuite struct {
	XMLName   xml.Name        `xml:"testsuite"`
	Name      string          `xml:"name,attr"`
	Tests     int             `xml:"tests,attr"`
	Failures  int             `xml:"failures,attr"`
	Errors    int             `xml:"errors,attr"`
	Time      string          `xml:"time,attr"`
	Timestamp string          `xml:"timestamp,attr,omitempty"`
	ID        string          `xml:"id,attr,omitempty"`
	TestCases []junitTestCase `xml:"testcase"`
}

type junitTestCase struct {
	XMLName   xml.Name      `xml:"testcase"`
	Name      string        `xml:"name,attr"`
	Classname string        `xml:"classname,attr"`
	Time      string        `xml:"time,attr"`
	Failures  []junitDetail `xml:"failure,omitempty"`
	Errors    []junitDetail `xml:"error,omitempty"`
	SystemOut string        `xml:"system-out,omitempty"`
}

type junitDetail struct {
	Message string `xml:"message,attr"`
	Type    string `xml:"type,attr"`
	Body    string `xml:",chardata"`
}

func formatJUnit(w io.Writer, run domain.RunResult, runID string) error {
	duration := run.EndedAt.Sub(run.StartedAt)
	if run.StartedAt.IsZero() || run.EndedAt.IsZero() {
		duration = 0
	}

	var totalFailures, totalErrors int
	cases := make([]junitTestCase, 0, len(run.Results))

	for _, r := range run.Results {
		tc := junitTestCase{
			Name:      r.Name,
			Classname: run.CollectionName,
			Time:      fmt.Sprintf("%.3f", float64(r.LatencyMS)/1000),
		}

		var failMsgs []string
		for _, a := range r.Assertions {
			if !a.Passed {
				failMsgs = append(failMsgs, fmt.Sprintf("[%s] %s", a.Name, a.Message))
			}
		}
		for _, e := range r.Extracts {
			if !e.Success {
				failMsgs = append(failMsgs, fmt.Sprintf("[extract:%s] %s", e.Name, e.Message))
			}
		}

		// A case is either an error OR a failure, never both: some CI
		// parsers reject reports where failures+errors exceeds tests.
		switch {
		case r.Error != nil:
			totalErrors++
			body := r.Error.Message
			if len(failMsgs) > 0 {
				body += "\n" + strings.Join(failMsgs, "\n")
			}
			tc.Errors = append(tc.Errors, junitDetail{
				Message: r.Error.Message,
				Type:    string(r.Error.Kind),
				Body:    body,
			})
		case len(failMsgs) > 0:
			totalFailures++
			tc.Failures = append(tc.Failures, junitDetail{
				Message: fmt.Sprintf("%d assertion(s) failed", len(failMsgs)),
				Type:    "assertion",
				Body:    strings.Join(failMsgs, "\n"),
			})
		}

		// Response context for diagnosing failures straight from the XML.
		if (r.Error != nil || len(failMsgs) > 0) && len(r.Response.Body) > 0 {
			tc.SystemOut = fmt.Sprintf("status: %d\nresponse (excerpt): %s",
				r.StatusCode, excerpt(r.Response.Body, 2000))
		}

		cases = append(cases, tc)
	}

	ts := ""
	if !run.StartedAt.IsZero() {
		ts = run.StartedAt.UTC().Format("2006-01-02T15:04:05")
	}

	suite := junitTestSuite{
		Name:      run.CollectionName,
		Tests:     len(run.Results),
		Failures:  totalFailures,
		Errors:    totalErrors,
		Time:      fmt.Sprintf("%.3f", duration.Seconds()),
		Timestamp: ts,
		ID:        runID,
		TestCases: cases,
	}

	root := junitTestSuites{
		Tests:      suite.Tests,
		Failures:   suite.Failures,
		Errors:     suite.Errors,
		Time:       suite.Time,
		TestSuites: []junitTestSuite{suite},
	}

	if _, err := fmt.Fprint(w, xml.Header); err != nil {
		return err
	}

	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	if err := enc.Encode(root); err != nil {
		return err
	}

	_, err := fmt.Fprintln(w)
	return err
}
