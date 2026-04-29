package handler

import (
	"fmt"
	"log/slog"
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
}

var commandBarRBAC = authorization.NewRBACEngine()

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
	case "story":
		return authorization.PermPMEdit, true
	case "task", "epic", "workspace":
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

func (h *CommandBarHandler) RetryPlan(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	actorID := middleware.GetUserID(r.Context())
	planID := chi.URLParam(r, "planID")
	var req model.CommandBarRetryPlanRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	plan, err := h.commandBarService.GetPlan(r.Context(), workspaceID, actorID, planID)
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

func (h *CommandBarHandler) ListUnmetIntents(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	includeSensitive := r.URL.Query().Get("include_sensitive") == "true"
	if includeSensitive {
		slog.InfoContext(r.Context(), "command bar unmet intents sensitive prompt requested", "workspace_id", workspaceID, "actor_id", middleware.GetUserID(r.Context()))
	}
	resp, err := h.commandBarService.ListUnmetIntents(r.Context(), workspaceID, r.URL.Query().Get("status"), limit, includeSensitive)
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
