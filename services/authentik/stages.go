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
	body := map[string]any{
		"sources":            sourcePKs,
		"show_source_labels": true,
		"user_fields":        []string{},
	}
	_, err := c.Patch(ctx, fmt.Sprintf("/api/v3/stages/identification/%s/", stagePK), body)
	return err
}
