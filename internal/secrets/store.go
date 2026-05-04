package secrets

import "context"

type GenerateOpts struct {
	Recipe string // e.g. "letters,digits,symbols,50" or "hex"
}

type SecretStore interface {
	Get(ctx context.Context, key string) (string, error)
	Put(ctx context.Context, key, value string) error
	Generate(ctx context.Context, key string, length int, opts GenerateOpts) (string, error)
	Delete(ctx context.Context, key string) error
	EnsureVault(ctx context.Context) error
}

type SecretDef struct {
	Key      string
	Length   int
	Opts     GenerateOpts
	Prompt   bool // if true, prompt user instead of generating
	Optional bool // if true, secret can be skipped
}

func BootstrapSecrets() []SecretDef {
	return []SecretDef{
		{Key: "AUTHENTIK_SECRET_KEY", Length: 50, Opts: GenerateOpts{Recipe: "letters,digits,symbols,50"}},
		{Key: "AUTHENTIK_PG_PASS", Length: 32, Opts: GenerateOpts{Recipe: "letters,digits,32"}},
		{Key: "AUTHENTIK_BOOTSTRAP_PASSWORD", Length: 32, Opts: GenerateOpts{Recipe: "letters,digits,symbols,32"}},
		{Key: "AUTHENTIK_BOOTSTRAP_TOKEN", Length: 64, Opts: GenerateOpts{Recipe: "hex"}},
		{Key: "AUTHENTIK_BOOTSTRAP_EMAIL", Prompt: true},
		{Key: "WOODPECKER_AGENT_SECRET", Length: 32, Opts: GenerateOpts{Recipe: "letters,digits,32"}},
		{Key: "GRAFANA_ADMIN_PASSWORD", Length: 32, Opts: GenerateOpts{Recipe: "letters,digits,symbols,32"}},
		{Key: "GITEA_ADMIN_PASSWORD", Length: 32, Opts: GenerateOpts{Recipe: "letters,digits,symbols,32"}},
		{Key: "PORTAINER_ADMIN_PASSWORD", Length: 32, Opts: GenerateOpts{Recipe: "letters,digits,symbols,32"}},
		{Key: "N8N_USER", Prompt: true},
		{Key: "N8N_PASSWORD", Length: 32, Opts: GenerateOpts{Recipe: "letters,digits,symbols,32"}},
		{Key: "SONAR_DB_PASS", Length: 32, Opts: GenerateOpts{Recipe: "letters,digits,32"}},
		{Key: "GHOST_DB_PASS", Length: 32, Opts: GenerateOpts{Recipe: "letters,digits,32"}},
		{Key: "MATTERMOST_DB_PASS", Length: 32, Opts: GenerateOpts{Recipe: "letters,digits,32"}},
	}
}

// DerivedSecretKeys returns the keys for OAuth credentials written to the
// secret store by service configurators during install. These are cleaned up
// on reset alongside the bootstrap secrets.
func DerivedSecretKeys() []string {
	return []string{
		"GF_AUTH_GENERIC_OAUTH_CLIENT_ID",
		"GF_AUTH_GENERIC_OAUTH_CLIENT_SECRET",
		"OPEN_WEBUI_OAUTH_CLIENT_ID",
		"OPEN_WEBUI_OAUTH_CLIENT_SECRET",
		"WOODPECKER_GITEA_CLIENT",
		"WOODPECKER_GITEA_SECRET",
		"TURNSTILE_SITE_KEY",
		"TURNSTILE_SECRET_KEY",
	}
}

func SocialSecrets() []SecretDef {
	return []SecretDef{
		{Key: "GITHUB_OAUTH_CLIENT_ID", Prompt: true, Optional: true},
		{Key: "GITHUB_OAUTH_CLIENT_SECRET", Prompt: true, Optional: true},
		{Key: "GOOGLE_OAUTH_CLIENT_ID", Prompt: true, Optional: true},
		{Key: "GOOGLE_OAUTH_CLIENT_SECRET", Prompt: true, Optional: true},
	}
}

func TurnstileSecrets() []SecretDef {
	return []SecretDef{
		{Key: "TURNSTILE_SITE_KEY", Prompt: true, Optional: true},
		{Key: "TURNSTILE_SECRET_KEY", Prompt: true, Optional: true},
	}
}
