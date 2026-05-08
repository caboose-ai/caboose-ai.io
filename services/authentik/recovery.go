package authentik

import (
	"context"
	"encoding/json"
	"fmt"
)

func (c *Client) GenerateRecoveryLink(ctx context.Context, userPK int) (string, error) {
	data, err := c.Post(ctx, fmt.Sprintf("/api/v3/core/users/%d/recovery/", userPK), map[string]any{})
	if err != nil {
		return "", fmt.Errorf("generating recovery link: %w", err)
	}

	var resp struct {
		Link string `json:"link"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return "", fmt.Errorf("parsing recovery link response: %w", err)
	}
	return resp.Link, nil
}
