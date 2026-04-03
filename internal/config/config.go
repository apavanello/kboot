package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config represents the ~/.kboot.yaml structure
type Config struct {
	Clusters            []Cluster `yaml:"clusters"`
	AWSCredentialsFile  string    `yaml:"aws_credentials_file,omitempty"`
	AWSConfigFile       string    `yaml:"aws_config_file,omitempty"`
	AWSSSOCacheDir      string    `yaml:"aws_sso_cache_dir,omitempty"`
	UseSystemAWS        bool      `yaml:"use_system_aws,omitempty"`
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
			return &Config{Clusters: []Cluster{}}, nil
		}
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	cfg.resolveDefaults()

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

// KbootDir returns the path to ~/.kboot/
func KbootDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".kboot"), nil
}

// AWSCredentialsPath returns the path to the AWS credentials file
func (c *Config) AWSCredentialsPath() string {
	if c.AWSCredentialsFile != "" {
		return c.AWSCredentialsFile
	}
	if c.UseSystemAWS {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".aws", "credentials")
	}
	kbootDir, _ := KbootDir()
	return filepath.Join(kbootDir, "aws", "credentials")
}

// AWSConfigPath returns the path to the AWS config file
func (c *Config) AWSConfigPath() string {
	if c.AWSConfigFile != "" {
		return c.AWSConfigFile
	}
	if c.UseSystemAWS {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".aws", "config")
	}
	kbootDir, _ := KbootDir()
	return filepath.Join(kbootDir, "aws", "config")
}

// AWSSSOCachePath returns the path to the SSO cache directory
func (c *Config) AWSSSOCachePath() string {
	if c.AWSSSOCacheDir != "" {
		return c.AWSSSOCacheDir
	}
	if c.UseSystemAWS {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".aws", "sso", "cache")
	}
	kbootDir, _ := KbootDir()
	return filepath.Join(kbootDir, "aws", "sso", "cache")
}

func (c *Config) resolveDefaults() {
	if c.AWSCredentialsFile == "" && !c.UseSystemAWS {
		kbootDir, err := KbootDir()
		if err == nil {
			c.AWSCredentialsFile = filepath.Join(kbootDir, "aws", "credentials")
		}
	}
	if c.AWSConfigFile == "" && !c.UseSystemAWS {
		kbootDir, err := KbootDir()
		if err == nil {
			c.AWSConfigFile = filepath.Join(kbootDir, "aws", "config")
		}
	}
	if c.AWSSSOCacheDir == "" && !c.UseSystemAWS {
		kbootDir, err := KbootDir()
		if err == nil {
			c.AWSSSOCacheDir = filepath.Join(kbootDir, "aws", "sso", "cache")
		}
	}
}
