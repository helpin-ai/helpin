package handler

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

type gitHubAppConfigurer interface {
	Status(ctx context.Context) (*model.GitHubAppStatusResponse, error)
	CreateManifest(ctx context.Context, workspaceID, actorID string, req model.GitHubAppManifestRequest) (*model.GitHubAppManifestResponse, error)
	CompleteManifest(ctx context.Context, code, state string) string
}

// GitHubAppHandler serves the instance GitHub App status and manifest flow.
type GitHubAppHandler struct {
	apps gitHubAppConfigurer
}

// NewGitHubAppHandler returns a handler backed by apps.
func NewGitHubAppHandler(apps gitHubAppConfigurer) *GitHubAppHandler {
	return &GitHubAppHandler{apps: apps}
}

// Status handles GET /api/workspaces/{id}/github/app-status.
func (h *GitHubAppHandler) Status(w http.ResponseWriter, r *http.Request) {
	status, err := h.apps.Status(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "github app status failed", "workspace_id", chi.URLParam(r, "id"), "error", err)
		writeError(w, http.StatusInternalServerError, "GitHub App credentials could not be read")
		return
	}
	writeJSON(w, http.StatusOK, status)
}

// CreateManifest handles POST /api/workspaces/{id}/github/app-manifest.
func (h *GitHubAppHandler) CreateManifest(w http.ResponseWriter, r *http.Request) {
	var req model.GitHubAppManifestRequest
	if r.Body != nil && r.ContentLength != 0 {
		if err := decodeJSON(r, &req); err != nil && !errors.Is(err, io.EOF) {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
	}
	workspaceID := chi.URLParam(r, "id")
	resp, err := h.apps.CreateManifest(r.Context(), workspaceID, middleware.GetUserID(r.Context()), req)
	switch {
	case err == nil:
		writeJSON(w, http.StatusOK, resp)
	case errors.Is(err, service.ErrGitHubAppManifestUnavailable):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, service.ErrGitHubAppAlreadyConfigured):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, service.ErrGitHubAppManifestInvalid):
		writeError(w, http.StatusBadRequest, err.Error())
	default:
		slog.ErrorContext(r.Context(), "github app manifest failed", "workspace_id", workspaceID, "error", err)
		writeError(w, http.StatusInternalServerError, "could not prepare the GitHub App manifest")
	}
}

// ManifestCallback handles GET /api/github/app-manifest/callback, where GitHub
// redirects the browser after the App is created.
func (h *GitHubAppHandler) ManifestCallback(w http.ResponseWriter, r *http.Request) {
	redirectURL := h.apps.CompleteManifest(r.Context(), r.URL.Query().Get("code"), r.URL.Query().Get("state"))
	http.Redirect(w, r, redirectURL, http.StatusFound)
}
