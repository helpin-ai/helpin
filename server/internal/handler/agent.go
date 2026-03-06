package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

// AgentHandler handles agent HTTP endpoints.
type AgentHandler struct {
	agentService *service.AgentService
}

// NewAgentHandler creates a new AgentHandler.
func NewAgentHandler(agentService *service.AgentService) *AgentHandler {
	return &AgentHandler{agentService: agentService}
}

// ListAgents handles GET /api/pm/agents.
func (h *AgentHandler) ListAgents(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}

	agents, err := h.agentService.ListAgents(r.Context(), workspaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if agents == nil {
		agents = []model.Agent{}
	}
	writeJSON(w, http.StatusOK, agents)
}

// GetAgent handles GET /api/pm/agents/{id}.
func (h *AgentHandler) GetAgent(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	id := chi.URLParam(r, "id")

	agent, err := h.agentService.GetAgent(r.Context(), workspaceID, id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, agent)
}

// ListRuntimeProfiles handles GET /api/pm/runtime-profiles.
func (h *AgentHandler) ListRuntimeProfiles(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.agentService.ListRuntimeProfiles())
}

// CreateAgent handles POST /api/pm/agents.
func (h *AgentHandler) CreateAgent(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	actorID := middleware.GetUserID(r.Context())

	var req model.CreateAgentRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.WorkspaceID = workspaceID

	agent, err := h.agentService.CreateAgent(r.Context(), req, actorID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, agent)
}

// UpdateAgent handles PUT /api/pm/agents/{id}.
func (h *AgentHandler) UpdateAgent(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	id := chi.URLParam(r, "id")
	actorID := middleware.GetUserID(r.Context())

	var req model.UpdateAgentRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	agent, err := h.agentService.UpdateAgent(r.Context(), workspaceID, id, req, actorID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, agent)
}

// DeleteAgent handles DELETE /api/pm/agents/{id}.
func (h *AgentHandler) DeleteAgent(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	id := chi.URLParam(r, "id")
	actorID := middleware.GetUserID(r.Context())

	if err := h.agentService.DeleteAgent(r.Context(), workspaceID, id, actorID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

// AssignAgentToStory handles POST /api/pm/stories/{id}/assign-agent.
func (h *AgentHandler) AssignAgentToStory(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	storyID := chi.URLParam(r, "id")
	actorID := middleware.GetUserID(r.Context())

	var req model.AssignAgentRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.agentService.AssignAgentToStory(r.Context(), workspaceID, storyID, req.AgentID, actorID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"assigned": true})
}

// RunAgent handles POST /api/pm/stories/{id}/run-agent.
func (h *AgentHandler) RunAgent(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	storyID := chi.URLParam(r, "id")
	actorID := middleware.GetUserID(r.Context())

	run, err := h.agentService.RunAgent(r.Context(), workspaceID, storyID, actorID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, run)
}

// CancelRun handles POST /api/pm/agent-runs/{id}/cancel.
func (h *AgentHandler) CancelRun(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	runID := chi.URLParam(r, "id")
	actorID := middleware.GetUserID(r.Context())

	run, err := h.agentService.CancelRun(r.Context(), workspaceID, runID, actorID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, run)
}

// ApproveRun handles POST /api/pm/agent-runs/{id}/approve.
func (h *AgentHandler) ApproveRun(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	runID := chi.URLParam(r, "id")
	actorID := middleware.GetUserID(r.Context())

	req := model.ApproveAgentRunRequest{SendMessage: true}
	if err := decodeJSON(r, &req); err != nil && r.ContentLength > 0 {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	run, err := h.agentService.ApproveRun(r.Context(), workspaceID, runID, actorID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, run)
}

// HandoffRun handles POST /api/pm/agent-runs/{id}/handoff.
func (h *AgentHandler) HandoffRun(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	runID := chi.URLParam(r, "id")
	actorID := middleware.GetUserID(r.Context())

	var req model.HandoffAgentRunRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	run, err := h.agentService.HandoffRun(r.Context(), workspaceID, runID, actorID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, run)
}

// ListAgentRuns handles GET /api/pm/agents/{id}/runs.
func (h *AgentHandler) ListAgentRuns(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	agentID := chi.URLParam(r, "id")
	pagination := queryPagination(r)

	runs, total, err := h.agentService.ListAgentRuns(r.Context(), workspaceID, agentID, pagination)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if runs == nil {
		runs = []model.AgentRun{}
	}

	totalPages := 0
	if pagination.PerPage > 0 {
		totalPages = int((total + int64(pagination.PerPage) - 1) / int64(pagination.PerPage))
	}
	writeJSON(w, http.StatusOK, model.PaginatedResponse{
		Data:       runs,
		Total:      int(total),
		Page:       pagination.Page,
		PerPage:    pagination.PerPage,
		TotalPages: totalPages,
	})
}

// GetAgentRun handles GET /api/pm/agent-runs/{id}.
func (h *AgentHandler) GetAgentRun(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	runID := chi.URLParam(r, "id")

	run, err := h.agentService.GetAgentRun(r.Context(), workspaceID, runID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, run)
}

// ListRunArtifacts handles GET /api/pm/agent-runs/{id}/artifacts.
func (h *AgentHandler) ListRunArtifacts(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	runID := chi.URLParam(r, "id")

	artifacts, err := h.agentService.ListRunArtifacts(r.Context(), workspaceID, runID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if artifacts == nil {
		artifacts = []model.AgentRunArtifact{}
	}
	writeJSON(w, http.StatusOK, artifacts)
}
