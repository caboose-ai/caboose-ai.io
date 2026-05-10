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
