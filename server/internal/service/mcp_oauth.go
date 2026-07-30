package service

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// MCPClientRegistrationRequest is the supported OAuth dynamic-registration payload.
type MCPClientRegistrationRequest struct {
	ClientName              string   `json:"client_name"`
	ClientURI               *string  `json:"client_uri,omitempty"`
	RedirectURIs            []string `json:"redirect_uris"`
	GrantTypes              []string `json:"grant_types,omitempty"`
	ResponseTypes           []string `json:"response_types,omitempty"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method,omitempty"`
}

// MCPClientRegistrationResponse is the public-client registration response.
type MCPClientRegistrationResponse struct {
	ClientID                string   `json:"client_id"`
	ClientName              string   `json:"client_name"`
	ClientURI               *string  `json:"client_uri,omitempty"`
	RedirectURIs            []string `json:"redirect_uris"`
	GrantTypes              []string `json:"grant_types"`
	ResponseTypes           []string `json:"response_types"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"`
}

// MCPAuthorizationQuery contains validated OAuth authorization parameters.
type MCPAuthorizationQuery struct {
	ClientID            string `json:"client_id"`
	RedirectURI         string `json:"redirect_uri"`
	ResponseType        string `json:"response_type"`
	Scope               string `json:"scope"`
	State               string `json:"state"`
	CodeChallenge       string `json:"code_challenge"`
	CodeChallengeMethod string `json:"code_challenge_method"`
}

// MCPAuthorizationWorkspace is a workspace option that does not expose billing data.
type MCPAuthorizationWorkspace struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	Slug             string   `json:"slug"`
	Role             string   `json:"role"`
	AllowedScopes    []string `json:"allowed_scopes"`
	AllowedToolsets  []string `json:"allowed_toolsets"`
	ReadOnlyRequired bool     `json:"read_only_required"`
}

// MCPAuthorizationRequest is the backend-authoritative consent-screen model.
type MCPAuthorizationRequest struct {
	Client              model.MCPClientRegistration `json:"client"`
	Query               MCPAuthorizationQuery       `json:"query"`
	RequestedScopes     []string                    `json:"requested_scopes"`
	ProposedToolsets    []string                    `json:"proposed_toolsets"`
	Workspaces          []MCPAuthorizationWorkspace `json:"workspaces"`
	ReadOnlyRecommended bool                        `json:"read_only_recommended"`
}

// MCPAuthorizeDecision is the authenticated user's narrowed consent decision.
type MCPAuthorizeDecision struct {
	Query       MCPAuthorizationQuery `json:"query"`
	WorkspaceID string                `json:"workspace_id"`
	Scopes      []string              `json:"scopes"`
	Toolsets    []string              `json:"toolsets"`
	ReadOnly    bool                  `json:"read_only"`
}

// MCPAuthorizeResult contains the exact client redirect after consent.
type MCPAuthorizeResult struct {
	RedirectURL string `json:"redirect_url"`
}

// MCPTokenResponse is an OAuth access/refresh token response.
type MCPTokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
	Scope        string `json:"scope"`
}

// RegisterClient registers a public OAuth client with exact redirect URIs.
func (s *MCPService) RegisterClient(ctx context.Context, req MCPClientRegistrationRequest) (*MCPClientRegistrationResponse, error) {
	if !s.config.ServerEnabled || !s.config.OAuthEnabled {
		return nil, ErrMCPDisabled
	}
	name := strings.TrimSpace(req.ClientName)
	if name == "" || len(name) > 100 {
		return nil, fmt.Errorf("client_name must be between 1 and 100 characters")
	}
	if len(req.RedirectURIs) == 0 || len(req.RedirectURIs) > 10 {
		return nil, fmt.Errorf("between 1 and 10 redirect_uris are required")
	}
	redirects := make([]string, 0, len(req.RedirectURIs))
	for _, raw := range req.RedirectURIs {
		redirect, err := validateMCPRedirectURI(raw)
		if err != nil {
			return nil, err
		}
		if !slices.Contains(redirects, redirect) {
			redirects = append(redirects, redirect)
		}
	}
	if req.TokenEndpointAuthMethod != "" && req.TokenEndpointAuthMethod != "none" {
		return nil, fmt.Errorf("only token_endpoint_auth_method=none is supported")
	}
	client := &model.MCPClientRegistration{
		ID:           uuid.NewString(),
		ClientID:     uuid.NewString(),
		ClientName:   name,
		ClientURI:    req.ClientURI,
		RedirectURIs: mustMCPJSON(redirects),
	}
	if err := s.repo.CreateClient(ctx, client); err != nil {
		return nil, err
	}
	return &MCPClientRegistrationResponse{
		ClientID:                client.ClientID,
		ClientName:              client.ClientName,
		ClientURI:               client.ClientURI,
		RedirectURIs:            redirects,
		GrantTypes:              []string{"authorization_code", "refresh_token"},
		ResponseTypes:           []string{"code"},
		TokenEndpointAuthMethod: "none",
	}, nil
}

// GetAuthorizationRequest validates an OAuth request and returns available enabled workspaces.
func (s *MCPService) GetAuthorizationRequest(ctx context.Context, userID string, query MCPAuthorizationQuery) (*MCPAuthorizationRequest, error) {
	if !s.config.ServerEnabled || !s.config.OAuthEnabled {
		return nil, ErrMCPDisabled
	}
	client, requestedScopes, proposedToolsets, err := s.validateAuthorizationQuery(ctx, query)
	if err != nil {
		return nil, err
	}
	workspaces, err := s.workspaceRepo.List(ctx, userID, "")
	if err != nil {
		return nil, err
	}
	options := make([]MCPAuthorizationWorkspace, 0, len(workspaces))
	for _, workspace := range workspaces {
		policy, err := s.Policy(ctx, workspace.ID)
		if err != nil {
			return nil, err
		}
		if !policy.Enabled {
			continue
		}
		allowedScopes, allowedToolsets, readOnlyRequired := mcpAuthorizationWorkspaceAccess(
			policy,
			requestedScopes,
			proposedToolsets,
		)
		if len(allowedScopes) == 0 || len(allowedToolsets) == 0 {
			continue
		}
		options = append(options, MCPAuthorizationWorkspace{
			ID: workspace.ID, Name: workspace.Name, Slug: workspace.Slug, Role: workspace.Role,
			AllowedScopes: allowedScopes, AllowedToolsets: allowedToolsets,
			ReadOnlyRequired: readOnlyRequired,
		})
	}
	return &MCPAuthorizationRequest{
		Client:              *client,
		Query:               query,
		RequestedScopes:     requestedScopes,
		ProposedToolsets:    proposedToolsets,
		Workspaces:          options,
		ReadOnlyRecommended: true,
	}, nil
}

func mcpAuthorizationWorkspaceAccess(
	policy *model.MCPWorkspacePolicy,
	requestedScopes, proposedToolsets []string,
) ([]string, []string, bool) {
	allowedScopes := intersectMCPValues(
		requestedScopes,
		decodeMCPStrings(policy.AllowedScopes),
	)
	allowedToolsets := intersectMCPValues(
		proposedToolsets,
		decodeMCPStrings(policy.AllowedToolsets),
	)
	readOnlyRequired := policy.EnforceReadOnly ||
		!hasMCPWriteGrant(allowedScopes, allowedToolsets)
	grantableScopes := allowedScopes
	if readOnlyRequired {
		grantableScopes = filterMCPReadScopes(grantableScopes)
	}
	allowedToolsets = intersectMCPValues(
		allowedToolsets,
		toolsetsForMCPScopes(grantableScopes),
	)
	if len(grantableScopes) == 0 || len(allowedToolsets) == 0 {
		return nil, nil, readOnlyRequired
	}
	return allowedScopes, allowedToolsets, readOnlyRequired
}

// Authorize creates a workspace-bound connection and single-use PKCE code.
func (s *MCPService) Authorize(ctx context.Context, userID string, decision MCPAuthorizeDecision) (*MCPAuthorizeResult, error) {
	if !s.config.ServerEnabled || !s.config.OAuthEnabled {
		return nil, ErrMCPDisabled
	}
	client, requestedScopes, proposedToolsets, err := s.validateAuthorizationQuery(ctx, decision.Query)
	if err != nil {
		return nil, err
	}
	if _, err := s.authz.ResolveActor(ctx, decision.WorkspaceID, userID); err != nil {
		return nil, ErrMCPForbidden
	}
	policy, err := s.Policy(ctx, decision.WorkspaceID)
	if err != nil {
		return nil, err
	}
	if !policy.Enabled {
		return nil, ErrMCPDisabled
	}
	if !isMCPSubset(decision.Scopes, requestedScopes) || !isMCPSubset(decision.Toolsets, proposedToolsets) {
		return nil, fmt.Errorf("consent can only narrow requested scopes and toolsets")
	}
	toolsets, scopes, readOnly, err := s.constrainGrant(policy, decision.Toolsets, decision.Scopes, decision.ReadOnly)
	if err != nil {
		return nil, err
	}
	connection := &model.MCPConnection{
		ID:           uuid.NewString(),
		WorkspaceID:  decision.WorkspaceID,
		UserID:       userID,
		ClientID:     client.ClientID,
		ClientName:   client.ClientName,
		Scopes:       mustMCPJSON(scopes),
		Toolsets:     mustMCPJSON(toolsets),
		ReadOnly:     readOnly,
		Status:       model.MCPConnectionStatusActive,
		TokenVersion: 1,
	}
	if err := s.repo.CreateConnection(ctx, connection); err != nil {
		return nil, err
	}
	rawCode, err := randomMCPSecret("hmc_")
	if err != nil {
		return nil, err
	}
	code := &model.MCPOAuthAuthorizationCode{
		ID:                  uuid.NewString(),
		CodeHash:            mcpHash(rawCode),
		ConnectionID:        connection.ID,
		ClientID:            client.ClientID,
		RedirectURI:         decision.Query.RedirectURI,
		CodeChallenge:       decision.Query.CodeChallenge,
		CodeChallengeMethod: "S256",
		ExpiresAt:           time.Now().Add(_mcpCodeTTL),
	}
	if err := s.repo.CreateAuthorizationCode(ctx, code); err != nil {
		return nil, err
	}
	redirect, _ := url.Parse(decision.Query.RedirectURI)
	values := redirect.Query()
	values.Set("code", rawCode)
	values.Set("state", decision.Query.State)
	redirect.RawQuery = values.Encode()
	s.audit(ctx, connectionPrincipal(connection), "connection.authorized", "", "success", "", nil, nil)
	return &MCPAuthorizeResult{RedirectURL: redirect.String()}, nil
}

// ExchangeAuthorizationCode validates PKCE and issues a rotating token pair.
func (s *MCPService) ExchangeAuthorizationCode(ctx context.Context, codeRaw, clientID, redirectURI, verifier string) (*MCPTokenResponse, error) {
	if !s.config.ServerEnabled || !s.config.OAuthEnabled {
		return nil, ErrMCPDisabled
	}
	code, err := s.repo.ConsumeAuthorizationCode(ctx, mcpHash(codeRaw), time.Now())
	if err != nil {
		return nil, err
	}
	if code == nil || code.ClientID != clientID || code.RedirectURI != redirectURI ||
		!verifyMCPCodeChallenge(verifier, code.CodeChallenge) {
		return nil, ErrMCPUnauthorized
	}
	connection, err := s.repo.GetConnection(ctx, code.ConnectionID)
	if err != nil {
		return nil, err
	}
	if connection == nil || connection.Status != model.MCPConnectionStatusActive {
		return nil, ErrMCPUnauthorized
	}
	return s.issueTokenPair(ctx, connection, uuid.NewString())
}

// RefreshAccessToken rotates a refresh token and issues a new access token.
func (s *MCPService) RefreshAccessToken(ctx context.Context, raw, clientID string) (*MCPTokenResponse, error) {
	if !s.config.ServerEnabled || !s.config.OAuthEnabled {
		return nil, ErrMCPDisabled
	}
	record, err := s.repo.GetRefreshToken(ctx, mcpHash(raw))
	if err != nil {
		return nil, err
	}
	if record == nil {
		return nil, ErrMCPUnauthorized
	}
	if record.ConsumedAt != nil {
		_ = s.repo.RevokeRefreshFamily(ctx, record.FamilyID)
		if connection, getErr := s.repo.GetConnection(ctx, record.ConnectionID); getErr == nil && connection != nil {
			s.audit(ctx, connectionPrincipal(connection), "token.refresh_reuse", "", "denied", "refresh_reuse", nil, nil)
		}
		return nil, ErrMCPUnauthorized
	}
	now := time.Now()
	if record.RevokedAt != nil || !record.ExpiresAt.After(now) {
		return nil, ErrMCPUnauthorized
	}
	connection, err := s.repo.GetConnection(ctx, record.ConnectionID)
	if err != nil {
		return nil, err
	}
	if connection == nil || connection.Status != model.MCPConnectionStatusActive || connection.ClientID != clientID {
		return nil, ErrMCPUnauthorized
	}
	principal, _, err := s.resolveEffectivePrincipal(ctx, connectionPrincipal(connection))
	if err != nil {
		return nil, err
	}
	access, expiresAt, err := s.jwt.GenerateMCPAccessToken(*principal, s.config.IssuerURL, s.config.ResourceURL)
	if err != nil {
		return nil, err
	}
	refreshRaw, err := randomMCPSecret("hmr_")
	if err != nil {
		return nil, err
	}
	replacement := &model.MCPRefreshToken{
		ID:           uuid.NewString(),
		TokenHash:    mcpHash(refreshRaw),
		FamilyID:     record.FamilyID,
		ConnectionID: connection.ID,
		ExpiresAt:    now.Add(_mcpRefreshTokenTTL),
	}
	if err := s.repo.RotateRefreshToken(ctx, record.ID, replacement, now); err != nil {
		return nil, ErrMCPUnauthorized
	}
	s.audit(ctx, principal, "token.refreshed", "", "success", "", nil, nil)
	return &MCPTokenResponse{
		AccessToken: access, TokenType: "Bearer", ExpiresIn: int64(time.Until(expiresAt).Seconds()),
		RefreshToken: refreshRaw, Scope: strings.Join(principal.Scopes, " "),
	}, nil
}

// RevokeOAuthToken revokes a refresh family or the connection behind an access token.
func (s *MCPService) RevokeOAuthToken(ctx context.Context, raw string) error {
	if strings.HasPrefix(raw, "hmr_") {
		record, err := s.repo.GetRefreshToken(ctx, mcpHash(raw))
		if err != nil || record == nil {
			return err
		}
		if err := s.repo.RevokeRefreshFamily(ctx, record.FamilyID); err != nil {
			return err
		}
		if connection, getErr := s.repo.GetConnection(ctx, record.ConnectionID); getErr == nil && connection != nil {
			s.audit(ctx, connectionPrincipal(connection), "token.revoked", "", "success", "", nil, nil)
		}
		return nil
	}
	claims, err := s.jwt.ValidateMCPAccessToken(raw, s.config.IssuerURL, s.config.ResourceURL)
	if err != nil {
		return nil
	}
	connection, err := s.repo.GetConnection(ctx, claims.ConnectionID)
	if err != nil || connection == nil {
		return err
	}
	if err := s.repo.RevokeConnection(ctx, claims.ConnectionID, ""); err != nil {
		return err
	}
	s.audit(ctx, connectionPrincipal(connection), "token.revoked", "", "success", "", nil, nil)
	return nil
}

// OAuthAuthorizationServerMetadata returns RFC 8414-compatible server metadata.
func (s *MCPService) OAuthAuthorizationServerMetadata() map[string]any {
	return map[string]any{
		"issuer":                                     s.config.IssuerURL,
		"authorization_endpoint":                     s.config.AuthorizationURL,
		"token_endpoint":                             s.config.TokenURL,
		"registration_endpoint":                      s.config.RegistrationURL,
		"revocation_endpoint":                        s.config.IssuerURL + "/api/mcp/oauth/revoke",
		"response_types_supported":                   []string{"code"},
		"grant_types_supported":                      []string{"authorization_code", "refresh_token"},
		"code_challenge_methods_supported":           []string{"S256"},
		"token_endpoint_auth_methods_supported":      []string{"none"},
		"revocation_endpoint_auth_methods_supported": []string{"none"},
		"scopes_supported":                           AllMCPScopes(),
	}
}

// OAuthProtectedResourceMetadata returns MCP protected-resource metadata.
func (s *MCPService) OAuthProtectedResourceMetadata() map[string]any {
	return map[string]any{
		"resource":                 s.config.ResourceURL,
		"authorization_servers":    []string{s.config.IssuerURL},
		"scopes_supported":         AllMCPScopes(),
		"bearer_methods_supported": []string{"header"},
	}
}

// AuthorizationFrontendURL returns the Helpin consent UI URL for a raw OAuth query.
func (s *MCPService) AuthorizationFrontendURL(rawQuery string) string {
	base := s.config.AppBaseURL + "/oauth/authorize"
	if rawQuery == "" {
		return base
	}
	return base + "?" + rawQuery
}

func (s *MCPService) issueTokenPair(ctx context.Context, connection *model.MCPConnection, familyID string) (*MCPTokenResponse, error) {
	principal, _, err := s.resolveEffectivePrincipal(ctx, connectionPrincipal(connection))
	if err != nil {
		return nil, err
	}
	access, expiresAt, err := s.jwt.GenerateMCPAccessToken(*principal, s.config.IssuerURL, s.config.ResourceURL)
	if err != nil {
		return nil, err
	}
	refreshRaw, err := randomMCPSecret("hmr_")
	if err != nil {
		return nil, err
	}
	refresh := &model.MCPRefreshToken{
		ID: uuid.NewString(), TokenHash: mcpHash(refreshRaw), FamilyID: familyID,
		ConnectionID: connection.ID, ExpiresAt: time.Now().Add(_mcpRefreshTokenTTL),
	}
	if err := s.repo.CreateRefreshToken(ctx, refresh); err != nil {
		return nil, err
	}
	s.audit(ctx, principal, "token.issued", "", "success", "", nil, nil)
	return &MCPTokenResponse{
		AccessToken: access, TokenType: "Bearer", ExpiresIn: int64(time.Until(expiresAt).Seconds()),
		RefreshToken: refreshRaw, Scope: strings.Join(principal.Scopes, " "),
	}, nil
}

func (s *MCPService) validateAuthorizationQuery(ctx context.Context, query MCPAuthorizationQuery) (*model.MCPClientRegistration, []string, []string, error) {
	if query.ResponseType != "code" || query.CodeChallengeMethod != "S256" || query.CodeChallenge == "" || query.State == "" {
		return nil, nil, nil, fmt.Errorf("response_type=code, state, and PKCE S256 are required")
	}
	client, err := s.repo.GetClient(ctx, query.ClientID)
	if err != nil {
		return nil, nil, nil, err
	}
	if client == nil || !containsMCPValue(decodeMCPStrings(client.RedirectURIs), query.RedirectURI) {
		return nil, nil, nil, ErrMCPUnauthorized
	}
	requestedScopes, err := validateMCPValues(strings.Fields(query.Scope), _allMCPScopes, "scope")
	if err != nil || len(requestedScopes) == 0 {
		return nil, nil, nil, fmt.Errorf("at least one supported scope is required")
	}
	return client, requestedScopes, toolsetsForMCPScopes(requestedScopes), nil
}

func validateMCPRedirectURI(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.Fragment != "" || parsed.User != nil {
		return "", fmt.Errorf("invalid redirect_uri %q", raw)
	}
	loopback := parsed.Hostname() == "localhost" || parsed.Hostname() == "127.0.0.1" || parsed.Hostname() == "::1"
	if parsed.Scheme != "https" && !(loopback && parsed.Scheme == "http") {
		return "", fmt.Errorf("redirect_uri must use HTTPS except for loopback clients")
	}
	return parsed.String(), nil
}

func toolsetsForMCPScopes(scopes []string) []string {
	var toolsets []string
	for _, scope := range scopes {
		var toolset string
		switch {
		case scope == MCPScopeContextRead:
			toolset = MCPToolsetContext
		case strings.HasPrefix(scope, "helpin.pm."):
			toolset = MCPToolsetPM
		case strings.HasPrefix(scope, "helpin.docs."):
			toolset = MCPToolsetDocs
		case strings.HasPrefix(scope, "helpin.crm."):
			toolset = MCPToolsetCRM
		case strings.HasPrefix(scope, "helpin.support."):
			toolset = MCPToolsetSupport
		case strings.HasPrefix(scope, "helpin.agents."):
			toolset = MCPToolsetAgents
		}
		if toolset != "" && !slices.Contains(toolsets, toolset) {
			toolsets = append(toolsets, toolset)
		}
	}
	return toolsets
}

func verifyMCPCodeChallenge(verifier, expected string) bool {
	if len(verifier) < 43 || len(verifier) > 128 {
		return false
	}
	hash := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(hash[:]) == expected
}

func isMCPSubset(values, allowed []string) bool {
	if len(values) == 0 {
		return false
	}
	for _, value := range values {
		if !slices.Contains(allowed, value) {
			return false
		}
	}
	return true
}
