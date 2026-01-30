package yamlenv

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/aalvaropc/lynix/internal/domain"
	"github.com/aalvaropc/lynix/internal/ports"
	"gopkg.in/yaml.v3"
)

type Loader struct {
	rootDir     string
	envDir      string
	secretsFile string
}

type Option func(*Loader)

func WithEnvDir(dir string) Option {
	return func(l *Loader) { l.envDir = dir }
}

func WithSecretsFile(name string) Option {
	return func(l *Loader) { l.secretsFile = name }
}

func NewLoader(root string, opts ...Option) *Loader {
	l := &Loader{
		rootDir:     root,
		envDir:      "env",
		secretsFile: "secrets.local.yaml",
	}
	for _, opt := range opts {
		opt(l)
	}
	return l
}

var _ ports.EnvironmentLoader = (*Loader)(nil)

// LoadEnvironment accepts either an env name (e.g., "dev") or a full path to a YAML file.
func (l *Loader) LoadEnvironment(nameOrPath string) (domain.Environment, error) {
	var envPath string
	var envName string

	if strings.HasSuffix(nameOrPath, ".yaml") || strings.HasSuffix(nameOrPath, ".yml") || strings.Contains(nameOrPath, string(filepath.Separator)) {
		envPath = filepath.Clean(nameOrPath)
		envName = strings.TrimSuffix(filepath.Base(envPath), filepath.Ext(envPath))
	} else {
		envName = nameOrPath
		envPath = filepath.Join(l.rootDir, l.envDir, envName+".yaml")
	}

	base, err := readVars(envPath)
	if err != nil {
		return domain.Environment{}, err
	}

	// Secrets are optional; they override base vars.
	secretsPath := filepath.Join(filepath.Dir(envPath), l.secretsFile)
	secrets, secErr := readVarsOptional(secretsPath)
	if secErr != nil {
		return domain.Environment{}, secErr
	}

	merged := domain.Vars{}
	for k, v := range base {
		merged[k] = v
	}
	for k, v := range secrets {
		merged[k] = v
	}

	return domain.Environment{
		Name: envName,
		Vars: merged,
	}, nil
}

type yamlEnv struct {
	Vars map[string]string `yaml:"vars"`
}

func readVars(path string) (domain.Vars, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, &domain.OpError{
			Op:   "yamlenv.load",
			Kind: domain.KindNotFound,
			Path: path,
			Err:  err,
		}
	}

	var y yamlEnv
	if err := yaml.Unmarshal(b, &y); err != nil {
		return nil, &domain.OpError{
			Op:   "yamlenv.load",
			Kind: domain.KindInvalidConfig,
			Path: path,
			Err:  err,
		}
	}

	if y.Vars == nil {
		y.Vars = map[string]string{}
	}

	return domain.Vars(y.Vars), nil
}

func readVarsOptional(path string) (domain.Vars, error) {
	_, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return domain.Vars{}, nil
		}
		return nil, &domain.OpError{
			Op:   "yamlenv.secrets",
			Kind: domain.KindExecution,
			Path: path,
			Err:  err,
		}
	}

	v, err := readVars(path)
	if err != nil {
		return nil, fmt.Errorf("failed to load secrets: %w", err)
	}
	return v, nil
}
