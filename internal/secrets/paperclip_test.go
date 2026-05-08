package secrets

import "testing"

func TestBootstrapSecretsIncludePaperclipSecrets(t *testing.T) {
	found := map[string]SecretDef{}
	for _, def := range BootstrapSecrets() {
		found[def.Key] = def
	}
	if found["PAPERCLIP_DB_PASS"].Length < 24 {
		t.Fatalf("PAPERCLIP_DB_PASS length = %d, want >= 24", found["PAPERCLIP_DB_PASS"].Length)
	}
	if found["PAPERCLIP_AUTH_SECRET"].Length < 32 {
		t.Fatalf("PAPERCLIP_AUTH_SECRET length = %d, want >= 32", found["PAPERCLIP_AUTH_SECRET"].Length)
	}
}
