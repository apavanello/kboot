package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config represents the ~/.kboot.yaml structure
type Config struct {
	Clusters []Cluster `yaml:"clusters"`
}

type Cluster struct {
	Alias    string `yaml:"alias"`
	Profile  string `yaml:"profile"`
	Region   string `yaml:"region"`
	Name     string `yaml:"name"`
	Optional bool   `yaml:"optional"`
}

// Load reads the config from ~/.kboot.yaml
func Load() (*Config, error) {
	path, err := GetPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// If not exists, return empty config but no error (first run)
			return &Config{Clusters: []Cluster{}}, nil
		}
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return &cfg, nil
}

// Save writes the config to ~/.kboot.yaml
func Save(cfg *Config) error {
	path, err := GetPath()
	if err != nil {
		return err
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	encoder := yaml.NewEncoder(f)
	encoder.SetIndent(2)
	return encoder.Encode(cfg)
}

// GetPath returns the secure location of the config file
func GetPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".kboot.yaml"), nil
}
