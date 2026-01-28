package fsworkspace

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/aalvaropc/lynix/internal/domain"
)

type Initializer struct{}

func NewInitializer() *Initializer {
	return &Initializer{}
}

func (i *Initializer) Init(spec domain.WorkspaceSpec, force bool) error {
	root := filepath.Clean(spec.Root)

	dirs := []string{
		filepath.Join(root, "collections"),
		filepath.Join(root, "env"),
		filepath.Join(root, "runs"),
	}

	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}

	return fs.WalkDir(templatesFS, "templates", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		rel := strings.TrimPrefix(p, "templates/")
		dst := filepath.Join(root, rel)

		if !force {
			if _, statErr := os.Stat(dst); statErr == nil {
				// Skip existing files unless --force is set.
				return nil
			}
		}

		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}

		b, err := fs.ReadFile(templatesFS, p)
		if err != nil {
			return err
		}

		return os.WriteFile(dst, b, 0o644)
	})
}
