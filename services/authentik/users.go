package authentik

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
)

type User struct {
	PK         int    `json:"pk"`
	UID        string `json:"uid"`
	Username   string `json:"username"`
	Name       string `json:"name"`
	Email      string `json:"email"`
	IsActive   bool   `json:"is_active"`
	DateJoined string `json:"date_joined"`
	LastLogin  string `json:"last_login"`
}

type userList struct {
	Results []User `json:"results"`
}

func (c *Client) ListPendingUsers(ctx context.Context) ([]User, error) {
	data, err := c.Get(ctx, "/api/v3/core/users/?is_active=false&page_size=50")
	if err != nil {
		return nil, err
	}

	var list userList
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, fmt.Errorf("parsing pending users: %w", err)
	}
	return list.Results, nil
}

func (c *Client) ActivateUser(ctx context.Context, pk int) error {
	body := map[string]bool{"is_active": true}
	_, err := c.Patch(ctx, fmt.Sprintf("/api/v3/core/users/%d/", pk), body)
	return err
}

func (c *Client) FindUser(ctx context.Context, username string) (*User, error) {
	data, err := c.Get(ctx, fmt.Sprintf("/api/v3/core/users/?search=%s", url.QueryEscape(username)))
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

	for _, candidate := range []string{from, "akauth"} {
		user, err := c.FindUser(ctx, candidate)
		if err != nil {
			return err
		}
		if user != nil {
			return c.RenameUser(ctx, user.PK, to)
		}
	}

	return fmt.Errorf("user %q not found", from)
}
