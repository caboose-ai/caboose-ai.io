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
	location := u.EscapedPath()
	if u.Fragment != "" {
		location += "#" + u.EscapedFragment()
	}
	failMarkers := []string{
		"/login",
		"/auth",
		"/error",
		"/user/link_account",
		"#!/auth",
	}
	for _, marker := range failMarkers {
		if strings.Contains(location, marker) {
			return false
		}
	}
	return true
}
