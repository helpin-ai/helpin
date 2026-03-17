package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

// PlanningSessionHandler handles HTTP requests for planning sessions.
type PlanningSessionHandler struct {
	sessionService *service.PlanningSessionService
}

// NewPlanningSessionHandler creates a new handler.
func NewPlanningSessionHandler(sessionService *service.PlanningSessionService) *PlanningSessionHandler {
	return &PlanningSessionHandler{sessionService: sessionService}
}

// Start creates a new interactive planning session for an epic.
func (h *PlanningSessionHandler) Start(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	epicID := chi.URLParam(r, "epicId")
	actorID := middleware.GetUserID(r.Context())

	var req model.StartPlanningSessionRequest
	if err := decodeJSON(r, &req); err != nil && r.ContentLength > 0 {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	session, err := h.sessionService.StartSession(r.Context(), workspaceID, epicID, actorID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, session)
}

// Get returns a planning session by ID.
func (h *PlanningSessionHandler) Get(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	sessionID := chi.URLParam(r, "sessionId")

	session, err := h.sessionService.GetSession(r.Context(), workspaceID, sessionID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, session)
}

// GetMessages returns all messages for a planning session.
func (h *PlanningSessionHandler) GetMessages(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	sessionID := chi.URLParam(r, "sessionId")

	messages, err := h.sessionService.GetSessionMessages(r.Context(), workspaceID, sessionID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, messages)
}

// SendMessage sends a human message and triggers the agent's streaming response.
func (h *PlanningSessionHandler) SendMessage(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	sessionID := chi.URLParam(r, "sessionId")
	actorID := middleware.GetUserID(r.Context())

	var req model.SendPlanningMessageRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Content == "" {
		writeError(w, http.StatusBadRequest, "content is required")
		return
	}

	msg, err := h.sessionService.SendMessage(r.Context(), workspaceID, sessionID, actorID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, msg)
}

// Finalize triggers the agent to write the final spec and complete the session.
func (h *PlanningSessionHandler) Finalize(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	sessionID := chi.URLParam(r, "sessionId")
	actorID := middleware.GetUserID(r.Context())

	session, err := h.sessionService.FinalizeSession(r.Context(), workspaceID, sessionID, actorID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, session)
}

// Abandon cancels the planning session and resets the epic planning state.
func (h *PlanningSessionHandler) Abandon(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	sessionID := chi.URLParam(r, "sessionId")
	actorID := middleware.GetUserID(r.Context())

	if err := h.sessionService.AbandonSession(r.Context(), workspaceID, sessionID, actorID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
