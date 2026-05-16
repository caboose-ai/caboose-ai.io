package install

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/caboose-ai/caboose-ai.io/internal/paperclip"
	"github.com/caboose-ai/caboose-ai.io/internal/runner"
)

const defaultPaperclipRepo = "/home/caboose/dev/caboose-ai.io"

type PaperclipBootstrapOptions struct {
	APIBase       string
	APIKey        string
	Repo          string
	Delivery      paperclip.InternalDeliveryConfig
	HealthTimeout time.Duration
}

func (inst *Installer) BootstrapPaperclip(ctx context.Context, opts PaperclipBootstrapOptions) (*paperclip.SeedReport, error) {
	if inst.State.DryRun {
		return &paperclip.SeedReport{Created: map[string]int{}, Existing: map[string]int{}}, nil
	}
	if inst.Config.Orchestrator != "" && inst.Config.Orchestrator != "compose" {
		return nil, fmt.Errorf("paperclip bootstrap requires compose orchestrator, got %q", inst.Config.Orchestrator)
	}

	if _, err := inst.Compose.UpProfileServices(ctx, "paperclip", "paperclip", "paperclip-db"); err != nil {
		return nil, fmt.Errorf("starting Paperclip profile: %w", err)
	}

	apiBase := strings.TrimRight(opts.APIBase, "/")
	if apiBase == "" {
		apiBase = "http://127.0.0.1:3100"
	}
	timeout := opts.HealthTimeout
	if timeout == 0 {
		timeout = 2 * time.Minute
	}
	if err := waitPaperclipHealthy(ctx, inst.HTTP, apiBase, timeout); err != nil {
		return nil, err
	}

	apiKey := opts.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("PAPERCLIP_API_KEY")
	}
	if apiKey == "" && inst.Secrets != nil {
		apiKey, _ = inst.Secrets.Get(ctx, "PAPERCLIP_API_KEY")
	}

	repo := opts.Repo
	if repo == "" {
		repo = os.Getenv("PAPERCLIP_WORKSPACE_ROOT")
	}
	if repo == "" {
		repo = defaultPaperclipRepo
	}

	delivery := opts.Delivery
	if delivery == (paperclip.InternalDeliveryConfig{}) {
		delivery = paperclip.DefaultInternalDeliveryConfig(inst.Config.Domain)
	}

	seedHTTP := paperclipHTTPClient(inst.HTTP)
	client := paperclip.NewClient(apiBase, apiKey, seedHTTP)
	report, err := paperclip.SeedSoftwareShopWithDelivery(ctx, client, repo, delivery)
	if err != nil {
		return nil, fmt.Errorf("seeding Paperclip software shop: %w", err)
	}
	return report, nil
}

func paperclipHTTPClient(client runner.HTTPClient) *http.Client {
	if typed, ok := client.(*http.Client); ok && typed != nil {
		return typed
	}
	return &http.Client{Timeout: 30 * time.Second}
}

func waitPaperclipHealthy(ctx context.Context, client runner.HTTPClient, apiBase string, timeout time.Duration) error {
	if client == nil {
		client = http.DefaultClient
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	var lastErr error
	for {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiBase+"/api/health", nil)
		if err != nil {
			return err
		}
		resp, err := client.Do(req)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode < http.StatusBadRequest {
				return nil
			}
			lastErr = fmt.Errorf("Paperclip health returned HTTP %d", resp.StatusCode)
		} else {
			lastErr = err
		}

		select {
		case <-ctx.Done():
			if lastErr != nil {
				return fmt.Errorf("waiting for Paperclip health: %w", lastErr)
			}
			return fmt.Errorf("waiting for Paperclip health: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}
