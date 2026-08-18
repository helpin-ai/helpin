package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/service"
)

const externalMCPRequestBodyLimit = 128 << 10

// ExternalMCPHandler manages workspace-owned outbound MCP installations.
type ExternalMCPHandler struct {
	service    *service.ExternalMCPService
	agents     *service.AgentService
	authz      *authorization.AuthzService
	appBaseURL string
}

func NewExternalMCPHandler(externalMCPService *service.ExternalMCPService, agents *service.AgentService, authz *authorization.AuthzService, appBaseURL string) *ExternalMCPHandler {
	return &ExternalMCPHandler{service: externalMCPService, agents: agents, authz: authz, appBaseURL: strings.TrimRight(strings.TrimSpace(appBaseURL), "/")}
}

func (h *ExternalMCPHandler) Providers(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"providers": h.service.Providers(), "enabled": h.service.Enabled()})
}

func (h *ExternalMCPHandler) ListServers(w http.ResponseWriter, r *http.Request) {
	servers, err := h.service.ListServers(r.Context(), middleware.GetWorkspaceID(r.Context()))
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"servers": servers})
}

func (h *ExternalMCPHandler) CreateServer(w http.ResponseWriter, r *http.Request) {
	var req service.CreateExternalMCPServerRequest
	if err := decodeExternalMCPJSON(w, r, &req); err != nil {
		writeErrorCode(w, http.StatusBadRequest, err.Error(), "invalid_request")
		return
	}
	server, err := h.service.CreateServer(r.Context(), middleware.GetWorkspaceID(r.Context()), middleware.GetUserID(r.Context()), req)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, server)
}

func (h *ExternalMCPHandler) UpdateServer(w http.ResponseWriter, r *http.Request) {
	var req service.UpdateExternalMCPServerRequest
	if err := decodeExternalMCPJSON(w, r, &req); err != nil {
		writeErrorCode(w, http.StatusBadRequest, err.Error(), "invalid_request")
		return
	}
	server, err := h.service.UpdateServer(r.Context(), middleware.GetWorkspaceID(r.Context()), chi.URLParam(r, "serverID"), req)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, server)
}

func (h *ExternalMCPHandler) DeleteServer(w http.ResponseWriter, r *http.Request) {
	if err := h.service.DeleteServer(r.Context(), middleware.GetWorkspaceID(r.Context()), chi.URLParam(r, "serverID")); err != nil {
		h.writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *ExternalMCPHandler) StartOAuth(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ReturnPath string `json:"return_path,omitempty"`
	}
	if err := decodeExternalMCPJSON(w, r, &req); err != nil {
		writeErrorCode(w, http.StatusBadRequest, err.Error(), "invalid_request")
		return
	}
	result, err := h.service.StartOAuth(
		r.Context(), middleware.GetWorkspaceID(r.Context()), chi.URLParam(r, "serverID"),
		middleware.GetUserID(r.Context()), req.ReturnPath,
	)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// OAuthCallback is authenticated but deliberately does not trust a workspace
// header; the single-use state record supplies and binds the workspace.
func (h *ExternalMCPHandler) OAuthCallback(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.CompleteOAuth(
		r.Context(), middleware.GetUserID(r.Context()), r.URL.Query().Get("state"),
		r.URL.Query().Get("code"), r.URL.Query().Get("error"), h.authorizeOAuthCallback,
	)
	returnPath := "/workspaces"
	if result != nil && strings.TrimSpace(result.ReturnPath) != "" {
		returnPath = result.ReturnPath
	}
	redirectURL, parseErr := url.Parse(h.appBaseURL + returnPath)
	if parseErr != nil {
		writeError(w, http.StatusBadRequest, "invalid OAuth return path")
		return
	}
	query := redirectURL.Query()
	if err != nil {
		query.Set("external_mcp_oauth", "failed")
	} else {
		if h.agents != nil {
			if resumeErr := h.agents.ResumeRunsAfterExternalMCPAuth(r.Context(), result.WorkspaceID, result.ServerID, middleware.GetUserID(r.Context())); resumeErr != nil {
				query.Set("external_mcp_resume", "failed")
			}
		}
		query.Set("external_mcp_oauth", "connected")
		query.Set("external_mcp_server_id", result.ServerID)
	}
	redirectURL.RawQuery = query.Encode()
	http.Redirect(w, r, redirectURL.String(), http.StatusFound)
}

func (h *ExternalMCPHandler) authorizeOAuthCallback(ctx context.Context, workspaceID, userID string) error {
	if h.authz == nil {
		return errors.New("authorization service is unavailable")
	}
	actor, err := h.authz.ResolveActor(ctx, workspaceID, userID)
	if err != nil {
		return err
	}
	if !h.authz.Can(actor, authorization.PermSettingsManage) {
		return errors.New("settings management permission is required")
	}
	return nil
}

func (h *ExternalMCPHandler) RefreshTools(w http.ResponseWriter, r *http.Request) {
	workspaceID := middleware.GetWorkspaceID(r.Context())
	serverID := chi.URLParam(r, "serverID")
	if err := h.service.SyncTools(r.Context(), workspaceID, serverID); err != nil {
		h.writeServiceError(w, err)
		return
	}
	servers, err := h.service.ListServers(r.Context(), workspaceID)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	for i := range servers {
		if servers[i].ID == serverID {
			writeJSON(w, http.StatusOK, servers[i])
			return
		}
	}
	writeErrorCode(w, http.StatusNotFound, "external MCP server not found", "not_found")
}

func (h *ExternalMCPHandler) UpdateTools(w http.ResponseWriter, r *http.Request) {
	var req service.UpdateExternalMCPToolsRequest
	if err := decodeExternalMCPJSON(w, r, &req); err != nil {
		writeErrorCode(w, http.StatusBadRequest, err.Error(), "invalid_request")
		return
	}
	server, err := h.service.UpdateToolPolicies(r.Context(), middleware.GetWorkspaceID(r.Context()), chi.URLParam(r, "serverID"), req)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, server)
}

func (h *ExternalMCPHandler) writeServiceError(w http.ResponseWriter, err error) {
	status := service.ExternalMCPErrorStatus(err)
	code := "external_mcp_error"
	switch {
	case errors.Is(err, service.ErrExternalMCPNotFound):
		code = "not_found"
	case errors.Is(err, service.ErrExternalMCPDisabled):
		code = "external_mcp_disabled"
	case errors.Is(err, service.ErrExternalMCPReauthorize):
		code = "reauthorization_required"
	}
	writeErrorCode(w, status, err.Error(), code)
}

func decodeExternalMCPJSON(w http.ResponseWriter, r *http.Request, target any) error {
	r.Body = http.MaxBytesReader(w, r.Body, externalMCPRequestBodyLimit)
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return errors.New("invalid request body")
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return errors.New("request body must contain one JSON object")
	}
	return nil
}
