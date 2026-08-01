package runstore

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/aalvaropc/lynix/internal/domain"
)

// RunSummary is a lightweight view of a stored run for listings.
type RunSummary struct {
	ID         string    `json:"id"`
	Collection string    `json:"collection"`
	Env        string    `json:"env"`
	StartedAt  time.Time `json:"started_at"`
	Passed     int       `json:"passed"`
	Failed     int       `json:"failed"`
	Errors     int       `json:"errors"`
}

// ListRuns scans the runs directory (newest first). The directory is the
// source of truth — index.jsonl can be stale or missing.
func (s *JSONStore) ListRuns() ([]RunSummary, error) {
	dir := filepath.Join(s.rootDir, s.runsDirName)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, &domain.OpError{
			Op:   "runstore.list",
			Kind: domain.KindExecution,
			Path: dir,
			Err:  err,
		}
	}

	var names []string
	for _, e := range entries {
		if !e.IsDir() && runFilePattern.MatchString(e.Name()) {
			names = append(names, e.Name())
		}
	}
	sort.Slice(names, func(i, j int) bool { return runFileLess(names[j], names[i]) }) // newest first

	out := make([]RunSummary, 0, len(names))
	for _, name := range names {
		run, err := s.loadRunFile(filepath.Join(dir, name))
		if err != nil {
			// A corrupt artifact should not hide the rest of the history.
			out = append(out, RunSummary{ID: strings.TrimSuffix(name, ".json"), Collection: "(unreadable)"})
			continue
		}
		summary := RunSummary{
			ID:         strings.TrimSuffix(name, ".json"),
			Collection: run.CollectionName,
			Env:        run.EnvironmentName,
			StartedAt:  run.StartedAt,
		}
		for _, r := range run.Results {
			switch {
			case r.Error != nil:
				summary.Errors++
			case r.Failed():
				summary.Failed++
			default:
				summary.Passed++
			}
		}
		out = append(out, summary)
	}
	return out, nil
}

// LoadRun reads a stored artifact by its ID (the filename without .json).
func (s *JSONStore) LoadRun(id string) (domain.RunArtifact, error) {
	if strings.ContainsAny(id, `/\`) || strings.Contains(id, "..") {
		return domain.RunArtifact{}, &domain.OpError{
			Op:   "runstore.load",
			Kind: domain.KindInvalidConfig,
			Err:  fmt.Errorf("invalid run id %q", id),
		}
	}
	path := filepath.Join(s.rootDir, s.runsDirName, id+".json")
	run, err := s.loadRunFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return domain.RunArtifact{}, &domain.OpError{
				Op:   "runstore.load",
				Kind: domain.KindNotFound,
				Path: path,
				Err:  fmt.Errorf("%w: run %q not found", domain.ErrNotFound, id),
			}
		}
		return domain.RunArtifact{}, err
	}
	return run, nil
}

func (s *JSONStore) loadRunFile(path string) (domain.RunArtifact, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return domain.RunArtifact{}, err
	}
	var run domain.RunArtifact
	if err := json.Unmarshal(b, &run); err != nil {
		return domain.RunArtifact{}, &domain.OpError{
			Op:   "runstore.parse",
			Kind: domain.KindInvalidConfig,
			Path: path,
			Err:  err,
		}
	}
	return run, nil
}
