package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/mcpserver"
	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/ratelimit"
	"github.com/helpin-ai/helpin/server/internal/service"
)

// MCPHandler exposes public MCP protocol, OAuth, and authenticated settings endpoints.
type MCPHandler struct {
	service  *service.MCPService
	protocol http.Handler
}

// NewMCPHandler creates the public MCP HTTP handler.
func NewMCPHandler(mcpService *service.MCPService, limiter *ratelimit.Limiter) *MCPHandler {
	return &MCPHandler{service: mcpService, protocol: mcpserver.NewHandler(mcpService, limiter)}
}

// Protocol serves Streamable HTTP MCP requests.
func (h *MCPHandler) Protocol(w http.ResponseWriter, r *http.Request) {
	h.protocol.ServeHTTP(w, r)
}

// AuthorizationServerMetadata serves OAuth authorization-server discovery.
func (h *MCPHandler) AuthorizationServerMetadata(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, h.service.OAuthAuthorizationServerMetadata())
}

// ProtectedResourceMetadata serves OAuth protected-resource discovery.
func (h *MCPHandler) ProtectedResourceMetadata(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, h.service.OAuthProtectedResourceMetadata())
}

// RegisterClient handles OAuth dynamic client registration.
func (h *MCPHandler) RegisterClient(w http.ResponseWriter, r *http.Request) {
	var req service.MCPClientRegistrationRequest
	if err := decodeJSON(r, &req); err != nil {
		writeOAuthError(w, http.StatusBadRequest, "invalid_client_metadata")
		return
	}
	result, err := h.service.RegisterClient(r.Context(), req)
	if err != nil {
		if errors.Is(err, service.ErrMCPDisabled) {
			writeOAuthError(w, http.StatusServiceUnavailable, "temporarily_unavailable")
			return
		}
		writeOAuthError(w, http.StatusBadRequest, "invalid_client_metadata")
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

// AuthorizeRedirect sends the browser to Helpin's authenticated consent UI.
func (h *MCPHandler) AuthorizeRedirect(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, h.service.AuthorizationFrontendURL(r.URL.RawQuery), http.StatusFound)
}

// AuthorizationRequest returns the backend-authoritative consent model.
func (h *MCPHandler) AuthorizationRequest(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.GetAuthorizationRequest(
		r.Context(), middleware.GetUserID(r.Context()), authorizationQuery(r),
	)
	if err != nil {
		writeMCPError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// Authorize records consent and returns the exact OAuth client redirect.
func (h *MCPHandler) Authorize(w http.ResponseWriter, r *http.Request) {
	var req service.MCPAuthorizeDecision
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid authorization decision")
		return
	}
	result, err := h.service.Authorize(r.Context(), middleware.GetUserID(r.Context()), req)
	if err != nil {
		writeMCPError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// Token exchanges an authorization code or rotating refresh token.
func (h *MCPHandler) Token(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeOAuthError(w, http.StatusBadRequest, "invalid_request")
		return
	}
	var (
		result *service.MCPTokenResponse
		err    error
	)
	switch r.Form.Get("grant_type") {
	case "authorization_code":
		result, err = h.service.ExchangeAuthorizationCode(
			r.Context(), r.Form.Get("code"), r.Form.Get("client_id"),
			r.Form.Get("redirect_uri"), r.Form.Get("code_verifier"),
		)
	case "refresh_token":
		result, err = h.service.RefreshAccessToken(
			r.Context(), r.Form.Get("refresh_token"), r.Form.Get("client_id"),
		)
	default:
		writeOAuthError(w, http.StatusBadRequest, "unsupported_grant_type")
		return
	}
	if err != nil {
		if errors.Is(err, service.ErrMCPDisabled) {
			writeOAuthError(w, http.StatusServiceUnavailable, "temporarily_unavailable")
			return
		}
		writeOAuthError(w, http.StatusBadRequest, "invalid_grant")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	writeJSON(w, http.StatusOK, result)
}

// RevokeToken handles OAuth token revocation without disclosing token validity.
func (h *MCPHandler) RevokeToken(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err == nil {
		_ = h.service.RevokeOAuthToken(r.Context(), r.Form.Get("token"))
	}
	w.WriteHeader(http.StatusOK)
}

// Dashboard returns setup, policy, connection, and service-account state.
func (h *MCPHandler) Dashboard(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.GetDashboard(r.Context(), getWorkspaceID(r), middleware.GetUserID(r.Context()))
	if err != nil {
		writeMCPError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// UpdatePolicy updates workspace-level MCP access controls.
func (h *MCPHandler) UpdatePolicy(w http.ResponseWriter, r *http.Request) {
	var req model.UpdateMCPWorkspacePolicyRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid policy")
		return
	}
	result, err := h.service.UpdatePolicy(r.Context(), getWorkspaceID(r), middleware.GetUserID(r.Context()), req)
	if err != nil {
		writeMCPError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// RevokeConnection revokes a personal or manager-authorized connection.
func (h *MCPHandler) RevokeConnection(w http.ResponseWriter, r *http.Request) {
	err := h.service.RevokeConnection(
		r.Context(), getWorkspaceID(r), middleware.GetUserID(r.Context()), chi.URLParam(r, "connectionID"),
	)
	if err != nil {
		writeMCPError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// RevokeWorkspaceAccess revokes every MCP user connection and service credential in a workspace.
func (h *MCPHandler) RevokeWorkspaceAccess(w http.ResponseWriter, r *http.Request) {
	revoked, err := h.service.RevokeWorkspaceAccess(r.Context(), getWorkspaceID(r), middleware.GetUserID(r.Context()))
	if err != nil {
		writeMCPError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int64{"revoked": revoked})
}

// CreateServicePrincipal creates a restricted service identity and one-time secret.
func (h *MCPHandler) CreateServicePrincipal(w http.ResponseWriter, r *http.Request) {
	var req model.CreateMCPServicePrincipalRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid service account")
		return
	}
	principal, secret, err := h.service.CreateServicePrincipal(
		r.Context(), getWorkspaceID(r), middleware.GetUserID(r.Context()), req,
	)
	if err != nil {
		writeMCPError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusCreated, map[string]any{"principal": principal, "secret": secret})
}

// RotateServiceToken creates a new one-time service secret.
func (h *MCPHandler) RotateServiceToken(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.RotateServiceToken(
		r.Context(), getWorkspaceID(r), middleware.GetUserID(r.Context()), chi.URLParam(r, "principalID"),
	)
	if err != nil {
		writeMCPError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusCreated, result)
}

// ListServiceTokens lists service-token metadata without secrets.
func (h *MCPHandler) ListServiceTokens(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.ListServiceTokens(
		r.Context(), getWorkspaceID(r), middleware.GetUserID(r.Context()), chi.URLParam(r, "principalID"),
	)
	if err != nil {
		writeMCPError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// RevokeServiceToken revokes one service secret.
func (h *MCPHandler) RevokeServiceToken(w http.ResponseWriter, r *http.Request) {
	err := h.service.RevokeServiceToken(
		r.Context(), getWorkspaceID(r), middleware.GetUserID(r.Context()),
		chi.URLParam(r, "principalID"), chi.URLParam(r, "tokenID"),
	)
	if err != nil {
		writeMCPError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// RevokeServicePrincipal revokes a service identity and all secrets.
func (h *MCPHandler) RevokeServicePrincipal(w http.ResponseWriter, r *http.Request) {
	err := h.service.RevokeServicePrincipal(
		r.Context(), getWorkspaceID(r), middleware.GetUserID(r.Context()), chi.URLParam(r, "principalID"),
	)
	if err != nil {
		writeMCPError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Activity lists sanitized recent MCP audit events.
func (h *MCPHandler) Activity(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	result, err := h.service.ListActivity(r.Context(), getWorkspaceID(r), middleware.GetUserID(r.Context()), limit)
	if err != nil {
		writeMCPError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func authorizationQuery(r *http.Request) service.MCPAuthorizationQuery {
	query := r.URL.Query()
	return service.MCPAuthorizationQuery{
		ClientID: query.Get("client_id"), RedirectURI: query.Get("redirect_uri"),
		ResponseType: query.Get("response_type"), Scope: query.Get("scope"), State: query.Get("state"),
		CodeChallenge: query.Get("code_challenge"), CodeChallengeMethod: query.Get("code_challenge_method"),
	}
}

func writeMCPError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrMCPUnauthorized):
		writeError(w, http.StatusUnauthorized, "MCP authorization is invalid")
	case errors.Is(err, service.ErrMCPForbidden), errors.Is(err, service.ErrMCPDisabled):
		writeError(w, http.StatusForbidden, err.Error())
	case errors.Is(err, service.ErrMCPNotFound), errors.Is(err, gorm.ErrRecordNotFound):
		writeError(w, http.StatusNotFound, "MCP record not found")
	case errors.Is(err, service.ErrMCPConflict):
		writeError(w, http.StatusConflict, "MCP request conflicts with existing state")
	default:
		writeError(w, http.StatusBadRequest, err.Error())
	}
}

func writeOAuthError(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": code})
}
