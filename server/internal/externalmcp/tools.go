package externalmcp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	maxToolDiscoveryPages = 32
	maxDiscoveredTools    = 512
)

type DiscoveredTool struct {
	Name        string
	Description string
	InputSchema json.RawMessage
	ReadOnly    bool
	SchemaHash  string
}

// ListTools initializes a short-lived MCP session and retrieves all paginated
// tools. The caller supplies decrypted headers only in memory.
func (c *Client) ListTools(ctx context.Context, headers http.Header) ([]DiscoveredTool, error) {
	outcome := &requestOutcome{}
	httpClient := c.clientWithHeaders(headers, outcome)
	client := mcp.NewClient(&mcp.Implementation{Name: "helpin", Version: "1.0"}, &mcp.ClientOptions{})
	transport := &mcp.StreamableClientTransport{
		Endpoint:             c.Endpoint(),
		HTTPClient:           httpClient,
		MaxRetries:           -1,
		DisableStandaloneSSE: true,
	}
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return nil, safeMCPError("connection", outcome)
	}
	defer session.Close()

	var discovered []DiscoveredTool
	cursor := ""
	for page := 0; ; page++ {
		if page >= maxToolDiscoveryPages {
			return nil, &RemoteError{Operation: "tool discovery", Code: "tool_limit_exceeded"}
		}
		result, err := session.ListTools(ctx, &mcp.ListToolsParams{Cursor: cursor})
		if err != nil {
			return nil, safeMCPError("tool discovery", outcome)
		}
		for _, tool := range result.Tools {
			if tool == nil || strings.TrimSpace(tool.Name) == "" {
				continue
			}
			schema, err := json.Marshal(tool.InputSchema)
			if err != nil || len(schema) == 0 || string(schema) == "null" {
				schema = []byte("{}")
			}
			hash := sha256.Sum256(schema)
			discovered = append(discovered, DiscoveredTool{
				Name:        strings.TrimSpace(tool.Name),
				Description: strings.TrimSpace(tool.Description),
				InputSchema: schema,
				ReadOnly:    tool.Annotations != nil && tool.Annotations.ReadOnlyHint,
				SchemaHash:  hex.EncodeToString(hash[:]),
			})
			if len(discovered) > maxDiscoveredTools {
				return nil, &RemoteError{Operation: "tool discovery", Code: "tool_limit_exceeded"}
			}
		}
		if result.NextCursor == "" {
			break
		}
		cursor = result.NextCursor
	}
	return discovered, nil
}

func safeMCPError(operation string, outcome *requestOutcome) error {
	status, redirectTarget := outcome.snapshot()
	code := "remote_protocol_error"
	if status == http.StatusUnauthorized {
		code = "authentication_required"
	} else if status == http.StatusForbidden {
		code = "remote_forbidden"
	} else if status >= http.StatusMultipleChoices && status < http.StatusBadRequest && redirectTarget != "" {
		code = "remote_redirect"
	}
	return &RemoteError{Operation: operation, Status: status, Code: code, RedirectTarget: redirectTarget}
}
