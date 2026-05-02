package health

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/caboose-ai/caboose-ai.io/internal/runner"
)

type Status struct {
	Name    string
	Healthy bool
	Err     error
	Elapsed time.Duration
}

type Checker interface {
	Name() string
	Check(ctx context.Context) error
}

type HTTPChecker struct {
	ServiceName string
	URL         string
	Client      runner.HTTPClient
	Headers     map[string]string
}

func (c *HTTPChecker) Name() string { return c.ServiceName }

func (c *HTTPChecker) Check(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.URL, nil)
	if err != nil {
		return err
	}
	for k, v := range c.Headers {
		req.Header.Set(k, v)
	}
	resp, err := c.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return nil
}
