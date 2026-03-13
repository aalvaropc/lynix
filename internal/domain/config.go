package domain

import "time"

// Config represents the minimal Lynix configuration loaded from lynix.yaml.
type Config struct {
	Masking   MaskingConfig
	Defaults  DefaultsConfig
	Paths     PathsConfig
	Artifacts ArtifactsConfig
	Run       RunConfig
}

// RunConfig holds runtime execution settings.
type RunConfig struct {
	Timeout    time.Duration
	Retries    int
	RetryDelay time.Duration
	Retry5xx   bool
}

// RedactionScope controls which surface a redaction rule applies to.
type RedactionScope string

const (
	RedactionScopeAll    RedactionScope = "all"
	RedactionScopeHeader RedactionScope = "header"
	RedactionScopeBody   RedactionScope = "body"
	RedactionScopeQuery  RedactionScope = "query"
)

// RedactionRule defines a custom sensitive-data pattern.
type RedactionRule struct {
	// Pattern is a case-insensitive substring to match against keys/field names.
	Pattern string

	// Scope limits where the pattern is applied ("all", "header", "body", "query").
	Scope RedactionScope
}

type MaskingConfig struct {
	Enabled bool

	// Per-surface toggles (all default to true when masking is enabled).
	MaskRequestHeaders bool
	MaskRequestBody    bool
	MaskResponseBody   bool
	MaskQueryParams    bool

	// ApplyToOutput controls whether masking also applies to CLI stdout output.
	// Default false: only artifacts in runs/ are masked.
	ApplyToOutput bool

	// Rules are custom redaction rules in addition to built-in defaults.
	Rules []RedactionRule
}

type DefaultsConfig struct {
	Environment string
}

type PathsConfig struct {
	CollectionsDir  string
	EnvironmentsDir string
	RunsDir         string
}

type ArtifactsConfig struct {
	SaveResponseHeaders bool
	SaveResponseBody    bool
}

// DefaultConfig provides sane defaults if lynix.yaml is partially missing.
func DefaultConfig() Config {
	return Config{
		Masking: MaskingConfig{
			Enabled:            true,
			MaskRequestHeaders: true,
			MaskRequestBody:    true,
			MaskResponseBody:   true,
			MaskQueryParams:    true,
			ApplyToOutput:      false,
		},
		Defaults: DefaultsConfig{
			Environment: "dev",
		},
		Paths: PathsConfig{
			CollectionsDir:  "collections",
			EnvironmentsDir: "env",
			RunsDir:         "runs",
		},
		Artifacts: ArtifactsConfig{
			SaveResponseHeaders: true,
			SaveResponseBody:    true,
		},
		Run: RunConfig{
			Timeout: 5 * time.Minute,
		},
	}
}
