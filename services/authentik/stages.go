package authentik

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
)

type CaptchaStage struct {
	PK   string `json:"pk"`
	Name string `json:"name"`
}

type CreateCaptchaStageParams struct {
	Name       string `json:"name"`
	PublicKey  string `json:"public_key"`
	PrivateKey string `json:"private_key"`
	JsURL      string `json:"js_url"`
	ApiURL     string `json:"api_url"`
}

type captchaStageList struct {
	Results []CaptchaStage `json:"results"`
}

type UserWriteStage struct {
	PK                    string `json:"pk"`
	Name                  string `json:"name"`
	CreateUsersAsInactive bool   `json:"create_users_as_inactive"`
}

type UserLogoutStage struct {
	PK   string `json:"pk"`
	Name string `json:"name"`
}

type userLogoutStageList struct {
	Results []UserLogoutStage `json:"results"`
}

type RedirectStage struct {
	PK           string `json:"pk"`
	Name         string `json:"name"`
	KeepContext  bool   `json:"keep_context"`
	Mode         string `json:"mode"`
	TargetStatic string `json:"target_static"`
}

type RedirectStageParams struct {
	Name         string `json:"name"`
	KeepContext  bool   `json:"keep_context"`
	Mode         string `json:"mode"`
	TargetStatic string `json:"target_static"`
}

type redirectStageList struct {
	Results []RedirectStage `json:"results"`
}

func (c *Client) GetCaptchaStage(ctx context.Context, name string) (*CaptchaStage, error) {
	data, err := c.Get(ctx, fmt.Sprintf("/api/v3/stages/captcha/?name=%s", url.QueryEscape(name)))
	if err != nil {
		return nil, err
	}

	var list captchaStageList
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, fmt.Errorf("parsing captcha stage response: %w", err)
	}
	if len(list.Results) == 0 {
		return nil, nil
	}
	return &list.Results[0], nil
}

func (c *Client) GetRedirectStage(ctx context.Context, name string) (*RedirectStage, error) {
	data, err := c.Get(ctx, fmt.Sprintf("/api/v3/stages/redirect/?name=%s", url.QueryEscape(name)))
	if err != nil {
		return nil, err
	}

	var list redirectStageList
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, fmt.Errorf("parsing redirect stage response: %w", err)
	}
	if len(list.Results) == 0 {
		return nil, nil
	}
	return &list.Results[0], nil
}

func (c *Client) CreateRedirectStage(ctx context.Context, params RedirectStageParams) (*RedirectStage, error) {
	data, err := c.Post(ctx, "/api/v3/stages/redirect/", params)
	if err != nil {
		return nil, fmt.Errorf("creating redirect stage %q: %w", params.Name, err)
	}

	var stage RedirectStage
	if err := json.Unmarshal(data, &stage); err != nil {
		return nil, fmt.Errorf("parsing created redirect stage: %w", err)
	}
	return &stage, nil
}

func (c *Client) PatchRedirectStage(ctx context.Context, pk string, params RedirectStageParams) error {
	_, err := c.Patch(ctx, fmt.Sprintf("/api/v3/stages/redirect/%s/", pk), params)
	if err != nil {
		return fmt.Errorf("patching redirect stage %q: %w", pk, err)
	}
	return nil
}

func (c *Client) GetUserLogoutStage(ctx context.Context, name string) (*UserLogoutStage, error) {
	data, err := c.Get(ctx, fmt.Sprintf("/api/v3/stages/user_logout/?name=%s", url.QueryEscape(name)))
	if err != nil {
		return nil, err
	}

	var list userLogoutStageList
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, fmt.Errorf("parsing user logout stage response: %w", err)
	}
	if len(list.Results) == 0 {
		return nil, nil
	}
	return &list.Results[0], nil
}

func (c *Client) CreateUserLogoutStage(ctx context.Context, name string) (*UserLogoutStage, error) {
	data, err := c.Post(ctx, "/api/v3/stages/user_logout/", map[string]string{"name": name})
	if err != nil {
		return nil, fmt.Errorf("creating user logout stage %q: %w", name, err)
	}

	var stage UserLogoutStage
	if err := json.Unmarshal(data, &stage); err != nil {
		return nil, fmt.Errorf("parsing created user logout stage: %w", err)
	}
	return &stage, nil
}

func (c *Client) CreateCaptchaStage(ctx context.Context, params CreateCaptchaStageParams) (*CaptchaStage, error) {
	data, err := c.Post(ctx, "/api/v3/stages/captcha/", params)
	if err != nil {
		return nil, fmt.Errorf("creating captcha stage %q: %w", params.Name, err)
	}

	var stage CaptchaStage
	if err := json.Unmarshal(data, &stage); err != nil {
		return nil, fmt.Errorf("parsing created captcha stage: %w", err)
	}
	return &stage, nil
}

func (c *Client) PatchCaptchaStage(ctx context.Context, pk string, params CreateCaptchaStageParams) error {
	_, err := c.Patch(ctx, fmt.Sprintf("/api/v3/stages/captcha/%s/", pk), params)
	if err != nil {
		return fmt.Errorf("patching captcha stage %q: %w", pk, err)
	}
	return nil
}

func (c *Client) GetUserWriteStage(ctx context.Context, pk string) (*UserWriteStage, error) {
	data, err := c.Get(ctx, fmt.Sprintf("/api/v3/stages/user_write/%s/", pk))
	if err != nil {
		return nil, err
	}

	var stage UserWriteStage
	if err := json.Unmarshal(data, &stage); err != nil {
		return nil, fmt.Errorf("parsing user write stage: %w", err)
	}
	return &stage, nil
}

func (c *Client) PatchUserWriteStage(ctx context.Context, pk string, inactive bool) error {
	body := map[string]bool{"create_users_as_inactive": inactive}
	_, err := c.Patch(ctx, fmt.Sprintf("/api/v3/stages/user_write/%s/", pk), body)
	return err
}

type IdentificationStage struct {
	PK               string   `json:"pk"`
	Name             string   `json:"name"`
	Sources          []string `json:"sources"`
	UserFields       []string `json:"user_fields"`
	ShowSourceLabels bool     `json:"show_source_labels"`
}

type identificationStageList struct {
	Results []IdentificationStage `json:"results"`
}

func (c *Client) GetIdentificationStage(ctx context.Context, flowSlug string) (*IdentificationStage, error) {
	data, err := c.Get(ctx, fmt.Sprintf("/api/v3/stages/identification/?flow=%s", url.QueryEscape(flowSlug)))
	if err != nil {
		return nil, err
	}

	var list identificationStageList
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, fmt.Errorf("parsing identification stage response: %w", err)
	}
	if len(list.Results) == 0 {
		return nil, nil
	}
	return &list.Results[0], nil
}

func (c *Client) SetIdentificationStageSources(ctx context.Context, stagePK string, sourcePKs []string) error {
	return c.PatchIdentificationStage(ctx, stagePK, sourcePKs, []string{}, true)
}

func (c *Client) PatchIdentificationStage(ctx context.Context, stagePK string, sourcePKs, userFields []string, showSourceLabels bool) error {
	if sourcePKs == nil {
		sourcePKs = []string{}
	}
	if userFields == nil {
		userFields = []string{}
	}
	body := map[string]any{
		"sources":            sourcePKs,
		"show_source_labels": showSourceLabels,
		"user_fields":        userFields,
	}
	_, err := c.Patch(ctx, fmt.Sprintf("/api/v3/stages/identification/%s/", stagePK), body)
	return err
}
