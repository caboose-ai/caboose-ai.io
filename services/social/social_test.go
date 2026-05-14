package social

import (
	"testing"

	"github.com/caboose-ai/caboose-ai.io/internal/config"
	"github.com/caboose-ai/caboose-ai.io/services/authentik"
)

func TestManagedSourceParams(t *testing.T) {
	creds := config.OAuthCredentials{
		ClientID:     "client-id",
		ClientSecret: "client-secret",
	}

	github := githubSourceParams(creds, "auth-flow", "enroll-flow")
	if github.Enabled {
		t.Fatal("GitHub source should be disabled by default")
	}
	if github.Promoted == nil || *github.Promoted {
		t.Fatalf("GitHub promoted = %v, want false", github.Promoted)
	}

	google := googleSourceParams(creds, "auth-flow", "enroll-flow")
	if !google.Enabled {
		t.Fatal("Google source should be enabled")
	}
	if google.Promoted == nil || *google.Promoted {
		t.Fatalf("Google promoted = %v, want false", google.Promoted)
	}
	if google.UserMatchingMode != googleUserMatchingMode {
		t.Fatalf("Google user matching mode = %q, want %q", google.UserMatchingMode, googleUserMatchingMode)
	}
	if google.ProviderType != "google" {
		t.Fatalf("Google provider type = %q, want google", google.ProviderType)
	}
}

func TestGoogleLoginSourcePKs(t *testing.T) {
	got := googleLoginSourcePKs([]authentik.OAuthSource{
		{PK: "github-pk", Slug: "github", Enabled: false},
		{PK: "google-pk", Slug: "google", Enabled: true},
		{PK: "other-pk", Slug: "other", Enabled: true},
		{PK: "disabled-google-pk", Slug: "google", Enabled: false},
	})
	if len(got) != 1 || got[0] != "google-pk" {
		t.Fatalf("googleLoginSourcePKs = %#v, want [google-pk]", got)
	}
}
