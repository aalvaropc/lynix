package domain

// Config represents the minimal Lynix configuration loaded from lynix.yaml.
type Config struct {
	Masking   MaskingConfig
	Defaults  DefaultsConfig
	Paths     PathsConfig
	Artifacts ArtifactsConfig
}

type MaskingConfig struct {
	Enabled bool
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
		Masking: MaskingConfig{Enabled: true},
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
	}
}
