package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/externalmcp"
	"github.com/helpin-ai/helpin/server/internal/model"
)

func (s *ExternalMCPService) SyncTools(ctx context.Context, workspaceID, serverID string) error {
	if err := s.requireEnabled(); err != nil {
		return err
	}
	server, err := s.repo.GetServer(ctx, workspaceID, serverID)
	if err != nil {
		return err
	}
	if server == nil {
		return ErrExternalMCPNotFound
	}
	credential, err := s.ensureFreshCredential(ctx, server)
	if err != nil {
		return err
	}
	headers, err := s.credentialHeaders(server, credential)
	if err != nil {
		return err
	}
	client, err := externalmcp.NewClient(server.EndpointURL, s.cfg.AllowedHosts, s.cfg.AllowInsecureLocalhost)
	if err != nil {
		return err
	}
	discovered, err := client.ListTools(ctx, headers)
	if err != nil {
		if message, redirected := externalMCPCrossHostRedirectMessage(err); redirected {
			s.setServerFailure(ctx, server, model.ExternalMCPStatusError, "mcp_endpoint_redirect", message)
		} else {
			switch {
			case externalmcp.IsStatus(err, 401):
				if server.AuthType == model.ExternalMCPAuthOAuth {
					s.markReauthorizationRequired(ctx, server, "remote_unauthorized")
				} else {
					s.setServerFailure(ctx, server, model.ExternalMCPStatusError, "credential_rejected", "The MCP server rejected the configured credential")
				}
			case externalmcp.IsStatus(err, 403):
				s.setServerFailure(ctx, server, model.ExternalMCPStatusRemoteDisabled, "remote_disabled", "The MCP server rejected access; verify it is enabled in the provider")
			default:
				s.setServerFailure(ctx, server, model.ExternalMCPStatusError, "tool_sync_failed", "Tool discovery failed")
			}
		}
		return err
	}
	tools := make([]model.ExternalMCPTool, 0, len(discovered))
	now := s.now()
	for _, remote := range discovered {
		if !validExternalMCPRemoteToolName.MatchString(remote.Name) {
			return fmt.Errorf("remote MCP tool %q has an unsupported name", remote.Name)
		}
		access := model.ExternalMCPToolAccessWrite
		if remote.ReadOnly || server.Provider == model.ExternalMCPProviderCustomerIO && customerIOReadTools[remote.Name] {
			access = model.ExternalMCPToolAccessRead
		}
		description := remote.Description
		if description == "" {
			description = fmt.Sprintf("%s tool from %s", remote.Name, server.Name)
		}
		tools = append(tools, model.ExternalMCPTool{
			WorkspaceID: workspaceID, ServerID: serverID, RemoteName: remote.Name,
			RuntimeAlias: "mcp__" + server.ServerName + "__" + remote.Name,
			Description:  description, InputSchema: remote.InputSchema, Access: access,
			Enabled: true, SchemaHash: remote.SchemaHash, LastSeenAt: now,
		})
	}
	if err := s.repo.ReplaceTools(ctx, workspaceID, serverID, tools); err != nil {
		return err
	}
	return s.repo.UpdateServer(ctx, workspaceID, serverID, map[string]any{
		"status": model.ExternalMCPStatusConnected, "last_tool_sync_at": now, "last_health_checked_at": now,
		"last_error_code": nil, "last_error_message": nil, "auth_incident_key": nil, "auth_incident_notified_at": nil,
	})
}

func externalMCPCrossHostRedirectMessage(err error) (string, bool) {
	target := externalmcp.RedirectTarget(err)
	parsed, parseErr := url.Parse(target)
	if parseErr != nil || parsed.Hostname() == "" {
		return "", false
	}
	return "The MCP endpoint redirected to " + parsed.Hostname() + ". Credentials were not forwarded. Remove this installation and reconnect using the provider's canonical endpoint.", true
}

// ResolveRunAttachments turns selected catalog aliases into the exact runtime
// server/tool allowlist and an in-memory current credential.
func (s *ExternalMCPService) ResolveRunAttachments(ctx context.Context, workspaceID string, aliases []string) (*ExternalMCPResolvedRun, error) {
	selected := make([]string, 0, len(aliases))
	seen := map[string]bool{}
	for _, alias := range aliases {
		alias = strings.TrimSpace(alias)
		if !strings.HasPrefix(alias, "mcp__") || seen[alias] {
			continue
		}
		seen[alias] = true
		selected = append(selected, alias)
	}
	if len(selected) == 0 {
		return &ExternalMCPResolvedRun{}, nil
	}
	if err := s.requireEnabled(); err != nil {
		return nil, err
	}
	tools, err := s.repo.ListEnabledToolsByAliases(ctx, workspaceID, selected)
	if err != nil {
		return nil, err
	}
	if len(tools) != len(selected) {
		return nil, fmt.Errorf("one or more selected external MCP tools are unavailable")
	}
	byServer := map[string][]model.ExternalMCPTool{}
	for _, tool := range tools {
		byServer[tool.ServerID] = append(byServer[tool.ServerID], tool)
	}
	serverIDs := make([]string, 0, len(byServer))
	for serverID := range byServer {
		serverIDs = append(serverIDs, serverID)
	}
	sort.Strings(serverIDs)
	resolved := &ExternalMCPResolvedRun{}
	for _, serverID := range serverIDs {
		server, err := s.repo.GetServer(ctx, workspaceID, serverID)
		if err != nil {
			return nil, err
		}
		if server == nil || !server.Enabled {
			return nil, fmt.Errorf("selected external MCP server is unavailable")
		}
		if server.Status == model.ExternalMCPStatusReauthorizationNeeded || server.Status == model.ExternalMCPStatusPendingOAuth {
			return nil, fmt.Errorf("%s: %w", server.Name, ErrExternalMCPReauthorize)
		}
		if server.Status != model.ExternalMCPStatusConnected {
			return nil, fmt.Errorf("external MCP server %s is not connected", server.Name)
		}
		credential, err := s.ensureFreshCredential(ctx, server)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", server.Name, err)
		}
		runtimeCredential, err := s.runtimeCredential(server, credential)
		if err != nil {
			return nil, err
		}
		runtimeTools := make([]ExternalMCPRunTool, 0, len(byServer[serverID]))
		toolAliases := make([]string, 0, len(byServer[serverID]))
		for _, tool := range byServer[serverID] {
			runtimeTools = append(runtimeTools, ExternalMCPRunTool{Name: tool.RemoteName, Access: tool.Access})
			toolAliases = append(toolAliases, tool.RuntimeAlias)
		}
		sort.Slice(runtimeTools, func(i, j int) bool { return runtimeTools[i].Name < runtimeTools[j].Name })
		sort.Strings(toolAliases)
		resolved.Servers = append(resolved.Servers, ExternalMCPRunServer{
			ServerID: server.ID, ServerName: server.ServerName, Transport: server.Transport,
			URL: server.EndpointURL, Tools: runtimeTools, Credential: runtimeCredential,
		})
		resolved.Bindings = append(resolved.Bindings, model.AgentRunExternalMCPBinding{
			WorkspaceID: workspaceID, ServerID: server.ID, ServerName: server.ServerName,
			ToolAliases: marshalStringList(toolAliases), CredentialExpiry: server.AccessTokenExpiresAt,
		})
	}
	return resolved, nil
}

func (s *ExternalMCPService) runtimeCredential(server *model.ExternalMCPServer, credential *model.ExternalMCPCredential) (*ExternalMCPRunCredential, error) {
	if credential == nil || server.AuthType == model.ExternalMCPAuthNone {
		return nil, nil
	}
	switch server.AuthType {
	case model.ExternalMCPAuthOAuth, model.ExternalMCPAuthBearerToken:
		token, err := s.decrypt(server.WorkspaceID, server.ID, "access_token", credential.EncryptedAccessToken)
		if err != nil {
			return nil, err
		}
		return &ExternalMCPRunCredential{Type: "bearer_token", AccessToken: token, ExpiresAt: credential.AccessTokenExpiresAt}, nil
	case model.ExternalMCPAuthHeaders:
		encoded, err := s.decrypt(server.WorkspaceID, server.ID, "headers", credential.EncryptedHeaders)
		if err != nil {
			return nil, err
		}
		var headers map[string]string
		if err := json.Unmarshal([]byte(encoded), &headers); err != nil {
			return nil, fmt.Errorf("decode external MCP credential")
		}
		return &ExternalMCPRunCredential{Type: "headers", Headers: headers}, nil
	default:
		return nil, nil
	}
}

// RefreshRunBindingCredential returns a current credential for a previously
// attached server. It cannot change URL or tool policy.
func (s *ExternalMCPService) RefreshRunBindingCredential(ctx context.Context, workspaceID, serverID string) (*ExternalMCPRunCredential, error) {
	server, err := s.repo.GetServer(ctx, workspaceID, serverID)
	if err != nil || server == nil {
		return nil, ErrExternalMCPNotFound
	}
	credential, err := s.ensureFreshCredential(ctx, server)
	if err != nil {
		return nil, err
	}
	return s.runtimeCredential(server, credential)
}

func (s *ExternalMCPService) PersistRunBindings(ctx context.Context, agentRunID, runtimeRunID string, bindings []model.AgentRunExternalMCPBinding) error {
	for i := range bindings {
		bindings[i].AgentRunID = agentRunID
		bindings[i].RuntimeRunID = runtimeRunID
	}
	return s.repo.CreateRunBindings(ctx, bindings)
}

// CredentialUpdatesForServer prepares current credentials for recent runs
// attached to one reauthorized server. Terminal runs are filtered by the agent
// service before any runtime call.
func (s *ExternalMCPService) CredentialUpdatesForServer(ctx context.Context, workspaceID, serverID string) ([]ExternalMCPRunCredentialUpdate, error) {
	bindings, err := s.repo.ListRunBindingsForServer(ctx, workspaceID, serverID)
	if err != nil {
		return nil, err
	}
	if len(bindings) == 0 {
		return []ExternalMCPRunCredentialUpdate{}, nil
	}
	credential, err := s.RefreshRunBindingCredential(ctx, workspaceID, serverID)
	if err != nil {
		return nil, err
	}
	updates := make([]ExternalMCPRunCredentialUpdate, 0, len(bindings))
	for _, binding := range bindings {
		updates = append(updates, ExternalMCPRunCredentialUpdate{
			AgentRunID: binding.AgentRunID, RuntimeRunID: binding.RuntimeRunID,
			ServerID: binding.ServerID, Credential: credential,
		})
	}
	return updates, nil
}

func (s *ExternalMCPService) markReauthorizationRequired(ctx context.Context, server *model.ExternalMCPServer, reason string) {
	incident := "reauthorization:" + reason
	now := s.now()
	updates := map[string]any{
		"status": model.ExternalMCPStatusReauthorizationNeeded, "last_error_code": reason,
		"last_error_message":      "Authorization expired or was revoked; reconnect this MCP server",
		"access_token_expires_at": server.AccessTokenExpiresAt,
	}
	shouldNotify := server.AuthIncidentKey == nil || *server.AuthIncidentKey != incident
	if shouldNotify {
		updates["auth_incident_key"] = incident
		updates["auth_incident_notified_at"] = now
	}
	_ = s.repo.UpdateServer(ctx, server.WorkspaceID, server.ID, updates)
	if shouldNotify && s.notifications != nil {
		recipient := server.CreatedBy
		if server.AuthorizedByUserID != nil && strings.TrimSpace(*server.AuthorizedByUserID) != "" {
			recipient = strings.TrimSpace(*server.AuthorizedByUserID)
		}
		_ = s.notifications.Emit(ctx, model.NotificationEventInput{
			WorkspaceID: server.WorkspaceID, EventType: "external_mcp.reauthorization_required",
			EntityType: "external_mcp_server", EntityID: server.ID,
			Title: "Reconnect " + server.Name, Body: "Authorization expired or was revoked. Reconnect the server before agents can use its tools.",
			Category: model.NotifCategoryStatusChanges, Priority: "high",
			Metadata:           model.JSONB{"server_id": server.ID, "settings_path": "/settings/mcp?tab=external"},
			ExplicitRecipients: []string{recipient}, SkipFollowers: true, SkipEmailDelivery: true,
		})
	}
}

func (s *ExternalMCPService) setServerFailure(ctx context.Context, server *model.ExternalMCPServer, status, code, message string) {
	if server == nil {
		return
	}
	_ = s.repo.UpdateServer(ctx, server.WorkspaceID, server.ID, map[string]any{
		"status": status, "last_error_code": code, "last_error_message": message, "last_health_checked_at": s.now(),
	})
}

// ExternalMCPErrorStatus maps service errors to stable API statuses.
func ExternalMCPErrorStatus(err error) int {
	switch {
	case err == nil:
		return 0
	case errors.Is(err, ErrExternalMCPNotFound):
		return 404
	case errors.Is(err, ErrExternalMCPDisabled):
		return 503
	case errors.Is(err, ErrExternalMCPReauthorize):
		return 409
	default:
		return 400
	}
}
