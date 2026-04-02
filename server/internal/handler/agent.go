package handler

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

// AgentHandler handles agent HTTP endpoints.
type AgentHandler struct {
	agentService *service.AgentService
}

func parseIntQuery(r *http.Request, key string, fallback int) int {
	if r == nil {
		return fallback
	}
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
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

// ListAgentPresets handles GET /api/pm/agent-presets.
func (h *AgentHandler) ListAgentPresets(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	writeJSON(w, http.StatusOK, h.agentService.ListAgentPresets(r.Context(), workspaceID))
}

// CreateWorkspacePresetVersion handles POST /api/pm/agent-preset-versions.
func (h *AgentHandler) CreateWorkspacePresetVersion(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	actorID := middleware.GetUserID(r.Context())

	var req model.CreateWorkspaceAgentPresetVersionRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.WorkspaceID = workspaceID

	preset, err := h.agentService.CreateWorkspacePresetVersion(r.Context(), req, actorID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, preset)
}

// ListModelProviders handles GET /api/pm/agent-model-providers.
func (h *AgentHandler) ListModelProviders(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.agentService.ListModelProviders())
}

// ListToolCatalog handles GET /api/pm/tool-catalog.
func (h *AgentHandler) ListToolCatalog(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.agentService.ListToolCatalog())
}

// GetRunnerHealth handles GET /api/pm/runner-health.
func (h *AgentHandler) GetRunnerHealth(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	writeJSON(w, http.StatusOK, h.agentService.GetRunnerHealth(r.Context(), workspaceID))
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

// AssignAgentToStory handles POST /api/pm/tasks/{id}/assign-agent.
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

// RunStoryAgent handles POST /api/pm/tasks/{id}/run-agent.
func (h *AgentHandler) RunStoryAgent(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	storyID := chi.URLParam(r, "id")
	actorID := middleware.GetUserID(r.Context())

	var req model.StartAgentRunRequest
	if err := decodeJSON(r, &req); err != nil && r.ContentLength > 0 {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	run, err := h.agentService.RunStoryAgent(r.Context(), workspaceID, storyID, actorID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, run)
}

// RunEpicAgent handles POST /api/pm/epics/{id}/run-agent.
func (h *AgentHandler) RunEpicAgent(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	epicID := chi.URLParam(r, "id")
	actorID := middleware.GetUserID(r.Context())

	var req model.StartAgentRunRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	run, err := h.agentService.RunEpicAgent(r.Context(), workspaceID, epicID, actorID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, run)
}

// StartTargetRun handles POST /api/pm/agent-runs.
func (h *AgentHandler) StartTargetRun(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	actorID := middleware.GetUserID(r.Context())

	var req model.StartTargetAgentRunRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	run, err := h.agentService.StartTargetRun(r.Context(), workspaceID, req.TargetType, req.TargetID, model.StartAgentRunRequest{
		AgentID:           req.AgentID,
		AdditionalContext: req.AdditionalContext,
	}, actorID)
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

// StartCodexDeviceCodeAuth handles POST /api/pm/agent-runs/{id}/codex-auth/device-code/start.
func (h *AgentHandler) StartCodexDeviceCodeAuth(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	runID := chi.URLParam(r, "id")
	actorID := middleware.GetUserID(r.Context())

	authState, err := h.agentService.StartCodexDeviceCodeAuth(r.Context(), workspaceID, runID, actorID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, authState)
}

// CancelCodexDeviceCodeAuth handles POST /api/pm/agent-runs/{id}/codex-auth/device-code/cancel.
func (h *AgentHandler) CancelCodexDeviceCodeAuth(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	runID := chi.URLParam(r, "id")
	actorID := middleware.GetUserID(r.Context())

	authState, err := h.agentService.CancelCodexDeviceCodeAuth(r.Context(), workspaceID, runID, actorID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, authState)
}

// GetCodingSession handles GET /api/pm/coding-sessions/{id}.
func (h *AgentHandler) GetCodingSession(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	sessionID := chi.URLParam(r, "id")

	session, err := h.agentService.GetCodingSession(r.Context(), workspaceID, sessionID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, session)
}

// ListCodingSessionEvents handles GET /api/pm/coding-sessions/{id}/events.
func (h *AgentHandler) ListCodingSessionEvents(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	sessionID := chi.URLParam(r, "id")
	after := parseIntQuery(r, "after", 0)

	events, err := h.agentService.ListCodingSessionEvents(r.Context(), workspaceID, sessionID, after)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, events)
}

// GetCodingSessionRepo handles GET /api/pm/coding-sessions/{id}/repo.
func (h *AgentHandler) GetCodingSessionRepo(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	sessionID := chi.URLParam(r, "id")

	repoState, err := h.agentService.GetCodingSessionRepo(r.Context(), workspaceID, sessionID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, repoState)
}

// GetCodingSessionDiff handles GET /api/pm/coding-sessions/{id}/diff.
func (h *AgentHandler) GetCodingSessionDiff(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	sessionID := chi.URLParam(r, "id")
	path := r.URL.Query().Get("path")

	diff, err := h.agentService.GetCodingSessionDiff(r.Context(), workspaceID, sessionID, path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, diff)
}

// ResolveCodingSessionInteraction handles POST /api/pm/coding-sessions/{id}/interactions/{interactionId}/resolve.
func (h *AgentHandler) ResolveCodingSessionInteraction(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	sessionID := chi.URLParam(r, "id")
	interactionID := chi.URLParam(r, "interactionId")
	actorID := middleware.GetUserID(r.Context())

	var req model.ResolveAgentRunInteractionRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	interaction, err := h.agentService.ResolveCodingSessionInteraction(r.Context(), workspaceID, sessionID, interactionID, actorID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, interaction)
}

// SendCodingSessionMessage handles POST /api/pm/coding-sessions/{id}/message.
func (h *AgentHandler) SendCodingSessionMessage(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	sessionID := chi.URLParam(r, "id")
	actorID := middleware.GetUserID(r.Context())

	var req model.SendAgentRunMessageRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	message, err := h.agentService.SendRunMessage(r.Context(), workspaceID, sessionID, actorID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, message)
}

// ResumeCodingSession handles POST /api/pm/coding-sessions/{id}/resume.
func (h *AgentHandler) ResumeCodingSession(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	sessionID := chi.URLParam(r, "id")
	actorID := middleware.GetUserID(r.Context())

	var req model.ResumeAgentRunRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	run, err := h.agentService.ResumeRun(r.Context(), workspaceID, sessionID, actorID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, run)
}

// ApproveCodingSession handles POST /api/pm/coding-sessions/{id}/approve.
func (h *AgentHandler) ApproveCodingSession(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	sessionID := chi.URLParam(r, "id")
	actorID := middleware.GetUserID(r.Context())

	var req model.ApproveAgentRunRequest
	if err := decodeJSON(r, &req); err != nil && r.ContentLength > 0 {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	run, err := h.agentService.ApproveRun(r.Context(), workspaceID, sessionID, actorID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, run)
}

// RequestCodingSessionChanges handles POST /api/pm/coding-sessions/{id}/request-changes.
func (h *AgentHandler) RequestCodingSessionChanges(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	sessionID := chi.URLParam(r, "id")
	actorID := middleware.GetUserID(r.Context())

	var req model.SendAgentRunRequestChangesRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	run, err := h.agentService.RequestRunChanges(r.Context(), workspaceID, sessionID, actorID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, run)
}

// CancelCodingSession handles POST /api/pm/coding-sessions/{id}/cancel.
func (h *AgentHandler) CancelCodingSession(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	sessionID := chi.URLParam(r, "id")
	actorID := middleware.GetUserID(r.Context())

	run, err := h.agentService.CancelRun(r.Context(), workspaceID, sessionID, actorID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, run)
}

// StartCodingSessionDeviceCodeAuth handles POST /api/pm/coding-sessions/{id}/auth/device-code/start.
func (h *AgentHandler) StartCodingSessionDeviceCodeAuth(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	sessionID := chi.URLParam(r, "id")
	actorID := middleware.GetUserID(r.Context())

	authState, err := h.agentService.StartCodexDeviceCodeAuth(r.Context(), workspaceID, sessionID, actorID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, authState)
}

// CancelCodingSessionDeviceCodeAuth handles POST /api/pm/coding-sessions/{id}/auth/device-code/cancel.
func (h *AgentHandler) CancelCodingSessionDeviceCodeAuth(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	sessionID := chi.URLParam(r, "id")
	actorID := middleware.GetUserID(r.Context())

	authState, err := h.agentService.CancelCodexDeviceCodeAuth(r.Context(), workspaceID, sessionID, actorID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, authState)
}

// ResumeRun handles POST /api/pm/agent-runs/{id}/resume.
func (h *AgentHandler) ResumeRun(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	runID := chi.URLParam(r, "id")
	actorID := middleware.GetUserID(r.Context())

	var req model.ResumeAgentRunRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	run, err := h.agentService.ResumeRun(r.Context(), workspaceID, runID, actorID, req)
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

// RequestRunChanges handles POST /api/pm/agent-runs/{id}/request-changes.
func (h *AgentHandler) RequestRunChanges(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	runID := chi.URLParam(r, "id")
	actorID := middleware.GetUserID(r.Context())

	var req model.SendAgentRunRequestChangesRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	run, err := h.agentService.RequestRunChanges(r.Context(), workspaceID, runID, actorID, req)
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

// ListWorkspaceRuns handles GET /api/pm/agent-runs/workspace.
func (h *AgentHandler) ListWorkspaceRuns(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	pagination := queryPagination(r)

	runs, total, err := h.agentService.ListWorkspaceRuns(r.Context(), workspaceID, pagination)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
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

// ListTargetRuns handles GET /api/pm/agent-runs?target_type=...&target_id=....
func (h *AgentHandler) ListTargetRuns(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	targetType := r.URL.Query().Get("target_type")
	targetID := r.URL.Query().Get("target_id")

	runs, err := h.agentService.ListTargetRuns(r.Context(), workspaceID, targetType, targetID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if runs == nil {
		runs = []model.AgentRun{}
	}
	writeJSON(w, http.StatusOK, runs)
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

// ListRunMessages handles GET /api/pm/agent-runs/{id}/messages.
func (h *AgentHandler) ListRunMessages(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	runID := chi.URLParam(r, "id")

	messages, err := h.agentService.ListRunMessages(r.Context(), workspaceID, runID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if messages == nil {
		messages = []model.AgentRunMessage{}
	}
	writeJSON(w, http.StatusOK, messages)
}

// SendRunMessage handles POST /api/pm/agent-runs/{id}/messages.
func (h *AgentHandler) SendRunMessage(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	runID := chi.URLParam(r, "id")
	actorID := middleware.GetUserID(r.Context())

	var req model.SendAgentRunMessageRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	message, err := h.agentService.SendRunMessage(r.Context(), workspaceID, runID, actorID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, message)
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
