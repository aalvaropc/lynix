package runstore

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aalvaropc/lynix/internal/domain"
	"github.com/aalvaropc/lynix/internal/ports"
)

const defaultRunsDir = "runs"
const maskValue = "********"

type JSONStore struct {
	rootDir        string
	runsDirName    string
	maskingEnabled bool
	writeIndex     bool
	now            func() time.Time
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

func NewJSONStore(root string, cfg domain.Config, opts ...Option) *JSONStore {
	runsDir := cfg.Paths.RunsDir
	if strings.TrimSpace(runsDir) == "" {
		runsDir = defaultRunsDir
	}

	s := &JSONStore{
		rootDir:        root,
		runsDirName:    runsDir,
		maskingEnabled: cfg.Masking.Enabled,
		writeIndex:     false,
		now:            time.Now,
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

	filename := fmt.Sprintf("%s_%s.json", ts.Format("20060102T150405Z"), slug)
	id := strings.TrimSuffix(filename, ".json")
	path := filepath.Join(dir, filename)

	if s.maskingEnabled {
		toSave = maskArtifact(toSave)
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
		_ = s.appendIndex(dir, id, filename, run)
	}

	return id, nil
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

	_, _ = f.Write(append(line, '\n'))
	return nil
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
