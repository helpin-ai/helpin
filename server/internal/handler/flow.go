package handler

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

type FlowHandler struct {
	flowService *service.FlowService
}

func NewFlowHandler(flowService *service.FlowService) *FlowHandler {
	return &FlowHandler{flowService: flowService}
}

func (h *FlowHandler) StartRun(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	actorID := middleware.GetUserID(r.Context())

	var req model.StartFlowRunRequest
	if err := decodeJSON(r, &req); err != nil && r.ContentLength > 0 {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	runView, err := h.flowService.StartRun(r.Context(), workspaceID, actorID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, runView)
}

func (h *FlowHandler) GetRun(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	flowRunID := chi.URLParam(r, "flowRunId")

	runView, err := h.flowService.GetRunView(r.Context(), workspaceID, flowRunID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, runView)
}

func (h *FlowHandler) ListNodeRuns(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	flowRunID := chi.URLParam(r, "flowRunId")

	nodeRuns, err := h.flowService.ListNodeRuns(r.Context(), workspaceID, flowRunID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, nodeRuns)
}

func (h *FlowHandler) SendNodeAction(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	flowRunID := chi.URLParam(r, "flowRunId")
	nodeRunID := chi.URLParam(r, "nodeRunId")
	actorID := middleware.GetUserID(r.Context())

	var req model.FlowNodeActionRequest
	if err := decodeJSON(r, &req); err != nil && r.ContentLength > 0 {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	runView, err := h.flowService.SendNodeAction(r.Context(), workspaceID, flowRunID, nodeRunID, actorID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, runView)
}

func (h *FlowHandler) ListInteractiveMessages(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	flowRunID := chi.URLParam(r, "flowRunId")
	nodeRunID := chi.URLParam(r, "nodeRunId")

	messages, err := h.flowService.ListInteractiveMessages(r.Context(), workspaceID, flowRunID, nodeRunID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, messages)
}

func (h *FlowHandler) SendInteractiveMessage(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	flowRunID := chi.URLParam(r, "flowRunId")
	nodeRunID := chi.URLParam(r, "nodeRunId")
	actorID := middleware.GetUserID(r.Context())

	var req model.FlowInteractiveMessageRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	message, err := h.flowService.SendInteractiveMessage(r.Context(), workspaceID, flowRunID, nodeRunID, actorID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, message)
}

func (h *FlowHandler) ListRuns(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)

	limit := 20
	if l, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && l > 0 {
		limit = l
	}
	offset := 0
	if o, err := strconv.Atoi(r.URL.Query().Get("offset")); err == nil && o >= 0 {
		offset = o
	}

	resp, err := h.flowService.ListRuns(r.Context(), workspaceID, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *FlowHandler) ListTemplates(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	templates := h.flowService.ListTemplates(r.Context(), workspaceID)
	writeJSON(w, http.StatusOK, templates)
}

func (h *FlowHandler) CancelRun(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	flowRunID := chi.URLParam(r, "flowRunId")
	actorID := middleware.GetUserID(r.Context())

	runView, err := h.flowService.CancelRun(r.Context(), workspaceID, flowRunID, actorID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, runView)
}

// CancelActiveRunByTarget cancels the active flow run for a given target.
func (h *FlowHandler) CancelActiveRunByTarget(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	actorID := middleware.GetUserID(r.Context())

	var req struct {
		TargetType string `json:"target_type"`
		TargetID   string `json:"target_id"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.TargetType == "" || req.TargetID == "" {
		writeError(w, http.StatusBadRequest, "target_type and target_id are required")
		return
	}

	runView, err := h.flowService.CancelActiveRunByTarget(r.Context(), workspaceID, req.TargetType, req.TargetID, actorID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if runView != nil {
		writeJSON(w, http.StatusOK, runView)
		return
	}
	// Orphaned planning session was abandoned — no flow run to return.
	writeJSON(w, http.StatusOK, map[string]string{"status": "cancelled"})
}

func (h *FlowHandler) RetryNode(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	flowRunID := chi.URLParam(r, "flowRunId")
	nodeRunID := chi.URLParam(r, "nodeRunId")
	actorID := middleware.GetUserID(r.Context())

	runView, err := h.flowService.RetryNode(r.Context(), workspaceID, flowRunID, nodeRunID, actorID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, runView)
}
