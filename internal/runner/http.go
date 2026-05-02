package runner

import (
	"net/http"
	"time"
)

type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

func NewHTTPClient() HTTPClient {
	return &http.Client{
		Timeout: 30 * time.Second,
	}
}
