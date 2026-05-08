package authentik

import (
	"strings"
	"testing"
)

func TestRedactJSONForLog(t *testing.T) {
	got := redactJSONForLog([]byte(`{
		"name":"github",
		"consumer_secret":"source-secret",
		"client_secret":"provider-secret",
		"nested":{"bootstrap_token":"token-value","safe":"visible"},
		"items":[{"password":"password-value"}]
	}`))

	for _, leaked := range []string{"source-secret", "provider-secret", "token-value", "password-value"} {
		if strings.Contains(got, leaked) {
			t.Fatalf("redacted log leaked %q in %s", leaked, got)
		}
	}
	if !strings.Contains(got, `"safe":"visible"`) {
		t.Fatalf("redacted log removed non-sensitive field: %s", got)
	}
}
