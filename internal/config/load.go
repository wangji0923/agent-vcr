package config

import (
	"errors"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const defaultConfigPath = ".agent-vcr/config.yml"

// Load starts from Default and applies an optional project config file.
// The returned path is empty when no config file was found.
func Load(projectDir string, explicitPath string) (Config, string, error) {
	if projectDir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return Config{}, "", err
		}
		projectDir = cwd
	}

	cfg := Default()
	configPath := explicitPath
	if configPath == "" {
		configPath = filepath.Join(projectDir, defaultConfigPath)
	} else if !filepath.IsAbs(configPath) {
		configPath = filepath.Join(projectDir, configPath)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, "", Validate(cfg)
		}
		return Config{}, configPath, err
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, configPath, err
	}
	if err := Validate(cfg); err != nil {
		return Config{}, configPath, err
	}
	return cfg, configPath, nil
}
