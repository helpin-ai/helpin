package handler

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

type CommandBarHandler struct {
	commandBarService *service.CommandBarService
}

func NewCommandBarHandler(commandBarService *service.CommandBarService) *CommandBarHandler {
	return &CommandBarHandler{commandBarService: commandBarService}
}

func (h *CommandBarHandler) ParseIntent(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	actorID := middleware.GetUserID(r.Context())
	var req model.CommandBarParseRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	resp, err := h.commandBarService.ParseIntent(r.Context(), workspaceID, actorID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *CommandBarHandler) DispatchPlan(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	actorID := middleware.GetUserID(r.Context())
	var req model.CommandBarDispatchRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	resp, err := h.commandBarService.DispatchPlan(r.Context(), workspaceID, actorID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, resp)
}

func (h *CommandBarHandler) ListPlans(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	actorID := middleware.GetUserID(r.Context())
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	resp, err := h.commandBarService.ListPlans(r.Context(), workspaceID, actorID, limit)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *CommandBarHandler) CancelPlan(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	actorID := middleware.GetUserID(r.Context())
	planID := chi.URLParam(r, "planID")
	resp, err := h.commandBarService.CancelPlan(r.Context(), workspaceID, actorID, planID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *CommandBarHandler) RetryPlan(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	actorID := middleware.GetUserID(r.Context())
	planID := chi.URLParam(r, "planID")
	var req model.CommandBarRetryPlanRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	resp, err := h.commandBarService.RetryPlanFromStep(r.Context(), workspaceID, actorID, planID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, resp)
}

func (h *CommandBarHandler) ListUnmetIntents(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	resp, err := h.commandBarService.ListUnmetIntents(r.Context(), workspaceID, r.URL.Query().Get("status"), limit)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *CommandBarHandler) ReviewUnmetIntent(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	intentID := chi.URLParam(r, "intentID")
	var req model.ReviewCommandBarUnmetIntentRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	resp, err := h.commandBarService.ReviewUnmetIntent(r.Context(), workspaceID, intentID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *CommandBarHandler) PromoteRunToAgent(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	actorID := middleware.GetUserID(r.Context())
	runID := chi.URLParam(r, "runID")
	var req model.PromoteCommandBarRunRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	resp, err := h.commandBarService.PromoteRunToAgent(r.Context(), workspaceID, actorID, runID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, resp)
}
