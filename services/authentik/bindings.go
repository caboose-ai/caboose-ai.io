package authentik

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
)

type FlowStageBinding struct {
	Target   string `json:"target"`
	Stage    string `json:"stage"`
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
	flow, err := c.GetFlow(ctx, flowSlug)
	if err != nil {
		return nil, err
	}

	data, err := c.Get(ctx, fmt.Sprintf("/api/v3/flows/bindings/?flow_slug=%s&page_size=50", url.QueryEscape(flowSlug)))
	if err != nil {
		return nil, err
	}

	var list flowStageBindingList
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, fmt.Errorf("parsing flow stage bindings: %w", err)
	}
	return filterFlowStageBindings(list.Results, flow.PK, ""), nil
}

func (c *Client) GetFlowStageBinding(ctx context.Context, flowSlug, stagePK string) (*FlowStageBinding, error) {
	flow, err := c.GetFlow(ctx, flowSlug)
	if err != nil {
		return nil, err
	}

	data, err := c.Get(ctx, fmt.Sprintf("/api/v3/flows/bindings/?flow_slug=%s&stage=%s",
		url.QueryEscape(flowSlug), url.QueryEscape(stagePK)))
	if err != nil {
		return nil, err
	}
	var list flowStageBindingList
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, fmt.Errorf("parsing flow stage binding: %w", err)
	}
	filtered := filterFlowStageBindings(list.Results, flow.PK, stagePK)
	if len(filtered) == 0 {
		return nil, nil
	}
	return &filtered[0], nil
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

func filterFlowStageBindings(bindings []FlowStageBinding, flowPK, stagePK string) []FlowStageBinding {
	filtered := make([]FlowStageBinding, 0, len(bindings))
	for _, binding := range bindings {
		if binding.Target != flowPK {
			continue
		}
		if stagePK != "" && binding.Stage != stagePK && binding.StageObj.PK != stagePK {
			continue
		}
		filtered = append(filtered, binding)
	}
	return filtered
}
