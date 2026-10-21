// Package mcpserver adapts Helpin's public MCP service boundary to the MCP protocol.
package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/ratelimit"
	"github.com/helpin-ai/helpin/server/internal/service"
)

const (
	_serverName    = "helpin"
	_serverVersion = "1.2.0"
	_maxBodyBytes  = 1024 * 1024
)

// Handler serves authenticated, stateless Streamable HTTP MCP requests.
type Handler struct {
	service    *service.MCPService
	streamable http.Handler
	limiter    *requestLimiter
}

// NewHandler creates the hosted public MCP protocol handler.
func NewHandler(mcpService *service.MCPService, limiter *ratelimit.Limiter) *Handler {
	handler := &Handler{service: mcpService, limiter: &requestLimiter{shared: limiter}}
	handler.streamable = mcp.NewStreamableHTTPHandler(handler.serverForRequest, &mcp.StreamableHTTPOptions{
		Stateless:    true,
		JSONResponse: true,
		Logger:       slog.Default().With("service", "public_mcp"),
	})
	return handler
}

// ServeHTTP authenticates and rate-limits a request before passing it to the MCP SDK.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !h.validOrigin(r) {
		http.Error(w, "origin is not allowed", http.StatusForbidden)
		return
	}
	bearer := bearerToken(r.Header.Get("Authorization"))
	principal, err := h.service.AuthenticateBearer(r.Context(), bearer)
	if err != nil {
		w.Header().Set("WWW-Authenticate", fmt.Sprintf(
			`Bearer resource_metadata=%q`, h.service.Config().IssuerURL+"/.well-known/oauth-protected-resource"))
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if !h.limiter.Allow(r.Context(), principal) {
		h.service.RecordProtocolEvent(r.Context(), principal, "rate_limit", "denied", "general_request_limit")
		w.Header().Set("Retry-After", "60")
		http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
		return
	}
	principal = principalForPath(principal, r.URL.Path)
	r.Body = http.MaxBytesReader(w, r.Body, _maxBodyBytes)
	ctx := context.WithValue(r.Context(), principalContextKey{}, principal)
	h.streamable.ServeHTTP(w, r.WithContext(ctx))
}

func (h *Handler) serverForRequest(r *http.Request) *mcp.Server {
	principal, _ := r.Context().Value(principalContextKey{}).(*model.MCPPrincipal)
	if principal == nil {
		return nil
	}
	tools, err := h.service.EffectiveTools(r.Context(), principal)
	if err != nil {
		return nil
	}
	server := mcp.NewServer(&mcp.Implementation{
		Name: _serverName, Title: "Helpin", Version: _serverVersion,
		WebsiteURL: h.service.Config().AppBaseURL,
	}, &mcp.ServerOptions{
		Instructions: "Use Helpin tools only in the connected workspace. Read current context first. Respect read-only mode, use idempotency keys for mutations, and poll agent runs with get_agent_run.",
		PageSize:     100,
		Capabilities: &mcp.ServerCapabilities{},
	})
	for _, definition := range tools {
		definition := definition
		server.AddTool(protocolTool(definition), func(ctx context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			if !h.limiter.AllowTool(ctx, principal, definition) {
				h.service.RecordProtocolEvent(ctx, principal, "rate_limit", "denied", "tool_class_limit")
				return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: "This Helpin tool is temporarily rate limited. Retry after 60 seconds."}}}, nil
			}
			callCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()
			result, err := h.service.ExecuteTool(callCtx, principal, definition.Name, request.Params.Arguments)
			if err != nil {
				message := publicToolError(err)
				return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: message}}}, nil
			}
			return &mcp.CallToolResult{
				Content:           []mcp.Content{&mcp.TextContent{Text: result.Summary}},
				StructuredContent: result,
			}, nil
		})
	}
	h.addResources(server, principal, tools)
	h.addWorkflowPrompts(server, tools)
	return server
}

func (h *Handler) addResources(server *mcp.Server, principal *model.MCPPrincipal, tools []service.MCPToolDefinition) {
	available := make(map[string]bool, len(tools))
	for _, tool := range tools {
		available[tool.Name] = true
	}
	if available["get_current_context"] {
		const uri = "helpin://workspace/current-context"
		server.AddResource(&mcp.Resource{
			URI: uri, Name: "current_context", Title: "Current Helpin context",
			Description: "Connected Helpin workspace, actor, scopes, modules, and read-only state.",
			MIMEType:    "application/json",
		}, func(ctx context.Context, _ *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
			return h.readToolResource(ctx, principal, uri, "get_current_context", json.RawMessage("{}"))
		})
	}

	templates := []struct {
		name, title, description, uriTemplate, prefix, tool, argument string
	}{
		{"task", "Helpin task", "A task in the connected workspace.", "helpin://tasks/{task_id}", "helpin://tasks/", "get_task", "task_id"},
		{"document", "Helpin document", "Document metadata in the connected workspace.", "helpin://documents/{document_id}", "helpin://documents/", "get_document", "document_id"},
		{"deal", "Helpin CRM deal", "A CRM deal in the connected workspace.", "helpin://crm/deals/{deal_id}", "helpin://crm/deals/", "get_crm_deal", "deal_id"},
		{"support_conversation", "Helpin support conversation", "A support conversation in the connected workspace.", "helpin://support/conversations/{conversation_id}", "helpin://support/conversations/", "get_support_conversation", "conversation_id"},
		{"agent_run", "Helpin agent run", "Status and artifacts for a durable Helpin agent run.", "helpin://agent-runs/{run_id}", "helpin://agent-runs/", "get_agent_run", "run_id"},
	}
	for _, template := range templates {
		template := template
		if !available[template.tool] {
			continue
		}
		server.AddResourceTemplate(&mcp.ResourceTemplate{
			Name: template.name, Title: template.title, Description: template.description,
			URITemplate: template.uriTemplate, MIMEType: "application/json",
		}, func(ctx context.Context, request *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
			rawID := strings.TrimPrefix(request.Params.URI, template.prefix)
			if rawID == request.Params.URI || strings.TrimSpace(rawID) == "" {
				return nil, mcp.ResourceNotFoundError(request.Params.URI)
			}
			id, err := url.PathUnescape(rawID)
			if err != nil || strings.Contains(id, "/") {
				return nil, mcp.ResourceNotFoundError(request.Params.URI)
			}
			arguments, err := json.Marshal(map[string]string{template.argument: id})
			if err != nil {
				return nil, err
			}
			return h.readToolResource(ctx, principal, request.Params.URI, template.tool, arguments)
		})
	}
}

func (h *Handler) readToolResource(ctx context.Context, principal *model.MCPPrincipal, uri, tool string, arguments json.RawMessage) (*mcp.ReadResourceResult, error) {
	result, err := h.service.ExecuteTool(ctx, principal, tool, arguments)
	if err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(result.Data)
	if err != nil {
		return nil, err
	}
	return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{{
		URI: uri, MIMEType: "application/json", Text: string(encoded),
	}}}, nil
}

func (h *Handler) addWorkflowPrompts(server *mcp.Server, tools []service.MCPToolDefinition) {
	available := make(map[string]bool, len(tools))
	for _, tool := range tools {
		available[tool.Name] = true
	}
	prompts := []struct {
		name         string
		title        string
		description  string
		text         string
		requiredTool string
	}{
		{
			name: "setup_workspace", title: "Set up your workspace",
			description:  "Configure workspace essentials and selected goals with MCP inspection and browser handoffs.",
			text:         "Call get_current_context and get_workspace_setup and confirm the intended workspace and identity. Follow the returned setup instructions, current checks and links to deliver a useful working result for the selected goals. Agree on one short plan and execute authorized work without repeated approvals. Use supported MCP writes and the client's browser for UI-only settings; identify missing grants and continue independent steps. A read grant alone does not authorize browser changes. Include recipients, roles, publishing, live activation and paid runs in the approved plan; let the user handle credentials and OAuth. Re-read get_workspace_setup after saved changes and verify the agreed real-world outcome. Report what works and the exact remaining handoffs; do not equate configuration with delivery or successful tests.",
			requiredTool: "get_workspace_setup",
		},
		{
			name: "plan_feature", title: "Plan a feature",
			description:  "Research workspace context, draft a PRD in Docs, and create linked implementation tasks.",
			text:         "Confirm the current Helpin context. Search for related tasks and documents. Draft a concise PRD, create it as a Helpin document, then create implementation tasks with explicit dependencies. Use one stable idempotency key per mutation and summarize links at the end.",
			requiredTool: "search_workspace",
		},
		{
			name: "prepare_release", title: "Prepare a release",
			description:  "Review Helpin release evidence, blockers, documentation, and durable agent work.",
			text:         "Confirm context, discover repositories, and search for the release, related tasks, and documents. Separate shipped evidence, blockers, rollout risk, and documentation gaps. Start a Helpin agent only after confirming the agent and target. Do not deploy or publish; return a readiness verdict and receipt.",
			requiredTool: "search_workspace",
		},
		{
			name: "setup_customer_support", title: "Set up customer support",
			description:  "Inspect support setup and continue through authorized browser handoffs.",
			text:         "Call get_support_setup and confirm the intended workspace. Follow its returned instructions to get the chosen channels working. Agree on a short plan including publishing, test messages and live activation, then execute approved work without asking again for each step. Reuse existing configuration, use authorized MCP tools or the browser handoffs, and continue independent steps while waiting for input. Let the user handle credentials and OAuth. Recheck setup after saved changes and verify actual receipt, reply delivery, routing and human handoff; configuration alone is not a successful test.",
			requiredTool: "get_support_setup",
		},
		{
			name: "triage_customer_issue", title: "Triage a customer issue",
			description:  "Investigate a support conversation and create bounded follow-up work without sending a customer reply.",
			text:         "Load the support conversation and public messages, search Helpin for related work and documentation, then summarize root cause and recommended follow-up. Do not send or publish customer-visible content. Create a task only after the user asks for it.",
			requiredTool: "get_support_conversation",
		},
		{
			name: "delegate_to_helpin_agent", title: "Delegate to a Helpin agent",
			description:  "Choose an available Helpin agent, start a durable run, and poll it to completion.",
			text:         "List available Helpin agents and verify the intended target. Start one agent run using a stable idempotency key. Poll get_agent_run until it reaches a terminal or user-input state, then report its artifacts and Helpin links.",
			requiredTool: "list_agents",
		},
		{
			name: "docs_maintenance", title: "Maintain Helpin documentation",
			description:  "Find stale Helpin documentation and prepare safe draft updates.",
			text:         "List Docs spaces and collections before choosing a location. Search and read the relevant Helpin documents and linked product work. Identify stale or unsupported claims before editing. Use the narrowest available draft mutation with a stable idempotency key. Create a space or collection only when the user requested a new location. Publish, unpublish, or archive documentation only when the user explicitly asks, and never delete it.",
			requiredTool: "list_documents",
		},
		{
			name: "review_pipeline", title: "Review the CRM pipeline",
			description:  "Review bounded CRM context and recommend evidence-backed next actions.",
			text:         "Confirm CRM access, list bounded deal and contact sets, and load only records that affect the review. Identify evidence-backed risks and next actions. Do not send email, enrich, delete, or bulk-change records. Make a bounded write only after explicit confirmation.",
			requiredTool: "list_deals",
		},
	}
	for _, promptDefinition := range prompts {
		promptDefinition := promptDefinition
		if !available[promptDefinition.requiredTool] {
			continue
		}
		server.AddPrompt(&mcp.Prompt{
			Name: promptDefinition.name, Title: promptDefinition.title, Description: promptDefinition.description,
		}, func(_ context.Context, _ *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
			return &mcp.GetPromptResult{
				Description: promptDefinition.description,
				Messages: []*mcp.PromptMessage{{
					Role: mcp.Role("user"), Content: &mcp.TextContent{Text: promptDefinition.text},
				}},
			}, nil
		})
	}
}

func protocolTool(definition service.MCPToolDefinition) *mcp.Tool {
	destructive := definition.Destructive
	openWorld := false
	return &mcp.Tool{
		Name: definition.Name, Title: definition.Title, Description: definition.Description,
		InputSchema: definition.InputSchema,
		OutputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"summary": map[string]any{"type": "string"},
				"data":    map[string]any{},
				"links":   map[string]any{"type": "object", "additionalProperties": map[string]any{"type": "string"}},
			},
			"required": []string{"summary", "data"},
		},
		Annotations: &mcp.ToolAnnotations{
			Title: definition.Title, ReadOnlyHint: !definition.Mutating,
			DestructiveHint: &destructive, IdempotentHint: definition.IdempotentHint,
			OpenWorldHint: &openWorld,
		},
	}
}

func (h *Handler) validOrigin(r *http.Request) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return true
	}
	for _, raw := range []string{h.service.Config().AppBaseURL, h.service.Config().IssuerURL, h.service.Config().ResourceURL} {
		parsed, err := url.Parse(raw)
		if err == nil && parsed.Scheme+"://"+parsed.Host == origin {
			return true
		}
	}
	return false
}

// ReadOnlyPath is the endpoint that exposes only read tools, whatever the
// connection's grant allows.
const ReadOnlyPath = "/mcp/readonly"

// principalForPath forces read-only mode on the read-only endpoint without
// mutating the authenticated principal.
func principalForPath(principal *model.MCPPrincipal, requestPath string) *model.MCPPrincipal {
	if principal == nil || strings.TrimSuffix(requestPath, "/") != ReadOnlyPath {
		return principal
	}
	readOnly := *principal
	readOnly.ReadOnly = true
	return &readOnly
}

func bearerToken(header string) string {
	parts := strings.Fields(header)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return parts[1]
}

func publicToolError(err error) string {
	var toolErr *service.MCPToolError
	if errors.As(err, &toolErr) {
		return toolErr.Code + ": " + toolErr.Message
	}
	switch {
	case errors.Is(err, service.ErrMCPUnauthorized):
		return "The Helpin connection is no longer authorized. Reconnect it and retry."
	case errors.Is(err, service.ErrMCPForbidden), errors.Is(err, service.ErrMCPDisabled):
		return "This action is not allowed by the current Helpin scopes, role, module access, or workspace policy."
	case errors.Is(err, service.ErrMCPNotFound):
		return "The requested Helpin record was not found in the connected workspace."
	case errors.Is(err, service.ErrMCPConflict):
		return "That idempotency key was already used for a different request. Use a new stable key."
	case errors.Is(err, service.ErrMCPInvalidArguments):
		return "The arguments do not match this tool's published schema. Check required fields and value types."
	case errors.Is(err, service.ErrMCPRateLimited):
		return "This Helpin operation reached a safety limit. Retry later or wait for an active agent run to finish."
	case errors.Is(err, context.DeadlineExceeded):
		return "The Helpin operation exceeded the 30 second synchronous deadline."
	default:
		return "The Helpin tool could not complete the request. Check the arguments and retry."
	}
}

type principalContextKey struct{}

type requestLimiter struct{ shared *ratelimit.Limiter }

func principalRateKey(principal *model.MCPPrincipal) string {
	actor := "user:" + principal.UserID
	if principal.ServicePrincipalID != "" {
		actor = "service:" + principal.ServicePrincipalID
	}
	return principal.WorkspaceID + ":" + actor
}

func (l *requestLimiter) Allow(ctx context.Context, principal *model.MCPPrincipal) bool {
	return l.shared.Allow(ctx, "mcp", principalRateKey(principal), false)
}

func (l *requestLimiter) AllowTool(ctx context.Context, principal *model.MCPPrincipal, tool service.MCPToolDefinition) bool {
	if !tool.Mutating && tool.Name != "search_workspace" {
		return true
	}
	return l.shared.Allow(ctx, "mcp", principalRateKey(principal), true)
}
