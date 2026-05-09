package install

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/caboose-ai/caboose-ai.io/services/authentik"
)

var (
	authentikDefaultFlowWait  = 90 * time.Second
	authentikDefaultFlowPoll  = 3 * time.Second
	authentikDefaultBrandWait = 90 * time.Second
	authentikDefaultBrandPoll = 3 * time.Second
)

func (inst *Installer) GenerateAdminRecoveryLink(ctx context.Context, progressFn func(string)) (string, error) {
	if progressFn == nil {
		progressFn = func(string) {}
	}
	if inst.State.DryRun {
		return "https://auth." + inst.Config.Domain + "/recovery/use-token/dry-run/", nil
	}
	if inst.DockerExec == nil {
		return "", fmt.Errorf("docker exec client not configured")
	}

	progressFn("Generating admin recovery link...")
	const container = "authentik-server"
	running, err := inst.DockerExec.ContainerRunning(ctx, container)
	if err != nil {
		return "", fmt.Errorf("checking authentik container %q: %w", container, err)
	}
	if !running {
		return "", fmt.Errorf("authentik container %q is not running; start it with: docker compose -f %s/docker-compose.yml up -d authentik-server", container, inst.Config.ComposeDir)
	}

	out, err := inst.DockerExec.Exec(ctx, container, "ak", "create_recovery_key", "10", "auth-admin")
	if err != nil {
		return "", fmt.Errorf("creating recovery key: %w", err)
	}
	token := parseRecoveryToken(strings.TrimSpace(string(out)))
	if token == "" {
		return "", fmt.Errorf("could not parse recovery token from output: %s", string(out))
	}
	return fmt.Sprintf("https://auth.%s/recovery/use-token/%s/", inst.Config.Domain, token), nil
}

func parseRecoveryToken(output string) string {
	prefix := "/recovery/use-token/"
	idx := strings.Index(output, prefix)
	if idx < 0 {
		return ""
	}
	rest := output[idx+len(prefix):]
	end := strings.Index(rest, "/")
	if end < 0 {
		return strings.TrimSpace(rest)
	}
	return rest[:end]
}

func (inst *Installer) ConfigureBrand(ctx context.Context) error {
	if inst.State.DryRun {
		return nil
	}

	flow, err := inst.ensureRecoveryFlow(ctx)
	if err != nil {
		return fmt.Errorf("ensuring recovery flow: %w", err)
	}

	brand, err := inst.waitForDefaultBrand(ctx)
	if err != nil {
		return fmt.Errorf("getting default brand: %w", err)
	}

	if err := inst.AK.SetBrandRecoveryFlow(ctx, brand.BrandUUID, flow.PK); err != nil {
		return fmt.Errorf("setting recovery flow: %w", err)
	}
	return nil
}

func IsAuthentikTokenError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "Token invalid/expired") ||
		strings.Contains(msg, "HTTP 401") ||
		strings.Contains(msg, "HTTP 403")
}

func (inst *Installer) RecoverAuthentikBootstrapToken(ctx context.Context) error {
	if inst.State.DryRun {
		return nil
	}
	if inst.DockerExec == nil {
		return fmt.Errorf("docker exec client not configured")
	}

	type recoveryResult struct {
		Token string `json:"token"`
		Error string `json:"error"`
	}

	script := strings.Join([]string{
		"import json",
		"import os",
		"import traceback",
		"try:",
		"    from authentik.core.models import Group, Token, TokenIntents, User, UserTypes, default_token_key",
		"    email = os.environ.get('AUTHENTIK_BOOTSTRAP_EMAIL') or ''",
		"    password = os.environ.get('AUTHENTIK_BOOTSTRAP_PASSWORD') or None",
		"    u = User.objects.filter(username='auth-admin').first() or User.objects.filter(username='akadmin').first()",
		"    if u is None:",
		"        u = User.objects.create(username='auth-admin', name='authentik Default Admin', email=email, type=UserTypes.INTERNAL)",
		"        if password:",
		"            u.set_password(password)",
		"    u.is_active = True",
		"    u.save()",
		"    g, _ = Group.objects.get_or_create(name='authentik Admins', defaults={'is_superuser': True})",
		"    if not g.is_superuser:",
		"        g.is_superuser = True",
		"        g.save(update_fields=['is_superuser'])",
		"    g.users.add(u)",
		"    t, _ = Token.objects.update_or_create(identifier='authentik-bootstrap-token', defaults={'intent': TokenIntents.INTENT_API, 'user': u, 'expiring': False, 'key': default_token_key()})",
		"    print(json.dumps({'token': t.key}))",
		"except Exception as exc:",
		"    print(json.dumps({'error': ''.join(traceback.format_exception_only(type(exc), exc)).strip()}))",
	}, "\n")

	out, err := inst.DockerExec.Exec(ctx, "authentik-server", "sh", "-c", "PATH=/ak-root/.venv/bin:$PATH /lifecycle/ak shell -c \"$1\"", "homelab-recover-token", script)
	if err != nil {
		return fmt.Errorf("creating Authentik bootstrap token in container: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	rawResult := strings.TrimSpace(lines[len(lines)-1])
	var result recoveryResult
	if err := json.Unmarshal([]byte(rawResult), &result); err != nil {
		return fmt.Errorf("creating Authentik bootstrap token in container: unexpected recovery output")
	}
	if result.Error != "" {
		return fmt.Errorf("creating Authentik bootstrap token in container: %s", result.Error)
	}
	if result.Token == "" {
		return fmt.Errorf("creating Authentik bootstrap token in container: empty token")
	}
	if err := inst.Secrets.Put(ctx, "AUTHENTIK_BOOTSTRAP_TOKEN", result.Token); err != nil {
		return fmt.Errorf("storing recovered Authentik bootstrap token: %w", err)
	}
	inst.InitAK(result.Token)
	return nil
}

func (inst *Installer) ensureRecoveryFlow(ctx context.Context) (*authentik.Flow, error) {
	flow, err := inst.ensureFlow(ctx, authentik.CreateFlowParams{
		Name:        "Default recovery flow",
		Slug:        "default-recovery-flow",
		Title:       "Reset your password",
		Designation: "recovery",
	})
	if err != nil {
		return nil, err
	}

	passwordChangeFlow, err := inst.waitForDefaultFlow(ctx, "default-password-change")
	if err != nil {
		return nil, fmt.Errorf("getting default-password-change flow for stage reuse: %w", err)
	}

	bindings, err := inst.AK.ListFlowStageBindings(ctx, passwordChangeFlow.Slug)
	if err != nil {
		return nil, fmt.Errorf("listing default-password-change bindings: %w", err)
	}

	for _, b := range bindings {
		existing, err := inst.AK.GetFlowStageBinding(ctx, flow.Slug, b.StageObj.PK)
		if err != nil {
			return nil, fmt.Errorf("checking recovery flow binding for stage %q: %w", b.StageObj.Name, err)
		}
		if existing != nil {
			continue
		}
		if err := inst.AK.CreateFlowStageBinding(ctx, flow.PK, b.StageObj.PK, b.Order); err != nil {
			if strings.Contains(err.Error(), "unique set") {
				continue
			}
			return nil, fmt.Errorf("binding stage %q to recovery flow: %w", b.StageObj.Name, err)
		}
	}

	return flow, nil
}

func (inst *Installer) waitForDefaultFlow(ctx context.Context, slug string) (*authentik.Flow, error) {
	deadline := time.Now().Add(authentikDefaultFlowWait)
	for {
		flow, err := inst.AK.GetFlow(ctx, slug)
		if err == nil {
			return flow, nil
		}
		if !strings.Contains(err.Error(), "not found") {
			return nil, err
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("flow %q not found after %s", slug, authentikDefaultFlowWait)
		}

		timer := time.NewTimer(authentikDefaultFlowPoll)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
}

func (inst *Installer) waitForDefaultBrand(ctx context.Context) (*authentik.Brand, error) {
	deadline := time.Now().Add(authentikDefaultBrandWait)
	for {
		brand, err := inst.AK.GetDefaultBrand(ctx)
		if err == nil {
			return brand, nil
		}
		if !strings.Contains(err.Error(), "default brand not found") {
			return nil, err
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("default brand not found after %s", authentikDefaultBrandWait)
		}

		timer := time.NewTimer(authentikDefaultBrandPoll)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
}
