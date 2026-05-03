package authentik

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/caboose-ai/caboose-ai.io/internal/runner"
)

type Client struct {
	BaseURL string
	Token   string
	HTTP    runner.HTTPClient
	Logger  *slog.Logger
}

func NewClient(baseURL, token string, httpClient runner.HTTPClient) *Client {
	return &Client{BaseURL: baseURL, Token: token, HTTP: httpClient}
}

func (c *Client) Get(ctx context.Context, path string) ([]byte, error) {
	return c.do(ctx, http.MethodGet, path, nil)
}

func (c *Client) Post(ctx context.Context, path string, body any) ([]byte, error) {
	return c.doJSON(ctx, http.MethodPost, path, body)
}

func (c *Client) Patch(ctx context.Context, path string, body any) ([]byte, error) {
	return c.doJSON(ctx, http.MethodPatch, path, body)
}

func (c *Client) log() *slog.Logger {
	if c.Logger != nil {
		return c.Logger
	}
	return slog.Default()
}

func (c *Client) doJSON(ctx context.Context, method, path string, body any) ([]byte, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshaling request body: %w", err)
	}
	c.log().Debug("authentik request", "method", method, "path", path, "body", string(data))
	return c.do(ctx, method, path, bytes.NewReader(data))
}

func (c *Client) do(ctx context.Context, method, path string, body io.Reader) ([]byte, error) {
	url := c.BaseURL + path
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	if body != nil && method != http.MethodGet {
		req.Header.Set("Content-Type", "application/json")
	}

	if body == nil {
		c.log().Debug("authentik request", "method", method, "path", path)
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("requesting %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response from %s: %w", path, err)
	}

	c.log().Debug("authentik response", "method", method, "path", path, "status", resp.StatusCode, "body", string(data))

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("%s %s returned HTTP %d: %s", method, path, resp.StatusCode, string(data))
	}
	return data, nil
}
