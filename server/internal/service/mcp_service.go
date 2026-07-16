package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/google/uuid"

	"github.com/helpin-ai/helpin/server/internal/auth"
	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

var (
	// ErrMCPUnauthorized indicates an invalid or inactive MCP credential.
	ErrMCPUnauthorized = errors.New("MCP credential is invalid or inactive")
	// ErrMCPForbidden indicates that current policy, RBAC, or scopes deny an operation.
	ErrMCPForbidden = errors.New("MCP operation is not allowed")
	// ErrMCPDisabled indicates that MCP is disabled for a workspace.
	ErrMCPDisabled = errors.New("MCP is disabled for this workspace")
	// ErrMCPNotFound indicates that an MCP-managed record does not exist.
	ErrMCPNotFound = errors.New("MCP record not found")
	// ErrMCPConflict indicates a replay or state conflict.
	ErrMCPConflict = errors.New("MCP request conflicts with existing state")
	// ErrMCPInvalidArguments indicates that tool input did not match its published schema.
	ErrMCPInvalidArguments = errors.New("MCP tool arguments are invalid")
	// ErrMCPRateLimited indicates that a public MCP safety limit was reached.
	ErrMCPRateLimited = errors.New("MCP rate limit reached")
)

const (
	_mcpRefreshTokenTTL = 30 * 24 * time.Hour
	_mcpCodeTTL         = 5 * time.Minute
	_mcpResultMaxBytes  = 256 * 1024
)

// MCPServiceConfig configures public URLs and token audiences.
type MCPServiceConfig struct {
	AppBaseURL           string
	IssuerURL            string
	ResourceURL          string
	AuthorizationURL     string
	TokenURL             string
	RegistrationURL      string
	ServerEnabled        bool
	OAuthEnabled         bool
	ServiceTokensEnabled bool
	PMWriteEnabled       bool
	DocsWriteEnabled     bool
	AgentRunEnabled      bool
	CRMEnabled           bool
	SupportEnabled       bool
}

type mcpDocsSpaceService interface {
	List(context.Context, string, *authorization.Actor) ([]model.DocsSpaceWithTeams, error)
	Get(context.Context, string, *authorization.Actor) (*model.DocsSpaceWithTeams, error)
	Create(context.Context, string, model.CreateDocsSpaceRequest, string) (*model.DocsSpaceWithTeams, error)
}

type mcpDocsCollectionService interface {
	List(context.Context, string) ([]model.DocsCollection, error)
	Create(context.Context, string, string, model.CreateDocsCollectionRequest, string) (*model.DocsCollection, error)
}

// MCPService is the public MCP authorization and product execution boundary.
type MCPService struct {
	repo          *repository.MCPRepository
	workspaceRepo *repository.WorkspaceRepository
	userRepo      *repository.UserRepository
	authz         *authorization.AuthzService
	jwt           *auth.JWTManager
	commands      *InternalCommandService
	agents        *AgentService
	search        *SearchService
	tasks         *PMTaskService
	documents     *DocsDocumentService
	spaces        mcpDocsSpaceService
	collections   mcpDocsCollectionService
	crmContacts   *CRMContactService
	crmDeals      *CRMDealService
	support       *SupportInboxService
	config        MCPServiceConfig
	tools         map[string]MCPToolDefinition
	toolSchemas   map[string]*jsonschema.Resolved
}

// NewMCPService creates the public MCP service boundary.
func NewMCPService(
	repo *repository.MCPRepository,
	workspaceRepo *repository.WorkspaceRepository,
	userRepo *repository.UserRepository,
	authz *authorization.AuthzService,
	jwt *auth.JWTManager,
	commands *InternalCommandService,
	agents *AgentService,
	search *SearchService,
	tasks *PMTaskService,
	documents *DocsDocumentService,
	spaces *DocsSpaceService,
	collections *DocsCollectionService,
	crmContacts *CRMContactService,
	crmDeals *CRMDealService,
	support *SupportInboxService,
	config MCPServiceConfig,
) *MCPService {
	service := &MCPService{
		repo:          repo,
		workspaceRepo: workspaceRepo,
		userRepo:      userRepo,
		authz:         authz,
		jwt:           jwt,
		commands:      commands,
		agents:        agents,
		search:        search,
		tasks:         tasks,
		documents:     documents,
		spaces:        spaces,
		collections:   collections,
		crmContacts:   crmContacts,
		crmDeals:      crmDeals,
		support:       support,
		config:        normalizeMCPConfig(config),
	}
	service.tools = make(map[string]MCPToolDefinition)
	service.toolSchemas = make(map[string]*jsonschema.Resolved)
	for _, tool := range service.buildToolCatalog() {
		service.tools[tool.Name] = tool
		if resolved, err := resolveMCPToolSchema(tool.InputSchema); err == nil {
			service.toolSchemas[tool.Name] = resolved
		}
	}
	return service
}

// Config returns the normalized public MCP endpoint configuration.
func (s *MCPService) Config() MCPServiceConfig { return s.config }

// ToolCatalog returns all curated public tools before principal filtering.
func (s *MCPService) ToolCatalog() []MCPToolDefinition {
	tools := make([]MCPToolDefinition, 0, len(s.tools))
	for _, tool := range s.tools {
		tools = append(tools, tool)
	}
	slices.SortFunc(tools, func(a, b MCPToolDefinition) int { return strings.Compare(a.Name, b.Name) })
	return tools
}

// AuthenticateBearer validates a user access token or opaque service token.
func (s *MCPService) AuthenticateBearer(ctx context.Context, bearer string) (*model.MCPPrincipal, error) {
	if !s.config.ServerEnabled {
		return nil, ErrMCPDisabled
	}
	bearer = strings.TrimSpace(bearer)
	if bearer == "" {
		return nil, ErrMCPUnauthorized
	}
	if strings.HasPrefix(bearer, "hmp_") {
		return s.authenticateServiceToken(ctx, bearer)
	}
	claims, err := s.jwt.ValidateMCPAccessToken(bearer, s.config.IssuerURL, s.config.ResourceURL)
	if err != nil {
		return nil, ErrMCPUnauthorized
	}
	connection, err := s.repo.GetConnection(ctx, claims.ConnectionID)
	if err != nil {
		return nil, err
	}
	if connection == nil || connection.Status != model.MCPConnectionStatusActive ||
		connection.TokenVersion != claims.TokenVersion || connection.WorkspaceID != claims.WorkspaceID ||
		connection.UserID != claims.UserID || connection.ClientID != claims.ClientID {
		return nil, ErrMCPUnauthorized
	}
	principal := connectionPrincipal(connection)
	if _, _, err := s.resolveEffectivePrincipal(ctx, principal); err != nil {
		return nil, err
	}
	if err := s.repo.TouchConnection(ctx, connection.ID, time.Now()); err != nil {
		return nil, err
	}
	return principal, nil
}

// EffectiveTools returns only tools currently authorized for the principal.
func (s *MCPService) EffectiveTools(ctx context.Context, principal *model.MCPPrincipal) ([]MCPToolDefinition, error) {
	effective, actor, err := s.resolveEffectivePrincipal(ctx, principal)
	if err != nil {
		return nil, err
	}
	tools := make([]MCPToolDefinition, 0, len(s.tools))
	for _, tool := range s.tools {
		if s.canUseTool(ctx, effective, actor, tool) {
			tools = append(tools, tool)
		}
	}
	slices.SortFunc(tools, func(a, b MCPToolDefinition) int { return strings.Compare(a.Name, b.Name) })
	return tools, nil
}

// MCPDashboard is the settings-page view of policy, connections, and capabilities.
type MCPDashboard struct {
	Policy            model.MCPWorkspacePolicy    `json:"policy"`
	Connections       []model.MCPConnection       `json:"connections"`
	ServicePrincipals []model.MCPServicePrincipal `json:"service_principals"`
	AvailableToolsets []string                    `json:"available_toolsets"`
	AvailableScopes   []string                    `json:"available_scopes"`
	MCPURL            string                      `json:"mcp_url"`
	CanManage         bool                        `json:"can_manage"`
	CanViewActivity   bool                        `json:"can_view_activity"`
	CanUseMCP         bool                        `json:"can_use_mcp"`
	PlatformEnabled   bool                        `json:"platform_enabled"`
}

// GetDashboard returns MCP setup and management state for a workspace member.
func (s *MCPService) GetDashboard(ctx context.Context, workspaceID, userID string) (*MCPDashboard, error) {
	actor, err := s.authz.ResolveActor(ctx, workspaceID, userID)
	if err != nil {
		return nil, ErrMCPForbidden
	}
	if !s.authz.Can(actor, authorization.PermWorkspaceRead) {
		return nil, ErrMCPForbidden
	}
	canManage := s.authz.Can(actor, authorization.PermSettingsManage)
	canViewActivity := s.authz.Can(actor, authorization.PermSettingsRead)
	policy, err := s.Policy(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	connections, err := s.repo.ListConnections(ctx, workspaceID, userID, canManage)
	if err != nil {
		return nil, err
	}
	var servicePrincipals []model.MCPServicePrincipal
	if canManage {
		servicePrincipals, err = s.repo.ListServicePrincipals(ctx, workspaceID)
		if err != nil {
			return nil, err
		}
	}
	return &MCPDashboard{
		Policy:            *policy,
		Connections:       connections,
		ServicePrincipals: servicePrincipals,
		AvailableToolsets: AllMCPToolsets(),
		AvailableScopes:   AllMCPScopes(),
		MCPURL:            s.config.ResourceURL,
		CanManage:         canManage,
		CanViewActivity:   canViewActivity,
		CanUseMCP:         s.config.ServerEnabled && policy.Enabled,
		PlatformEnabled:   s.config.ServerEnabled,
	}, nil
}

// Policy returns the configured policy or the secure default without persisting it.
func (s *MCPService) Policy(ctx context.Context, workspaceID string) (*model.MCPWorkspacePolicy, error) {
	policy, err := s.repo.GetPolicy(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	if policy != nil {
		return policy, nil
	}
	return &model.MCPWorkspacePolicy{
		WorkspaceID:            workspaceID,
		Enabled:                true,
		EnforceReadOnly:        true,
		ServiceAccountsEnabled: false,
		AllowedToolsets:        mustMCPJSON(DefaultMCPToolsets()),
		AllowedScopes:          mustMCPJSON(DefaultMCPScopes()),
	}, nil
}

// UpdatePolicy validates and persists workspace MCP policy.
func (s *MCPService) UpdatePolicy(ctx context.Context, workspaceID, userID string, req model.UpdateMCPWorkspacePolicyRequest) (*model.MCPWorkspacePolicy, error) {
	actor, err := s.authz.ResolveActor(ctx, workspaceID, userID)
	if err != nil || !s.authz.Can(actor, authorization.PermSettingsManage) {
		return nil, ErrMCPForbidden
	}
	toolsets, err := validateMCPValues(req.AllowedToolsets, _allMCPToolsets, "toolset")
	if err != nil {
		return nil, err
	}
	scopes, err := validateMCPValues(req.AllowedScopes, _allMCPScopes, "scope")
	if err != nil {
		return nil, err
	}
	if len(toolsets) == 0 || len(scopes) == 0 {
		return nil, fmt.Errorf("at least one toolset and scope are required")
	}
	policy := &model.MCPWorkspacePolicy{
		WorkspaceID:            workspaceID,
		Enabled:                req.Enabled,
		EnforceReadOnly:        req.EnforceReadOnly,
		ServiceAccountsEnabled: req.ServiceAccountsEnabled,
		AllowedToolsets:        mustMCPJSON(toolsets),
		AllowedScopes:          mustMCPJSON(scopes),
		UpdatedBy:              &userID,
	}
	if err := s.repo.UpsertPolicy(ctx, policy); err != nil {
		return nil, err
	}
	s.audit(ctx, &model.MCPPrincipal{WorkspaceID: workspaceID, UserID: userID, ClientName: "Helpin settings"}, "policy.updated", "", "success", "", nil, nil)
	return policy, nil
}

// RevokeConnection revokes an owned connection, or any workspace connection for managers.
func (s *MCPService) RevokeConnection(ctx context.Context, workspaceID, userID, connectionID string) error {
	actor, err := s.authz.ResolveActor(ctx, workspaceID, userID)
	if err != nil {
		return ErrMCPForbidden
	}
	connection, err := s.repo.GetConnection(ctx, connectionID)
	if err != nil {
		return err
	}
	if connection == nil || connection.WorkspaceID != workspaceID {
		return ErrMCPNotFound
	}
	if connection.UserID != userID && !s.authz.Can(actor, authorization.PermSettingsManage) {
		return ErrMCPForbidden
	}
	if err := s.repo.RevokeConnection(ctx, connectionID, userID); err != nil {
		return err
	}
	principal := connectionPrincipal(connection)
	s.audit(ctx, principal, "connection.revoked", "", "success", "", nil, nil)
	return nil
}

// RevokeWorkspaceAccess immediately revokes every user and service credential for a workspace.
func (s *MCPService) RevokeWorkspaceAccess(ctx context.Context, workspaceID, userID string) (int64, error) {
	actor, err := s.authz.ResolveActor(ctx, workspaceID, userID)
	if err != nil || !s.authz.Can(actor, authorization.PermSettingsManage) {
		return 0, ErrMCPForbidden
	}
	revoked, err := s.repo.RevokeWorkspaceAccess(ctx, workspaceID, userID)
	if err != nil {
		return 0, err
	}
	s.audit(ctx, &model.MCPPrincipal{WorkspaceID: workspaceID, UserID: userID, ClientName: "Helpin settings"}, "workspace_access.revoked", "", "success", "", nil, nil)
	return revoked, nil
}

// CreateServicePrincipal creates a headless identity and returns its secret once.
func (s *MCPService) CreateServicePrincipal(ctx context.Context, workspaceID, userID string, req model.CreateMCPServicePrincipalRequest) (*model.MCPServicePrincipal, *model.MCPServiceTokenSecret, error) {
	if !s.config.ServerEnabled || !s.config.ServiceTokensEnabled {
		return nil, nil, ErrMCPDisabled
	}
	actor, err := s.authz.ResolveActor(ctx, workspaceID, userID)
	if err != nil || !s.authz.Can(actor, authorization.PermSettingsManage) {
		return nil, nil, ErrMCPForbidden
	}
	policy, err := s.Policy(ctx, workspaceID)
	if err != nil {
		return nil, nil, err
	}
	if !policy.Enabled || !policy.ServiceAccountsEnabled {
		return nil, nil, ErrMCPForbidden
	}
	name := strings.TrimSpace(req.Name)
	if name == "" || len(name) > 100 {
		return nil, nil, fmt.Errorf("name must be between 1 and 100 characters")
	}
	toolsets, scopes, readOnly, err := s.constrainGrant(policy, req.Toolsets, req.Scopes, req.ReadOnly)
	if err != nil {
		return nil, nil, err
	}
	principal := &model.MCPServicePrincipal{
		ID:          uuid.NewString(),
		WorkspaceID: workspaceID,
		Name:        name,
		Description: req.Description,
		ActorUserID: userID,
		Scopes:      mustMCPJSON(scopes),
		Toolsets:    mustMCPJSON(toolsets),
		ReadOnly:    readOnly,
		Status:      model.MCPServicePrincipalStatusActive,
		ExpiresAt:   req.ExpiresAt,
		CreatedBy:   userID,
	}
	if err := s.repo.CreateServicePrincipal(ctx, principal); err != nil {
		return nil, nil, err
	}
	secret, err := s.createServiceToken(ctx, principal, userID)
	if err != nil {
		return nil, nil, err
	}
	s.audit(ctx, &model.MCPPrincipal{Kind: model.MCPPrincipalKindService, WorkspaceID: workspaceID, ServicePrincipalID: principal.ID, UserID: userID, ClientName: principal.Name}, "service_principal.created", "", "success", "", nil, nil)
	return principal, secret, nil
}

// RotateServiceToken creates another secret without exposing existing secrets.
func (s *MCPService) RotateServiceToken(ctx context.Context, workspaceID, userID, principalID string) (*model.MCPServiceTokenSecret, error) {
	if !s.config.ServerEnabled || !s.config.ServiceTokensEnabled {
		return nil, ErrMCPDisabled
	}
	actor, err := s.authz.ResolveActor(ctx, workspaceID, userID)
	if err != nil || !s.authz.Can(actor, authorization.PermSettingsManage) {
		return nil, ErrMCPForbidden
	}
	principal, err := s.repo.GetServicePrincipal(ctx, principalID)
	if err != nil {
		return nil, err
	}
	if principal == nil || principal.WorkspaceID != workspaceID || principal.Status != model.MCPServicePrincipalStatusActive {
		return nil, ErrMCPNotFound
	}
	return s.createServiceToken(ctx, principal, userID)
}

// RevokeServiceToken revokes one service secret.
func (s *MCPService) RevokeServiceToken(ctx context.Context, workspaceID, userID, principalID, tokenID string) error {
	actor, err := s.authz.ResolveActor(ctx, workspaceID, userID)
	if err != nil || !s.authz.Can(actor, authorization.PermSettingsManage) {
		return ErrMCPForbidden
	}
	principal, err := s.repo.GetServicePrincipal(ctx, principalID)
	if err != nil {
		return err
	}
	if principal == nil || principal.WorkspaceID != workspaceID {
		return ErrMCPNotFound
	}
	return s.repo.RevokeServiceToken(ctx, principalID, tokenID)
}

// RevokeServicePrincipal revokes a service identity and every secret it owns.
func (s *MCPService) RevokeServicePrincipal(ctx context.Context, workspaceID, userID, principalID string) error {
	actor, err := s.authz.ResolveActor(ctx, workspaceID, userID)
	if err != nil || !s.authz.Can(actor, authorization.PermSettingsManage) {
		return ErrMCPForbidden
	}
	return s.repo.RevokeServicePrincipal(ctx, workspaceID, principalID)
}

// ListServiceTokens returns token metadata without secrets.
func (s *MCPService) ListServiceTokens(ctx context.Context, workspaceID, userID, principalID string) ([]model.MCPServiceToken, error) {
	actor, err := s.authz.ResolveActor(ctx, workspaceID, userID)
	if err != nil || !s.authz.Can(actor, authorization.PermSettingsManage) {
		return nil, ErrMCPForbidden
	}
	principal, err := s.repo.GetServicePrincipal(ctx, principalID)
	if err != nil {
		return nil, err
	}
	if principal == nil || principal.WorkspaceID != workspaceID {
		return nil, ErrMCPNotFound
	}
	return s.repo.ListServiceTokens(ctx, principalID)
}

// ListActivity returns recent sanitized MCP events to settings readers.
func (s *MCPService) ListActivity(ctx context.Context, workspaceID, userID string, limit int) ([]model.MCPAuditEvent, error) {
	actor, err := s.authz.ResolveActor(ctx, workspaceID, userID)
	if err != nil || !s.authz.Can(actor, authorization.PermSettingsRead) {
		return nil, ErrMCPForbidden
	}
	return s.repo.ListAuditEvents(ctx, workspaceID, limit)
}

// RecordProtocolEvent appends a sanitized transport or rate-limit audit event.
func (s *MCPService) RecordProtocolEvent(ctx context.Context, principal *model.MCPPrincipal, eventType, outcome, reason string) {
	s.audit(ctx, principal, eventType, "", outcome, reason, nil, nil)
}

func (s *MCPService) authenticateServiceToken(ctx context.Context, raw string) (*model.MCPPrincipal, error) {
	if !s.config.ServiceTokensEnabled {
		return nil, ErrMCPDisabled
	}
	token, err := s.repo.GetServiceTokenByHash(ctx, mcpHash(raw))
	if err != nil {
		return nil, err
	}
	now := time.Now()
	if token == nil || token.RevokedAt != nil || (token.ExpiresAt != nil && !token.ExpiresAt.After(now)) {
		return nil, ErrMCPUnauthorized
	}
	servicePrincipal, err := s.repo.GetServicePrincipal(ctx, token.ServicePrincipalID)
	if err != nil {
		return nil, err
	}
	if servicePrincipal == nil || servicePrincipal.Status != model.MCPServicePrincipalStatusActive ||
		(servicePrincipal.ExpiresAt != nil && !servicePrincipal.ExpiresAt.After(now)) {
		return nil, ErrMCPUnauthorized
	}
	principal := &model.MCPPrincipal{
		Kind:               model.MCPPrincipalKindService,
		WorkspaceID:        servicePrincipal.WorkspaceID,
		UserID:             servicePrincipal.ActorUserID,
		ServicePrincipalID: servicePrincipal.ID,
		ClientName:         servicePrincipal.Name,
		Scopes:             decodeMCPStrings(servicePrincipal.Scopes),
		Toolsets:           decodeMCPStrings(servicePrincipal.Toolsets),
		ReadOnly:           servicePrincipal.ReadOnly,
	}
	if _, _, err := s.resolveEffectivePrincipal(ctx, principal); err != nil {
		return nil, err
	}
	if err := s.repo.TouchServiceUse(ctx, token.ID, servicePrincipal.ID, now); err != nil {
		return nil, err
	}
	return principal, nil
}

func (s *MCPService) resolveEffectivePrincipal(ctx context.Context, principal *model.MCPPrincipal) (*model.MCPPrincipal, *authorization.Actor, error) {
	if !s.config.ServerEnabled {
		return nil, nil, ErrMCPDisabled
	}
	if principal == nil || principal.WorkspaceID == "" || principal.UserID == "" {
		return nil, nil, ErrMCPUnauthorized
	}
	policy, err := s.Policy(ctx, principal.WorkspaceID)
	if err != nil {
		return nil, nil, err
	}
	if !policy.Enabled {
		return nil, nil, ErrMCPDisabled
	}
	if principal.Kind == model.MCPPrincipalKindService && !policy.ServiceAccountsEnabled {
		return nil, nil, ErrMCPForbidden
	}
	actor, err := s.authz.ResolveActor(ctx, principal.WorkspaceID, principal.UserID)
	if err != nil {
		return nil, nil, ErrMCPUnauthorized
	}
	effective := *principal
	effective.Scopes = intersectMCPValues(principal.Scopes, decodeMCPStrings(policy.AllowedScopes))
	effective.Toolsets = intersectMCPValues(principal.Toolsets, decodeMCPStrings(policy.AllowedToolsets))
	effective.ReadOnly = principal.ReadOnly || policy.EnforceReadOnly
	return &effective, actor, nil
}

func (s *MCPService) canUseTool(ctx context.Context, principal *model.MCPPrincipal, actor *authorization.Actor, tool MCPToolDefinition) bool {
	if !s.platformToolEnabled(tool) {
		return false
	}
	if !containsMCPValue(principal.Toolsets, tool.Toolset) || !containsMCPValue(principal.Scopes, tool.Scope) {
		return false
	}
	if tool.Mutating && principal.ReadOnly {
		return false
	}
	if tool.Permission != "" && !s.authz.Can(actor, tool.Permission) {
		return false
	}
	if tool.Module != "" {
		allowed, err := s.authz.CanAccessModule(ctx, actor, tool.Module)
		return err == nil && allowed
	}
	return true
}

func (s *MCPService) platformToolEnabled(tool MCPToolDefinition) bool {
	switch tool.Toolset {
	case MCPToolsetPM:
		return !tool.Mutating || s.config.PMWriteEnabled
	case MCPToolsetDocs:
		return !tool.Mutating || s.config.DocsWriteEnabled
	case MCPToolsetAgents:
		return tool.Name != "start_agent_run" && tool.Name != "cancel_agent_run" || s.config.AgentRunEnabled
	case MCPToolsetCRM:
		return s.config.CRMEnabled
	case MCPToolsetSupport:
		return s.config.SupportEnabled
	default:
		return true
	}
}

func (s *MCPService) constrainGrant(policy *model.MCPWorkspacePolicy, requestedToolsets, requestedScopes []string, readOnly bool) ([]string, []string, bool, error) {
	toolsets, err := validateMCPValues(requestedToolsets, _allMCPToolsets, "toolset")
	if err != nil {
		return nil, nil, true, err
	}
	scopes, err := validateMCPValues(requestedScopes, _allMCPScopes, "scope")
	if err != nil {
		return nil, nil, true, err
	}
	toolsets = intersectMCPValues(toolsets, decodeMCPStrings(policy.AllowedToolsets))
	scopes = intersectMCPValues(scopes, decodeMCPStrings(policy.AllowedScopes))
	if len(toolsets) == 0 || len(scopes) == 0 {
		return nil, nil, true, fmt.Errorf("requested access is not allowed by workspace policy")
	}
	readOnly = readOnly || policy.EnforceReadOnly
	if readOnly {
		scopes = filterMCPReadScopes(scopes)
	}
	return toolsets, scopes, readOnly, nil
}

func (s *MCPService) createServiceToken(ctx context.Context, principal *model.MCPServicePrincipal, userID string) (*model.MCPServiceTokenSecret, error) {
	raw, err := randomMCPSecret("hmp_")
	if err != nil {
		return nil, err
	}
	record := model.MCPServiceToken{
		ID:                 uuid.NewString(),
		ServicePrincipalID: principal.ID,
		TokenHash:          mcpHash(raw),
		TokenPrefix:        raw[:min(len(raw), 12)],
		ExpiresAt:          principal.ExpiresAt,
		CreatedBy:          userID,
	}
	if err := s.repo.CreateServiceToken(ctx, &record); err != nil {
		return nil, err
	}
	return &model.MCPServiceTokenSecret{Token: raw, Record: record}, nil
}

func (s *MCPService) audit(ctx context.Context, principal *model.MCPPrincipal, eventType, toolName, outcome, reason string, request, result []byte) {
	if principal == nil || principal.WorkspaceID == "" {
		return
	}
	event := &model.MCPAuditEvent{
		ID:          uuid.NewString(),
		WorkspaceID: principal.WorkspaceID,
		ClientName:  principal.ClientName,
		EventType:   eventType,
		Outcome:     outcome,
	}
	if principal.ConnectionID != "" {
		event.ConnectionID = &principal.ConnectionID
	}
	if principal.ServicePrincipalID != "" {
		event.ServicePrincipalID = &principal.ServicePrincipalID
	}
	if principal.UserID != "" {
		event.UserID = &principal.UserID
	}
	if toolName != "" {
		event.ToolName = &toolName
	}
	if reason != "" {
		event.ReasonCode = &reason
	}
	if len(request) > 0 {
		hash := mcpHashBytes(request)
		event.RequestHash = &hash
	}
	if len(result) > 0 {
		hash := mcpHashBytes(result)
		event.ResultHash = &hash
	}
	_ = s.repo.CreateAuditEvent(ctx, event)
}

func connectionPrincipal(connection *model.MCPConnection) *model.MCPPrincipal {
	return &model.MCPPrincipal{
		Kind:         model.MCPPrincipalKindUser,
		WorkspaceID:  connection.WorkspaceID,
		UserID:       connection.UserID,
		ConnectionID: connection.ID,
		ClientID:     connection.ClientID,
		ClientName:   connection.ClientName,
		Scopes:       decodeMCPStrings(connection.Scopes),
		Toolsets:     decodeMCPStrings(connection.Toolsets),
		ReadOnly:     connection.ReadOnly,
		TokenVersion: connection.TokenVersion,
	}
}

func normalizeMCPConfig(config MCPServiceConfig) MCPServiceConfig {
	config.AppBaseURL = strings.TrimRight(strings.TrimSpace(config.AppBaseURL), "/")
	config.ResourceURL = strings.TrimRight(strings.TrimSpace(config.ResourceURL), "/")
	if config.ResourceURL == "" {
		config.ResourceURL = "http://localhost:8080/mcp"
	}
	config.IssuerURL = strings.TrimRight(strings.TrimSpace(config.IssuerURL), "/")
	if config.IssuerURL == "" {
		config.IssuerURL = strings.TrimSuffix(config.ResourceURL, "/mcp")
	}
	if config.AuthorizationURL == "" {
		config.AuthorizationURL = config.IssuerURL + "/api/mcp/oauth/authorize"
	}
	if config.TokenURL == "" {
		config.TokenURL = config.IssuerURL + "/api/mcp/oauth/token"
	}
	if config.RegistrationURL == "" {
		config.RegistrationURL = config.IssuerURL + "/api/mcp/oauth/register"
	}
	return config
}

func validateMCPValues(values, allowed []string, label string) ([]string, error) {
	seen := make(map[string]struct{}, len(values))
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if !containsMCPValue(allowed, value) {
			return nil, fmt.Errorf("unknown MCP %s %q", label, value)
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		normalized = append(normalized, value)
	}
	return normalized, nil
}

func intersectMCPValues(values, allowed []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if containsMCPValue(allowed, value) && !containsMCPValue(result, value) {
			result = append(result, value)
		}
	}
	return result
}

func containsMCPValue(values []string, target string) bool { return slices.Contains(values, target) }

func filterMCPReadScopes(scopes []string) []string {
	result := make([]string, 0, len(scopes))
	for _, scope := range scopes {
		if !strings.HasSuffix(scope, ".write") && scope != MCPScopeAgentsRun {
			result = append(result, scope)
		}
	}
	return result
}

func decodeMCPStrings(raw json.RawMessage) []string {
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil
	}
	return values
}

func mustMCPJSON(value any) json.RawMessage {
	encoded, err := json.Marshal(value)
	if err != nil {
		return json.RawMessage("[]")
	}
	return encoded
}

func randomMCPSecret(prefix string) (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generate MCP secret: %w", err)
	}
	return prefix + base64.RawURLEncoding.EncodeToString(bytes), nil
}

func mcpHash(value string) string { return mcpHashBytes([]byte(value)) }

func mcpHashBytes(value []byte) string {
	hash := sha256.Sum256(value)
	return hex.EncodeToString(hash[:])
}

func resolveMCPToolSchema(input map[string]any) (*jsonschema.Resolved, error) {
	encoded, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("encode MCP tool schema: %w", err)
	}
	var schema jsonschema.Schema
	if err := json.Unmarshal(encoded, &schema); err != nil {
		return nil, fmt.Errorf("decode MCP tool schema: %w", err)
	}
	resolved, err := schema.Resolve(nil)
	if err != nil {
		return nil, fmt.Errorf("resolve MCP tool schema: %w", err)
	}
	return resolved, nil
}
