package install

import (
	"context"
	"fmt"
	"strings"
)

func (inst *Installer) InitForgejo(ctx context.Context) error {
	if inst.State.DryRun {
		return nil
	}

	giteaPass, err := inst.Secrets.Get(ctx, "GITEA_ADMIN_PASSWORD")
	if err != nil {
		return fmt.Errorf("retrieving GITEA_ADMIN_PASSWORD: %w", err)
	}

	email := inst.Config.Email
	if email == "" {
		email = "admin@" + inst.Config.Domain
	}

	_, err = inst.DockerExec.ExecAs(ctx, "forgejo", "git",
		"gitea", "admin", "user", "create",
		"--username", "auth-admin",
		"--password", giteaPass,
		"--email", email,
		"--admin",
	)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			return nil
		}
		return fmt.Errorf("creating Forgejo admin user: %w", err)
	}
	return nil
}
