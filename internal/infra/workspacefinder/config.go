package workspacefinder

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

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
			Err:  fmt.Errorf("%w: %w", domain.ErrNotFound, err),
		}
	}

	var y yamlConfig
	if err := yaml.Unmarshal(b, &y); err != nil {
		return cfg, &domain.OpError{
			Op:   "workspacefinder.loadconfig",
			Kind: domain.KindInvalidConfig,
			Path: path,
			Err:  fmt.Errorf("%w: %w", domain.ErrInvalidConfig, err),
		}
	}

	// Apply parsed values on top of defaults.
	if y.Lynix.Masking.Enabled != nil {
		cfg.Masking.Enabled = *y.Lynix.Masking.Enabled
	}
	if y.Lynix.Masking.MaskRequestHeaders != nil {
		cfg.Masking.MaskRequestHeaders = *y.Lynix.Masking.MaskRequestHeaders
	}
	if y.Lynix.Masking.MaskRequestBody != nil {
		cfg.Masking.MaskRequestBody = *y.Lynix.Masking.MaskRequestBody
	}
	if y.Lynix.Masking.MaskResponseBody != nil {
		cfg.Masking.MaskResponseBody = *y.Lynix.Masking.MaskResponseBody
	}
	if y.Lynix.Masking.MaskQueryParams != nil {
		cfg.Masking.MaskQueryParams = *y.Lynix.Masking.MaskQueryParams
	}
	if y.Lynix.Masking.ApplyToOutput != nil {
		cfg.Masking.ApplyToOutput = *y.Lynix.Masking.ApplyToOutput
	}
	for _, r := range y.Lynix.Masking.Rules {
		scope := domain.RedactionScope(r.Scope)
		if scope == "" {
			scope = domain.RedactionScopeAll
		}
		cfg.Masking.Rules = append(cfg.Masking.Rules, domain.RedactionRule{
			Pattern: r.Pattern,
			Scope:   scope,
		})
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
	if y.Lynix.Artifacts.SaveResponseHeaders != nil {
		cfg.Artifacts.SaveResponseHeaders = *y.Lynix.Artifacts.SaveResponseHeaders
	}
	if y.Lynix.Artifacts.SaveResponseBody != nil {
		cfg.Artifacts.SaveResponseBody = *y.Lynix.Artifacts.SaveResponseBody
	}
	if y.Lynix.Run.TimeoutSeconds > 0 {
		cfg.Run.Timeout = time.Duration(y.Lynix.Run.TimeoutSeconds) * time.Second
	}
	if y.Lynix.Run.Retries != nil {
		cfg.Run.Retries = *y.Lynix.Run.Retries
	}
	if y.Lynix.Run.RetryDelayMS != nil {
		cfg.Run.RetryDelay = time.Duration(*y.Lynix.Run.RetryDelayMS) * time.Millisecond
	}
	if y.Lynix.Run.Retry5xx != nil {
		cfg.Run.Retry5xx = *y.Lynix.Run.Retry5xx
	}

	return cfg, nil
}

type yamlConfig struct {
	Lynix struct {
		Masking struct {
			Enabled            *bool `yaml:"enabled"`
			MaskRequestHeaders *bool `yaml:"mask_request_headers"`
			MaskRequestBody    *bool `yaml:"mask_request_body"`
			MaskResponseBody   *bool `yaml:"mask_response_body"`
			MaskQueryParams    *bool `yaml:"mask_query_params"`
			ApplyToOutput      *bool `yaml:"apply_to_output"`
			Rules              []struct {
				Pattern string `yaml:"pattern"`
				Scope   string `yaml:"scope"`
			} `yaml:"rules"`
		} `yaml:"masking"`

		Defaults struct {
			Env string `yaml:"env"`
		} `yaml:"defaults"`

		Paths struct {
			CollectionsDir  string `yaml:"collections_dir"`
			EnvironmentsDir string `yaml:"environments_dir"`
			RunsDir         string `yaml:"runs_dir"`
		} `yaml:"paths"`

		Artifacts struct {
			SaveResponseHeaders *bool `yaml:"save_response_headers"`
			SaveResponseBody    *bool `yaml:"save_response_body"`
		} `yaml:"artifacts"`

		Run struct {
			TimeoutSeconds int   `yaml:"timeout_seconds"`
			Retries        *int  `yaml:"retries"`
			RetryDelayMS   *int  `yaml:"retry_delay_ms"`
			Retry5xx       *bool `yaml:"retry_5xx"`
		} `yaml:"run"`
	} `yaml:"lynix"`
}
