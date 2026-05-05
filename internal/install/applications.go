package install

import (
	"context"
	"fmt"
)

func (inst *Installer) ensureOpenApplication(ctx context.Context, name, slug string, providerPK int) error {
	app, err := inst.AK.GetApplication(ctx, slug)
	if err != nil {
		return fmt.Errorf("looking up application %q: %w", slug, err)
	}
	if app == nil {
		if _, err := inst.AK.CreateApplication(ctx, name, slug, providerPK); err != nil {
			return fmt.Errorf("creating application %q: %w", slug, err)
		}
		return nil
	}
	if err := inst.AK.EnsureApplicationOpen(ctx, slug); err != nil {
		return fmt.Errorf("ensuring application %q is open: %w", slug, err)
	}
	return nil
}
