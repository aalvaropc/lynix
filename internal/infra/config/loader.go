package config

import (
	"os"

	"github.com/aalvaropc/lynix/internal/domain"
	"gopkg.in/yaml.v3"
)

func LoadCollection(path string) (domain.Collection, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return domain.Collection{}, &domain.OpError{
			Op:   "config.load_collection",
			Kind: domain.KindNotFound,
			Path: path,
			Err:  err,
		}
	}

	var dto YAMLCollection
	if err := yaml.Unmarshal(b, &dto); err != nil {
		return domain.Collection{}, &domain.OpError{
			Op:   "config.load_collection",
			Kind: domain.KindInvalidConfig,
			Path: path,
			Err:  err,
		}
	}

	return MapCollection(path, dto)
}

func LoadEnvironment(path string) (domain.Environment, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return domain.Environment{}, &domain.OpError{
			Op:   "config.load_environment",
			Kind: domain.KindNotFound,
			Path: path,
			Err:  err,
		}
	}

	var dto YAMLEnvironment
	if err := yaml.Unmarshal(b, &dto); err != nil {
		return domain.Environment{}, &domain.OpError{
			Op:   "config.load_environment",
			Kind: domain.KindInvalidConfig,
			Path: path,
			Err:  err,
		}
	}

	return MapEnvironment(path, dto)
}
