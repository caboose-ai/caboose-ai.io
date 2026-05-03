package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Domain       string           `yaml:"domain"`
	Email        string           `yaml:"email"`
	ComposeDir   string           `yaml:"compose_dir"`
	Orchestrator string           `yaml:"orchestrator"`
	Kubernetes   KubernetesConfig `yaml:"kubernetes"`
	N8NUser      string           `yaml:"n8n_user"`
	Social       SocialConfig     `yaml:"social"`
	OPVault      string           `yaml:"op_vault"`

	DryRun         bool `yaml:"-"`
	Force          bool `yaml:"-"`
	Verbose        bool `yaml:"-"`
	NonInteractive bool `yaml:"-"`
}

type KubernetesConfig struct {
	Namespace  string `yaml:"namespace"`
	Kubeconfig string `yaml:"kubeconfig,omitempty"`
	Context    string `yaml:"context,omitempty"`
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
		ComposeDir:   "dev/homelab",
		Orchestrator: "compose",
		Kubernetes:   KubernetesConfig{Namespace: "homelab"},
		OPVault:      "Homelab",
		N8NUser:      "admin",
		Email:        "cxm6467@gmail.com",
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
	if c.Orchestrator == "" {
		c.Orchestrator = "compose"
	}
	if c.Kubernetes.Namespace == "" {
		c.Kubernetes.Namespace = "homelab"
	}
	if c.Orchestrator != "compose" && c.Orchestrator != "kubernetes" {
		return fmt.Errorf("orchestrator must be one of: compose, kubernetes")
	}
	return nil
}

func (c *Config) URLs() URLs {
	return DeriveURLs(c.Domain)
}

func (c *Config) EnvPath() string {
	return c.ComposeDir + "/.env"
}
