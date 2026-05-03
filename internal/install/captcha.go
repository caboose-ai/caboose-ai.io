package install

import (
	"context"
	"fmt"
	"strings"

	"github.com/caboose-ai/caboose-ai.io/internal/services/authentik"
)

func (inst *Installer) SetupCaptcha(ctx context.Context, progressFn func(string)) error {
	if progressFn == nil {
		progressFn = func(string) {}
	}
	if inst.State.DryRun {
		return nil
	}

	siteKey, err := inst.Secrets.Get(ctx, "TURNSTILE_SITE_KEY")
	if err != nil {
		return fmt.Errorf("getting TURNSTILE_SITE_KEY: %w", err)
	}
	secretKey, err := inst.Secrets.Get(ctx, "TURNSTILE_SECRET_KEY")
	if err != nil {
		return fmt.Errorf("getting TURNSTILE_SECRET_KEY: %w", err)
	}
	if siteKey == "" || secretKey == "" {
		return nil
	}

	progressFn("Creating Turnstile captcha stage...")

	var stagePK string
	existing, err := inst.AK.GetCaptchaStage(ctx, "turnstile-captcha")
	if err != nil {
		return fmt.Errorf("checking captcha stage: %w", err)
	}
	if existing != nil {
		stagePK = existing.PK
	} else {
		created, err := inst.AK.CreateCaptchaStage(ctx, authentik.CreateCaptchaStageParams{
			Name:       "turnstile-captcha",
			PublicKey:  siteKey,
			PrivateKey: secretKey,
			JsURL:      "https://challenges.cloudflare.com/turnstile/v0/api.js",
			ApiURL:     "https://challenges.cloudflare.com/turnstile/v0/siteverify",
		})
		if err != nil {
			return fmt.Errorf("creating captcha stage: %w", err)
		}
		stagePK = created.PK
	}

	progressFn("Binding captcha to authentication flow...")

	flow, err := inst.AK.GetFlow(ctx, "default-authentication-flow")
	if err != nil {
		return fmt.Errorf("getting authentication flow: %w", err)
	}

	bound, err := inst.AK.GetFlowStageBinding(ctx, "default-authentication-flow", stagePK)
	if err != nil {
		return fmt.Errorf("checking flow stage binding: %w", err)
	}
	if bound != nil {
		return nil
	}

	if err := inst.AK.CreateFlowStageBinding(ctx, flow.PK, stagePK, 0); err != nil {
		return fmt.Errorf("binding captcha to flow: %w", err)
	}
	return nil
}

func (inst *Installer) SetupInactiveEnrollment(ctx context.Context, progressFn func(string)) error {
	if progressFn == nil {
		progressFn = func(string) {}
	}
	if inst.State.DryRun {
		return nil
	}

	progressFn("Configuring enrollment flow for inactive users...")

	bindings, err := inst.AK.ListFlowStageBindings(ctx, "default-source-enrollment")
	if err != nil {
		return fmt.Errorf("listing enrollment flow bindings: %w", err)
	}

	var userWritePK string
	for _, b := range bindings {
		if strings.Contains(b.StageObj.Component, "user-write") {
			userWritePK = b.StageObj.PK
			break
		}
	}
	if userWritePK == "" {
		return fmt.Errorf("user-write stage not found in default-source-enrollment flow")
	}

	if err := inst.AK.PatchUserWriteStage(ctx, userWritePK, true); err != nil {
		return fmt.Errorf("patching user-write stage: %w", err)
	}
	return nil
}
