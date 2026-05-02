package authentik

import (
	"context"
	"encoding/json"
	"fmt"
)

type User struct {
	PK       int    `json:"pk"`
	Username string `json:"username"`
}

type userList struct {
	Results []User `json:"results"`
}

func (c *Client) FindUser(ctx context.Context, username string) (*User, error) {
	data, err := c.Get(ctx, fmt.Sprintf("/api/v3/core/users/?search=%s", username))
	if err != nil {
		return nil, err
	}

	var list userList
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, fmt.Errorf("parsing user response: %w", err)
	}

	for _, u := range list.Results {
		if u.Username == username {
			return &u, nil
		}
	}
	return nil, nil
}

func (c *Client) RenameUser(ctx context.Context, pk int, newUsername string) error {
	body := map[string]string{"username": newUsername}
	_, err := c.Patch(ctx, fmt.Sprintf("/api/v3/core/users/%d/", pk), body)
	return err
}

func (c *Client) RenameAdminUser(ctx context.Context, from, to string) error {
	existing, err := c.FindUser(ctx, to)
	if err != nil {
		return err
	}
	if existing != nil {
		return nil
	}

	user, err := c.FindUser(ctx, from)
	if err != nil {
		return err
	}
	if user == nil {
		return fmt.Errorf("user %q not found", from)
	}

	return c.RenameUser(ctx, user.PK, to)
}
