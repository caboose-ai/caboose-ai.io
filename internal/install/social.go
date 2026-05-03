package install

import (
	"context"

	"github.com/caboose-ai/caboose-ai.io/internal/config"
)

type socialProvider struct {
	name      string
	idKey     string
	secretKey string
	getCreds  func(*config.SocialConfig) *config.OAuthCredentials
	setCreds  func(*config.SocialConfig, *config.OAuthCredentials)
}

var socialProviders = []socialProvider{
	{
		name:      "GitHub",
		idKey:     "GITHUB_OAUTH_CLIENT_ID",
		secretKey: "GITHUB_OAUTH_CLIENT_SECRET",
		getCreds:  func(s *config.SocialConfig) *config.OAuthCredentials { return s.GitHub },
		setCreds:  func(s *config.SocialConfig, c *config.OAuthCredentials) { s.GitHub = c },
	},
	{
		name:      "Google",
		idKey:     "GOOGLE_OAUTH_CLIENT_ID",
		secretKey: "GOOGLE_OAUTH_CLIENT_SECRET",
		getCreds:  func(s *config.SocialConfig) *config.OAuthCredentials { return s.Google },
		setCreds:  func(s *config.SocialConfig, c *config.OAuthCredentials) { s.Google = c },
	},
}

func (inst *Installer) ResolveSocialCredentials(ctx context.Context, promptFn func(key string) (string, error)) error {
	for _, sp := range socialProviders {
		creds := sp.getCreds(&inst.Config.Social)
		if creds != nil && creds.ClientID != "" && creds.ClientSecret != "" {
			continue
		}

		clientID, _ := inst.Secrets.Get(ctx, sp.idKey)
		clientSecret, _ := inst.Secrets.Get(ctx, sp.secretKey)

		if clientID == "" && promptFn != nil {
			var err error
			clientID, err = promptFn(sp.idKey)
			if err != nil {
				return err
			}
		}
		if clientID == "" {
			continue
		}

		if clientSecret == "" && promptFn != nil {
			var err error
			clientSecret, err = promptFn(sp.secretKey)
			if err != nil {
				return err
			}
		}
		if clientSecret == "" {
			continue
		}

		sp.setCreds(&inst.Config.Social, &config.OAuthCredentials{
			ClientID:     clientID,
			ClientSecret: clientSecret,
		})

		_ = inst.Secrets.Put(ctx, sp.idKey, clientID)
		_ = inst.Secrets.Put(ctx, sp.secretKey, clientSecret)
	}
	return nil
}
