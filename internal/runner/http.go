package runner

import "net/http"

type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

func NewHTTPClient() HTTPClient {
	return &http.Client{}
}
