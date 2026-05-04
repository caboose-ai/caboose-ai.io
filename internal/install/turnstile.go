package install

import "context"

const (
	turnstileSiteKeyKey   = "TURNSTILE_SITE_KEY"
	turnstileSecretKeyKey = "TURNSTILE_SECRET_KEY"
)

func (inst *Installer) ResolveTurnstileCredentials(ctx context.Context, promptFn func(key string) (string, error)) error {
	// If config already has both values, persist them to the secret store and return.
	if inst.Config.Turnstile.SiteKey != "" && inst.Config.Turnstile.SecretKey != "" {
		_ = inst.Secrets.Put(ctx, turnstileSiteKeyKey, inst.Config.Turnstile.SiteKey)
		_ = inst.Secrets.Put(ctx, turnstileSecretKeyKey, inst.Config.Turnstile.SecretKey)
		return nil
	}

	// Try the secret store.
	siteKey, _ := inst.Secrets.Get(ctx, turnstileSiteKeyKey)
	secretKey, _ := inst.Secrets.Get(ctx, turnstileSecretKeyKey)

	// Fall back to interactive prompt.
	if siteKey == "" && promptFn != nil {
		var err error
		siteKey, err = promptFn(turnstileSiteKeyKey)
		if err != nil {
			return err
		}
	}
	if siteKey == "" {
		return nil
	}

	if secretKey == "" && promptFn != nil {
		var err error
		secretKey, err = promptFn(turnstileSecretKeyKey)
		if err != nil {
			return err
		}
	}
	if secretKey == "" {
		return nil
	}

	// Both obtained — store to config and secrets.
	inst.Config.Turnstile.SiteKey = siteKey
	inst.Config.Turnstile.SecretKey = secretKey
	_ = inst.Secrets.Put(ctx, turnstileSiteKeyKey, siteKey)
	_ = inst.Secrets.Put(ctx, turnstileSecretKeyKey, secretKey)
	return nil
}
