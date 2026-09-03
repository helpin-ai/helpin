package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

type CommandBarHandler struct {
	commandBarService *service.CommandBarService
	authz             *authorization.AuthzService
}

var commandBarRBAC = authorization.NewRBACEngine()

func NewCommandBarHandler(commandBarService *service.CommandBarService, authz *authorization.AuthzService) *CommandBarHandler {
	return &CommandBarHandler{commandBarService: commandBarService, authz: authz}
}

func (h *CommandBarHandler) DispatchPlan(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	actorID := middleware.GetUserID(r.Context())
	var req model.CommandBarDispatchRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := authorizeCommandBarDispatch(r, req); err != nil {
		writeError(w, http.StatusForbidden, err.Error())
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

// ListEpicPlans returns command-bar deliveries targeting an epic, visible to
// any caller with epic read access (gated at the route by PermPMRead) rather
// than only to the actor who triggered them.
func (h *CommandBarHandler) ListEpicPlans(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	epicID := chi.URLParam(r, "id")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	resp, err := h.commandBarService.ListEntityPlans(r.Context(), workspaceID, "epic", epicID, limit)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// StartEpicDeliveryPipeline handles POST /api/pm/epics/{id}/delivery-pipeline:
// the epic-page button that implements, reviews, and merges every open epic
// task on the integration branch, then opens the epic PR.
func (h *CommandBarHandler) StartEpicDeliveryPipeline(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	actorID := middleware.GetUserID(r.Context())
	epicID := chi.URLParam(r, "id")
	resp, err := h.commandBarService.StartEpicDeliveryPipeline(r.Context(), workspaceID, actorID, epicID, service.DirectDispatchParams())
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *CommandBarHandler) GetPlan(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	actorID := middleware.GetUserID(r.Context())
	planID := chi.URLParam(r, "planID")
	resp, err := h.commandBarService.GetPlan(r.Context(), workspaceID, actorID, planID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func authorizeCommandBarDispatch(r *http.Request, req model.CommandBarDispatchRequest) error {
	return authorizeCommandBarSteps(r, req.PageContext, req.Steps, 0)
}

func authorizeCommandBarSteps(r *http.Request, pageContext model.CommandBarPageContext, steps []model.CommandBarPlanStep, stepOffset int) error {
	actor := authorization.GetActor(r.Context())
	if actor == nil {
		return fmt.Errorf("authorization context missing")
	}
	for i, step := range steps {
		targetType := strings.TrimSpace(step.Target.EntityType)
		if targetType == "" {
			targetType = strings.TrimSpace(pageContext.EntityType)
		}
		if targetType == "" {
			targetType = "workspace"
		}
		perm, ok := commandBarDispatchPermissionForTarget(targetType)
		if !ok {
			return fmt.Errorf("unsupported command-bar target %q in step %d", targetType, stepOffset+i+1)
		}
		if !commandBarRBAC.Can(actor.Role, perm) {
			return fmt.Errorf("insufficient permission for %s target in step %d: requires %s", targetType, stepOffset+i+1, perm)
		}
	}
	return nil
}

func commandBarDispatchPermissionForTarget(targetType string) (authorization.Permission, bool) {
	switch strings.TrimSpace(strings.ToLower(targetType)) {
	case "task", "epic", "workspace":
		return authorization.PermPMEdit, true
	case "repository", "repo", "git_repo", "git_repository":
		return authorization.PermPMEdit, true
	case "doc", "document":
		return authorization.PermDocsEdit, true
	case "contact", "crm_contact", "deal", "crm_deal":
		return authorization.PermCRMEdit, true
	default:
		return "", false
	}
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

func (h *CommandBarHandler) ResumePlan(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	actorID := middleware.GetUserID(r.Context())
	planID := chi.URLParam(r, "planID")
	resp, err := h.commandBarService.ResumePlan(r.Context(), workspaceID, actorID, planID)
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
	// Retry is team-actionable (the epic Delivery panel offers it to any
	// editor, including for plans triggered by someone else), so look the
	// plan up without the dock's owner gate; per-step authorization below
	// still applies to the retrying actor.
	plan, err := h.commandBarService.GetWorkspacePlan(r.Context(), workspaceID, planID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.StepIndex >= 0 && req.StepIndex < len(plan.Plan.Steps) {
		if err := authorizeCommandBarSteps(r, plan.Plan.PageContext, plan.Plan.Steps[req.StepIndex:], req.StepIndex); err != nil {
			writeError(w, http.StatusForbidden, err.Error())
			return
		}
	}
	resp, err := h.commandBarService.RetryPlanFromStep(r.Context(), workspaceID, actorID, planID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, resp)
}

// DismissPlans hides multiple plans from the actor's command runs rail.
// Body: {"plan_ids": ["..."]}.
func (h *CommandBarHandler) DismissPlans(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	actorID := middleware.GetUserID(r.Context())
	var req model.DismissCommandBarPlansRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.commandBarService.DismissPlans(r.Context(), workspaceID, actorID, req.PlanIDs); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusNoContent, nil)
}

// DismissPlan hides a single plan from the actor's command runs rail.
func (h *CommandBarHandler) DismissPlan(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	actorID := middleware.GetUserID(r.Context())
	planID := chi.URLParam(r, "planID")
	if err := h.commandBarService.DismissPlans(r.Context(), workspaceID, actorID, []string{planID}); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusNoContent, nil)
}

func (h *CommandBarHandler) ListAgentToolCatalog(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	agentID := chi.URLParam(r, "agentID")
	selectedTools := r.URL.Query()["selected"]
	if len(selectedTools) == 0 {
		selectedTools = r.URL.Query()["selected_tools"]
	}
	resp, err := h.commandBarService.GetAgentToolCatalog(r.Context(), workspaceID, agentID, selectedTools)
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
