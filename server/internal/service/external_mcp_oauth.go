package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	mcpauth "github.com/helpin-ai/agent-runtime-go/mcpauth"

	"github.com/helpin-ai/helpin/server/internal/externalmcp"
	"github.com/helpin-ai/helpin/server/internal/model"
)

const externalMCPOAuthStateTTL = 10 * time.Minute
const externalMCPRefreshSkew = 2 * time.Minute

func (s *ExternalMCPService) StartOAuth(ctx context.Context, workspaceID, serverID, userID, returnPath string) (*ExternalMCPOAuthStart, error) {
	if err := s.requireEnabled(); err != nil {
		return nil, err
	}
	server, err := s.repo.GetServer(ctx, workspaceID, serverID)
	if err != nil {
		return nil, err
	}
	if server == nil {
		return nil, ErrExternalMCPNotFound
	}
	if server.AuthType != model.ExternalMCPAuthOAuth {
		return nil, fmt.Errorf("external MCP server does not use OAuth")
	}
	client, err := externalmcp.NewClient(server.EndpointURL, s.cfg.AllowedHosts, s.cfg.AllowInsecureLocalhost)
	if err != nil {
		return nil, err
	}
	discovery, err := client.DiscoverOAuth(ctx)
	if err != nil {
		s.setServerFailure(ctx, server, model.ExternalMCPStatusError, "oauth_discovery_failed", "OAuth discovery failed")
		return nil, err
	}
	credential, err := s.repo.GetCredential(ctx, workspaceID, serverID)
	if err != nil {
		return nil, err
	}
	if credential == nil {
		credential = &model.ExternalMCPCredential{ServerID: serverID, WorkspaceID: workspaceID}
	}
	credential.AuthorizationEndpoint = discovery.Authorization.AuthorizationEndpoint
	credential.TokenEndpoint = discovery.Authorization.TokenEndpoint
	credential.RegistrationEndpoint = discovery.Authorization.RegistrationEndpoint
	credential.ResourceURL = strings.TrimSpace(discovery.Resource.Resource)
	if credential.ResourceURL == "" {
		credential.ResourceURL = server.EndpointURL
	}
	if credential.ClientID == "" {
		credential.ClientID = strings.TrimSpace(s.cfg.OAuthClientID)
		credential.TokenEndpointAuthMethod = strings.TrimSpace(s.cfg.OAuthClientAuthMethod)
		if credential.TokenEndpointAuthMethod == "" {
			credential.TokenEndpointAuthMethod = "none"
		}
		if s.cfg.OAuthClientSecret != "" {
			credential.EncryptedClientSecret, err = s.encrypt(workspaceID, serverID, "client_secret", s.cfg.OAuthClientSecret)
			if err != nil {
				return nil, err
			}
		}
	}
	if credential.ClientID == "" {
		registration, registerErr := client.RegisterOAuthClient(ctx, discovery.Authorization.RegistrationEndpoint, s.cfg.OAuthRedirectURL)
		if registerErr != nil {
			s.setServerFailure(ctx, server, model.ExternalMCPStatusError, "oauth_registration_failed", "OAuth client registration failed")
			return nil, registerErr
		}
		credential.ClientID = registration.ClientID
		credential.TokenEndpointAuthMethod = registration.TokenEndpointAuthMethod
		if credential.TokenEndpointAuthMethod == "" {
			credential.TokenEndpointAuthMethod = "none"
		}
		credential.EncryptedClientSecret, err = s.encrypt(workspaceID, serverID, "client_secret", registration.ClientSecret)
		if err != nil {
			return nil, err
		}
	}
	if !validExternalMCPClientAuthMethod(credential.TokenEndpointAuthMethod) {
		s.setServerFailure(ctx, server, model.ExternalMCPStatusError, "oauth_client_auth_unsupported", "OAuth client authentication method is unsupported")
		return nil, fmt.Errorf("OAuth token endpoint authentication method is unsupported")
	}
	if credential.TokenEndpointAuthMethod != "none" && credential.EncryptedClientSecret == "" {
		s.setServerFailure(ctx, server, model.ExternalMCPStatusError, "oauth_client_secret_missing", "OAuth client configuration is incomplete")
		return nil, fmt.Errorf("OAuth client secret is required for the token endpoint authentication method")
	}
	if err := s.repo.UpsertCredential(ctx, credential); err != nil {
		return nil, err
	}
	var scopes []string
	_ = json.Unmarshal(server.OAuthScopes, &scopes)
	authorization, err := client.NewOAuthAuthorizationRequest(
		discovery, credential.ClientID, s.cfg.OAuthRedirectURL, scopes,
	)
	if err != nil {
		return nil, err
	}
	stateHash := mcpauth.HashState(authorization.State)
	encryptedVerifier, err := s.encrypt(workspaceID, serverID, "oauth_verifier:"+stateHash, authorization.Verifier)
	if err != nil {
		return nil, err
	}
	if !validExternalMCPReturnPath(returnPath) {
		returnPath = "/workspaces"
	}
	oauthState := &model.ExternalMCPOAuthState{
		ID: uuid.NewString(), StateHash: stateHash, WorkspaceID: workspaceID, ServerID: serverID,
		UserID: userID, EncryptedPKCEVerifier: encryptedVerifier, RequestedScopes: server.OAuthScopes,
		ReturnPath: returnPath, ExpiresAt: s.now().Add(externalMCPOAuthStateTTL),
	}
	if err := s.repo.CreateOAuthState(ctx, oauthState); err != nil {
		return nil, err
	}
	_ = s.repo.UpdateServer(ctx, workspaceID, serverID, map[string]any{
		"status": model.ExternalMCPStatusPendingOAuth, "last_error_code": nil, "last_error_message": nil,
	})
	return &ExternalMCPOAuthStart{AuthorizationURL: authorization.URL}, nil
}

func (s *ExternalMCPService) CompleteOAuth(ctx context.Context, userID, rawState, code, providerError string, authorize ExternalMCPOAuthAuthorizer) (*ExternalMCPOAuthCallbackResult, error) {
	if err := s.requireEnabled(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(rawState) == "" {
		return nil, fmt.Errorf("OAuth state is required")
	}
	state, err := s.repo.ConsumeOAuthState(ctx, mcpauth.HashState(rawState), s.now())
	if err != nil {
		return nil, err
	}
	if state == nil || state.UserID != userID {
		return nil, fmt.Errorf("OAuth state is invalid or expired")
	}
	result := &ExternalMCPOAuthCallbackResult{ServerID: state.ServerID, WorkspaceID: state.WorkspaceID, ReturnPath: state.ReturnPath}
	if authorize == nil {
		return result, fmt.Errorf("OAuth callback authorization is required")
	}
	if err := authorize(ctx, state.WorkspaceID, userID); err != nil {
		return result, fmt.Errorf("OAuth initiator is no longer authorized: %w", err)
	}
	server, err := s.repo.GetServer(ctx, state.WorkspaceID, state.ServerID)
	if err != nil || server == nil {
		return result, ErrExternalMCPNotFound
	}
	if providerError != "" {
		s.setServerFailure(ctx, server, model.ExternalMCPStatusPendingOAuth, "oauth_denied", "Authorization was not completed")
		return result, fmt.Errorf("external MCP authorization was not completed")
	}
	if strings.TrimSpace(code) == "" {
		return result, fmt.Errorf("OAuth authorization code is required")
	}
	verifier, err := s.decrypt(state.WorkspaceID, state.ServerID, "oauth_verifier:"+state.StateHash, state.EncryptedPKCEVerifier)
	if err != nil {
		return result, fmt.Errorf("decrypt PKCE verifier: %w", err)
	}
	credential, err := s.repo.GetCredential(ctx, state.WorkspaceID, state.ServerID)
	if err != nil || credential == nil {
		return result, fmt.Errorf("OAuth client configuration is missing")
	}
	clientSecret, err := s.decrypt(state.WorkspaceID, state.ServerID, "client_secret", credential.EncryptedClientSecret)
	if err != nil {
		return result, err
	}
	client, err := externalmcp.NewClient(server.EndpointURL, s.cfg.AllowedHosts, s.cfg.AllowInsecureLocalhost)
	if err != nil {
		return result, err
	}
	token, err := client.ExchangeCode(ctx, credential.TokenEndpoint, credential.ClientID, clientSecret, credential.TokenEndpointAuthMethod, code, verifier, s.cfg.OAuthRedirectURL, credential.ResourceURL)
	if err != nil {
		s.setServerFailure(ctx, server, model.ExternalMCPStatusPendingOAuth, "oauth_exchange_failed", "OAuth token exchange failed")
		return result, err
	}
	credential.EncryptedAccessToken, err = s.encrypt(state.WorkspaceID, state.ServerID, "access_token", token.AccessToken)
	if err != nil {
		return result, err
	}
	if token.RefreshToken != "" {
		credential.EncryptedRefreshToken, err = s.encrypt(state.WorkspaceID, state.ServerID, "refresh_token", token.RefreshToken)
		if err != nil {
			return result, err
		}
	}
	credential.TokenType = token.TokenType
	credential.AccessTokenExpiresAt = token.ExpiresAt
	if err := s.repo.UpsertCredential(ctx, credential); err != nil {
		return result, err
	}
	if err := s.repo.UpdateServer(ctx, state.WorkspaceID, state.ServerID, map[string]any{
		"authorized_by_user_id": userID, "access_token_expires_at": token.ExpiresAt,
		"status": model.ExternalMCPStatusConnected, "last_error_code": nil, "last_error_message": nil,
		"auth_incident_key": nil, "auth_incident_notified_at": nil,
	}); err != nil {
		return result, err
	}
	if err := s.SyncTools(ctx, state.WorkspaceID, state.ServerID); err != nil {
		return result, err
	}
	return result, nil
}

func validExternalMCPReturnPath(value string) bool {
	if value == "" {
		return false
	}
	u, err := url.Parse(value)
	return err == nil && u.IsAbs() == false && strings.HasPrefix(u.Path, "/") && !strings.HasPrefix(value, "//")
}

func (s *ExternalMCPService) ensureFreshCredential(ctx context.Context, server *model.ExternalMCPServer) (*model.ExternalMCPCredential, error) {
	credential, err := s.repo.GetCredential(ctx, server.WorkspaceID, server.ID)
	if err != nil {
		return nil, err
	}
	if server.AuthType != model.ExternalMCPAuthOAuth || credential == nil || credential.AccessTokenExpiresAt == nil || credential.AccessTokenExpiresAt.After(s.now().Add(externalMCPRefreshSkew)) {
		return credential, nil
	}
	if credential.EncryptedRefreshToken == "" {
		s.markReauthorizationRequired(ctx, server, "refresh_token_missing")
		return nil, ErrExternalMCPReauthorize
	}
	var refreshed *model.ExternalMCPCredential
	refreshErr := s.repo.WithCredentialLock(ctx, server.WorkspaceID, server.ID, func(locked *model.ExternalMCPCredential) error {
		if locked.AccessTokenExpiresAt != nil && locked.AccessTokenExpiresAt.After(s.now().Add(externalMCPRefreshSkew)) {
			copy := *locked
			refreshed = &copy
			return nil
		}
		refreshToken, err := s.decrypt(server.WorkspaceID, server.ID, "refresh_token", locked.EncryptedRefreshToken)
		if err != nil {
			return err
		}
		clientSecret, err := s.decrypt(server.WorkspaceID, server.ID, "client_secret", locked.EncryptedClientSecret)
		if err != nil {
			return err
		}
		client, err := externalmcp.NewClient(server.EndpointURL, s.cfg.AllowedHosts, s.cfg.AllowInsecureLocalhost)
		if err != nil {
			return err
		}
		resourceURL := locked.ResourceURL
		if resourceURL == "" {
			resourceURL = server.EndpointURL
		}
		token, err := client.RefreshToken(ctx, locked.TokenEndpoint, locked.ClientID, clientSecret, locked.TokenEndpointAuthMethod, refreshToken, resourceURL)
		if err != nil {
			return err
		}
		locked.EncryptedAccessToken, err = s.encrypt(server.WorkspaceID, server.ID, "access_token", token.AccessToken)
		if err != nil {
			return err
		}
		if token.RefreshToken != "" {
			locked.EncryptedRefreshToken, err = s.encrypt(server.WorkspaceID, server.ID, "refresh_token", token.RefreshToken)
			if err != nil {
				return err
			}
		}
		locked.TokenType = token.TokenType
		locked.AccessTokenExpiresAt = token.ExpiresAt
		copy := *locked
		refreshed = &copy
		return nil
	})
	if refreshErr != nil {
		if externalmcp.IsStatus(refreshErr, http.StatusBadRequest) || externalmcp.IsStatus(refreshErr, http.StatusUnauthorized) {
			s.markReauthorizationRequired(ctx, server, "refresh_rejected")
			return nil, ErrExternalMCPReauthorize
		}
		s.setServerFailure(ctx, server, model.ExternalMCPStatusError, "refresh_failed", "OAuth refresh could not reach the provider")
		return nil, refreshErr
	}
	_ = s.repo.UpdateServer(ctx, server.WorkspaceID, server.ID, map[string]any{
		"status": model.ExternalMCPStatusConnected, "access_token_expires_at": refreshed.AccessTokenExpiresAt,
		"last_error_code": nil, "last_error_message": nil, "auth_incident_key": nil, "auth_incident_notified_at": nil,
	})
	return refreshed, nil
}

func (s *ExternalMCPService) credentialHeaders(server *model.ExternalMCPServer, credential *model.ExternalMCPCredential) (http.Header, error) {
	headers := make(http.Header)
	if credential == nil {
		return headers, nil
	}
	switch server.AuthType {
	case model.ExternalMCPAuthOAuth, model.ExternalMCPAuthBearerToken:
		token, err := s.decrypt(server.WorkspaceID, server.ID, "access_token", credential.EncryptedAccessToken)
		if err != nil {
			return nil, err
		}
		if token != "" {
			headers.Set("Authorization", "Bearer "+token)
		}
	case model.ExternalMCPAuthHeaders:
		encoded, err := s.decrypt(server.WorkspaceID, server.ID, "headers", credential.EncryptedHeaders)
		if err != nil {
			return nil, err
		}
		var values map[string]string
		if err := json.Unmarshal([]byte(encoded), &values); err != nil {
			return nil, fmt.Errorf("decode external MCP credential headers")
		}
		for key, value := range values {
			headers.Set(key, value)
		}
	}
	return headers, nil
}
