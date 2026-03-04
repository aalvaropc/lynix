package cli

import (
	"bytes"
	"encoding/xml"
	"strings"
	"testing"
	"time"

	"github.com/aalvaropc/lynix/internal/domain"
)

func TestFormatJUnit_Roundtrip(t *testing.T) {
	now := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name           string
		run            domain.RunResult
		runID          string
		wantTests      int
		wantFailures   int
		wantErrors     int
		wantContains   []string
		wantNotContain []string
	}{
		{
			name: "empty run",
			run: domain.RunResult{
				CollectionName: "empty",
				StartedAt:      now,
				EndedAt:        now,
			},
			runID:        "",
			wantTests:    0,
			wantFailures: 0,
			wantErrors:   0,
		},
		{
			name: "all pass",
			run: domain.RunResult{
				CollectionName: "demo",
				StartedAt:      now,
				EndedAt:        now.Add(500 * time.Millisecond),
				Results: []domain.RequestResult{
					{
						Name:       "Get Users",
						Method:     domain.MethodGet,
						StatusCode: 200,
						LatencyMS:  120,
						Assertions: []domain.AssertionResult{
							{Name: "status", Passed: true, Message: "status 200"},
						},
					},
					{
						Name:       "Get User",
						Method:     domain.MethodGet,
						StatusCode: 200,
						LatencyMS:  80,
						Assertions: []domain.AssertionResult{
							{Name: "status", Passed: true, Message: "status 200"},
						},
					},
				},
			},
			runID:        "abc123",
			wantTests:    2,
			wantFailures: 0,
			wantErrors:   0,
			wantContains: []string{`name="Get Users"`, `name="Get User"`, `classname="demo"`, `id="abc123"`},
		},
		{
			name: "mixed failures",
			run: domain.RunResult{
				CollectionName: "api-tests",
				StartedAt:      now,
				EndedAt:        now.Add(1 * time.Second),
				Results: []domain.RequestResult{
					{
						Name:       "OK Request",
						Method:     domain.MethodGet,
						StatusCode: 200,
						LatencyMS:  50,
						Assertions: []domain.AssertionResult{
							{Name: "status", Passed: true, Message: "status 200"},
						},
					},
					{
						Name:       "Failed Request",
						Method:     domain.MethodPost,
						StatusCode: 500,
						LatencyMS:  200,
						Assertions: []domain.AssertionResult{
							{Name: "status", Passed: false, Message: "expected status 200, got 500"},
							{Name: "jsonpath.eq", Passed: false, Message: `jsonpath "$.id": expected "1", got "2"`},
						},
					},
				},
			},
			runID:        "run-1",
			wantTests:    2,
			wantFailures: 1,
			wantErrors:   0,
			wantContains: []string{`2 assertion(s) failed`, `expected status 200, got 500`},
		},
		{
			name: "runner error",
			run: domain.RunResult{
				CollectionName: "error-test",
				StartedAt:      now,
				EndedAt:        now.Add(100 * time.Millisecond),
				Results: []domain.RequestResult{
					{
						Name:      "DNS Fail",
						Method:    domain.MethodGet,
						LatencyMS: 0,
						Error: &domain.RunError{
							Kind:    domain.RunErrorDNS,
							Message: "no such host: api.invalid",
						},
					},
				},
			},
			runID:        "",
			wantTests:    1,
			wantFailures: 0,
			wantErrors:   1,
			wantContains: []string{`type="dns"`, `no such host`},
		},
		{
			name: "error and failures combined",
			run: domain.RunResult{
				CollectionName: "combo",
				StartedAt:      now,
				EndedAt:        now.Add(300 * time.Millisecond),
				Results: []domain.RequestResult{
					{
						Name:      "Conn Error",
						Method:    domain.MethodGet,
						LatencyMS: 0,
						Error: &domain.RunError{
							Kind:    domain.RunErrorConn,
							Message: "connection refused",
						},
					},
					{
						Name:       "Bad Status",
						Method:     domain.MethodGet,
						StatusCode: 404,
						LatencyMS:  100,
						Assertions: []domain.AssertionResult{
							{Name: "status", Passed: false, Message: "expected status 200, got 404"},
						},
					},
					{
						Name:       "OK",
						Method:     domain.MethodGet,
						StatusCode: 200,
						LatencyMS:  50,
					},
				},
			},
			runID:        "",
			wantTests:    3,
			wantFailures: 1,
			wantErrors:   1,
		},
		{
			name: "extract failure counts as assertion failure",
			run: domain.RunResult{
				CollectionName: "extract-fail",
				StartedAt:      now,
				EndedAt:        now.Add(50 * time.Millisecond),
				Results: []domain.RequestResult{
					{
						Name:       "Extract Fail",
						Method:     domain.MethodGet,
						StatusCode: 200,
						LatencyMS:  30,
						Extracts: []domain.ExtractResult{
							{Name: "token", Success: false, Message: "jsonpath $.token: not found"},
						},
					},
				},
			},
			runID:        "",
			wantTests:    1,
			wantFailures: 1,
			wantErrors:   0,
			wantContains: []string{`[extract:token]`},
		},
		{
			name: "zero times",
			run: domain.RunResult{
				CollectionName: "zero-time",
			},
			runID:        "",
			wantTests:    0,
			wantFailures: 0,
			wantErrors:   0,
			wantContains: []string{`time="0.000"`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := formatJUnit(&buf, tt.run, tt.runID)
			if err != nil {
				t.Fatalf("formatJUnit returned error: %v", err)
			}

			output := buf.String()

			// Verify valid XML by round-tripping.
			var parsed junitTestSuites
			if err := xml.Unmarshal(buf.Bytes(), &parsed); err != nil {
				t.Fatalf("output is not valid XML: %v\nOutput:\n%s", err, output)
			}

			if parsed.Tests != tt.wantTests {
				t.Errorf("tests: got %d, want %d", parsed.Tests, tt.wantTests)
			}
			if parsed.Failures != tt.wantFailures {
				t.Errorf("failures: got %d, want %d", parsed.Failures, tt.wantFailures)
			}
			if parsed.Errors != tt.wantErrors {
				t.Errorf("errors: got %d, want %d", parsed.Errors, tt.wantErrors)
			}

			// Verify suite-level counts match.
			if len(parsed.TestSuites) != 1 {
				t.Fatalf("expected 1 test suite, got %d", len(parsed.TestSuites))
			}
			suite := parsed.TestSuites[0]
			if suite.Tests != tt.wantTests {
				t.Errorf("suite tests: got %d, want %d", suite.Tests, tt.wantTests)
			}
			if suite.Name != tt.run.CollectionName {
				t.Errorf("suite name: got %q, want %q", suite.Name, tt.run.CollectionName)
			}

			for _, s := range tt.wantContains {
				if !strings.Contains(output, s) {
					t.Errorf("output should contain %q but does not.\nOutput:\n%s", s, output)
				}
			}

			for _, s := range tt.wantNotContain {
				if strings.Contains(output, s) {
					t.Errorf("output should NOT contain %q but does.\nOutput:\n%s", s, output)
				}
			}
		})
	}
}

func TestFormatJUnit_LatencyConversion(t *testing.T) {
	now := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	run := domain.RunResult{
		CollectionName: "latency",
		StartedAt:      now,
		EndedAt:        now.Add(1500 * time.Millisecond),
		Results: []domain.RequestResult{
			{
				Name:       "Slow",
				Method:     domain.MethodGet,
				StatusCode: 200,
				LatencyMS:  1500,
			},
		},
	}

	var buf bytes.Buffer
	if err := formatJUnit(&buf, run, ""); err != nil {
		t.Fatalf("formatJUnit error: %v", err)
	}

	var parsed junitTestSuites
	if err := xml.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("invalid XML: %v", err)
	}

	tc := parsed.TestSuites[0].TestCases[0]
	if tc.Time != "1.500" {
		t.Errorf("testcase time: got %q, want %q", tc.Time, "1.500")
	}
}

func TestFormatJUnit_XMLHeader(t *testing.T) {
	run := domain.RunResult{CollectionName: "header-check"}

	var buf bytes.Buffer
	if err := formatJUnit(&buf, run, ""); err != nil {
		t.Fatalf("formatJUnit error: %v", err)
	}

	if !strings.HasPrefix(buf.String(), `<?xml version="1.0" encoding="UTF-8"?>`) {
		t.Error("output should start with XML declaration")
	}
}
