//go:build integration

package smoketest

import (
	"context"
	"testing"

	"github.com/caboose-ai/caboose-ai.io/internal/install"
	"github.com/caboose-ai/caboose-ai.io/services/authentik"
)

func TestSSO_Config(t *testing.T) {
	s := NewSuite(t)
	ctx := context.Background()

	t.Run("Providers", func(t *testing.T) {
		for _, spec := range install.DefaultProviderSpecs(s.URLs) {
			t.Run(spec.Name, func(t *testing.T) {
				provider, err := s.AK.GetProvider(ctx, spec.Name)
				if err != nil {
					t.Fatalf("GetProvider(%q): %v", spec.Name, err)
				}
				if provider == nil {
					t.Fatalf("provider %q not found", spec.Name)
				}
				t.Logf("provider %q: pk=%d client_id=%s", spec.Name, provider.PK, provider.ClientID)
			})
		}
	})

	t.Run("Applications", func(t *testing.T) {
		for _, spec := range install.DefaultProviderSpecs(s.URLs) {
			t.Run(spec.Slug, func(t *testing.T) {
				app, err := s.AK.GetApplication(ctx, spec.Slug)
				if err != nil {
					t.Fatalf("GetApplication(%q): %v", spec.Slug, err)
				}
				if app == nil {
					t.Fatalf("application %q not found", spec.Slug)
				}
				if app.PolicyEngineMode != "all" {
					t.Errorf("application %q has policy_engine_mode=%q, want \"all\"", spec.Slug, app.PolicyEngineMode)
				}
				t.Logf("application %q: pk=%s policy_engine_mode=%s", spec.Slug, app.PK, app.PolicyEngineMode)
			})
		}
	})

	t.Run("ProxyProviders", func(t *testing.T) {
		for _, spec := range install.DefaultProxySpecs(s.URLs) {
			t.Run(spec.Name, func(t *testing.T) {
				proxy, err := s.AK.GetProxyProvider(ctx, spec.Name)
				if err != nil {
					t.Fatalf("GetProxyProvider(%q): %v", spec.Name, err)
				}
				if proxy == nil {
					t.Fatalf("proxy provider %q not found", spec.Name)
				}
				t.Logf("proxy provider %q: pk=%d mode=%s", spec.Name, proxy.PK, proxy.Mode)
				if spec.InternalHost != "" && proxy.InternalHost != spec.InternalHost {
					t.Errorf("proxy provider %q internal_host=%q, want %q", spec.Name, proxy.InternalHost, spec.InternalHost)
				}

				app, err := s.AK.GetApplication(ctx, spec.Slug)
				if err != nil {
					t.Fatalf("GetApplication(%q): %v", spec.Slug, err)
				}
				if app == nil {
					t.Fatalf("proxy application %q not found", spec.Slug)
				}
				if app.PolicyEngineMode != "all" {
					t.Errorf("proxy application %q has policy_engine_mode=%q, want \"all\"", spec.Slug, app.PolicyEngineMode)
				}
			})
		}
	})

	t.Run("OutpostBindings", func(t *testing.T) {
		outpost, err := s.AK.GetEmbeddedOutpost(ctx)
		if err != nil {
			t.Fatalf("GetEmbeddedOutpost: %v", err)
		}

		boundPKs := make(map[int]bool)
		for _, pk := range outpost.Providers {
			boundPKs[pk] = true
		}

		for _, spec := range install.DefaultProxySpecs(s.URLs) {
			t.Run(spec.Name, func(t *testing.T) {
				proxy, err := s.AK.GetProxyProvider(ctx, spec.Name)
				if err != nil {
					t.Fatalf("GetProxyProvider(%q): %v", spec.Name, err)
				}
				if proxy == nil {
					t.Fatalf("proxy provider %q not found", spec.Name)
				}
				if !boundPKs[proxy.PK] {
					t.Errorf("proxy provider %q (pk=%d) not bound to embedded outpost", spec.Name, proxy.PK)
				}
			})
		}
	})

	t.Run("BrandRecoveryFlow", func(t *testing.T) {
		brand, err := s.AK.GetDefaultBrand(ctx)
		if err != nil {
			t.Fatalf("GetDefaultBrand: %v", err)
		}

		flow, err := s.AK.GetFlowByDesignation(ctx, "recovery")
		if err != nil {
			t.Fatalf("GetFlowByDesignation(recovery): %v", err)
		}

		t.Logf("brand %q: uuid=%s, recovery flow pk=%s", brand.Domain, brand.BrandUUID, flow.PK)
	})

	t.Run("LogoutFlows", func(t *testing.T) {
		redirect, err := s.AK.GetRedirectStage(ctx, "google-account-logout-redirect")
		if err != nil {
			t.Fatalf("GetRedirectStage: %v", err)
		}
		if redirect == nil {
			t.Fatal("google-account-logout-redirect stage not found")
		}
		if redirect.Mode != "static" {
			t.Fatalf("google-account-logout-redirect mode=%q, want static", redirect.Mode)
		}
		if redirect.TargetStatic != "https://accounts.google.com/Logout" {
			t.Fatalf("google-account-logout-redirect target_static=%q, want Google logout", redirect.TargetStatic)
		}

		logout, err := s.AK.GetUserLogoutStage(ctx, "default-invalidation-logout")
		if err != nil {
			t.Fatalf("GetUserLogoutStage: %v", err)
		}
		if logout == nil {
			t.Fatal("default-invalidation-logout stage not found")
		}

		for _, flowSlug := range []string{"default-invalidation-flow", "default-provider-invalidation-flow"} {
			t.Run(flowSlug, func(t *testing.T) {
				assertFlowStageBinding(t, s.AK, ctx, flowSlug, logout.PK, "user logout")
				assertFlowStageBinding(t, s.AK, ctx, flowSlug, redirect.PK, "Google logout redirect")
			})
		}
	})

	t.Run("SocialSources", func(t *testing.T) {
		stage, err := s.AK.GetIdentificationStage(ctx, "default-authentication-flow")
		if err != nil {
			t.Fatalf("GetIdentificationStage(default-authentication-flow): %v", err)
		}
		if stage == nil {
			t.Fatal("default-authentication-flow identification stage not found")
		}
		if len(stage.UserFields) != 0 {
			t.Fatalf("default-authentication-flow user_fields=%v, want empty to remove local email login", stage.UserFields)
		}
		if !stage.ShowSourceLabels {
			t.Fatal("default-authentication-flow show_source_labels=false, want true for branded Google source button")
		}

		boundSources := make(map[string]bool, len(stage.Sources))
		for _, pk := range stage.Sources {
			boundSources[pk] = true
		}

		sources, err := s.AK.ListSources(ctx)
		if err != nil {
			t.Fatalf("ListSources: %v", err)
		}
		sourceBySlug := make(map[string]authentik.OAuthSource, len(sources))
		for _, source := range sources {
			sourceBySlug[source.Slug] = source
		}

		for _, slug := range []string{"github", "google"} {
			t.Run(slug, func(t *testing.T) {
				source, ok := sourceBySlug[slug]
				if !ok || source.PK == "" {
					t.Skipf("social source %q not configured", slug)
				}
				if slug == "github" {
					if boundSources[source.PK] {
						t.Fatalf("github source is bound to default-authentication-flow; only Google should be offered")
					}
					if source.Enabled {
						t.Fatalf("github source is enabled; it should remain configured but unavailable for login")
					}
					if source.Promoted {
						t.Fatalf("github source is promoted; it should not be shown as a login option")
					}
				}
				if slug == "google" {
					if !source.Enabled {
						t.Fatalf("google source enabled=%v, want enabled", source.Enabled)
					}
					if !boundSources[source.PK] {
						t.Fatalf("google source exists but is not bound to default-authentication-flow")
					}
					if source.Promoted {
						t.Fatalf("google source promoted=%v, want false so Authentik renders the source icon", source.Promoted)
					}
					if source.UserMatchingMode != "email_link" {
						t.Fatalf("google user_matching_mode=%q, want email_link", source.UserMatchingMode)
					}
					if source.IconURL == "" {
						t.Fatalf("google source has empty icon_url")
					}
				}
				t.Logf("source %q: pk=%s enabled=%v promoted=%v matching=%s icon=%s", slug, source.PK, source.Enabled, source.Promoted, source.UserMatchingMode, source.IconURL)
			})
		}
	})

	t.Run("CaptchaStage", func(t *testing.T) {
		stage, err := s.AK.GetCaptchaStage(ctx, "turnstile-captcha")
		if err != nil {
			t.Fatalf("GetCaptchaStage: %v", err)
		}
		if stage == nil {
			t.Skip("captcha stage not configured")
		}

		binding, err := s.AK.GetFlowStageBinding(ctx, "default-authentication-flow", stage.PK)
		if err != nil {
			t.Fatalf("GetFlowStageBinding: %v", err)
		}
		if binding == nil {
			t.Error("captcha stage exists but is not bound to default-authentication-flow")
		}
	})

	t.Run("EnrollmentStage", func(t *testing.T) {
		bindings, err := s.AK.ListFlowStageBindings(ctx, "default-source-enrollment")
		if err != nil {
			t.Fatalf("ListFlowStageBindings: %v", err)
		}

		var userWritePK string
		for _, b := range bindings {
			if b.StageObj.Component == "ak-stage-user-write-form" {
				userWritePK = b.StageObj.PK
				break
			}
		}
		if userWritePK == "" {
			t.Fatal("user-write stage not found in default-source-enrollment flow")
		}

		stage, err := s.AK.GetUserWriteStage(ctx, userWritePK)
		if err != nil {
			t.Fatalf("GetUserWriteStage: %v", err)
		}
		if !stage.CreateUsersAsInactive {
			t.Error("user-write stage does not have create_users_as_inactive=true")
		}
	})
}

func assertFlowStageBinding(t *testing.T, client *authentik.Client, ctx context.Context, flowSlug, stagePK, label string) {
	t.Helper()
	binding, err := client.GetFlowStageBinding(ctx, flowSlug, stagePK)
	if err != nil {
		t.Fatalf("GetFlowStageBinding(%s, %s): %v", flowSlug, label, err)
	}
	if binding == nil {
		t.Fatalf("%s stage is not bound to %s", label, flowSlug)
	}
}
