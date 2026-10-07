package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/service"
)

type AIProfileHandler struct{ service *service.AIProfileService }

func NewAIProfileHandler(s *service.AIProfileService) *AIProfileHandler {
	return &AIProfileHandler{service: s}
}

func (h *AIProfileHandler) failure(w http.ResponseWriter, err error) {
	if errors.Is(err, repository.ErrAIProfileInUse) {
		writeError(w, http.StatusConflict, "This model configuration is used by agents. Change their model before deleting it.")
		return
	}
	if errors.Is(err, repository.ErrAIProfileChanged) {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	if errors.Is(err, service.ErrAIConnection) {
		writeError(w, http.StatusForbidden, "AI profile or connection is unavailable for this workspace member")
		return
	}
	writeError(w, http.StatusBadRequest, "Unable to save AI settings. Check the connection, model, and controls.")
}

func (h *AIProfileHandler) List(w http.ResponseWriter, r *http.Request) {
	profiles, err := h.service.List(r.Context(), middleware.GetWorkspaceID(r.Context()), middleware.GetUserID(r.Context()))
	if err != nil {
		h.failure(w, err)
		return
	}
	writeJSON(w, http.StatusOK, profiles)
}

func (h *AIProfileHandler) Save(w http.ResponseWriter, r *http.Request) {
	var req model.SaveAIProfileRequest
	if decodeAIConnection(w, r, &req) != nil {
		writeError(w, http.StatusBadRequest, "invalid AI profile request")
		return
	}
	id := chi.URLParam(r, "profileID")
	p, err := h.service.Save(r.Context(), middleware.GetWorkspaceID(r.Context()), middleware.GetUserID(r.Context()), id, req)
	if err != nil {
		h.failure(w, err)
		return
	}
	status := http.StatusOK
	if id == "" {
		status = http.StatusCreated
	}
	writeJSON(w, status, p)
}

func (h *AIProfileHandler) Delete(w http.ResponseWriter, r *http.Request) {
	revision, err := strconv.ParseInt(r.URL.Query().Get("revision"), 10, 64)
	if err != nil || revision < 1 {
		writeError(w, http.StatusBadRequest, "profile revision is required")
		return
	}
	if err := h.service.Delete(r.Context(), middleware.GetWorkspaceID(r.Context()), middleware.GetUserID(r.Context()), chi.URLParam(r, "profileID"), revision); err != nil {
		h.failure(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *AIProfileHandler) Settings(w http.ResponseWriter, r *http.Request) {
	settings, err := h.service.Settings(r.Context(), middleware.GetWorkspaceID(r.Context()), middleware.GetUserID(r.Context()))
	if err != nil {
		h.failure(w, err)
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

func (h *AIProfileHandler) SetDefault(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ProfileID *string `json:"default_profile_id"`
	}
	if decodeAIConnection(w, r, &req) != nil {
		writeError(w, http.StatusBadRequest, "invalid AI settings request")
		return
	}
	id := ""
	if req.ProfileID != nil {
		id = *req.ProfileID
	}
	if err := h.service.SetDefault(r.Context(), middleware.GetWorkspaceID(r.Context()), middleware.GetUserID(r.Context()), id); err != nil {
		h.failure(w, err)
		return
	}
	h.Settings(w, r)
}

func (h *AIProfileHandler) SetVisibility(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Revision int64 `json:"revision"`
		Hidden   *bool `json:"hidden_from_ask_agent"`
	}
	if decodeAIConnection(w, r, &req) != nil || req.Hidden == nil || req.Revision < 1 {
		writeError(w, 400, "model visibility and revision are required")
		return
	}
	p, err := h.service.SetVisibility(r.Context(), middleware.GetWorkspaceID(r.Context()), middleware.GetUserID(r.Context()), chi.URLParam(r, "profileID"), req.Revision, *req.Hidden)
	if err != nil {
		h.failure(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (h *AIProfileHandler) EnableModel(w http.ResponseWriter, r *http.Request) {
	var req model.EnableAIModelRequest
	if decodeAIConnection(w, r, &req) != nil {
		writeError(w, 400, "invalid model selection")
		return
	}
	p, err := h.service.EnableModel(r.Context(), middleware.GetWorkspaceID(r.Context()), middleware.GetUserID(r.Context()), req)
	if err != nil {
		h.failure(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (h *AIProfileHandler) SetPersonalDefault(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ProfileID *string `json:"default_profile_id"`
	}
	if decodeAIConnection(w, r, &req) != nil {
		writeError(w, http.StatusBadRequest, "invalid AI settings request")
		return
	}
	id := ""
	if req.ProfileID != nil {
		id = *req.ProfileID
	}
	if err := h.service.SetPersonalDefault(r.Context(), middleware.GetWorkspaceID(r.Context()), middleware.GetUserID(r.Context()), id); err != nil {
		h.failure(w, err)
		return
	}
	h.Settings(w, r)
}
