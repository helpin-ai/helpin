package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

// CRMWritingProfileHandler handles CRM writing profile HTTP endpoints.
type CRMWritingProfileHandler struct {
	profileService *service.CRMWritingProfileService
}

// NewCRMWritingProfileHandler creates a new CRMWritingProfileHandler.
func NewCRMWritingProfileHandler(profileService *service.CRMWritingProfileService) *CRMWritingProfileHandler {
	return &CRMWritingProfileHandler{profileService: profileService}
}

// List handles GET /api/crm/writing-profiles.
func (h *CRMWritingProfileHandler) List(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	profiles, err := h.profileService.List(r.Context(), workspaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if profiles == nil {
		profiles = []model.CRMWritingProfile{}
	}
	writeJSON(w, http.StatusOK, profiles)
}

// GetByMember handles GET /api/crm/writing-profiles/member/{memberId}.
func (h *CRMWritingProfileHandler) GetByMember(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	memberID := chi.URLParam(r, "memberId")
	profile, err := h.profileService.GetByMemberID(r.Context(), workspaceID, memberID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, profile)
}

// Create handles POST /api/crm/writing-profiles.
func (h *CRMWritingProfileHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req model.CreateCRMWritingProfileRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.WorkspaceID == "" {
		req.WorkspaceID = getWorkspaceID(r)
	}
	profile, err := h.profileService.Create(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, profile)
}

// Update handles PUT /api/crm/writing-profiles/{id}.
func (h *CRMWritingProfileHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req model.UpdateCRMWritingProfileRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	profile, err := h.profileService.Update(r.Context(), id, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, profile)
}

// Delete handles DELETE /api/crm/writing-profiles/{id}.
func (h *CRMWritingProfileHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.profileService.Delete(r.Context(), id); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "writing profile deleted"})
}
