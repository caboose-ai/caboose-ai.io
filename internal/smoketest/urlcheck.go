package smoketest

import (
	"net/url"
	"strings"
)

func isLoggedInURL(currentURL, host string) bool {
	u, err := url.Parse(currentURL)
	if err != nil || u.Hostname() != host {
		return false
	}
	failMarkers := []string{
		"/login",
		"/auth",
		"/error",
		"/user/link_account",
		"#!/auth",
	}
	for _, marker := range failMarkers {
		if strings.Contains(currentURL, marker) {
			return false
		}
	}
	return true
}
