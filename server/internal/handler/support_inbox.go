package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

// SupportInboxHandler handles internal support HTTP endpoints.
type SupportInboxHandler struct {
	supportService *service.SupportInboxService
	agentService   *service.AgentService
}

// NewSupportInboxHandler creates a new SupportInboxHandler.
func NewSupportInboxHandler(supportService *service.SupportInboxService, agentService *service.AgentService) *SupportInboxHandler {
	return &SupportInboxHandler{supportService: supportService, agentService: agentService}
}

// ListConversations handles GET /api/support/tickets.
func (h *SupportInboxHandler) ListConversations(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	status := r.URL.Query().Get("status")
	priority := r.URL.Query().Get("priority")
	pagination := queryPagination(r)

	conversations, total, err := h.supportService.ListConversations(r.Context(), workspaceID, status, priority, pagination)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if conversations == nil {
		conversations = []model.SupportConversation{}
	}

	totalPages := 0
	if pagination.PerPage > 0 {
		totalPages = int((total + int64(pagination.PerPage) - 1) / int64(pagination.PerPage))
	}
	writeJSON(w, http.StatusOK, model.PaginatedResponse{
		Data:       conversations,
		Total:      int(total),
		Page:       pagination.Page,
		PerPage:    pagination.PerPage,
		TotalPages: totalPages,
	})
}

// GetConversation handles GET /api/support/tickets/{id}.
func (h *SupportInboxHandler) GetConversation(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	id := chi.URLParam(r, "id")

	conversation, err := h.supportService.GetConversation(r.Context(), workspaceID, id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, conversation)
}

// CreateConversation handles POST /api/support/tickets.
func (h *SupportInboxHandler) CreateConversation(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	actorID := middleware.GetUserID(r.Context())

	var req model.CreateConversationRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.WorkspaceID = workspaceID

	conversation, err := h.supportService.CreateConversation(r.Context(), req, actorID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, conversation)
}

// UpdateConversationStatus handles PUT /api/support/tickets/{id}/status.
func (h *SupportInboxHandler) UpdateConversationStatus(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	ticketID := chi.URLParam(r, "id")
	actorID := middleware.GetUserID(r.Context())

	var req model.UpdateConversationStatusRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	conversation, err := h.supportService.UpdateConversationStatus(r.Context(), workspaceID, ticketID, req.Status, actorID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, conversation)
}

// ListConversationMessages handles GET /api/support/tickets/{id}/messages.
func (h *SupportInboxHandler) ListConversationMessages(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	ticketID := chi.URLParam(r, "id")

	messages, err := h.supportService.ListConversationMessages(r.Context(), workspaceID, ticketID, true)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if messages == nil {
		messages = []model.SupportMessage{}
	}
	writeJSON(w, http.StatusOK, messages)
}

// CreateConversationMessage handles POST /api/support/tickets/{id}/messages.
func (h *SupportInboxHandler) CreateConversationMessage(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	ticketID := chi.URLParam(r, "id")
	actorID := middleware.GetUserID(r.Context())

	var req model.CreateMessageRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	msg, err := h.supportService.CreateConversationMessage(r.Context(), workspaceID, ticketID, req, "user", &actorID, nil, nil)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, msg)
}

// LinkConversationStory handles POST /api/support/tickets/{id}/link-story.
func (h *SupportInboxHandler) LinkConversationStory(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	ticketID := chi.URLParam(r, "id")
	actorID := middleware.GetUserID(r.Context())

	var req model.LinkStoryRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.supportService.LinkConversationStory(r.Context(), workspaceID, ticketID, req.StoryID, actorID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"linked": true})
}

// AssignConversationAgent handles POST /api/support/tickets/{id}/assign-agent.
func (h *SupportInboxHandler) AssignConversationAgent(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	ticketID := chi.URLParam(r, "id")
	actorID := middleware.GetUserID(r.Context())

	var req model.AssignConversationAgentRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.supportService.AssignConversationAgent(r.Context(), workspaceID, ticketID, req.AgentID, actorID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"assigned": true})
}

// ListContactConversations handles GET /api/crm/contacts/{id}/support-tickets.
func (h *SupportInboxHandler) ListContactConversations(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	contactID := chi.URLParam(r, "id")
	pagination := queryPagination(r)

	conversations, total, err := h.supportService.ListContactConversations(r.Context(), workspaceID, contactID, pagination)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if conversations == nil {
		conversations = []model.SupportConversation{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data":  conversations,
		"total": total,
		"page":  pagination.Page,
	})
}

// RunAgent handles POST /api/support/tickets/{id}/run-agent.
func (h *SupportInboxHandler) RunAgent(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	ticketID := chi.URLParam(r, "id")
	actorID := middleware.GetUserID(r.Context())

	run, err := h.agentService.RunConversationAgent(r.Context(), workspaceID, ticketID, actorID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, run)
}

// ListCannedResponses handles GET /api/support/inbox/canned-responses.
func (h *SupportInboxHandler) ListCannedResponses(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}

	responses, err := h.supportService.ListCannedResponses(r.Context(), workspaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if responses == nil {
		responses = []model.SupportCannedResponse{}
	}
	writeJSON(w, http.StatusOK, responses)
}

// SearchCannedResponses handles GET /api/support/inbox/canned-responses/search.
func (h *SupportInboxHandler) SearchCannedResponses(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	query := r.URL.Query().Get("q")
	if query == "" {
		writeError(w, http.StatusBadRequest, "query is required")
		return
	}

	responses, err := h.supportService.SearchCannedResponses(r.Context(), workspaceID, query)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if responses == nil {
		responses = []model.SupportCannedResponse{}
	}
	writeJSON(w, http.StatusOK, responses)
}

// CreateCannedResponse handles POST /api/support/inbox/canned-responses.
func (h *SupportInboxHandler) CreateCannedResponse(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	actorID := middleware.GetUserID(r.Context())

	var req model.CannedResponseRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	response, err := h.supportService.CreateCannedResponse(r.Context(), workspaceID, req, actorID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, response)
}

// UpdateCannedResponse handles PUT /api/support/inbox/canned-responses/{id}.
func (h *SupportInboxHandler) UpdateCannedResponse(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	id := chi.URLParam(r, "id")

	var req model.CannedResponseRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	response, err := h.supportService.UpdateCannedResponse(r.Context(), workspaceID, id, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, response)
}

// DeleteCannedResponse handles DELETE /api/support/inbox/canned-responses/{id}.
func (h *SupportInboxHandler) DeleteCannedResponse(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	id := chi.URLParam(r, "id")

	if err := h.supportService.DeleteCannedResponse(r.Context(), workspaceID, id); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

// GetInstallation handles GET /api/support/inbox/installations.
func (h *SupportInboxHandler) GetInstallation(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}

	inst, settings, err := h.supportService.GetInstallation(r.Context(), workspaceID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, model.InstallationSettingsResponse{
		ID:          inst.ID,
		WorkspaceID: inst.WorkspaceID,
		WidgetKey:   inst.WidgetKey,
		Settings:    *settings,
		Active:      inst.Active,
		CreatedAt:   inst.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:   inst.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	})
}

// UpdateInstallationSettings handles PATCH /api/support/inbox/installations.
func (h *SupportInboxHandler) UpdateInstallationSettings(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}

	var req model.UpdateInstallationSettingsRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	inst, settings, err := h.supportService.UpdateInstallationSettings(r.Context(), workspaceID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, model.InstallationSettingsResponse{
		ID:          inst.ID,
		WorkspaceID: inst.WorkspaceID,
		WidgetKey:   inst.WidgetKey,
		Settings:    *settings,
		Active:      inst.Active,
		CreatedAt:   inst.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:   inst.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	})
}

// RegenerateWidgetKey handles POST /api/support/inbox/installations/regenerate-key.
func (h *SupportInboxHandler) RegenerateWidgetKey(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}

	inst, err := h.supportService.RegenerateWidgetKey(r.Context(), workspaceID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	settings := model.DefaultSupportInboxSettings()
	writeJSON(w, http.StatusOK, model.InstallationSettingsResponse{
		ID:          inst.ID,
		WorkspaceID: inst.WorkspaceID,
		WidgetKey:   inst.WidgetKey,
		Settings:    settings,
		Active:      inst.Active,
		CreatedAt:   inst.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:   inst.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	})
}

// TypingIndicator handles POST /api/support/inbox/conversations/{id}/typing.
func (h *SupportInboxHandler) TypingIndicator(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	conversationID := chi.URLParam(r, "id")

	var req model.TypingIndicatorRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	h.supportService.PublishTypingIndicator(r.Context(), workspaceID, conversationID, req.IsTyping)
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}
