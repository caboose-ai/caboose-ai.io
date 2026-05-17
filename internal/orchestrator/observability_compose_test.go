package orchestrator

import (
	"os"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestGrafanaUsesHostNetworkForHostPrometheus(t *testing.T) {
	data, err := os.ReadFile("../../dev/homelab/docker-compose.yml")
	if err != nil {
		t.Fatalf("read compose: %v", err)
	}

	var compose struct {
		Services map[string]struct {
			NetworkMode string            `yaml:"network_mode"`
			Environment map[string]string `yaml:"environment"`
			ExtraHosts  []string          `yaml:"extra_hosts"`
			Ports       []string          `yaml:"ports"`
			Networks    []string          `yaml:"networks"`
		} `yaml:"services"`
	}
	if err := yaml.Unmarshal(data, &compose); err != nil {
		t.Fatalf("parse compose: %v", err)
	}

	grafana, ok := compose.Services["grafana"]
	if !ok {
		t.Fatal("grafana service missing")
	}
	if grafana.NetworkMode != "host" {
		t.Fatalf("grafana network_mode = %q, want host", grafana.NetworkMode)
	}
	if len(grafana.Ports) != 0 {
		t.Fatalf("grafana ports = %v, want none with host networking", grafana.Ports)
	}
	if len(grafana.Networks) != 0 {
		t.Fatalf("grafana networks = %v, want none with host networking", grafana.Networks)
	}
	if len(grafana.ExtraHosts) != 0 {
		t.Fatalf("grafana extra_hosts = %v, want none with host networking", grafana.ExtraHosts)
	}
	if grafana.Environment["GF_SERVER_HTTP_ADDR"] != "${HOMELAB_BIND_ADDRESS:-127.0.0.1}" {
		t.Fatalf("GF_SERVER_HTTP_ADDR = %q", grafana.Environment["GF_SERVER_HTTP_ADDR"])
	}
	if grafana.Environment["GF_SERVER_HTTP_PORT"] != "3001" {
		t.Fatalf("GF_SERVER_HTTP_PORT = %q", grafana.Environment["GF_SERVER_HTTP_PORT"])
	}
}

func TestLokiPublishesLoopbackForHostNetworkGrafana(t *testing.T) {
	data, err := os.ReadFile("../../dev/homelab/docker-compose.yml")
	if err != nil {
		t.Fatalf("read compose: %v", err)
	}

	var compose struct {
		Services map[string]struct {
			Ports []string `yaml:"ports"`
		} `yaml:"services"`
	}
	if err := yaml.Unmarshal(data, &compose); err != nil {
		t.Fatalf("parse compose: %v", err)
	}

	loki, ok := compose.Services["loki"]
	if !ok {
		t.Fatal("loki service missing")
	}
	if !contains(loki.Ports, "${HOMELAB_BIND_ADDRESS:-127.0.0.1}:3101:3100") {
		t.Fatalf("loki ports = %v, missing loopback 3101 mapping", loki.Ports)
	}
}

func TestGrafanaDatasourcesUseHostLoopback(t *testing.T) {
	for _, tt := range []struct {
		path string
		want string
	}{
		{path: "../../dev/homelab/grafana/provisioning/datasources/prometheus.yml", want: "http://127.0.0.1:9090"},
		{path: "../../dev/homelab/grafana/provisioning/datasources/loki.yml", want: "http://127.0.0.1:3101"},
	} {
		t.Run(tt.path, func(t *testing.T) {
			data, err := os.ReadFile(tt.path)
			if err != nil {
				t.Fatalf("read datasource: %v", err)
			}
			var cfg struct {
				Datasources []struct {
					URL string `yaml:"url"`
				} `yaml:"datasources"`
			}
			if err := yaml.Unmarshal(data, &cfg); err != nil {
				t.Fatalf("parse datasource: %v", err)
			}
			if len(cfg.Datasources) != 1 {
				t.Fatalf("datasources = %d, want 1", len(cfg.Datasources))
			}
			if cfg.Datasources[0].URL != tt.want {
				t.Fatalf("datasource url = %q, want %q", cfg.Datasources[0].URL, tt.want)
			}
		})
	}
}

func TestPrometheusDoesNotScrapeMattermostTeamEditionMetrics(t *testing.T) {
	data, err := os.ReadFile("../../dev/homelab/prometheus.yml")
	if err != nil {
		t.Fatalf("read prometheus config: %v", err)
	}

	var cfg struct {
		ScrapeConfigs []struct {
			JobName string `yaml:"job_name"`
		} `yaml:"scrape_configs"`
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("parse prometheus config: %v", err)
	}

	for _, scrape := range cfg.ScrapeConfigs {
		if scrape.JobName == "mattermost" {
			t.Fatal("mattermost Team Edition does not expose HTTP /metrics; do not ship a permanently down scrape target")
		}
	}
}

func TestMattermostDoesNotPublishTeamEditionProfilingAsMetrics(t *testing.T) {
	data, err := os.ReadFile("../../dev/homelab/docker-compose.yml")
	if err != nil {
		t.Fatalf("read compose: %v", err)
	}

	var compose struct {
		Services map[string]struct {
			Environment map[string]string `yaml:"environment"`
			Ports       []string          `yaml:"ports"`
		} `yaml:"services"`
	}
	if err := yaml.Unmarshal(data, &compose); err != nil {
		t.Fatalf("parse compose: %v", err)
	}

	mattermost, ok := compose.Services["mattermost"]
	if !ok {
		t.Fatal("mattermost service missing")
	}
	if contains(mattermost.Ports, "${HOMELAB_BIND_ADDRESS:-127.0.0.1}:8067:8067") {
		t.Fatal("mattermost Team Edition profiling listener must not be published as a metrics port")
	}
	if mattermost.Environment["MM_METRICSSETTINGS_ENABLE"] == "true" {
		t.Fatal("mattermost Team Edition metrics env must stay disabled unless the image/license exposes /metrics")
	}
}
