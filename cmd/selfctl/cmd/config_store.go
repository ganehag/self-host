package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	homedir "github.com/mitchellh/go-homedir"
	"gopkg.in/yaml.v2"
)

type selfctlConfig struct {
	API apiConfigSection `yaml:"api"`
}

type apiConfigSection struct {
	Server string `yaml:"server,omitempty"`
	Domain string `yaml:"domain,omitempty"`
	Token  string `yaml:"token,omitempty"`
}

func resolveConfigPath() (string, error) {
	if cfgFile != "" {
		return cfgFile, nil
	}
	home, err := homedir.Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".selfctl", "config.yaml"), nil
}

func loadSelfctlConfig() (selfctlConfig, string, error) {
	path, err := resolveConfigPath()
	if err != nil {
		return selfctlConfig{}, "", err
	}
	cfg := selfctlConfig{}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, path, nil
		}
		return selfctlConfig{}, "", err
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return selfctlConfig{}, "", fmt.Errorf("read config %s: %w", path, err)
	}
	return cfg, path, nil
}

func saveSelfctlConfig(cfg selfctlConfig) (string, error) {
	path, err := resolveConfigPath()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	data, err := yaml.Marshal(&cfg)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", err
	}
	return path, nil
}
