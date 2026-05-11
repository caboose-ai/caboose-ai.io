package smoketest

import "testing"

func TestIsLoggedInURL(t *testing.T) {
	host := "openclaw.example.com"

	for _, tc := range []struct {
		name string
		url  string
		want bool
	}{
		{name: "landing", url: "https://openclaw.example.com/chat?session=main", want: true},
		{name: "login", url: "https://openclaw.example.com/login", want: false},
		{name: "auth", url: "https://openclaw.example.com/auth", want: false},
		{name: "error", url: "https://openclaw.example.com/error", want: false},
		{name: "link account", url: "https://openclaw.example.com/user/link_account", want: false},
		{name: "auth hash", url: "https://openclaw.example.com/#!/auth", want: false},
		{name: "wrong host", url: "https://auth.example.com/", want: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := isLoggedInURL(tc.url, host); got != tc.want {
				t.Fatalf("isLoggedInURL(%q, %q) = %v, want %v", tc.url, host, got, tc.want)
			}
		})
	}
}
