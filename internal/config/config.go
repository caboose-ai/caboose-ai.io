package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Domain        string           `yaml:"domain"`
	Email         string           `yaml:"email"`
	ComposeDir    string           `yaml:"compose_dir"`
	Orchestrator  string           `yaml:"orchestrator"`
	ServeMode     string           `yaml:"serve_mode"`
	Kubernetes    KubernetesConfig `yaml:"kubernetes"`
	Social        SocialConfig     `yaml:"social"`
	Turnstile     TurnstileConfig  `yaml:"turnstile"`
	OPVault       string           `yaml:"op_vault"`
	OPStaticVault string           `yaml:"op_static_vault"`

	DryRun         bool `yaml:"-"`
	Force          bool `yaml:"-"`
	Verbose        bool `yaml:"-"`
	NonInteractive bool `yaml:"-"`
	SecretsEnvOnly bool `yaml:"-"`
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

type TurnstileConfig struct {
	SiteKey   string `yaml:"site_key"`
	SecretKey string `yaml:"secret_key"`
}

type Overrides struct {
	Domain         string
	Email          string
	ComposeDir     string
	Orchestrator   string
	ServeMode      string
	OPVault        string
	OPStaticVault  string
	KubeNamespace  string
	Kubeconfig     string
	KubeContext    string
	DryRun         bool
	Force          bool
	Verbose        bool
	NonInteractive bool
	SecretsEnvOnly bool
}

func DefaultConfig() *Config {
	return &Config{
		ComposeDir:    "dev/homelab",
		Orchestrator:  "compose",
		ServeMode:     "public",
		Kubernetes:    KubernetesConfig{Namespace: "homelab"},
		OPVault:       "Homelab - Dynamic",
		OPStaticVault: "Homelab - Static",
		Email:         "cxm6467@gmail.com",
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

func LoadWithOverrides(path string, overrides Overrides) (*Config, error) {
	cfg := DefaultConfig()
	if path != "" {
		loaded, err := LoadFromFile(path)
		if err != nil {
			return nil, err
		}
		cfg = loaded
	}
	cfg.ApplyOverrides(overrides)
	return cfg, nil
}

func (c *Config) ApplyOverrides(overrides Overrides) {
	if overrides.Domain != "" {
		c.Domain = overrides.Domain
	}
	if overrides.Email != "" {
		c.Email = overrides.Email
	}
	if overrides.ComposeDir != "" {
		c.ComposeDir = overrides.ComposeDir
	}
	if overrides.Orchestrator != "" {
		c.Orchestrator = overrides.Orchestrator
	}
	if overrides.ServeMode != "" {
		c.ServeMode = overrides.ServeMode
	}
	if overrides.OPVault != "" {
		c.OPVault = overrides.OPVault
	}
	if overrides.OPStaticVault != "" {
		c.OPStaticVault = overrides.OPStaticVault
	}
	if overrides.KubeNamespace != "" {
		c.Kubernetes.Namespace = overrides.KubeNamespace
	}
	if overrides.Kubeconfig != "" {
		c.Kubernetes.Kubeconfig = overrides.Kubeconfig
	}
	if overrides.KubeContext != "" {
		c.Kubernetes.Context = overrides.KubeContext
	}
	c.DryRun = overrides.DryRun
	c.Force = overrides.Force
	c.Verbose = overrides.Verbose
	c.NonInteractive = overrides.NonInteractive
	c.SecretsEnvOnly = overrides.SecretsEnvOnly
}

func (c *Config) Validate() error {
	if c.Domain == "" {
		return fmt.Errorf("domain is required")
	}
	if c.Orchestrator == "" {
		c.Orchestrator = "compose"
	}
	if c.ServeMode == "" {
		c.ServeMode = "public"
	}
	if c.Kubernetes.Namespace == "" {
		c.Kubernetes.Namespace = "homelab"
	}
	if c.Orchestrator != "compose" && c.Orchestrator != "kubernetes" {
		return fmt.Errorf("orchestrator must be one of: compose, kubernetes")
	}
	if c.ServeMode != "public" && c.ServeMode != "local" {
		return fmt.Errorf("serve_mode must be one of: public, local")
	}
	return nil
}

func (c *Config) BindAddress() string {
	if c.ServeMode == "local" {
		return "0.0.0.0"
	}
	return "127.0.0.1"
}

func (c *Config) URLs() URLs {
	return DeriveURLs(c.Domain)
}

func (c *Config) EnvPath() string {
	return c.ComposeDir + "/.env"
}
