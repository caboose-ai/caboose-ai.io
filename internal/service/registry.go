package service

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type Manifest struct {
	Slug            string     `json:"slug" yaml:"slug"`
	DisplayName     string     `json:"display_name" yaml:"display_name"`
	URLKey          string     `json:"url_key" yaml:"url_key"`
	ComposeServices []string   `json:"compose_services" yaml:"compose_services"`
	Secrets         []string   `json:"secrets" yaml:"secrets"`
	Configurator    string     `json:"configurator" yaml:"configurator"`
	SmokeFlow       string     `json:"smoke_flow" yaml:"smoke_flow"`
	Health          HealthSpec `json:"health" yaml:"health"`
	Docs            []string   `json:"docs" yaml:"docs"`
}

type HealthSpec struct {
	URLKey string `json:"url_key,omitempty" yaml:"url_key,omitempty"`
	Path   string `json:"path,omitempty" yaml:"path,omitempty"`
	Kind   string `json:"kind,omitempty" yaml:"kind,omitempty"`
}

type Registry struct {
	manifests     map[string]Manifest
	configurators map[string]ServiceConfigurator
}

func NewRegistry(manifests map[string]Manifest, configurators []ServiceConfigurator) *Registry {
	normalized := make(map[string]Manifest, len(manifests))
	for slug, manifest := range manifests {
		if manifest.Slug == "" {
			manifest.Slug = slug
		}
		normalized[strings.ToLower(manifest.Slug)] = manifest
	}

	bySlug := make(map[string]ServiceConfigurator, len(configurators))
	for _, configurator := range configurators {
		if configurator == nil {
			continue
		}
		bySlug[strings.ToLower(configurator.Slug())] = configurator
	}

	return &Registry{manifests: normalized, configurators: bySlug}
}

func (r *Registry) Manifest(slug string) (Manifest, bool) {
	manifest, ok := r.manifests[strings.ToLower(slug)]
	return manifest, ok
}

func (r *Registry) Configurator(slug string) (ServiceConfigurator, bool) {
	configurator, ok := r.configurators[strings.ToLower(slug)]
	return configurator, ok
}

func (r *Registry) Slugs() []string {
	slugs := make([]string, 0, len(r.manifests))
	for slug := range r.manifests {
		slugs = append(slugs, slug)
	}
	sort.Strings(slugs)
	return slugs
}

func (r *Registry) ConfigurableSlugs() []string {
	slugs := make([]string, 0, len(r.configurators))
	for slug := range r.configurators {
		slugs = append(slugs, slug)
	}
	sort.Strings(slugs)
	return slugs
}

func LoadManifests(root string) (map[string]Manifest, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("reading service manifest root: %w", err)
	}

	manifests := make(map[string]Manifest)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		path := filepath.Join(root, entry.Name(), "service.yaml")
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("reading %s: %w", path, err)
		}
		var manifest Manifest
		if err := yaml.Unmarshal(data, &manifest); err != nil {
			return nil, fmt.Errorf("parsing %s: %w", path, err)
		}
		if manifest.Slug == "" {
			manifest.Slug = entry.Name()
		}
		if manifest.Slug != entry.Name() {
			return nil, fmt.Errorf("%s: slug %q must match directory %q", path, manifest.Slug, entry.Name())
		}
		manifests[manifest.Slug] = manifest
	}
	return manifests, nil
}

func FindManifestRoot(start string) (string, error) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	for {
		candidate := filepath.Join(dir, "services")
		if stat, err := os.Stat(candidate); err == nil && stat.IsDir() {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("services directory not found from %s", start)
		}
		dir = parent
	}
}
