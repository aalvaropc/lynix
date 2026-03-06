package cli

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/aalvaropc/lynix/internal/domain"
)

func TestWriteJUnitReport_WritesValidXML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "report.xml")

	now := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	run := domain.RunResult{
		CollectionName: "demo",
		StartedAt:      now,
		EndedAt:        now.Add(200 * time.Millisecond),
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
		},
	}

	if err := writeJUnitReport(path, run, "run-123"); err != nil {
		t.Fatalf("writeJUnitReport returned error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read report file: %v", err)
	}

	var parsed junitTestSuites
	if err := xml.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("report is not valid XML: %v\nContent:\n%s", err, string(data))
	}

	if parsed.Tests != 1 {
		t.Errorf("tests: got %d, want 1", parsed.Tests)
	}
	if parsed.Failures != 0 {
		t.Errorf("failures: got %d, want 0", parsed.Failures)
	}
}

func TestWriteJUnitReport_InvalidPath(t *testing.T) {
	run := domain.RunResult{CollectionName: "demo"}
	err := writeJUnitReport("/nonexistent/dir/report.xml", run, "")
	if err == nil {
		t.Fatal("expected error for invalid path, got nil")
	}
}

func TestValidateReportFlags(t *testing.T) {
	tests := []struct {
		name       string
		report     string
		reportPath string
		wantErr    string
	}{
		{
			name:       "both empty is ok",
			report:     "",
			reportPath: "",
		},
		{
			name:       "both set with junit is ok",
			report:     "junit",
			reportPath: "results.xml",
		},
		{
			name:       "report without path",
			report:     "junit",
			reportPath: "",
			wantErr:    "--report-path is required when --report is set",
		},
		{
			name:       "path without report",
			report:     "",
			reportPath: "results.xml",
			wantErr:    "--report is required when --report-path is set",
		},
		{
			name:       "unsupported report type",
			report:     "csv",
			reportPath: "results.csv",
			wantErr:    `unsupported report type "csv" (expected "junit")`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateReportFlags(tt.report, tt.reportPath)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			}
			if got := err.Error(); got != tt.wantErr {
				t.Errorf("error = %q, want %q", got, tt.wantErr)
			}
		})
	}
}
