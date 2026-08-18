package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

type publicShareServicer interface {
	Create(ctx context.Context, workspaceID, actorID, resourceType, resourceID string) (*model.PublicShareLink, error)
	GetLink(ctx context.Context, workspaceID, actorID, resourceType, resourceID string) (*model.PublicShareLink, error)
	Revoke(ctx context.Context, workspaceID, actorID, resourceType, resourceID string) error
	GetPublic(ctx context.Context, token string) (*model.PublicSharedResource, error)
}

// GetLink returns the active link for authenticated menu state.
func (h *PublicShareHandler) GetLink(w http.ResponseWriter, r *http.Request) {
	link, err := h.service.GetLink(r.Context(), getWorkspaceID(r), middleware.GetUserID(r.Context()), chi.URLParam(r, "resourceType"), chi.URLParam(r, "resourceID"))
	if err != nil {
		writePublicShareError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, link)
}

// PublicShareHandler serves authenticated share management and anonymous reads.
type PublicShareHandler struct {
	service publicShareServicer
}

// NewPublicShareHandler creates a PublicShareHandler.
func NewPublicShareHandler(publicShareService publicShareServicer) *PublicShareHandler {
	return &PublicShareHandler{service: publicShareService}
}

// Create creates or returns the active public link for a resource.
func (h *PublicShareHandler) Create(w http.ResponseWriter, r *http.Request) {
	link, err := h.service.Create(r.Context(), getWorkspaceID(r), middleware.GetUserID(r.Context()), chi.URLParam(r, "resourceType"), chi.URLParam(r, "resourceID"))
	if err != nil {
		writePublicShareError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, link)
}

// Revoke disables the active public link for a resource.
func (h *PublicShareHandler) Revoke(w http.ResponseWriter, r *http.Request) {
	err := h.service.Revoke(r.Context(), getWorkspaceID(r), middleware.GetUserID(r.Context()), chi.URLParam(r, "resourceType"), chi.URLParam(r, "resourceID"))
	if err != nil {
		writePublicShareError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GetPublic returns a live public resource without authentication.
func (h *PublicShareHandler) GetPublic(w http.ResponseWriter, r *http.Request) {
	resource, err := h.service.GetPublic(r.Context(), chi.URLParam(r, "token"))
	if err != nil {
		writePublicShareError(w, err)
		return
	}
	w.Header().Set("X-Robots-Tag", "noindex, nofollow, noarchive")
	w.Header().Set("Cache-Control", "private, no-store")
	writeJSON(w, http.StatusOK, resource)
}

func writePublicShareError(w http.ResponseWriter, err error) {
	if errors.Is(err, service.ErrPublicShareNotFound) {
		writeError(w, http.StatusNotFound, "shared resource not found")
		return
	}
	writeError(w, http.StatusInternalServerError, "failed to manage public link")
}
