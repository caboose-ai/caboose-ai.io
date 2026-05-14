package authentik

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

type Brand struct {
	BrandUUID         string `json:"brand_uuid"`
	Domain            string `json:"domain"`
	Default           bool   `json:"default"`
	BrandingCustomCSS string `json:"branding_custom_css"`
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

func (c *Client) SetBrandCustomCSS(ctx context.Context, brandUUID, css string) error {
	payload := map[string]string{"branding_custom_css": css}
	_, err := c.Patch(ctx, fmt.Sprintf("/api/v3/core/brands/%s/", brandUUID), payload)
	if err != nil {
		return fmt.Errorf("setting custom CSS on brand: %w", err)
	}
	return nil
}

const (
	googleSignInCSSStart = "/* homelab-google-sign-in:start */"
	googleSignInCSSEnd   = "/* homelab-google-sign-in:end */"
	GoogleSignInBrandCSS = `/* homelab-google-sign-in:start */
button.source-button[name="source-Google"] {
  width: 100%;
  min-height: 42px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 12px;
  border: 1px solid #dadce0;
  border-radius: 4px;
  background: #fff;
  color: #3c4043;
  box-shadow: none;
  font-family: Roboto, Arial, sans-serif;
  font-size: 14px;
  font-weight: 500;
}
button.source-button[name="source-Google"]:hover,
button.source-button[name="source-Google"]:focus {
  background: #f8fafd;
  border-color: #d2e3fc;
  color: #202124;
  box-shadow: 0 1px 2px rgba(60, 64, 67, 0.2);
}
button.source-button[name="source-Google"] .pf-c-button__icon,
button.source-button[name="source-Google"] .pf-c-button__icon img {
  width: 18px;
  height: 18px;
  margin: 0;
  filter: none !important;
}
/* homelab-google-sign-in:end */`
)

func EnsureGoogleSignInBrandCSS(existing string) string {
	css := strings.TrimSpace(existing)
	start := strings.Index(css, googleSignInCSSStart)
	if start >= 0 {
		end := strings.Index(css[start:], googleSignInCSSEnd)
		if end >= 0 {
			end += start + len(googleSignInCSSEnd)
			merged := strings.TrimSpace(css[:start])
			if merged != "" {
				merged += "\n\n"
			}
			merged += GoogleSignInBrandCSS
			trailing := strings.TrimSpace(css[end:])
			if trailing != "" {
				merged += "\n\n" + trailing
			}
			return merged + "\n"
		}
	}
	if css == "" {
		return GoogleSignInBrandCSS + "\n"
	}
	return css + "\n\n" + GoogleSignInBrandCSS + "\n"
}
