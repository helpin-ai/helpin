package handler

import (
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

// PMTaskHandler handles PM task HTTP endpoints.
type PMTaskHandler struct {
	taskService *service.PMTaskService
}

// NewPMTaskHandler creates a new PMTaskHandler.
func NewPMTaskHandler(taskService *service.PMTaskService) *PMTaskHandler {
	return &PMTaskHandler{taskService: taskService}
}

// List handles GET /api/pm/tasks.
func (h *PMTaskHandler) List(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	archived, err := queryBoolPtr(r, "archived")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid archived query param")
		return
	}
	filters := model.PMTaskFilters{
		TeamID:                queryStringPtr(r, "team_id"),
		EpicID:                queryStringPtr(r, "epic_id"),
		SprintID:              queryStringPtr(r, "sprint_id"),
		ContactID:             queryStringPtr(r, "contact_id"),
		CompanyID:             queryStringPtr(r, "company_id"),
		DealID:                queryStringPtr(r, "deal_id"),
		SupportConversationID: queryStringPtr(r, "support_conversation_id"),
		IncludeContacts:       r.URL.Query().Get("include_contacts") == "true",
		IncludeCompanies:      r.URL.Query().Get("include_companies") == "true",
		IncludeDeals:          r.URL.Query().Get("include_deals") == "true",
		IncludeSupport:        r.URL.Query().Get("include_support") == "true",
		WorkflowID:            queryStringPtr(r, "workflow_id"),
		WorkflowStateID:       queryStringPtr(r, "state_id"),
		TaskType:             queryStringPtr(r, "task_type"),
		OwnerID:               queryStringPtr(r, "owner_id"),
		OwnerMemberID:         queryStringPtr(r, "owner_member_id"),
		RequesterID:           queryStringPtr(r, "requester_id"),
		RequesterMemberID:     queryStringPtr(r, "requester_member_id"),
		LabelID:               queryStringPtr(r, "label_id"),
		Priority:              queryStringPtr(r, "priority"),
		Severity:              queryStringPtr(r, "severity"),
		Blocked:               queryStringPtr(r, "blocked"),
		Blocking:              queryStringPtr(r, "blocking"),
		Archived:              archived,
	}
	pagination := queryPagination(r)

	tasks, total, err := h.taskService.List(r.Context(), workspaceID, filters, pagination)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if tasks == nil {
		tasks = []model.BoardTask{}
	}

	totalPages := 0
	if pagination.PerPage > 0 {
		totalPages = int((total + int64(pagination.PerPage) - 1) / int64(pagination.PerPage))
	}
	writeJSON(w, http.StatusOK, model.PaginatedResponse{
		Data:       tasks,
		Total:      int(total),
		Page:       pagination.Page,
		PerPage:    pagination.PerPage,
		TotalPages: totalPages,
	})
}

// ListBoard handles GET /api/pm/tasks/board?workflow_id=...&team_id=...&per_state_limit=...
func (h *PMTaskHandler) ListBoard(w http.ResponseWriter, r *http.Request) {
	workflowID := r.URL.Query().Get("workflow_id")
	if workflowID == "" {
		writeError(w, http.StatusBadRequest, "workflow_id is required")
		return
	}
	filters := boardFilters(r)
	perStateLimit := queryInt(r, "per_state_limit", 0)
	columns, err := h.taskService.ListByWorkflowState(r.Context(), workflowID, filters, perStateLimit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if columns == nil {
		columns = []model.TaskStateColumn{}
	}
	writeJSON(w, http.StatusOK, columns)
}

// ListBoardColumn handles GET /api/pm/tasks/board/column?state_id=...&offset=...&limit=...
func (h *PMTaskHandler) ListBoardColumn(w http.ResponseWriter, r *http.Request) {
	stateID := r.URL.Query().Get("state_id")
	if stateID == "" {
		writeError(w, http.StatusBadRequest, "state_id is required")
		return
	}
	filters := boardFilters(r)
	offset := queryInt(r, "offset", 0)
	limit := queryInt(r, "limit", 50)
	tasks, taskGroups, total, err := h.taskService.ListColumnTasks(r.Context(), stateID, filters, offset, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if tasks == nil {
		tasks = []model.BoardTask{}
	}
	if taskGroups == nil {
		taskGroups = []model.TaskGroup{}
	}
	writeJSON(w, http.StatusOK, model.ColumnTasksResponse{
		Tasks:       tasks,
		TaskGroups:  taskGroups,
		Total:       total,
	})
}

// ListBoardByMember handles GET /api/pm/tasks/board/members?workflow_id=...&workspace_id=...
func (h *PMTaskHandler) ListBoardByMember(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	workflowID := r.URL.Query().Get("workflow_id")
	if workflowID == "" {
		writeError(w, http.StatusBadRequest, "workflow_id is required")
		return
	}
	filters := boardFilters(r)
	perMemberLimit := queryInt(r, "per_member_limit", 0)
	includeEmpty := r.URL.Query().Get("include_empty") == "true"
	var memberIDs []string
	if raw := r.URL.Query().Get("member_ids"); raw != "" {
		memberIDs = strings.Split(raw, ",")
	}
	columns, err := h.taskService.ListByMember(r.Context(), workspaceID, workflowID, filters, perMemberLimit, includeEmpty, memberIDs)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if columns == nil {
		columns = []model.TaskMemberColumn{}
	}
	writeJSON(w, http.StatusOK, columns)
}

// ListBoardMemberColumn handles GET /api/pm/tasks/board/members/column?workspace_id=...&workflow_id=...&member_id=...
func (h *PMTaskHandler) ListBoardMemberColumn(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	workflowID := r.URL.Query().Get("workflow_id")
	if workflowID == "" {
		writeError(w, http.StatusBadRequest, "workflow_id is required")
		return
	}
	filters := boardFilters(r)
	offset := queryInt(r, "offset", 0)
	limit := queryInt(r, "limit", 50)
	var memberID *string
	if raw := r.URL.Query().Get("member_id"); raw != "" {
		memberID = &raw
	}
	tasks, total, err := h.taskService.ListMemberColumnTasks(r.Context(), workspaceID, workflowID, memberID, filters, offset, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if tasks == nil {
		tasks = []model.BoardTask{}
	}
	writeJSON(w, http.StatusOK, model.ColumnTasksResponse{
		Tasks:  tasks,
		Total:  total,
	})
}

func boardFilters(r *http.Request) model.PMTaskFilters {
	return model.PMTaskFilters{
		TeamID:                queryStringPtr(r, "team_id"),
		Priority:              queryStringPtr(r, "priority"),
		Severity:              queryStringPtr(r, "severity"),
		TaskType:             queryStringPtr(r, "task_type"),
		EpicID:                queryStringPtr(r, "epic_id"),
		SprintID:              queryStringPtr(r, "sprint_id"),
		ContactID:             queryStringPtr(r, "contact_id"),
		CompanyID:             queryStringPtr(r, "company_id"),
		DealID:                queryStringPtr(r, "deal_id"),
		SupportConversationID: queryStringPtr(r, "support_conversation_id"),
		IncludeContacts:       r.URL.Query().Get("include_contacts") == "true",
		IncludeCompanies:      r.URL.Query().Get("include_companies") == "true",
		IncludeDeals:          r.URL.Query().Get("include_deals") == "true",
		IncludeSupport:        r.URL.Query().Get("include_support") == "true",
		LabelID:               queryStringPtr(r, "label_id"),
		OwnerID:               queryStringPtr(r, "owner_id"),
		OwnerMemberID:         queryStringPtr(r, "owner_member_id"),
		RequesterID:           queryStringPtr(r, "requester_id"),
		RequesterMemberID:     queryStringPtr(r, "requester_member_id"),
		Blocked:               queryStringPtr(r, "blocked"),
		Blocking:              queryStringPtr(r, "blocking"),
		UpdatedAfter:          queryStringPtr(r, "updated_after"),
	}
}

// CountByState handles GET /api/pm/tasks/counts?workflow_id=...
func (h *PMTaskHandler) CountByState(w http.ResponseWriter, r *http.Request) {
	workflowID := r.URL.Query().Get("workflow_id")
	if workflowID == "" {
		writeError(w, http.StatusBadRequest, "workflow_id is required")
		return
	}
	counts, err := h.taskService.CountByState(r.Context(), workflowID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if counts == nil {
		counts = []model.TaskStateCount{}
	}
	writeJSON(w, http.StatusOK, counts)
}

// Create handles POST /api/pm/tasks.
func (h *PMTaskHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	var req model.CreateTaskRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.WorkspaceID == "" {
		req.WorkspaceID = getWorkspaceID(r)
	}
	task, err := h.taskService.Create(r.Context(), req, userID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, task)
}

// Get handles GET /api/pm/tasks/{id}.
func (h *PMTaskHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	task, err := h.taskService.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, task)
}

// GetByDisplayID handles GET /api/pm/tasks/display/{displayID}.
func (h *PMTaskHandler) GetByDisplayID(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	rawDisplayID := chi.URLParam(r, "displayID")
	displayID, err := strconv.Atoi(rawDisplayID)
	if err != nil || displayID <= 0 {
		writeError(w, http.StatusBadRequest, "displayID must be a positive integer")
		return
	}
	task, err := h.taskService.GetByDisplayID(r.Context(), workspaceID, displayID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, task)
}

// Update handles PUT /api/pm/tasks/{id}.
func (h *PMTaskHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	id := chi.URLParam(r, "id")
	var req model.UpdateTaskRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	task, err := h.taskService.Update(r.Context(), id, req, userID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, task)
}

// Delete handles DELETE /api/pm/tasks/{id}.
func (h *PMTaskHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	id := chi.URLParam(r, "id")
	if err := h.taskService.Delete(r.Context(), id, userID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "task archived"})
}

// Move handles PUT /api/pm/tasks/{id}/move.
func (h *PMTaskHandler) Move(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	id := chi.URLParam(r, "id")
	var req model.MoveTaskRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	slog.InfoContext(r.Context(), "[pm-dnd] handler move request",
		"trace_id", req.DebugTraceID,
		"task_id", id,
		"actor_id", userID,
		"state_id", req.StateID,
		"position", req.Position,
	)
	task, err := h.taskService.MoveToState(r.Context(), id, req, userID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, task)
}

// Reorder handles PUT /api/pm/tasks/{id}/reorder.
func (h *PMTaskHandler) Reorder(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	id := chi.URLParam(r, "id")
	var req model.ReorderTaskRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	slog.InfoContext(r.Context(), "[pm-dnd] handler reorder request",
		"trace_id", req.DebugTraceID,
		"task_id", id,
		"actor_id", userID,
		"position", req.Position,
	)
	if err := h.taskService.Reorder(r.Context(), id, req, userID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "task reordered"})
}

// AddOwner handles POST /api/pm/tasks/{id}/owners.
func (h *PMTaskHandler) AddOwner(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	id := chi.URLParam(r, "id")
	var req model.TaskUserLinkRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.taskService.AddOwner(r.Context(), id, req.UserID, userID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, model.MessageResponse{Message: "owner added"})
}

// RemoveOwner handles DELETE /api/pm/tasks/{id}/owners/{userId}.
func (h *PMTaskHandler) RemoveOwner(w http.ResponseWriter, r *http.Request) {
	actorID := middleware.GetUserID(r.Context())
	id := chi.URLParam(r, "id")
	userID := chi.URLParam(r, "userId")
	if err := h.taskService.RemoveOwner(r.Context(), id, userID, actorID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "owner removed"})
}

// AddFollower handles POST /api/pm/tasks/{id}/followers.
func (h *PMTaskHandler) AddFollower(w http.ResponseWriter, r *http.Request) {
	actorID := middleware.GetUserID(r.Context())
	id := chi.URLParam(r, "id")
	var req model.TaskUserLinkRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.taskService.AddFollower(r.Context(), id, req.UserID, actorID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, model.MessageResponse{Message: "follower added"})
}

// RemoveFollower handles DELETE /api/pm/tasks/{id}/followers.
func (h *PMTaskHandler) RemoveFollower(w http.ResponseWriter, r *http.Request) {
	actorID := middleware.GetUserID(r.Context())
	id := chi.URLParam(r, "id")
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		userID = actorID
	}
	if err := h.taskService.RemoveFollower(r.Context(), id, userID, actorID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "follower removed"})
}

// AddLabel handles POST /api/pm/tasks/{id}/labels.
func (h *PMTaskHandler) AddLabel(w http.ResponseWriter, r *http.Request) {
	actorID := middleware.GetUserID(r.Context())
	id := chi.URLParam(r, "id")
	var req model.TaskLabelLinkRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.taskService.AddLabel(r.Context(), id, req.LabelID, actorID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, model.MessageResponse{Message: "label added"})
}

// RemoveLabel handles DELETE /api/pm/tasks/{id}/labels/{labelId}.
func (h *PMTaskHandler) RemoveLabel(w http.ResponseWriter, r *http.Request) {
	actorID := middleware.GetUserID(r.Context())
	id := chi.URLParam(r, "id")
	labelID := chi.URLParam(r, "labelId")
	if err := h.taskService.RemoveLabel(r.Context(), id, labelID, actorID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "label removed"})
}

// ListActivity handles GET /api/pm/tasks/{id}/activity.
func (h *PMTaskHandler) ListActivity(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	pagination := queryPagination(r)
	entries, total, err := h.taskService.ListActivity(r.Context(), id, pagination)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if entries == nil {
		entries = []model.ActivityLogEntry{}
	}
	totalPages := 0
	if pagination.PerPage > 0 {
		totalPages = int((total + int64(pagination.PerPage) - 1) / int64(pagination.PerPage))
	}
	writeJSON(w, http.StatusOK, model.PaginatedResponse{
		Data:       entries,
		Total:      int(total),
		Page:       pagination.Page,
		PerPage:    pagination.PerPage,
		TotalPages: totalPages,
	})
}
