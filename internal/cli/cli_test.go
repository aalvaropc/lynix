package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aalvaropc/lynix/internal/domain"
)

// --- looksLikePath ---

func TestLooksLikePath(t *testing.T) {
	cases := []struct {
		input string
		want  bool
	}{
		{"demo", false},
		{"demo.yaml", false},
		{"./demo.yaml", true},
		{"collections/demo.yaml", true},
		{"/abs/path/demo.yaml", true},
	}
	for _, c := range cases {
		if got := looksLikePath(c.input); got != c.want {
			t.Errorf("looksLikePath(%q) = %v, want %v", c.input, got, c.want)
		}
	}
}

// --- hasYAMLExt ---

func TestHasYAMLExt(t *testing.T) {
	cases := []struct {
		input string
		want  bool
	}{
		{"demo.yaml", true},
		{"demo.yml", true},
		{"DEMO.YAML", true},
		{"demo.json", false},
		{"demo", false},
		{"", false},
	}
	for _, c := range cases {
		if got := hasYAMLExt(c.input); got != c.want {
			t.Errorf("hasYAMLExt(%q) = %v, want %v", c.input, got, c.want)
		}
	}
}

// --- fileExists ---

func TestFileExists_True(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "exists.txt")
	if err := os.WriteFile(p, []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !fileExists(p) {
		t.Errorf("expected fileExists=true for %s", p)
	}
}

func TestFileExists_False(t *testing.T) {
	tmp := t.TempDir()
	if fileExists(filepath.Join(tmp, "not_there.txt")) {
		t.Error("expected fileExists=false for non-existent file")
	}
}

// --- isRequestFailed ---

func TestIsRequestFailed_ErrorSet(t *testing.T) {
	r := domain.RequestResult{Error: &domain.RunError{Kind: domain.RunErrorConn, Message: "refused"}}
	if !isRequestFailed(r) {
		t.Error("expected failed=true when Error is set")
	}
}

func TestIsRequestFailed_AssertionFail(t *testing.T) {
	r := domain.RequestResult{
		Assertions: []domain.AssertionResult{{Passed: false}},
	}
	if !isRequestFailed(r) {
		t.Error("expected failed=true when assertion fails")
	}
}

func TestIsRequestFailed_ExtractFail(t *testing.T) {
	r := domain.RequestResult{
		Extracts: []domain.ExtractResult{{Success: false}},
	}
	if !isRequestFailed(r) {
		t.Error("expected failed=true when extract fails")
	}
}

func TestIsRequestFailed_AllPass(t *testing.T) {
	r := domain.RequestResult{
		Assertions: []domain.AssertionResult{{Passed: true}},
		Extracts:   []domain.ExtractResult{{Success: true}},
	}
	if isRequestFailed(r) {
		t.Error("expected failed=false when all assertions and extracts pass")
	}
}

func TestIsRequestFailed_Empty(t *testing.T) {
	if isRequestFailed(domain.RequestResult{}) {
		t.Error("expected failed=false for empty result")
	}
}

// --- countFailures ---

func TestCountFailures_Empty(t *testing.T) {
	if n := countFailures(domain.RunResult{}); n != 0 {
		t.Errorf("expected 0, got %d", n)
	}
}

func TestCountFailures_AllPass(t *testing.T) {
	run := domain.RunResult{
		Results: []domain.RequestResult{
			{Assertions: []domain.AssertionResult{{Passed: true}}},
			{Assertions: []domain.AssertionResult{{Passed: true}}},
		},
	}
	if n := countFailures(run); n != 0 {
		t.Errorf("expected 0, got %d", n)
	}
}

func TestCountFailures_SomeFail(t *testing.T) {
	run := domain.RunResult{
		Results: []domain.RequestResult{
			{Assertions: []domain.AssertionResult{{Passed: true}}},
			{Assertions: []domain.AssertionResult{{Passed: false}}},
			{Error: &domain.RunError{Kind: domain.RunErrorTimeout}},
		},
	}
	if n := countFailures(run); n != 2 {
		t.Errorf("expected 2, got %d", n)
	}
}

// --- countAssertionPassFail ---

func TestCountAssertionPassFail_Mixed(t *testing.T) {
	in := []domain.AssertionResult{
		{Passed: true},
		{Passed: false},
		{Passed: true},
	}
	pass, fail := countAssertionPassFail(in)
	if pass != 2 || fail != 1 {
		t.Errorf("expected pass=2 fail=1, got pass=%d fail=%d", pass, fail)
	}
}

func TestCountAssertionPassFail_Empty(t *testing.T) {
	pass, fail := countAssertionPassFail(nil)
	if pass != 0 || fail != 0 {
		t.Errorf("expected 0/0, got %d/%d", pass, fail)
	}
}

// --- countExtractPassFail ---

func TestCountExtractPassFail_Mixed(t *testing.T) {
	in := []domain.ExtractResult{
		{Success: true},
		{Success: false},
	}
	ok, bad := countExtractPassFail(in)
	if ok != 1 || bad != 1 {
		t.Errorf("expected ok=1 bad=1, got ok=%d bad=%d", ok, bad)
	}
}

// --- printRun ---

func TestPrintRun_JSON_ValidOutput(t *testing.T) {
	now := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	run := domain.RunResult{
		CollectionName:  "myapi",
		EnvironmentName: "dev",
		StartedAt:       now,
		EndedAt:         now.Add(100 * time.Millisecond),
	}
	var buf bytes.Buffer
	if err := printRun(&buf, run, "abc123", "json"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(buf.Bytes(), &payload); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, buf.String())
	}
	if payload["run_id"] != "abc123" {
		t.Errorf("expected run_id=abc123, got %v", payload["run_id"])
	}
	if payload["run"] == nil {
		t.Error("expected 'run' key in JSON output")
	}
}

func TestPrintRun_Pretty_ContainsCollectionName(t *testing.T) {
	run := domain.RunResult{
		CollectionName:  "myapi",
		EnvironmentName: "dev",
		StartedAt:       time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		EndedAt:         time.Date(2024, 1, 1, 0, 0, 1, 0, time.UTC),
	}
	var buf bytes.Buffer
	if err := printRun(&buf, run, "run-42", "pretty"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "myapi") {
		t.Errorf("expected collection name in pretty output, got:\n%s", out)
	}
	if !strings.Contains(out, "run-42") {
		t.Errorf("expected run ID in pretty output, got:\n%s", out)
	}
}

func TestPrintRun_EmptyFormat_IsPretty(t *testing.T) {
	var buf bytes.Buffer
	if err := printRun(&buf, domain.RunResult{}, "", ""); err != nil {
		t.Fatalf("empty format should behave like pretty, got error: %v", err)
	}
}

func TestPrintRun_UnknownFormat_ReturnsError(t *testing.T) {
	var buf bytes.Buffer
	err := printRun(&buf, domain.RunResult{}, "", "xml")
	if err == nil {
		t.Fatal("expected error for unknown format")
	}
	if !strings.Contains(err.Error(), "xml") {
		t.Errorf("expected error to mention format, got: %v", err)
	}
}

// --- printPrettyRun with assertions, extracts, and extracted vars ---

func TestPrintPrettyRun_WithResults(t *testing.T) {
	run := domain.RunResult{
		CollectionName:  "api",
		EnvironmentName: "prod",
		Results: []domain.RequestResult{
			{
				Name:       "health",
				Method:     domain.MethodGet,
				URL:        "http://x/health",
				LatencyMS:  42,
				StatusCode: 200,
				Assertions: []domain.AssertionResult{
					{Name: "status", Passed: true, Message: "status 200"},
					{Name: "jsonpath.exists", Passed: false, Message: "not found"},
				},
				Extracts: []domain.ExtractResult{
					{Name: "token", Success: true, Message: "extracted"},
				},
				Extracted: domain.Vars{"token": "abc"},
			},
		},
	}
	var buf bytes.Buffer
	printPrettyRun(&buf, run, "")
	out := buf.String()

	if !strings.Contains(out, "health") {
		t.Errorf("expected request name in output, got:\n%s", out)
	}
	if !strings.Contains(out, "1 pass / 1 fail") {
		t.Errorf("expected assertion pass/fail count, got:\n%s", out)
	}
	if !strings.Contains(out, "1 ok / 0 fail") {
		t.Errorf("expected extract ok/fail count, got:\n%s", out)
	}
	if !strings.Contains(out, "token") {
		t.Errorf("expected extracted var in output, got:\n%s", out)
	}
}

func TestPrintPrettyRun_RequestWithError(t *testing.T) {
	run := domain.RunResult{
		Results: []domain.RequestResult{
			{
				Name:   "fail-req",
				Method: domain.MethodGet,
				Error:  &domain.RunError{Kind: domain.RunErrorConn, Message: "connection refused"},
			},
		},
	}
	var buf bytes.Buffer
	printPrettyRun(&buf, run, "")
	out := buf.String()

	if !strings.Contains(out, "connection refused") {
		t.Errorf("expected error message in output, got:\n%s", out)
	}
	if !strings.Contains(out, "FAIL") {
		t.Errorf("expected FAIL status for errored request, got:\n%s", out)
	}
}

// --- command structure ---

func TestRootCmd_RegistersSubcommands(t *testing.T) {
	cmd := newRootCmd()
	names := map[string]bool{}
	for _, sub := range cmd.Commands() {
		names[sub.Use] = true
	}
	for _, expected := range []string{"run", "validate", "version", "init", "collections", "envs", "import"} {
		if !names[expected] {
			t.Errorf("expected subcommand %q to be registered", expected)
		}
	}
}

func TestRunCmd_Flags(t *testing.T) {
	cmd := runCmd()
	if cmd.Use != "run" {
		t.Errorf("expected Use=run, got %q", cmd.Use)
	}
	for _, flag := range []string{"collection", "env", "workspace", "no-save", "format", "report", "report-path", "fail-fast", "only", "tags", "retries", "retry-delay", "retry-5xx"} {
		if cmd.Flags().Lookup(flag) == nil {
			t.Errorf("expected --%s flag on run command", flag)
		}
	}
}

func TestValidateCmd_Flags(t *testing.T) {
	cmd := validateCmd()
	if cmd.Use != "validate" {
		t.Errorf("expected Use=validate, got %q", cmd.Use)
	}
	for _, flag := range []string{"collection", "env", "workspace"} {
		if cmd.Flags().Lookup(flag) == nil {
			t.Errorf("expected --%s flag on validate command", flag)
		}
	}
}

func TestCollectionsCmd_HasListSubcommand(t *testing.T) {
	cmd := collectionsCmd()
	found := false
	for _, sub := range cmd.Commands() {
		if sub.Use == "list" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'list' subcommand under collections")
	}
}

func TestEnvsCmd_HasListSubcommand(t *testing.T) {
	cmd := envsCmd()
	found := false
	for _, sub := range cmd.Commands() {
		if sub.Use == "list" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'list' subcommand under envs")
	}
}

func TestImportCmd_HasCurlSubcommand(t *testing.T) {
	cmd := importCmd()
	found := false
	for _, sub := range cmd.Commands() {
		if sub.Name() == "curl" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'curl' subcommand under import")
	}
}

func TestImportCmd_HasPostmanSubcommand(t *testing.T) {
	cmd := importCmd()
	found := false
	for _, sub := range cmd.Commands() {
		if sub.Name() == "postman" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'postman' subcommand under import")
	}
}

func TestImportCmd_Use(t *testing.T) {
	cmd := importCmd()
	if cmd.Use != "import" {
		t.Errorf("expected Use=import, got %q", cmd.Use)
	}
}

func TestImportCmd_Short(t *testing.T) {
	cmd := importCmd()
	if cmd.Short == "" {
		t.Error("expected non-empty Short description")
	}
}

func TestImportCmd_HasTwoSubcommands(t *testing.T) {
	cmd := importCmd()
	if len(cmd.Commands()) != 2 {
		t.Errorf("expected 2 subcommands, got %d", len(cmd.Commands()))
	}
}

func TestImportCurlCmd_Flags(t *testing.T) {
	cmd := importCurlCmd()
	for _, flag := range []string{"output", "from-file", "name"} {
		if cmd.Flags().Lookup(flag) == nil {
			t.Errorf("expected --%s flag on import curl command", flag)
		}
	}
}

func TestImportCurlCmd_OutputShortFlag(t *testing.T) {
	cmd := importCurlCmd()
	f := cmd.Flags().ShorthandLookup("o")
	if f == nil {
		t.Error("expected -o short flag for output")
	}
}

func TestImportCurlCmd_MaxArgs(t *testing.T) {
	cmd := importCurlCmd()
	// Should accept 0 or 1 args
	err := cmd.Args(cmd, []string{"arg1"})
	if err != nil {
		t.Errorf("expected 1 arg to be accepted: %v", err)
	}
	err = cmd.Args(cmd, []string{})
	if err != nil {
		t.Errorf("expected 0 args to be accepted: %v", err)
	}
	err = cmd.Args(cmd, []string{"a", "b"})
	if err == nil {
		t.Error("expected error for 2 args")
	}
}

func TestImportCurlCmd_NoArgs_NoFromFile_ReturnsError(t *testing.T) {
	cmd := importCurlCmd()
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when no arg and no --from-file")
	}
}

func TestImportCurlCmd_WithArg_Succeeds(t *testing.T) {
	cmd := importCurlCmd()
	cmd.SetArgs([]string{"curl https://api.example.com/health"})
	err := cmd.Execute()
	if err != nil {
		t.Errorf("expected success with positional arg: %v", err)
	}
}

func TestImportCurlCmd_FromFile(t *testing.T) {
	tmp := t.TempDir()
	curlFile := filepath.Join(tmp, "curl.txt")
	if err := os.WriteFile(curlFile, []byte("curl https://api.example.com/health"), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := importCurlCmd()
	cmd.SetArgs([]string{"--from-file", curlFile})
	err := cmd.Execute()
	if err != nil {
		t.Errorf("expected success with --from-file: %v", err)
	}
}

func TestImportCurlCmd_FromFile_NotFound(t *testing.T) {
	cmd := importCurlCmd()
	cmd.SetArgs([]string{"--from-file", "/nonexistent/curl.txt"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for non-existent --from-file")
	}
}

func TestImportCurlCmd_OutputToFile(t *testing.T) {
	tmp := t.TempDir()
	outFile := filepath.Join(tmp, "output.yaml")

	cmd := importCurlCmd()
	cmd.SetArgs([]string{"curl https://api.example.com/health", "-o", outFile})
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(outFile); os.IsNotExist(err) {
		t.Error("expected output file to be created")
	}
}

func TestImportCurlCmd_NameOverride(t *testing.T) {
	cmd := importCurlCmd()
	tmp := t.TempDir()
	outFile := filepath.Join(tmp, "named.yaml")

	cmd.SetArgs([]string{"curl https://api.example.com/health", "--name", "My Custom API", "-o", outFile})
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	b, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "My Custom API") {
		t.Errorf("expected custom name in output:\n%s", string(b))
	}
}

func TestImportPostmanCmd_Flags(t *testing.T) {
	cmd := importPostmanCmd()
	for _, flag := range []string{"output", "name"} {
		if cmd.Flags().Lookup(flag) == nil {
			t.Errorf("expected --%s flag on import postman command", flag)
		}
	}
}

func TestImportPostmanCmd_OutputShortFlag(t *testing.T) {
	cmd := importPostmanCmd()
	f := cmd.Flags().ShorthandLookup("o")
	if f == nil {
		t.Error("expected -o short flag for output")
	}
}

func TestImportPostmanCmd_ExactArgs(t *testing.T) {
	cmd := importPostmanCmd()
	err := cmd.Args(cmd, []string{})
	if err == nil {
		t.Error("expected error for 0 args")
	}
	err = cmd.Args(cmd, []string{"a", "b"})
	if err == nil {
		t.Error("expected error for 2 args")
	}
	err = cmd.Args(cmd, []string{"file.json"})
	if err != nil {
		t.Errorf("expected 1 arg accepted: %v", err)
	}
}

func TestImportPostmanCmd_FileNotFound(t *testing.T) {
	cmd := importPostmanCmd()
	cmd.SetArgs([]string{"/nonexistent/file.json"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

func TestImportPostmanCmd_ValidFile(t *testing.T) {
	tmp := t.TempDir()
	jsonFile := filepath.Join(tmp, "collection.json")
	content := `{"info":{"name":"Test","schema":""},"item":[{"name":"R","request":{"method":"GET","url":"https://e.com/"}}]}`
	if err := os.WriteFile(jsonFile, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := importPostmanCmd()
	cmd.SetArgs([]string{jsonFile})
	err := cmd.Execute()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestImportPostmanCmd_OutputToFile(t *testing.T) {
	tmp := t.TempDir()
	jsonFile := filepath.Join(tmp, "collection.json")
	content := `{"info":{"name":"Test","schema":""},"item":[{"name":"R","request":{"method":"GET","url":"https://e.com/"}}]}`
	if err := os.WriteFile(jsonFile, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	outFile := filepath.Join(tmp, "output.yaml")
	cmd := importPostmanCmd()
	cmd.SetArgs([]string{jsonFile, "-o", outFile})
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(outFile); os.IsNotExist(err) {
		t.Error("expected output file to be created")
	}
}

func TestImportPostmanCmd_NameOverride(t *testing.T) {
	tmp := t.TempDir()
	jsonFile := filepath.Join(tmp, "collection.json")
	content := `{"info":{"name":"Original","schema":""},"item":[{"name":"R","request":{"method":"GET","url":"https://e.com/"}}]}`
	if err := os.WriteFile(jsonFile, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	outFile := filepath.Join(tmp, "named.yaml")
	cmd := importPostmanCmd()
	cmd.SetArgs([]string{jsonFile, "--name", "Overridden Name", "-o", outFile})
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	b, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "Overridden Name") {
		t.Errorf("expected custom name in output:\n%s", string(b))
	}
	if strings.Contains(string(b), "Original") {
		t.Errorf("expected original name to be overridden:\n%s", string(b))
	}
}

func TestImportPostmanCmd_InvalidJSON(t *testing.T) {
	tmp := t.TempDir()
	jsonFile := filepath.Join(tmp, "bad.json")
	if err := os.WriteFile(jsonFile, []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := importPostmanCmd()
	cmd.SetArgs([]string{jsonFile})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for invalid JSON file")
	}
}

func TestInitCmd_Flags(t *testing.T) {
	cmd := initCmd()
	if cmd.Flags().Lookup("path") == nil {
		t.Error("expected --path flag on init command")
	}
	if cmd.Flags().Lookup("force") == nil {
		t.Error("expected --force flag on init command")
	}
}

// --- resolveWorkspaceRoot ---

func TestResolveWorkspaceRoot_ExplicitPath(t *testing.T) {
	tmp := t.TempDir()
	got, err := resolveWorkspaceRoot(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != tmp {
		t.Errorf("expected %q, got %q", tmp, got)
	}
}

func TestResolveWorkspaceRoot_RelativePath(t *testing.T) {
	got, err := resolveWorkspaceRoot(".")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !filepath.IsAbs(got) {
		t.Errorf("expected absolute path, got %q", got)
	}
}
