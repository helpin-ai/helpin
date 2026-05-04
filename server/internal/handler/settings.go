package handler

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

// SettingsHandler handles workspace settings HTTP requests.
type SettingsHandler struct {
	settingsService   *service.SettingsService
	automationService *service.AutomationInventoryService
}

// NewSettingsHandler creates a new SettingsHandler.
func NewSettingsHandler(settingsService *service.SettingsService, automationService *service.AutomationInventoryService) *SettingsHandler {
	return &SettingsHandler{settingsService: settingsService, automationService: automationService}
}

// GetAll handles GET /api/settings?workspace_id=xxx.
func (h *SettingsHandler) GetAll(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.URL.Query().Get("workspace_id")
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}

	cfg, err := h.settingsService.GetAll(r.Context(), workspaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, cfg)
}

// GetAIAutomations handles GET /api/settings/ai-automations?workspace_id=xxx.
func (h *SettingsHandler) GetAIAutomations(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.URL.Query().Get("workspace_id")
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

	writeJSON(w, http.StatusOK, inventory)
}

// GetAIAutomationExecutions handles
// GET /api/settings/ai-automations/executions?workspace_id=xxx.
func (h *SettingsHandler) GetAIAutomationExecutions(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.URL.Query().Get("workspace_id")
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

func parseExecutionTimeFilter(raw string, endOfDay bool) (*time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	if ts, err := time.Parse(time.RFC3339, raw); err == nil {
		return &ts, nil
	}
	if date, err := time.Parse("2006-01-02", raw); err == nil {
		if endOfDay {
			date = date.Add(24*time.Hour - time.Nanosecond)
		}
		date = date.UTC()
		return &date, nil
	}
	return nil, errors.New("invalid time format")
}

// GetModuleAccess handles GET /api/settings/module-access?workspace_id=xxx.
func (h *SettingsHandler) GetModuleAccess(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.URL.Query().Get("workspace_id")
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}

	grants, err := h.settingsService.ListModuleGrants(r.Context(), workspaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if grants == nil {
		grants = []model.WorkspaceModuleGrant{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"modules": model.ManagedWorkspaceModules(),
		"grants":  grants,
	})
}

// Initialize handles POST /api/settings/initialize.
func (h *SettingsHandler) Initialize(w http.ResponseWriter, r *http.Request) {
	var req model.InitializeSettingsRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	settings, err := h.settingsService.Initialize(r.Context(), req.WorkspaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, settings)
}

// UpsertModuleAccessGrant handles POST /api/settings/module-access.
func (h *SettingsHandler) UpsertModuleAccessGrant(w http.ResponseWriter, r *http.Request) {
	var req model.CreateWorkspaceModuleGrantRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	actorUserID := middleware.GetUserID(r.Context())
	grant, err := h.settingsService.UpsertModuleGrant(r.Context(), req, actorUserID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, grant)
}

// DeleteModuleAccessGrant handles DELETE /api/settings/module-access/{id}.
func (h *SettingsHandler) DeleteModuleAccessGrant(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.URL.Query().Get("workspace_id")
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}

	id := chi.URLParam(r, "id")
	if err := h.settingsService.DeleteModuleGrant(r.Context(), workspaceID, id); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "module grant deleted"})
}

// CreateTeam handles POST /api/settings/teams.
func (h *SettingsHandler) CreateTeam(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	var req model.CreateTeamRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	team, err := h.settingsService.CreateTeam(r.Context(), req, userID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, team)
}

// EnsureDefaultTeam handles POST /api/settings/teams/ensure-default.
// Returns the workspace's canonical team of the requested type, creating one
// lazily if none exists. Used by CRM surfaces that need to resolve a target
// team (e.g. sales) without requiring onboarding to have pre-created it.
func (h *SettingsHandler) EnsureDefaultTeam(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	var req model.EnsureDefaultTeamRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	team, err := h.settingsService.EnsureDefaultTeam(r.Context(), req.WorkspaceID, req.TeamType, userID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, team)
}

// UpdateTeam handles PUT /api/settings/teams/{id}.
func (h *SettingsHandler) UpdateTeam(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req model.UpdateTeamRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	team, err := h.settingsService.UpdateTeam(r.Context(), id, req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, team)
}

// DeleteTeam handles DELETE /api/settings/teams/{id}.
func (h *SettingsHandler) DeleteTeam(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.settingsService.DeleteTeam(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "team deleted"})
}

// AddTeamMember handles POST /api/settings/teams/{id}/members.
func (h *SettingsHandler) AddTeamMember(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req model.AddTeamMemberRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	membership, err := h.settingsService.AddTeamMember(r.Context(), id, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, membership)
}

// UpdateTeamMember handles PUT /api/settings/teams/{id}/members/{userId}.
func (h *SettingsHandler) UpdateTeamMember(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userID := chi.URLParam(r, "userId")
	actor := authorization.GetActor(r.Context())

	var req model.UpdateTeamMemberRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	membership, err := h.settingsService.UpdateTeamMember(r.Context(), id, userID, actor, req)
	if err != nil {
		writeError(w, http.StatusForbidden, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, membership)
}

// DeleteTeamMember handles DELETE /api/settings/teams/{id}/members/{userId}.
func (h *SettingsHandler) DeleteTeamMember(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userID := chi.URLParam(r, "userId")
	actor := authorization.GetActor(r.Context())
	if err := h.settingsService.RemoveTeamMember(r.Context(), id, userID, actor); err != nil {
		writeError(w, http.StatusForbidden, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "team member removed"})
}

// AddTeamInvitation handles POST /api/settings/teams/{id}/invitations.
func (h *SettingsHandler) AddTeamInvitation(w http.ResponseWriter, r *http.Request) {
	teamID := chi.URLParam(r, "id")
	var req struct {
		InvitationID string `json:"invitation_id"`
	}
	if err := decodeJSON(r, &req); err != nil || req.InvitationID == "" {
		writeError(w, http.StatusBadRequest, "invitation_id is required")
		return
	}

	entry, err := h.settingsService.AddInvitationTeamPreassignment(r.Context(), teamID, req.InvitationID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, entry)
}

// DeleteTeamInvitation handles DELETE /api/settings/teams/{id}/invitations/{invitationId}.
func (h *SettingsHandler) DeleteTeamInvitation(w http.ResponseWriter, r *http.Request) {
	teamID := chi.URLParam(r, "id")
	invitationID := chi.URLParam(r, "invitationId")
	if err := h.settingsService.RemoveInvitationTeamPreassignment(r.Context(), teamID, invitationID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "invitation preassignment removed"})
}

// CreatePerson handles POST /api/settings/people.
func (h *SettingsHandler) CreatePerson(w http.ResponseWriter, r *http.Request) {
	var req model.CreatePersonRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	person, err := h.settingsService.CreatePerson(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, person)
}

// UpdatePerson handles PUT /api/settings/people/{id}.
func (h *SettingsHandler) UpdatePerson(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req model.UpdatePersonRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	person, err := h.settingsService.UpdatePerson(r.Context(), id, req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, person)
}

// DeletePerson handles DELETE /api/settings/people/{id}.
func (h *SettingsHandler) DeletePerson(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.settingsService.DeletePerson(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "person deleted"})
}

// UpdateJobRoleCriteria handles PUT /api/settings/job-roles.
func (h *SettingsHandler) UpdateJobRoleCriteria(w http.ResponseWriter, r *http.Request) {
	var req model.UpdateJobRoleCriteriaRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	criteria, err := h.settingsService.UpdateJobRoleCriteria(r.Context(), req.WorkspaceID, req.JobRole, req.Criteria)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, criteria)
}

// DeleteJobRole handles DELETE /api/settings/job-roles?workspace_id=xxx&job_role=xxx.
func (h *SettingsHandler) DeleteJobRole(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.URL.Query().Get("workspace_id")
	jobRole := r.URL.Query().Get("job_role")
	if workspaceID == "" || jobRole == "" {
		writeError(w, http.StatusBadRequest, "workspace_id and job_role are required")
		return
	}

	if err := h.settingsService.DeleteJobRole(r.Context(), workspaceID, jobRole); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "job role deleted"})
}

// GetTeamEstimateSettings handles GET /api/settings/teams/{id}/estimates.
func (h *SettingsHandler) GetTeamEstimateSettings(w http.ResponseWriter, r *http.Request) {
	teamID := chi.URLParam(r, "id")

	settings, err := h.settingsService.GetTeamEstimateSettings(r.Context(), teamID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if settings == nil {
		writeJSON(w, http.StatusOK, nil)
		return
	}

	writeJSON(w, http.StatusOK, settings)
}

// UpdateTeamEstimateSettings handles PUT /api/settings/teams/{id}/estimates.
func (h *SettingsHandler) UpdateTeamEstimateSettings(w http.ResponseWriter, r *http.Request) {
	teamID := chi.URLParam(r, "id")

	var req model.UpdateTeamEstimateSettingsRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	settings, err := h.settingsService.UpdateTeamEstimateSettings(r.Context(), teamID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, settings)
}

// GetTeamFieldVisibility handles GET /api/settings/teams/{id}/field-visibility.
func (h *SettingsHandler) GetTeamFieldVisibility(w http.ResponseWriter, r *http.Request) {
	teamID := chi.URLParam(r, "id")

	settings, err := h.settingsService.GetTeamFieldVisibility(r.Context(), teamID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if settings == nil {
		writeJSON(w, http.StatusOK, nil)
		return
	}

	writeJSON(w, http.StatusOK, settings)
}

// UpdateTeamFieldVisibility handles PUT /api/settings/teams/{id}/field-visibility.
func (h *SettingsHandler) UpdateTeamFieldVisibility(w http.ResponseWriter, r *http.Request) {
	teamID := chi.URLParam(r, "id")

	var req model.UpdateTeamFieldVisibilityRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	settings, err := h.settingsService.UpdateTeamFieldVisibility(r.Context(), teamID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, settings)
}

// GetTeamRepoDefault handles GET /api/settings/teams/{id}/repo-default.
func (h *SettingsHandler) GetTeamRepoDefault(w http.ResponseWriter, r *http.Request) {
	teamID := chi.URLParam(r, "id")
	cfg, err := h.settingsService.GetTeamRepoDefault(r.Context(), teamID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if cfg == nil {
		writeJSON(w, http.StatusOK, map[string]any{})
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

// UpdateTeamRepoDefault handles PUT /api/settings/teams/{id}/repo-default.
func (h *SettingsHandler) UpdateTeamRepoDefault(w http.ResponseWriter, r *http.Request) {
	teamID := chi.URLParam(r, "id")
	var req model.UpdateTeamRepoDefaultRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	cfg, err := h.settingsService.UpdateTeamRepoDefault(r.Context(), teamID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

// UpdateSystem handles PUT /api/settings/system.
func (h *SettingsHandler) UpdateSystem(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.URL.Query().Get("workspace_id")
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}

	var req model.UpdateSystemSettingsRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.EnforceTwoFactor != nil && *req.EnforceTwoFactor {
		claims := middleware.ClaimsFrom(r.Context())
		if claims == nil || !claims.MFASatisfied {
			writeError(w, http.StatusForbidden, "verify two-factor authentication before enabling enforcement")
			return
		}
	}

	settings, err := h.settingsService.UpdateSystem(r.Context(), workspaceID, req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, settings)
}
