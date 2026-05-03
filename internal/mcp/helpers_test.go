package mcp

import (
	"net/http"

	"github.com/caboose-ai/caboose-ai.io/internal/services/authentik"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func dummyAK() *authentik.Client {
	return authentik.NewClient("http://localhost:9999", "test-token", &http.Client{})
}

func extractText(result *sdkmcp.CallToolResult) string {
	if result == nil || len(result.Content) == 0 {
		return ""
	}
	tc, ok := result.Content[0].(*sdkmcp.TextContent)
	if !ok {
		return ""
	}
	return tc.Text
}
