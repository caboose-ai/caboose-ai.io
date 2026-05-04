package authentik

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
)

type FlowStageBinding struct {
	PK       string `json:"pk"`
	StageObj struct {
		PK        string `json:"pk"`
		Name      string `json:"name"`
		Component string `json:"component"`
	} `json:"stage_obj"`
	Order int `json:"order"`
}

type flowStageBindingList struct {
	Results []FlowStageBinding `json:"results"`
}

func (c *Client) ListFlowStageBindings(ctx context.Context, flowSlug string) ([]FlowStageBinding, error) {
	data, err := c.Get(ctx, fmt.Sprintf("/api/v3/flows/bindings/?flow_slug=%s&page_size=50", url.QueryEscape(flowSlug)))
	if err != nil {
		return nil, err
	}

	var list flowStageBindingList
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, fmt.Errorf("parsing flow stage bindings: %w", err)
	}
	return list.Results, nil
}

func (c *Client) GetFlowStageBinding(ctx context.Context, flowSlug, stagePK string) (*FlowStageBinding, error) {
	data, err := c.Get(ctx, fmt.Sprintf("/api/v3/flows/bindings/?flow_slug=%s&stage=%s",
		url.QueryEscape(flowSlug), url.QueryEscape(stagePK)))
	if err != nil {
		return nil, err
	}

	var list flowStageBindingList
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, fmt.Errorf("parsing flow stage binding: %w", err)
	}
	if len(list.Results) == 0 {
		return nil, nil
	}
	return &list.Results[0], nil
}

func (c *Client) CreateFlowStageBinding(ctx context.Context, flowPK, stagePK string, order int) error {
	payload := map[string]any{
		"target": flowPK,
		"stage":  stagePK,
		"order":  order,
	}
	_, err := c.Post(ctx, "/api/v3/flows/bindings/", payload)
	if err != nil {
		return fmt.Errorf("creating flow stage binding: %w", err)
	}
	return nil
}
