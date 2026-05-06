package cli

import (
	"fmt"
	"io"

	"github.com/caboose-ai/caboose-ai.io/internal/config"
)

func PrintOAuthSetup(w io.Writer, cfg *config.Config) {
	urls := cfg.URLs()
	githubCallback := urls.Authentik + "/source/oauth/callback/github/"
	googleCallback := urls.Authentik + "/source/oauth/callback/google/"

	fmt.Fprintf(w, "External credential setup for %s\n\n", cfg.Domain)

	fmt.Fprintln(w, "GitHub OAuth App")
	fmt.Fprintln(w, "  Create/edit: https://github.com/settings/developers")
	fmt.Fprintf(w, "  Homepage URL: %s\n", urls.Dashboard)
	fmt.Fprintf(w, "  Authorization callback URL: %s\n", githubCallback)
	fmt.Fprintln(w, "  Store as:")
	fmt.Fprintln(w, "    GITHUB_OAUTH_CLIENT_ID")
	fmt.Fprintln(w, "    GITHUB_OAUTH_CLIENT_SECRET")
	fmt.Fprintln(w)

	fmt.Fprintln(w, "Google OAuth Client")
	fmt.Fprintln(w, "  Create/edit: https://console.cloud.google.com/apis/credentials")
	fmt.Fprintln(w, "  Application type: Web application")
	fmt.Fprintf(w, "  Authorized redirect URI: %s\n", googleCallback)
	fmt.Fprintf(w, "  Authorized JavaScript origin, if requested: %s\n", urls.Authentik)
	fmt.Fprintln(w, "  Store as:")
	fmt.Fprintln(w, "    GOOGLE_OAUTH_CLIENT_ID")
	fmt.Fprintln(w, "    GOOGLE_OAUTH_CLIENT_SECRET")
	fmt.Fprintln(w)

	fmt.Fprintln(w, "Cloudflare Turnstile")
	fmt.Fprintln(w, "  Create/edit: https://dash.cloudflare.com/?to=/:account/turnstile")
	fmt.Fprintf(w, "  Hostname: auth.%s\n", cfg.Domain)
	fmt.Fprintln(w, "  Store as:")
	fmt.Fprintln(w, "    TURNSTILE_SITE_KEY")
	fmt.Fprintln(w, "    TURNSTILE_SECRET_KEY")
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
