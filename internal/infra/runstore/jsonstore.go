package runstore

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/aalvaropc/lynix/internal/domain"
	"github.com/aalvaropc/lynix/internal/ports"
)

const defaultRunsDir = "runs"
const maskValue = "********"

// Redacter is the interface for artifact redaction (avoids import cycle).
type Redacter interface {
	Redact(run domain.RunArtifact) domain.RunArtifact
}

// SecretChecker is an optional interface a Redacter may implement.
type SecretChecker interface {
	CheckForSecrets(run domain.RunArtifact) error
}

type JSONStore struct {
	rootDir        string
	runsDirName    string
	maskingEnabled bool
	failOnSecret   bool
	saveHeaders    bool
	saveBody       bool
	writeIndex     bool
	maxRuns        int // 0 = unlimited
	redacter       Redacter
	now            func() time.Time
	log            *slog.Logger
}

type Option func(*JSONStore)

// WithIndex enables a simple JSONL index: runs/index.jsonl
func WithIndex(enabled bool) Option {
	return func(s *JSONStore) { s.writeIndex = enabled }
}

// WithNow is useful for tests.
func WithNow(now func() time.Time) Option {
	return func(s *JSONStore) { s.now = now }
}

// WithLogger sets a structured logger for the store.
func WithLogger(log *slog.Logger) Option {
	return func(s *JSONStore) { s.log = log }
}

// WithRedacter injects an external redacter that replaces the built-in maskArtifact.
func WithRedacter(r Redacter) Option {
	return func(s *JSONStore) { s.redacter = r }
}

func NewJSONStore(root string, cfg domain.Config, opts ...Option) *JSONStore {
	runsDir := cfg.Paths.RunsDir
	if strings.TrimSpace(runsDir) == "" {
		runsDir = defaultRunsDir
	}

	s := &JSONStore{
		rootDir:        root,
		runsDirName:    runsDir,
		maskingEnabled: cfg.Masking.Enabled,
		failOnSecret:   cfg.Masking.FailOnDetectedSecret,
		saveHeaders:    cfg.Artifacts.SaveResponseHeaders,
		saveBody:       cfg.Artifacts.SaveResponseBody,
		maxRuns:        cfg.Artifacts.MaxRuns,
		writeIndex:     false,
		now:            time.Now,
		log:            slog.New(slog.NewJSONHandler(io.Discard, nil)),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

var _ ports.ArtifactStore = (*JSONStore)(nil)

func (s *JSONStore) SaveRun(run domain.RunArtifact) (string, error) {
	dir := filepath.Join(s.rootDir, s.runsDirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", &domain.OpError{
			Op:   "runstore.mkdir",
			Kind: domain.KindExecution,
			Path: dir,
			Err:  err,
		}
	}

	ts := run.StartedAt
	if ts.IsZero() {
		ts = s.now()
	}
	ts = ts.UTC()

	toSave := run
	if toSave.StartedAt.IsZero() {
		toSave.StartedAt = ts
	}
	collectionPart := run.CollectionName
	if strings.TrimSpace(collectionPart) == "" {
		collectionPart = strings.TrimSuffix(filepath.Base(run.CollectionPath), filepath.Ext(run.CollectionPath))
	}
	slug := slugify(collectionPart)
	if slug == "" {
		slug = "run"
	}

	base := fmt.Sprintf("%s_%s", ts.Format("20060102T150405Z"), slug)
	filename, err := uniqueRunFilename(dir, base)
	if err != nil {
		return "", &domain.OpError{
			Op:   "runstore.filename",
			Kind: domain.KindExecution,
			Path: dir,
			Err:  err,
		}
	}
	id := strings.TrimSuffix(filename, ".json")
	path := filepath.Join(dir, filename)

	if !s.saveHeaders || !s.saveBody {
		toSave = applyResponseSavePolicy(toSave, s.saveHeaders, s.saveBody)
	}
	if s.maskingEnabled {
		if s.redacter != nil {
			toSave = s.redacter.Redact(toSave)
		} else {
			toSave = maskArtifact(toSave)
		}
	}

	if s.failOnSecret {
		if checker, ok := s.redacter.(SecretChecker); ok {
			if err := checker.CheckForSecrets(toSave); err != nil {
				return "", err
			}
		}
	}

	b, err := json.MarshalIndent(toSave, "", "  ")
	if err != nil {
		return "", &domain.OpError{
			Op:   "runstore.marshal",
			Kind: domain.KindExecution,
			Path: path,
			Err:  err,
		}
	}

	// Atomic-ish write: tmp then rename.
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return "", &domain.OpError{
			Op:   "runstore.write",
			Kind: domain.KindExecution,
			Path: tmp,
			Err:  err,
		}
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return "", &domain.OpError{
			Op:   "runstore.rename",
			Kind: domain.KindExecution,
			Path: path,
			Err:  err,
		}
	}

	if s.writeIndex {
		if err := s.appendIndex(dir, id, filename, run); err != nil {
			s.log.Error("runstore.appendIndex.failed", "err", err, "path", dir)
		}
	}

	if s.maxRuns > 0 {
		if err := s.rotate(dir); err != nil {
			s.log.Error("runstore.rotate.failed", "err", err, "path", dir)
		}
	}

	return id, nil
}

func applyResponseSavePolicy(run domain.RunArtifact, saveHeaders bool, saveBody bool) domain.RunArtifact {
	out := run
	out.Results = make([]domain.RequestResult, 0, len(run.Results))

	for _, rr := range run.Results {
		c := rr

		snap := cloneResponseSnapshot(rr.Response)
		if !saveHeaders {
			snap.Headers = map[string][]string{}
		}
		if !saveBody {
			snap.Body = nil
			snap.Truncated = false
		}

		c.Response = snap
		out.Results = append(out.Results, c)
	}

	return out
}

func uniqueRunFilename(dir, base string) (string, error) {
	filename := base + ".json"
	path := filepath.Join(dir, filename)
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return filename, nil
		}
		return "", err
	}

	for i := 2; i <= 999; i++ {
		filename = fmt.Sprintf("%s_%d.json", base, i)
		path = filepath.Join(dir, filename)
		if _, err := os.Stat(path); err != nil {
			if os.IsNotExist(err) {
				return filename, nil
			}
			return "", err
		}
	}

	return "", fmt.Errorf("too many run artifacts named %q", base)
}

func (s *JSONStore) appendIndex(dir, id, filename string, run domain.RunArtifact) error {
	type idx struct {
		ID         string    `json:"id"`
		File       string    `json:"file"`
		Collection string    `json:"collection"`
		Env        string    `json:"env"`
		StartedAt  time.Time `json:"started_at"`
	}
	line, err := json.Marshal(idx{
		ID:         id,
		File:       filename,
		Collection: run.CollectionName,
		Env:        run.EnvironmentName,
		StartedAt:  run.StartedAt,
	})
	if err != nil {
		return err
	}

	indexPath := filepath.Join(dir, "index.jsonl")
	f, err := os.OpenFile(indexPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := f.Write(append(line, '\n')); err != nil {
		return err
	}
	return nil
}

// rotate removes the oldest run artifacts when the count exceeds maxRuns.
// Files are sorted lexicographically (timestamp-prefixed → chronological order).
// The index.jsonl is rewritten to match the surviving files.
func (s *JSONStore) rotate(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	var jsonFiles []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			jsonFiles = append(jsonFiles, e.Name())
		}
	}

	if len(jsonFiles) <= s.maxRuns {
		return nil
	}

	sort.Strings(jsonFiles) // timestamp prefix → oldest first

	toDelete := jsonFiles[:len(jsonFiles)-s.maxRuns]
	deleteSet := make(map[string]bool, len(toDelete))
	for _, f := range toDelete {
		deleteSet[f] = true
		if err := os.Remove(filepath.Join(dir, f)); err != nil && !os.IsNotExist(err) {
			s.log.Error("runstore.rotate.remove", "file", f, "err", err)
		}
	}

	if s.writeIndex {
		s.pruneIndex(dir, deleteSet)
	}

	return nil
}

// pruneIndex rewrites index.jsonl to remove entries for deleted files.
func (s *JSONStore) pruneIndex(dir string, deleted map[string]bool) {
	indexPath := filepath.Join(dir, "index.jsonl")
	b, err := os.ReadFile(indexPath)
	if err != nil {
		return // no index to prune
	}

	var kept [][]byte
	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		if line == "" {
			continue
		}
		var entry struct {
			File string `json:"file"`
		}
		if json.Unmarshal([]byte(line), &entry) == nil && !deleted[entry.File] {
			kept = append(kept, []byte(line))
		}
	}

	var out []byte
	for _, line := range kept {
		out = append(out, line...)
		out = append(out, '\n')
	}
	_ = os.WriteFile(indexPath, out, 0o600)
}

// maskArtifact returns a masked copy (does NOT mutate the input).
func maskArtifact(run domain.RunArtifact) domain.RunArtifact {
	out := run
	out.Results = make([]domain.RequestResult, 0, len(run.Results))

	for _, rr := range run.Results {
		c := rr

		// Deep copy maps/slices we will touch.
		c.Extracted = cloneVars(rr.Extracted)
		c.Extracts = cloneExtractResults(rr.Extracts)
		c.Assertions = cloneAssertionResults(rr.Assertions)
		c.Response = cloneResponseSnapshot(rr.Response)

		for k := range c.Extracted {
			if isSensitiveKey(k) {
				c.Extracted[k] = maskValue
			}
		}

		for k := range c.Response.Headers {
			if isSensitiveHeaderKey(k) {
				vals := c.Response.Headers[k]
				for i := range vals {
					vals[i] = maskValue
				}
				c.Response.Headers[k] = vals
			}
		}

		out.Results = append(out.Results, c)
	}

	return out
}

func isSensitiveKey(k string) bool {
	kk := strings.ToLower(k)
	return strings.Contains(kk, "token") ||
		strings.Contains(kk, "secret") ||
		strings.Contains(kk, "password")
}

func isSensitiveHeaderKey(k string) bool {
	kk := strings.ToLower(strings.TrimSpace(k))
	switch kk {
	case "authorization", "proxy-authorization", "cookie", "set-cookie", "x-api-key", "x-auth-token":
		return true
	}

	return strings.Contains(kk, "token") ||
		strings.Contains(kk, "secret") ||
		strings.Contains(kk, "password") ||
		strings.Contains(kk, "api-key") ||
		strings.Contains(kk, "apikey")
}

func cloneVars(in domain.Vars) domain.Vars {
	if in == nil {
		return domain.Vars{}
	}
	out := domain.Vars{}
	for k, v := range in {
		out[k] = v
	}
	return out
}

func cloneExtractResults(in []domain.ExtractResult) []domain.ExtractResult {
	if in == nil {
		return []domain.ExtractResult{}
	}
	out := make([]domain.ExtractResult, len(in))
	copy(out, in)
	return out
}

func cloneAssertionResults(in []domain.AssertionResult) []domain.AssertionResult {
	if in == nil {
		return []domain.AssertionResult{}
	}
	out := make([]domain.AssertionResult, len(in))
	copy(out, in)
	return out
}

func cloneResponseSnapshot(in domain.ResponseSnapshot) domain.ResponseSnapshot {
	out := domain.ResponseSnapshot{
		Truncated: in.Truncated,
	}

	// Headers deep copy
	if in.Headers != nil {
		out.Headers = make(map[string][]string, len(in.Headers))
		for k, v := range in.Headers {
			cp := make([]string, len(v))
			copy(cp, v)
			out.Headers[k] = cp
		}
	} else {
		out.Headers = map[string][]string{}
	}

	// Body copy (optional)
	if in.Body != nil {
		out.Body = make([]byte, len(in.Body))
		copy(out.Body, in.Body)
	}

	return out
}

// slugify produces a safe filename component.
func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return ""
	}

	var b strings.Builder
	b.Grow(len(s))

	lastDash := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
			lastDash = false
		case r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		case r == ' ' || r == '_' || r == '-' || r == '.':
			if !lastDash {
				b.WriteByte('-')
				lastDash = true
			}
		default:
			// any other char -> dash
			if !lastDash {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}

	out := strings.Trim(b.String(), "-")
	out = strings.ReplaceAll(out, "--", "-")
	return out
}
