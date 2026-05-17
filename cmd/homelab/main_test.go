package main

import "testing"

func TestValidateResetConfirmationRequiresYesForDestructiveReset(t *testing.T) {
	if err := validateResetConfirmation(cliOpts{}); err == nil {
		t.Fatal("validateResetConfirmation succeeded without --yes")
	}
}

func TestValidateResetConfirmationAllowsDryRunOrYes(t *testing.T) {
	for _, opts := range []cliOpts{{dryRun: true}, {yes: true}} {
		if err := validateResetConfirmation(opts); err != nil {
			t.Fatalf("validateResetConfirmation(%+v): %v", opts, err)
		}
	}
}

func TestValidateResetConfirmationRejectsStaticEnvStoreWithKeepEnv(t *testing.T) {
	err := validateResetConfirmation(cliOpts{yes: true, keepEnv: true, storeStaticEnv: true})
	if err == nil {
		t.Fatal("validateResetConfirmation allowed --store-static-env with --keep-env")
	}
}

func TestValidateResetConfirmationRejectsStaticEnvStoreWithEnvOnlySecrets(t *testing.T) {
	err := validateResetConfirmation(cliOpts{yes: true, secretsEnvOnly: true, storeStaticEnv: true})
	if err == nil {
		t.Fatal("validateResetConfirmation allowed --store-static-env with --secrets-env-only")
	}
}
