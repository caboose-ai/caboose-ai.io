package authentik

import (
	"context"
	"encoding/json"
	"fmt"
)

type Brand struct {
	BrandUUID string `json:"brand_uuid"`
	Domain    string `json:"domain"`
	Default   bool   `json:"default"`
}

type brandList struct {
	Results []Brand `json:"results"`
}

func (c *Client) GetDefaultBrand(ctx context.Context) (*Brand, error) {
	data, err := c.Get(ctx, "/api/v3/core/brands/?brand_default=true")
	if err != nil {
		return nil, err
	}

	var list brandList
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, fmt.Errorf("parsing brand response: %w", err)
	}
	if len(list.Results) == 0 {
		return nil, fmt.Errorf("default brand not found")
	}
	return &list.Results[0], nil
}

func (c *Client) SetBrandRecoveryFlow(ctx context.Context, brandUUID, flowUUID string) error {
	payload := map[string]string{"flow_recovery": flowUUID}
	_, err := c.Patch(ctx, fmt.Sprintf("/api/v3/core/brands/%s/", brandUUID), payload)
	if err != nil {
		return fmt.Errorf("setting recovery flow on brand: %w", err)
	}
	return nil
}
