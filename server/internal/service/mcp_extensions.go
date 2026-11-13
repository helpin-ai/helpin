package service

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// MCPToolCall says who is calling an extension tool and in which workspace.
// The MCP service has already authenticated the caller and checked the
// tool's toolset, scope, permission, and workspace policy.
type MCPToolCall struct {
	WorkspaceID string
	// UserID is the user the call acts as: the connected user, or the
	// workspace member a service account runs for.
	UserID string
	// ServicePrincipalID is set when a service account makes the call.
	ServicePrincipalID string
}

// MCPToolExtension is a tool that another package adds to the public MCP
// server, such as an edition-specific capability. It goes through the same
// authentication, argument validation, idempotency, result size limit, and
// audit trail as every built-in tool.
type MCPToolExtension struct {
	Definition MCPToolDefinition
	// Available limits the tool to some workspaces. It is consulted on every
	// listing and call, so a workspace it excludes neither sees the tool nor
	// can call it. Nil offers the tool everywhere.
	Available func(workspaceID string) bool
	// Execute runs the tool. A *MCPToolError it returns is shown to the
	// caller as written; any other error is reported generically.
	Execute func(ctx context.Context, call MCPToolCall, arguments json.RawMessage) (*MCPToolResult, error)
}

// RegisterToolExtensions adds tools to the catalog. Call it during startup,
// before the server takes requests; the catalog is not guarded for
// concurrent registration. A tool must use an existing toolset and scope, so
// workspace policy and consent screens keep describing every grant.
func (s *MCPService) RegisterToolExtensions(extensions ...MCPToolExtension) error {
	for _, extension := range extensions {
		definition := extension.Definition
		name := strings.TrimSpace(definition.Name)
		switch {
		case name == "":
			return fmt.Errorf("MCP tool extension has no name")
		case extension.Execute == nil:
			return fmt.Errorf("MCP tool extension %q has no handler", name)
		case definition.CommandName != "":
			return fmt.Errorf("MCP tool extension %q cannot bind an internal command", name)
		case !slices.Contains(_allMCPToolsets, definition.Toolset):
			return fmt.Errorf("MCP tool extension %q uses unknown toolset %q", name, definition.Toolset)
		case !slices.Contains(_allMCPScopes, definition.Scope):
			return fmt.Errorf("MCP tool extension %q uses unknown scope %q", name, definition.Scope)
		}
		if _, exists := s.tools[name]; exists {
			return fmt.Errorf("MCP tool %q is already registered", name)
		}
		if definition.Mutating {
			definition.InputSchema = withMCPIdempotencyKey(definition.InputSchema)
			definition.IdempotentHint = true
		}
		resolved, err := resolveMCPToolSchema(definition.InputSchema)
		if err != nil {
			return fmt.Errorf("MCP tool extension %q has an invalid input schema: %w", name, err)
		}
		if definition.Title == "" {
			definition.Title = mcpToolTitle(name)
		}
		definition.Name = name
		extension.Definition = definition
		if s.extensions == nil {
			s.extensions = make(map[string]MCPToolExtension)
		}
		s.tools[name] = definition
		s.toolSchemas[name] = resolved
		s.extensions[name] = extension
	}
	return nil
}

// extensionToolAvailable reports whether a tool may be offered in a
// workspace. Built-in tools always may.
func (s *MCPService) extensionToolAvailable(toolName, workspaceID string) bool {
	extension, ok := s.extensions[toolName]
	if !ok || extension.Available == nil {
		return true
	}
	return extension.Available(workspaceID)
}

// executeMCPToolExtension runs an extension tool, or reports that the tool
// is not one.
func (s *MCPService) executeMCPToolExtension(ctx context.Context, principal *model.MCPPrincipal, name string, arguments json.RawMessage) (*MCPToolResult, bool, error) {
	extension, ok := s.extensions[name]
	if !ok {
		return nil, false, nil
	}
	result, err := extension.Execute(ctx, MCPToolCall{
		WorkspaceID: principal.WorkspaceID, UserID: principal.UserID, ServicePrincipalID: principal.ServicePrincipalID,
	}, arguments)
	return result, true, err
}
