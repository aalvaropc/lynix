package workspacefinder

import (
	"os"
	"path/filepath"

	"github.com/aalvaropc/lynix/internal/domain"
	"gopkg.in/yaml.v3"
)

// LoadConfig loads lynix.yaml from the workspace root and applies defaults.
func LoadConfig(root string) (domain.Config, error) {
	cfg := domain.DefaultConfig()

	path := filepath.Join(root, "lynix.yaml")
	b, err := os.ReadFile(path)
	if err != nil {
		return cfg, &domain.OpError{
			Op:   "workspacefinder.loadconfig",
			Kind: domain.KindNotFound,
			Path: path,
			Err:  err,
		}
	}

	var y yamlConfig
	if err := yaml.Unmarshal(b, &y); err != nil {
		return cfg, &domain.OpError{
			Op:   "workspacefinder.loadconfig",
			Kind: domain.KindInvalidConfig,
			Path: path,
			Err:  err,
		}
	}

	// Apply parsed values on top of defaults.
	if y.Lynix.Masking.Enabled != nil {
		cfg.Masking.Enabled = *y.Lynix.Masking.Enabled
	}
	if y.Lynix.Defaults.Env != "" {
		cfg.Defaults.Environment = y.Lynix.Defaults.Env
	}
	if y.Lynix.Paths.CollectionsDir != "" {
		cfg.Paths.CollectionsDir = y.Lynix.Paths.CollectionsDir
	}
	if y.Lynix.Paths.EnvironmentsDir != "" {
		cfg.Paths.EnvironmentsDir = y.Lynix.Paths.EnvironmentsDir
	}
	if y.Lynix.Paths.RunsDir != "" {
		cfg.Paths.RunsDir = y.Lynix.Paths.RunsDir
	}

	return cfg, nil
}

type yamlConfig struct {
	Lynix struct {
		Masking struct {
			Enabled *bool `yaml:"enabled"`
		} `yaml:"masking"`

		Defaults struct {
			Env string `yaml:"env"`
		} `yaml:"defaults"`

		Paths struct {
			CollectionsDir  string `yaml:"collections_dir"`
			EnvironmentsDir string `yaml:"environments_dir"`
			RunsDir         string `yaml:"runs_dir"`
		} `yaml:"paths"`
	} `yaml:"lynix"`
}
