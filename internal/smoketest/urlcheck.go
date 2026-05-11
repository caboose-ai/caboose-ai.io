package smoketest

import "strings"

func isLoggedInURL(currentURL, host string) bool {
	if !strings.Contains(currentURL, host) {
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
