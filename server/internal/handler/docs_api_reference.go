package handler

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

func writeDocsAPIReferenceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrDocsAPIReferenceNotFound),
		errors.Is(err, service.ErrDocsSpaceNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, service.ErrDocsCrossWorkspace):
		writeError(w, http.StatusForbidden, err.Error())
	case errors.Is(err, service.ErrDocsAPIReferenceExternalSpace),
		errors.Is(err, service.ErrDocsAPIReferenceInvalidSpec),
		errors.Is(err, service.ErrDocsAPIReferenceInvalidSource):
		writeError(w, http.StatusBadRequest, err.Error())
	default:
		writeDocsError(w, err)
	}
}

func (h *DocsHandler) requireAPIReferenceService(w http.ResponseWriter) bool {
	if h.apiReferenceSvc != nil {
		return true
	}
	writeError(w, http.StatusServiceUnavailable, "API reference service is not configured")
	return false
}

// ListAPIReferences handles GET /docs/spaces/{spaceId}/api-references.
func (h *DocsHandler) ListAPIReferences(w http.ResponseWriter, r *http.Request) {
	if !h.requireAPIReferenceService(w) {
		return
	}
	references, err := h.apiReferenceSvc.List(r.Context(), getWorkspaceID(r), chi.URLParam(r, "spaceId"))
	if err != nil {
		writeDocsAPIReferenceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, references)
}

// CreateAPIReference handles POST /docs/spaces/{spaceId}/api-references.
func (h *DocsHandler) CreateAPIReference(w http.ResponseWriter, r *http.Request) {
	if !h.requireAPIReferenceService(w) {
		return
	}
	var req model.CreateDocsAPIReferenceRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	reference, err := h.apiReferenceSvc.Create(
		r.Context(),
		getWorkspaceID(r),
		middleware.GetUserID(r.Context()),
		chi.URLParam(r, "spaceId"),
		req,
	)
	if err != nil {
		writeDocsAPIReferenceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, reference)
}

// GetAPIReference handles GET /docs/api-references/{referenceId}.
func (h *DocsHandler) GetAPIReference(w http.ResponseWriter, r *http.Request) {
	if !h.requireAPIReferenceService(w) {
		return
	}
	reference, err := h.apiReferenceSvc.Get(r.Context(), getWorkspaceID(r), chi.URLParam(r, "referenceId"))
	if err != nil {
		writeDocsAPIReferenceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, reference)
}

// UpdateAPIReference handles PATCH /docs/api-references/{referenceId}.
func (h *DocsHandler) UpdateAPIReference(w http.ResponseWriter, r *http.Request) {
	if !h.requireAPIReferenceService(w) {
		return
	}
	var req model.UpdateDocsAPIReferenceRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	reference, err := h.apiReferenceSvc.Update(
		r.Context(),
		getWorkspaceID(r),
		middleware.GetUserID(r.Context()),
		chi.URLParam(r, "referenceId"),
		req,
	)
	if err != nil {
		writeDocsAPIReferenceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, reference)
}

// SyncAPIReference handles POST /docs/api-references/{referenceId}/sync.
func (h *DocsHandler) SyncAPIReference(w http.ResponseWriter, r *http.Request) {
	if !h.requireAPIReferenceService(w) {
		return
	}
	reference, err := h.apiReferenceSvc.Sync(
		r.Context(),
		getWorkspaceID(r),
		middleware.GetUserID(r.Context()),
		chi.URLParam(r, "referenceId"),
	)
	if err != nil {
		writeDocsAPIReferenceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, reference)
}

// PublishAPIReference handles POST /docs/api-references/{referenceId}/publish.
func (h *DocsHandler) PublishAPIReference(w http.ResponseWriter, r *http.Request) {
	if !h.requireAPIReferenceService(w) {
		return
	}
	reference, err := h.apiReferenceSvc.Publish(r.Context(), getWorkspaceID(r), chi.URLParam(r, "referenceId"))
	if err != nil {
		writeDocsAPIReferenceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, reference)
}

// UnpublishAPIReference handles POST /docs/api-references/{referenceId}/unpublish.
func (h *DocsHandler) UnpublishAPIReference(w http.ResponseWriter, r *http.Request) {
	if !h.requireAPIReferenceService(w) {
		return
	}
	reference, err := h.apiReferenceSvc.Unpublish(r.Context(), getWorkspaceID(r), chi.URLParam(r, "referenceId"))
	if err != nil {
		writeDocsAPIReferenceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, reference)
}

// DeleteAPIReference handles DELETE /docs/api-references/{referenceId}.
func (h *DocsHandler) DeleteAPIReference(w http.ResponseWriter, r *http.Request) {
	if !h.requireAPIReferenceService(w) {
		return
	}
	if err := h.apiReferenceSvc.Delete(r.Context(), getWorkspaceID(r), chi.URLParam(r, "referenceId")); err != nil {
		writeDocsAPIReferenceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// PublicListAPIReferences returns published API references for a public space.
func (h *DocsHandler) PublicListAPIReferences(w http.ResponseWriter, r *http.Request) {
	if !h.requireAPIReferenceService(w) {
		return
	}
	cfg := h.resolveSubdomain(w, r)
	if cfg == nil {
		return
	}
	if chi.URLParam(r, "locale") == "" && publicMultilingualEnabled(cfg) {
		http.Redirect(w, r, h.publicLocaleRedirectTarget(r, cfg), http.StatusFound)
		return
	}
	locale, ok := h.resolveRequestedPublicLocale(w, r, cfg)
	if !ok {
		return
	}
	spaceID, ok := h.resolvePublicAPIReferenceSpace(w, r, cfg, locale)
	if !ok {
		return
	}
	references, err := h.apiReferenceSvc.ListPublic(
		r.Context(),
		cfg.WorkspaceID,
		spaceID,
	)
	if err != nil {
		writeDocsAPIReferenceError(w, err)
		return
	}
	setHelpcenterCacheHeader(w, helpcenterCachePublicRead)
	writeJSONWithETag(w, r, http.StatusOK, references)
}

// PublicGetAPIReference returns one published OpenAPI snapshot.
func (h *DocsHandler) PublicGetAPIReference(w http.ResponseWriter, r *http.Request) {
	if !h.requireAPIReferenceService(w) {
		return
	}
	cfg := h.resolveSubdomain(w, r)
	if cfg == nil {
		return
	}
	if chi.URLParam(r, "locale") == "" && publicMultilingualEnabled(cfg) {
		http.Redirect(w, r, h.publicLocaleRedirectTarget(r, cfg), http.StatusFound)
		return
	}
	locale, ok := h.resolveRequestedPublicLocale(w, r, cfg)
	if !ok {
		return
	}
	spaceID, ok := h.resolvePublicAPIReferenceSpace(w, r, cfg, locale)
	if !ok {
		return
	}
	reference, err := h.apiReferenceSvc.GetPublic(
		r.Context(),
		cfg.WorkspaceID,
		spaceID,
		chi.URLParam(r, "referenceSlug"),
	)
	if err != nil {
		if errors.Is(err, service.ErrDocsAPIReferenceNotFound) ||
			errors.Is(err, service.ErrDocsSpaceNotFound) {
			writeError(w, http.StatusNotFound, "API reference not found")
			return
		}
		writeDocsAPIReferenceError(w, err)
		return
	}
	setHelpcenterCacheHeader(w, helpcenterCachePublicRead)
	writeJSONWithETag(w, r, http.StatusOK, reference)
}

func (h *DocsHandler) resolvePublicAPIReferenceSpace(
	w http.ResponseWriter,
	r *http.Request,
	cfg *model.DocsHelpcenterConfig,
	locale string,
) (string, bool) {
	spaces, err := h.helpcenterSvc.ListPublicSpaces(r.Context(), cfg.WorkspaceID, locale)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to resolve help center space")
		return "", false
	}
	requestedSlug := strings.TrimSpace(chi.URLParam(r, "spaceSlug"))
	for _, space := range spaces {
		if space.Slug == requestedSlug {
			return space.ID, true
		}
	}
	writeError(w, http.StatusNotFound, "API reference not found")
	return "", false
}
