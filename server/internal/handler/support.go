package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/d4interactive/teampulse/server/internal/middleware"
	"github.com/d4interactive/teampulse/server/internal/model"
	"github.com/d4interactive/teampulse/server/internal/service"
)

// SupportHandler handles internal support HTTP endpoints.
type SupportHandler struct {
	supportService *service.SupportService
	agentService   *service.AgentService
}

// NewSupportHandler creates a new SupportHandler.
func NewSupportHandler(supportService *service.SupportService, agentService *service.AgentService) *SupportHandler {
	return &SupportHandler{supportService: supportService, agentService: agentService}
}

// ListTickets handles GET /api/support/tickets.
func (h *SupportHandler) ListTickets(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	status := r.URL.Query().Get("status")
	priority := r.URL.Query().Get("priority")
	pagination := queryPagination(r)

	tickets, total, err := h.supportService.ListTickets(r.Context(), workspaceID, status, priority, pagination)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if tickets == nil {
		tickets = []model.SupportTicket{}
	}

	totalPages := 0
	if pagination.PerPage > 0 {
		totalPages = int((total + int64(pagination.PerPage) - 1) / int64(pagination.PerPage))
	}
	writeJSON(w, http.StatusOK, model.PaginatedResponse{
		Data:       tickets,
		Total:      int(total),
		Page:       pagination.Page,
		PerPage:    pagination.PerPage,
		TotalPages: totalPages,
	})
}

// GetTicket handles GET /api/support/tickets/{id}.
func (h *SupportHandler) GetTicket(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	id := chi.URLParam(r, "id")

	ticket, err := h.supportService.GetTicket(r.Context(), workspaceID, id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, ticket)
}

// CreateTicket handles POST /api/support/tickets.
func (h *SupportHandler) CreateTicket(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	actorID := middleware.GetUserID(r.Context())

	var req model.CreateTicketRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.WorkspaceID = workspaceID

	ticket, err := h.supportService.CreateTicket(r.Context(), req, actorID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, ticket)
}

// UpdateTicketStatus handles PUT /api/support/tickets/{id}/status.
func (h *SupportHandler) UpdateTicketStatus(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	ticketID := chi.URLParam(r, "id")
	actorID := middleware.GetUserID(r.Context())

	var req model.UpdateTicketStatusRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	ticket, err := h.supportService.UpdateTicketStatus(r.Context(), workspaceID, ticketID, req.Status, actorID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, ticket)
}

// ListMessages handles GET /api/support/tickets/{id}/messages.
func (h *SupportHandler) ListMessages(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	ticketID := chi.URLParam(r, "id")

	messages, err := h.supportService.ListMessages(r.Context(), workspaceID, ticketID, true)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if messages == nil {
		messages = []model.SupportMessage{}
	}
	writeJSON(w, http.StatusOK, messages)
}

// CreateMessage handles POST /api/support/tickets/{id}/messages.
func (h *SupportHandler) CreateMessage(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	ticketID := chi.URLParam(r, "id")
	actorID := middleware.GetUserID(r.Context())

	var req model.CreateMessageRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	msg, err := h.supportService.CreateMessage(r.Context(), workspaceID, ticketID, req, "user", &actorID, nil, nil)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, msg)
}

// LinkStory handles POST /api/support/tickets/{id}/link-story.
func (h *SupportHandler) LinkStory(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	ticketID := chi.URLParam(r, "id")
	actorID := middleware.GetUserID(r.Context())

	var req model.LinkStoryRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.supportService.LinkStory(r.Context(), workspaceID, ticketID, req.StoryID, actorID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"linked": true})
}

// AssignAgent handles POST /api/support/tickets/{id}/assign-agent.
func (h *SupportHandler) AssignAgent(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	ticketID := chi.URLParam(r, "id")
	actorID := middleware.GetUserID(r.Context())

	var req model.AssignTicketAgentRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.supportService.AssignAgent(r.Context(), workspaceID, ticketID, req.AgentID, actorID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"assigned": true})
}

// RunAgent handles POST /api/support/tickets/{id}/run-agent.
func (h *SupportHandler) RunAgent(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	ticketID := chi.URLParam(r, "id")
	actorID := middleware.GetUserID(r.Context())

	run, err := h.agentService.RunTicketAgent(r.Context(), workspaceID, ticketID, actorID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, run)
}
