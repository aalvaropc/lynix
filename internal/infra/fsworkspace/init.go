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
		filepath.Join(root, ".lynix", "logs"),
	}

	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}

	if err := ensureGitignore(root); err != nil {
		return err
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

		mode := fs.FileMode(0o644)
		if strings.Contains(strings.ToLower(rel), "secrets") {
			mode = 0o600
		}

		return os.WriteFile(dst, b, mode)
	})
}

func ensureGitignore(root string) error {
	const header = "# Lynix"
	entries := []string{
		"runs/",
		".lynix/",
		"lynix.lock",
		"env/secrets.local.yaml",
	}

	path := filepath.Join(root, ".gitignore")
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			lines := append([]string{header}, entries...)
			lines = append(lines, "")
			return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644)
		}
		return err
	}

	existing := string(b)
	present := map[string]bool{}
	for _, line := range strings.Split(existing, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		present[trimmed] = true
	}

	var missing []string
	for _, e := range entries {
		if !present[e] {
			missing = append(missing, e)
		}
	}
	if len(missing) == 0 {
		return nil
	}

	var out strings.Builder
	out.Grow(len(existing) + 64)

	out.WriteString(existing)
	if existing != "" && !strings.HasSuffix(existing, "\n") {
		out.WriteByte('\n')
	}
	out.WriteByte('\n')
	if !present[header] {
		out.WriteString(header)
		out.WriteByte('\n')
	}
	for _, e := range missing {
		out.WriteString(e)
		out.WriteByte('\n')
	}

	return os.WriteFile(path, []byte(out.String()), 0o644)
}
