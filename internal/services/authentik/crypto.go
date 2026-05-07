package authentik

import (
	"context"
	"encoding/json"
	"fmt"
)

type CertificateKeyPair struct {
	PK   string `json:"pk"`
	Name string `json:"name"`
}

type certificateKeyPairList struct {
	Results []CertificateKeyPair `json:"results"`
}

func (c *Client) ListCertificateKeyPairs(ctx context.Context) ([]CertificateKeyPair, error) {
	data, err := c.Get(ctx, "/api/v3/crypto/certificatekeypairs/?page_size=50")
	if err != nil {
		return nil, err
	}

	var list certificateKeyPairList
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, fmt.Errorf("parsing certificate keypairs: %w", err)
	}
	return list.Results, nil
}
