package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

// AutomationRuleHandler handles automation rule HTTP endpoints.
type AutomationRuleHandler struct {
	ruleEngine *service.AutomationRuleEngine
}

// NewAutomationRuleHandler creates a new AutomationRuleHandler.
func NewAutomationRuleHandler(engine *service.AutomationRuleEngine) *AutomationRuleHandler {
	return &AutomationRuleHandler{ruleEngine: engine}
}

// List handles GET /api/pm/automation-rules.
func (h *AutomationRuleHandler) List(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}

	workflowID := r.URL.Query().Get("workflow_id")
	var rules []model.AutomationRule
	var err error
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

// Create handles POST /api/pm/automation-rules.
func (h *AutomationRuleHandler) Create(w http.ResponseWriter, r *http.Request) {
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

// Get handles GET /api/pm/automation-rules/{ruleId}.
func (h *AutomationRuleHandler) Get(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	ruleID := chi.URLParam(r, "ruleId")
	if workspaceID == "" || ruleID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id and rule_id are required")
		return
	}

	rule, err := h.ruleEngine.GetRule(r.Context(), workspaceID, ruleID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if rule == nil {
		writeError(w, http.StatusNotFound, "automation rule not found")
		return
	}
	writeJSON(w, http.StatusOK, rule)
}

// Update handles PUT /api/pm/automation-rules/{ruleId}.
func (h *AutomationRuleHandler) Update(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	ruleID := chi.URLParam(r, "ruleId")
	if workspaceID == "" || ruleID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id and rule_id are required")
		return
	}

	var req model.UpdateAutomationRuleRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	rule, err := h.ruleEngine.UpdateRule(r.Context(), workspaceID, ruleID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, rule)
}

// Delete handles DELETE /api/pm/automation-rules/{ruleId}.
func (h *AutomationRuleHandler) Delete(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	ruleID := chi.URLParam(r, "ruleId")
	if workspaceID == "" || ruleID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id and rule_id are required")
		return
	}

	if err := h.ruleEngine.DeleteRule(r.Context(), workspaceID, ruleID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "automation rule deleted"})
}
