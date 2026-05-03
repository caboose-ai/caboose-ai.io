package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Domain     string        `yaml:"domain"`
	Email      string        `yaml:"email"`
	ComposeDir string        `yaml:"compose_dir"`
	N8NUser    string        `yaml:"n8n_user"`
	Social     SocialConfig  `yaml:"social"`
	OPVault    string        `yaml:"op_vault"`

	DryRun         bool `yaml:"-"`
	Force          bool `yaml:"-"`
	Verbose        bool `yaml:"-"`
	NonInteractive bool `yaml:"-"`
}

type SocialConfig struct {
	GitHub *OAuthCredentials `yaml:"github,omitempty"`
	Google *OAuthCredentials `yaml:"google,omitempty"`
}

type OAuthCredentials struct {
	ClientID     string `yaml:"client_id"`
	ClientSecret string `yaml:"client_secret"`
}

func DefaultConfig() *Config {
	return &Config{
		ComposeDir: "dev/homelab",
		OPVault:    "Homelab",
		N8NUser:    "admin",
	}
}

func LoadFromFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}
	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}
	return cfg, nil
}

func (c *Config) Validate() error {
	if c.Domain == "" {
		return fmt.Errorf("domain is required")
	}
	return nil
}

func (c *Config) URLs() URLs {
	return DeriveURLs(c.Domain)
}

func (c *Config) EnvPath() string {
	return c.ComposeDir + "/.env"
}
