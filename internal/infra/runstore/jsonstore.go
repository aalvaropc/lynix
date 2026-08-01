package runstore

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
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
	toSave.SchemaVersion = domain.ArtifactSchemaVersion
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
	// Masking requires an injected redacter: a silent fallback to a weaker
	// built-in masker would degrade the security guarantee without notice.
	if s.maskingEnabled {
		if s.redacter == nil {
			return "", &domain.OpError{
				Op:   "runstore.redact",
				Kind: domain.KindInvalidConfig,
				Path: path,
				Err:  errors.New("masking is enabled but no redacter is configured"),
			}
		}
		toSave = s.redacter.Redact(toSave)
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

// runFilePattern matches only artifacts this store wrote (timestamp-prefixed),
// so rotation never deletes unrelated files a user drops into runs/.
var runFilePattern = regexp.MustCompile(`^\d{8}T\d{6}Z_.+\.json$`)

// rotate removes the oldest run artifacts when the count exceeds maxRuns.
// Ordering uses the timestamp prefix plus the numeric collision suffix:
// plain lexicographic order would delete "_10" before "_2".
func (s *JSONStore) rotate(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	var jsonFiles []string
	for _, e := range entries {
		if !e.IsDir() && runFilePattern.MatchString(e.Name()) {
			jsonFiles = append(jsonFiles, e.Name())
		}
	}

	if len(jsonFiles) <= s.maxRuns {
		return nil
	}

	sort.Slice(jsonFiles, func(i, j int) bool {
		return runFileLess(jsonFiles[i], jsonFiles[j])
	})

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

// runFileLess orders artifacts chronologically: timestamp prefix first, then
// the numeric collision suffix ("_2" ... "_999") as a number.
func runFileLess(a, b string) bool {
	ta, na := splitRunFilename(a)
	tb, nb := splitRunFilename(b)
	if ta != tb {
		return ta < tb
	}
	if na != nb {
		return na < nb
	}
	return a < b
}

var collisionSuffix = regexp.MustCompile(`_(\d+)\.json$`)

func splitRunFilename(name string) (prefix string, suffix int) {
	prefix = strings.TrimSuffix(name, ".json")
	if m := collisionSuffix.FindStringSubmatch(name); m != nil {
		if n, err := strconv.Atoi(m[1]); err == nil {
			return strings.TrimSuffix(name, m[0]), n
		}
	}
	return prefix, 0
}

// pruneIndex rewrites index.jsonl to remove entries for deleted files.
// The rewrite is atomic (tmp + rename), like the artifact writes.
func (s *JSONStore) pruneIndex(dir string, deleted map[string]bool) {
	indexPath := filepath.Join(dir, "index.jsonl")
	b, err := os.ReadFile(indexPath)
	if err != nil {
		return // no index to prune
	}

	var out []byte
	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		if line == "" {
			continue
		}
		var entry struct {
			File string `json:"file"`
		}
		if json.Unmarshal([]byte(line), &entry) == nil && !deleted[entry.File] {
			out = append(out, line...)
			out = append(out, '\n')
		}
	}

	tmp := indexPath + ".tmp"
	if err := os.WriteFile(tmp, out, 0o600); err != nil {
		s.log.Error("runstore.pruneIndex.write", "err", err)
		return
	}
	if err := os.Rename(tmp, indexPath); err != nil {
		_ = os.Remove(tmp)
		s.log.Error("runstore.pruneIndex.rename", "err", err)
	}
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
