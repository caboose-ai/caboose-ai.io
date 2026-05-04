//go:build integration

package smoketest

import (
	"context"
	"testing"

	"github.com/caboose-ai/caboose-ai.io/internal/install"
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
				t.Logf("application %q: pk=%s", spec.Slug, app.PK)
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

	t.Run("SocialSources", func(t *testing.T) {
		for _, slug := range []string{"github", "google"} {
			t.Run(slug, func(t *testing.T) {
				pk, err := s.AK.GetSourcePK(ctx, slug)
				if err != nil {
					t.Fatalf("GetSourcePK(%q): %v", slug, err)
				}
				if pk == "" {
					t.Skipf("social source %q not configured", slug)
				}
				t.Logf("source %q: pk=%s", slug, pk)
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
