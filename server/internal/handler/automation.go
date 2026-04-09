package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

// AutomationHandler exposes the product-level Automation API facade.
type AutomationHandler struct {
	automationService *service.AutomationInventoryService
	ruleEngine        *service.AutomationRuleEngine
	agentService      *service.AgentService
}

// NewAutomationHandler creates a new AutomationHandler.
func NewAutomationHandler(
	automationService *service.AutomationInventoryService,
	ruleEngine *service.AutomationRuleEngine,
	agentService *service.AgentService,
) *AutomationHandler {
	return &AutomationHandler{
		automationService: automationService,
		ruleEngine:        ruleEngine,
		agentService:      agentService,
	}
}

// GetOverview handles GET /api/automation/overview.
func (h *AutomationHandler) GetOverview(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	if h.automationService == nil {
		writeError(w, http.StatusServiceUnavailable, "automation inventory is unavailable")
		return
	}

	inventory, err := h.automationService.GetWorkspaceInventory(r.Context(), workspaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if inventory == nil {
		writeJSON(w, http.StatusOK, model.AutomationInventoryResponse{})
		return
	}
	writeJSON(w, http.StatusOK, inventory)
}

// ListFlows handles GET /api/automation/flows.
func (h *AutomationHandler) ListFlows(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}

	workflowID := r.URL.Query().Get("workflow_id")
	var (
		rules []model.AutomationRule
		err   error
	)
	if workflowID != "" {
		rules, err = h.ruleEngine.ListRulesByWorkflow(r.Context(), workspaceID, workflowID)
	} else {
		rules, err = h.ruleEngine.ListRules(r.Context(), workspaceID)
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if rules == nil {
		rules = []model.AutomationRule{}
	}
	writeJSON(w, http.StatusOK, rules)
}

// CreateFlow handles POST /api/automation/flows.
func (h *AutomationHandler) CreateFlow(w http.ResponseWriter, r *http.Request) {
	var req model.CreateAutomationRuleRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	if req.WorkspaceID == "" {
		req.WorkspaceID = workspaceID
	}

	rule, err := h.ruleEngine.CreateRule(r.Context(), workspaceID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, rule)
}

// GetFlow handles GET /api/automation/flows/{id}.
func (h *AutomationHandler) GetFlow(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	flowID := chi.URLParam(r, "id")
	if workspaceID == "" || flowID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id and flow_id are required")
		return
	}

	rule, err := h.ruleEngine.GetRule(r.Context(), workspaceID, flowID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if rule == nil {
		writeError(w, http.StatusNotFound, "flow not found")
		return
	}
	writeJSON(w, http.StatusOK, rule)
}

// UpdateFlow handles PUT /api/automation/flows/{id}.
func (h *AutomationHandler) UpdateFlow(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	flowID := chi.URLParam(r, "id")
	if workspaceID == "" || flowID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id and flow_id are required")
		return
	}

	var req model.UpdateAutomationRuleRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	rule, err := h.ruleEngine.UpdateRule(r.Context(), workspaceID, flowID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, rule)
}

// DeleteFlow handles DELETE /api/automation/flows/{id}.
func (h *AutomationHandler) DeleteFlow(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	flowID := chi.URLParam(r, "id")
	if workspaceID == "" || flowID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id and flow_id are required")
		return
	}

	if err := h.ruleEngine.DeleteRule(r.Context(), workspaceID, flowID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "flow deleted"})
}

// ListActivity handles GET /api/automation/activity.
func (h *AutomationHandler) ListActivity(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	if h.automationService == nil {
		writeError(w, http.StatusServiceUnavailable, "automation inventory is unavailable")
		return
	}

	firedAfter, err := parseExecutionTimeFilter(r.URL.Query().Get("fired_after"), false)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid fired_after")
		return
	}
	firedBefore, err := parseExecutionTimeFilter(r.URL.Query().Get("fired_before"), true)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid fired_before")
		return
	}

	filters := model.TriggerExecutionListFilters{
		AgentID:     queryStringPtr(r, "agent_id"),
		BindingID:   queryStringPtr(r, "binding_id"),
		TriggerType: queryStringPtrWithFallback(r, "trigger_type"),
		BindingKind: queryStringPtrWithFallback(r, "binding_kind", "source"),
		Status:      queryStringPtr(r, "status"),
		ReferenceID: queryStringPtr(r, "reference_id"),
		FiredAfter:  firedAfter,
		FiredBefore: firedBefore,
	}
	pagination := queryPagination(r)
	if pagination.PerPage <= 0 {
		pagination.PerPage = 25
	}
	if pagination.PerPage > 100 {
		pagination.PerPage = 100
	}

	result, err := h.automationService.ListTriggerExecutions(r.Context(), workspaceID, filters, pagination)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// ListTriggerCatalog handles GET /api/automation/library/triggers.
func (h *AutomationHandler) ListTriggerCatalog(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	if h.automationService == nil {
		writeError(w, http.StatusServiceUnavailable, "automation inventory is unavailable")
		return
	}

	inventory, err := h.automationService.GetWorkspaceInventory(r.Context(), workspaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if inventory == nil || inventory.TriggerCatalog == nil {
		writeJSON(w, http.StatusOK, []model.AutomationTriggerCatalogEntry{})
		return
	}
	writeJSON(w, http.StatusOK, inventory.TriggerCatalog)
}

// ListToolCatalog handles GET /api/automation/library/tools.
func (h *AutomationHandler) ListToolCatalog(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.agentService.ListToolCatalog())
}

// ListAgents handles GET /api/automation/agents.
func (h *AutomationHandler) ListAgents(w http.ResponseWriter, r *http.Request) {
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

// CreateAgent handles POST /api/automation/agents.
func (h *AutomationHandler) CreateAgent(w http.ResponseWriter, r *http.Request) {
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

// GetAgent handles GET /api/automation/agents/{id}.
func (h *AutomationHandler) GetAgent(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	id := chi.URLParam(r, "id")

	agent, err := h.agentService.GetAgent(r.Context(), workspaceID, id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, agent)
}

// UpdateAgent handles PUT /api/automation/agents/{id}.
func (h *AutomationHandler) UpdateAgent(w http.ResponseWriter, r *http.Request) {
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

// DeleteAgent handles DELETE /api/automation/agents/{id}.
func (h *AutomationHandler) DeleteAgent(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	id := chi.URLParam(r, "id")
	actorID := middleware.GetUserID(r.Context())

	if err := h.agentService.DeleteAgent(r.Context(), workspaceID, id, actorID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

// GetAgentUsage handles GET /api/automation/agents/{id}/usage.
func (h *AutomationHandler) GetAgentUsage(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	id := chi.URLParam(r, "id")

	summary, err := h.agentService.GetAgentUsageSummary(r.Context(), workspaceID, id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

// ListRuns handles GET /api/automation/runs.
func (h *AutomationHandler) ListRuns(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	targetType := r.URL.Query().Get("target_type")
	targetID := r.URL.Query().Get("target_id")

	if targetType != "" || targetID != "" {
		runs, err := h.agentService.ListTargetRuns(r.Context(), workspaceID, targetType, targetID)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if runs == nil {
			runs = []model.AgentRun{}
		}
		writeJSON(w, http.StatusOK, runs)
		return
	}

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

// StartRun handles POST /api/automation/runs.
func (h *AutomationHandler) StartRun(w http.ResponseWriter, r *http.Request) {
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
		BaseBranch:        req.BaseBranch,
		WorkingBranch:     req.WorkingBranch,
	}, actorID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, run)
}

// GetRun handles GET /api/automation/runs/{id}.
func (h *AutomationHandler) GetRun(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	runID := chi.URLParam(r, "id")

	run, err := h.agentService.GetAgentRun(r.Context(), workspaceID, runID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, run)
}

// ListRunMessages handles GET /api/automation/runs/{id}/messages.
func (h *AutomationHandler) ListRunMessages(w http.ResponseWriter, r *http.Request) {
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

// SendRunMessage handles POST /api/automation/runs/{id}/messages.
func (h *AutomationHandler) SendRunMessage(w http.ResponseWriter, r *http.Request) {
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

// ListRunArtifacts handles GET /api/automation/runs/{id}/artifacts.
func (h *AutomationHandler) ListRunArtifacts(w http.ResponseWriter, r *http.Request) {
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

// ResumeRun handles POST /api/automation/runs/{id}/resume.
func (h *AutomationHandler) ResumeRun(w http.ResponseWriter, r *http.Request) {
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

// ApproveRun handles POST /api/automation/runs/{id}/approve.
func (h *AutomationHandler) ApproveRun(w http.ResponseWriter, r *http.Request) {
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

// RequestRunChanges handles POST /api/automation/runs/{id}/request-changes.
func (h *AutomationHandler) RequestRunChanges(w http.ResponseWriter, r *http.Request) {
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

// CancelRun handles POST /api/automation/runs/{id}/cancel.
func (h *AutomationHandler) CancelRun(w http.ResponseWriter, r *http.Request) {
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

// HandoffRun handles POST /api/automation/runs/{id}/handoff.
func (h *AutomationHandler) HandoffRun(w http.ResponseWriter, r *http.Request) {
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
