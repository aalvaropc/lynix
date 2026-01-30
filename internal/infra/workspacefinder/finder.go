package workspacefinder

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/aalvaropc/lynix/internal/domain"
)

// Finder locates a Lynix workspace root by searching for lynix.yaml upward.
type Finder struct {
	ConfigFile string // defaults to "lynix.yaml"
}

func NewFinder() *Finder {
	return &Finder{ConfigFile: "lynix.yaml"}
}

func (f *Finder) FindRoot(startDir string) (string, error) {
	if startDir == "" {
		return "", &domain.OpError{
			Op:   "workspacefinder.findroot",
			Kind: domain.KindInvalidConfig,
			Err:  errors.New("startDir is empty"),
		}
	}

	abs, err := filepath.Abs(startDir)
	if err != nil {
		return "", &domain.OpError{
			Op:   "workspacefinder.findroot",
			Kind: domain.KindExecution,
			Err:  err,
		}
	}

	// If user passes a file path, use its directory.
	info, statErr := os.Stat(abs)
	if statErr == nil && !info.IsDir() {
		abs = filepath.Dir(abs)
	}

	cur := filepath.Clean(abs)
	for {
		cfgPath := filepath.Join(cur, f.ConfigFile)
		if _, err := os.Stat(cfgPath); err == nil {
			return cur, nil
		}

		parent := filepath.Dir(cur)
		if parent == cur {
			// Reached filesystem root.
			return "", &domain.OpError{
				Op:   "workspacefinder.findroot",
				Kind: domain.KindNotFound,
				Err:  domain.ErrNotFound,
			}
		}
		cur = parent
	}
}
