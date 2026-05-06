package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/caboose-ai/caboose-ai.io/internal/config"
)

const defaultCloudflareAPIBaseURL = "https://api.cloudflare.com/client/v4"

type OAuthSetupOptions struct {
	CreateTurnstile      bool
	CloudflareAccountID  string
	CloudflareAPIToken   string
	CloudflareAPIBaseURL string
	HTTPClient           *http.Client
	TurnstileWidgetName  string
}

type OAuthSetupResult struct {
	Turnstile *TurnstileWidget
}

type TurnstileWidget struct {
	SiteKey   string
	SecretKey string
}

func PrintOAuthSetup(w io.Writer, cfg *config.Config) {
	printOAuthSetup(w, cfg, nil)
}

func RunOAuthSetup(ctx context.Context, w io.Writer, cfg *config.Config, opts OAuthSetupOptions) (*OAuthSetupResult, error) {
	if cfg == nil {
		return nil, errors.New("config is required")
	}

	result := &OAuthSetupResult{}
	if opts.CreateTurnstile {
		widget, err := createTurnstileWidget(ctx, cfg, opts)
		if err != nil {
			return nil, err
		}
		result.Turnstile = widget
		cfg.Turnstile.SiteKey = widget.SiteKey
		cfg.Turnstile.SecretKey = widget.SecretKey
	}

	printOAuthSetup(w, cfg, result)
	return result, nil
}

func printOAuthSetup(w io.Writer, cfg *config.Config, result *OAuthSetupResult) {
	urls := cfg.URLs()
	githubCallback := urls.Authentik + "/source/oauth/callback/github/"
	googleCallback := urls.Authentik + "/source/oauth/callback/google/"

	fmt.Fprintf(w, "External credential setup for %s\n\n", cfg.Domain)

	fmt.Fprintln(w, "GitHub OAuth App")
	fmt.Fprintln(w, "  Create/edit: https://github.com/settings/developers")
	fmt.Fprintln(w, "  API create: not available for normal OAuth App registration")
	fmt.Fprintf(w, "  Homepage URL: %s\n", urls.Dashboard)
	fmt.Fprintf(w, "  Authorization callback URL: %s\n", githubCallback)
	fmt.Fprintln(w, "  Store as:")
	fmt.Fprintln(w, "    GITHUB_OAUTH_CLIENT_ID")
	fmt.Fprintln(w, "    GITHUB_OAUTH_CLIENT_SECRET")
	fmt.Fprintln(w)

	fmt.Fprintln(w, "Google OAuth Client")
	fmt.Fprintln(w, "  Create/edit: https://console.cloud.google.com/apis/credentials")
	fmt.Fprintln(w, "  API create: not available for this public web OAuth client flow")
	fmt.Fprintln(w, "  Application type: Web application")
	fmt.Fprintf(w, "  Authorized redirect URI: %s\n", googleCallback)
	fmt.Fprintf(w, "  Authorized JavaScript origin, if requested: %s\n", urls.Authentik)
	fmt.Fprintln(w, "  Store as:")
	fmt.Fprintln(w, "    GOOGLE_OAUTH_CLIENT_ID")
	fmt.Fprintln(w, "    GOOGLE_OAUTH_CLIENT_SECRET")
	fmt.Fprintln(w)

	fmt.Fprintln(w, "Cloudflare Turnstile")
	if result != nil && result.Turnstile != nil {
		fmt.Fprintln(w, "  Created via Cloudflare API")
	} else {
		fmt.Fprintln(w, "  Create/edit: https://dash.cloudflare.com/?to=/:account/turnstile")
		fmt.Fprintln(w, "  API create: homelab oauth-setup --domain <domain> --create-turnstile")
	}
	fmt.Fprintf(w, "  Hostname: auth.%s\n", cfg.Domain)
	fmt.Fprintln(w, "  Store as:")
	if result != nil && result.Turnstile != nil {
		fmt.Fprintf(w, "    TURNSTILE_SITE_KEY=%s\n", result.Turnstile.SiteKey)
		fmt.Fprintf(w, "    TURNSTILE_SECRET_KEY=%s\n", result.Turnstile.SecretKey)
		fmt.Fprintln(w, "  The secret key is shown here for storage; do not commit it.")
	} else {
		fmt.Fprintln(w, "    TURNSTILE_SITE_KEY")
		fmt.Fprintln(w, "    TURNSTILE_SECRET_KEY")
	}
	fmt.Fprintln(w)

	fmt.Fprintln(w, "Optional config YAML shape")
	fmt.Fprintln(w, "  social:")
	fmt.Fprintln(w, "    github:")
	fmt.Fprintln(w, "      client_id: <GITHUB_OAUTH_CLIENT_ID>")
	fmt.Fprintln(w, "      client_secret: <GITHUB_OAUTH_CLIENT_SECRET>")
	fmt.Fprintln(w, "    google:")
	fmt.Fprintln(w, "      client_id: <GOOGLE_OAUTH_CLIENT_ID>")
	fmt.Fprintln(w, "      client_secret: <GOOGLE_OAUTH_CLIENT_SECRET>")
	fmt.Fprintln(w, "  turnstile:")
	fmt.Fprintln(w, "    site_key: <TURNSTILE_SITE_KEY>")
	fmt.Fprintln(w, "    secret_key: <TURNSTILE_SECRET_KEY>")
	fmt.Fprintln(w)

	fmt.Fprintln(w, "The installer also accepts these values from 1Password or prompts. Blank optional values skip that provider.")
}

func createTurnstileWidget(ctx context.Context, cfg *config.Config, opts OAuthSetupOptions) (*TurnstileWidget, error) {
	if opts.CloudflareAccountID == "" {
		return nil, errors.New("cloudflare account id is required for --create-turnstile")
	}
	if opts.CloudflareAPIToken == "" {
		return nil, errors.New("cloudflare api token is required for --create-turnstile")
	}

	baseURL := strings.TrimRight(opts.CloudflareAPIBaseURL, "/")
	if baseURL == "" {
		baseURL = defaultCloudflareAPIBaseURL
	}

	client := opts.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}

	name := opts.TurnstileWidgetName
	if name == "" {
		name = defaultTurnstileWidgetName(cfg.Domain)
	}

	payload := turnstileWidgetCreateRequest{
		Domains: []string{"auth." + cfg.Domain},
		Mode:    "managed",
		Name:    name,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encoding turnstile widget request: %w", err)
	}

	endpoint := fmt.Sprintf("%s/accounts/%s/challenges/widgets", baseURL, url.PathEscape(opts.CloudflareAccountID))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating cloudflare request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+opts.CloudflareAPIToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("creating turnstile widget: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading cloudflare response: %w", err)
	}

	var envelope cloudflareTurnstileCreateEnvelope
	if err := json.Unmarshal(respBody, &envelope); err != nil {
		return nil, fmt.Errorf("decoding cloudflare response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 || !envelope.Success {
		return nil, fmt.Errorf("cloudflare turnstile create failed: %s", formatCloudflareErrors(envelope.Errors, resp.Status))
	}
	if envelope.Result.SiteKey == "" || envelope.Result.Secret == "" {
		return nil, errors.New("cloudflare turnstile create response omitted sitekey or secret")
	}

	return &TurnstileWidget{
		SiteKey:   envelope.Result.SiteKey,
		SecretKey: envelope.Result.Secret,
	}, nil
}

func defaultTurnstileWidgetName(domain string) string {
	if domain == "" {
		return "Authentik login"
	}
	return "auth." + domain + " Authentik"
}

func formatCloudflareErrors(messages []cloudflareMessage, fallback string) string {
	if len(messages) == 0 {
		return fallback
	}

	parts := make([]string, 0, len(messages))
	for _, msg := range messages {
		if msg.Message == "" {
			continue
		}
		if msg.Code != 0 {
			parts = append(parts, fmt.Sprintf("%d: %s", msg.Code, msg.Message))
		} else {
			parts = append(parts, msg.Message)
		}
	}
	if len(parts) == 0 {
		return fallback
	}
	return strings.Join(parts, "; ")
}

type turnstileWidgetCreateRequest struct {
	Domains []string `json:"domains"`
	Mode    string   `json:"mode"`
	Name    string   `json:"name"`
}

type cloudflareTurnstileCreateEnvelope struct {
	Success bool                `json:"success"`
	Errors  []cloudflareMessage `json:"errors"`
	Result  struct {
		SiteKey string `json:"sitekey"`
		Secret  string `json:"secret"`
	} `json:"result"`
}

type cloudflareMessage struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}
