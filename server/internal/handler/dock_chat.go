package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

// DockChatHandler serves the dock's private and shared chats backed by
// agent-runtime chat-mode runs. Run-scoped reads are proxied through the
// chat's visibility check.
type DockChatHandler struct {
	dockChatService *service.DockChatService
	agentService    *service.AgentService
}

// NewDockChatHandler creates a DockChatHandler.
func NewDockChatHandler(dockChatService *service.DockChatService, agentService *service.AgentService) *DockChatHandler {
	return &DockChatHandler{dockChatService: dockChatService, agentService: agentService}
}

// AIDefaults exposes the Ask Agent launch default to workspace members.
func (h *DockChatHandler) AIDefaults(w http.ResponseWriter, r *http.Request) {
	defaults, err := h.agentService.AskAgentDefaults(r.Context(), getWorkspaceID(r))
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to load Ask Agent default", "error", err)
		writeError(w, http.StatusInternalServerError, "Unable to load the agent's AI default")
		return
	}
	writeJSON(w, http.StatusOK, defaults)
}

// ListChats handles GET /api/dock/chats.
func (h *DockChatHandler) ListChats(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	userID := middleware.GetUserID(r.Context())
	response, err := h.dockChatService.ListChats(
		r.Context(), workspaceID, userID,
		parseIntQuery(r, "limit", 30), r.URL.Query().Get("cursor"),
	)
	if err != nil {
		if errors.Is(err, service.ErrDockChatInvalidCursor) {
			writeError(w, http.StatusBadRequest, "invalid cursor")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to list chats")
		return
	}
	writeJSON(w, http.StatusOK, response)
}

// FindSupportConversationChat handles GET
// /api/dock/chats/support-conversation?conversation_id=... without creating a
// chat for an untouched support conversation.
func (h *DockChatHandler) FindSupportConversationChat(w http.ResponseWriter, r *http.Request) {
	chat, err := h.dockChatService.FindSupportConversationChat(
		r.Context(),
		getWorkspaceID(r),
		middleware.GetUserID(r.Context()),
		r.URL.Query().Get("conversation_id"),
	)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, chat)
}

// CreateChat handles POST /api/dock/chats.
func (h *DockChatHandler) CreateChat(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	userID := middleware.GetUserID(r.Context())
	var req model.CreateDockChatRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	chat, err := h.dockChatService.CreateChat(r.Context(), workspaceID, userID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, chat)
}

// GetChat handles GET /api/dock/chats/{chatID}.
func (h *DockChatHandler) GetChat(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	userID := middleware.GetUserID(r.Context())
	detail, err := h.dockChatService.GetChat(r.Context(), workspaceID, userID, chi.URLParam(r, "chatID"))
	if err != nil {
		writeDockChatError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

// UpdateChat handles PATCH /api/dock/chats/{chatID} (rename / archive).
func (h *DockChatHandler) UpdateChat(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	userID := middleware.GetUserID(r.Context())
	var req model.UpdateDockChatRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	chat, err := h.dockChatService.UpdateChat(r.Context(), workspaceID, userID, chi.URLParam(r, "chatID"), req)
	if err != nil {
		writeDockChatError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, chat)
}

// SendMessage handles POST /api/dock/chats/{chatID}/messages.
func (h *DockChatHandler) SendMessage(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	userID := middleware.GetUserID(r.Context())
	var req model.SendDockChatMessageRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	detail, err := h.dockChatService.SendMessage(r.Context(), workspaceID, userID, chi.URLParam(r, "chatID"), req)
	if err != nil {
		writeDockChatError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

// ListMessages handles GET /api/dock/chats/{chatID}/messages and returns a
// stable persisted transcript across all backing runs.
func (h *DockChatHandler) ListMessages(w http.ResponseWriter, r *http.Request) {
	var before *int64
	if raw := strings.TrimSpace(r.URL.Query().Get("before")); raw != "" {
		value, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || value <= 0 {
			writeError(w, http.StatusBadRequest, "invalid before cursor")
			return
		}
		before = &value
	}
	response, err := h.dockChatService.ListMessages(
		r.Context(), getWorkspaceID(r), middleware.GetUserID(r.Context()), chi.URLParam(r, "chatID"), before, parseIntQuery(r, "limit", 50),
	)
	if err != nil {
		writeDockChatError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, response)
}

// GetMessageWorkDetail handles GET
// /api/dock/chats/{chatID}/messages/{messageID}/work.
func (h *DockChatHandler) GetMessageWorkDetail(w http.ResponseWriter, r *http.Request) {
	response, err := h.dockChatService.GetMessageWorkDetail(
		r.Context(),
		getWorkspaceID(r),
		middleware.GetUserID(r.Context()),
		chi.URLParam(r, "chatID"),
		chi.URLParam(r, "messageID"),
	)
	if err != nil {
		writeDockChatError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, response)
}

// GenerateTitle handles POST /api/dock/chats/{chatID}/title.
func (h *DockChatHandler) GenerateTitle(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	userID := middleware.GetUserID(r.Context())
	var req model.GenerateDockChatTitleRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	chat, err := h.dockChatService.GenerateTitle(r.Context(), workspaceID, userID, chi.URLParam(r, "chatID"), req)
	if err != nil {
		writeDockChatError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, chat)
}

// GetChatRun handles GET /api/dock/chats/{chatID}/run — the backing run's
// coding-session snapshot, proxied through the chat ownership check.
func (h *DockChatHandler) GetChatRun(w http.ResponseWriter, r *http.Request) {
	run, ok := h.resolveChatRun(w, r)
	if !ok {
		return
	}
	session, err := h.agentService.GetCodingSession(r.Context(), getWorkspaceID(r), run.ID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, session)
}

// ListChatRunEvents handles GET /api/dock/chats/{chatID}/run/events.
func (h *DockChatHandler) ListChatRunEvents(w http.ResponseWriter, r *http.Request) {
	run, ok := h.resolveChatRun(w, r)
	if !ok {
		return
	}
	events, err := h.agentService.ListCodingSessionEvents(
		r.Context(), getWorkspaceID(r), run.ID, parseIntQuery(r, "after", 0),
		service.CodingSessionEventListOptions{IncludeSnapshot: r.URL.Query().Get("include_snapshot") != "false"},
	)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, events)
}

// ListChatRunInteractions handles GET /api/dock/chats/{chatID}/run/interactions.
// The chat view uses this as the authoritative pending-interaction source so
// approval cards render even when a websocket event was missed.
func (h *DockChatHandler) ListChatRunInteractions(w http.ResponseWriter, r *http.Request) {
	run, ok := h.resolveChatRun(w, r)
	if !ok {
		return
	}
	interactions, err := h.agentService.ListRunInteractions(r.Context(), getWorkspaceID(r), run.ID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"interactions": interactions})
}

// ResolveChatRunInteraction handles
// POST /api/dock/chats/{chatID}/interactions/{interactionID}/resolve.
func (h *DockChatHandler) ResolveChatRunInteraction(w http.ResponseWriter, r *http.Request) {
	run, ok := h.resolveChatRun(w, r)
	if !ok {
		return
	}
	var req model.ResolveAgentRunInteractionRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	interaction, err := h.agentService.ResolveCodingSessionInteraction(
		r.Context(), getWorkspaceID(r), run.ID, chi.URLParam(r, "interactionID"),
		middleware.GetUserID(r.Context()), req,
	)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, interaction)
}

// CancelChatRun handles POST /api/dock/chats/{chatID}/run/cancel.
func (h *DockChatHandler) CancelChatRun(w http.ResponseWriter, r *http.Request) {
	run, ok := h.resolveChatRun(w, r)
	if !ok {
		return
	}
	cancelled, err := h.agentService.CancelRun(r.Context(), getWorkspaceID(r), run.ID, middleware.GetUserID(r.Context()))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, cancelled)
}

// ListRuns handles GET /api/dock/runs.
func (h *DockChatHandler) ListRuns(w http.ResponseWriter, r *http.Request) {
	response, err := h.agentService.ListDockRunsForActor(
		r.Context(),
		getWorkspaceID(r),
		middleware.GetUserID(r.Context()),
		parseIntQuery(r, "limit", 30),
		r.URL.Query().Get("cursor"),
	)
	if err != nil {
		if errors.Is(err, service.ErrDockRunInvalidCursor) {
			writeError(w, http.StatusBadRequest, "invalid cursor")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to list agent runs")
		return
	}
	writeJSON(w, http.StatusOK, response)
}

// GetRunSnapshot handles GET /api/dock/runs/{runID}/snapshot.
func (h *DockChatHandler) GetRunSnapshot(w http.ResponseWriter, r *http.Request) {
	run, ok := h.resolveDockRun(w, r)
	if !ok {
		return
	}
	session, err := h.agentService.GetCodingSession(r.Context(), getWorkspaceID(r), run.ID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to load agent run")
		return
	}
	writeJSON(w, http.StatusOK, session)
}

// ListRunEvents handles GET /api/dock/runs/{runID}/events.
func (h *DockChatHandler) ListRunEvents(w http.ResponseWriter, r *http.Request) {
	run, ok := h.resolveDockRun(w, r)
	if !ok {
		return
	}
	events, err := h.agentService.ListCodingSessionEvents(
		r.Context(), getWorkspaceID(r), run.ID, parseIntQuery(r, "after", 0),
		service.CodingSessionEventListOptions{IncludeSnapshot: r.URL.Query().Get("include_snapshot") != "false"},
	)
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to load agent activity")
		return
	}
	writeJSON(w, http.StatusOK, events)
}

// ListRunInteractions handles GET /api/dock/runs/{runID}/interactions.
func (h *DockChatHandler) ListRunInteractions(w http.ResponseWriter, r *http.Request) {
	run, ok := h.resolveDockRun(w, r)
	if !ok {
		return
	}
	interactions, err := h.agentService.ListRunInteractions(r.Context(), getWorkspaceID(r), run.ID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to load agent interactions")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"interactions": interactions})
}

// ResolveRunInteraction handles POST /api/dock/runs/{runID}/interactions/{interactionID}/resolve.
func (h *DockChatHandler) ResolveRunInteraction(w http.ResponseWriter, r *http.Request) {
	run, ok := h.resolveDockRun(w, r)
	if !ok {
		return
	}
	var req model.ResolveAgentRunInteractionRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	interaction, err := h.agentService.ResolveCodingSessionInteraction(
		r.Context(), getWorkspaceID(r), run.ID, chi.URLParam(r, "interactionID"),
		middleware.GetUserID(r.Context()), req,
	)
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to resolve agent interaction")
		return
	}
	writeJSON(w, http.StatusOK, interaction)
}

// SendRunMessage handles POST /api/dock/runs/{runID}/messages.
func (h *DockChatHandler) SendRunMessage(w http.ResponseWriter, r *http.Request) {
	run, ok := h.resolveDockRun(w, r)
	if !ok {
		return
	}
	var req model.SendAgentRunMessageRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	message, err := h.agentService.SendRunMessage(
		r.Context(), getWorkspaceID(r), run.ID, middleware.GetUserID(r.Context()), req,
	)
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to send message")
		return
	}
	writeJSON(w, http.StatusOK, message)
}

// ContinueRun handles POST /api/dock/runs/{runID}/continue.
func (h *DockChatHandler) ContinueRun(w http.ResponseWriter, r *http.Request) {
	run, ok := h.resolveDockRun(w, r)
	if !ok {
		return
	}
	var req model.ContinueAgentRunRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	continued, err := h.agentService.ContinueTerminalRun(
		r.Context(), getWorkspaceID(r), run.ID, middleware.GetUserID(r.Context()), req,
	)
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to continue agent run")
		return
	}
	writeJSON(w, http.StatusOK, continued)
}

// CancelRun handles POST /api/dock/runs/{runID}/cancel.
func (h *DockChatHandler) CancelRun(w http.ResponseWriter, r *http.Request) {
	run, ok := h.resolveDockRun(w, r)
	if !ok {
		return
	}
	cancelled, err := h.agentService.CancelRun(
		r.Context(), getWorkspaceID(r), run.ID, middleware.GetUserID(r.Context()),
	)
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to cancel agent run")
		return
	}
	writeJSON(w, http.StatusOK, cancelled)
}

// StartRunAuth handles POST /api/dock/runs/{runID}/auth/device-code/start.

// CancelRunAuth handles POST /api/dock/runs/{runID}/auth/device-code/cancel.

func (h *DockChatHandler) resolveChatRun(w http.ResponseWriter, r *http.Request) (*model.AgentRun, bool) {
	workspaceID := getWorkspaceID(r)
	userID := middleware.GetUserID(r.Context())
	run, err := h.dockChatService.ActiveRunForChat(r.Context(), workspaceID, userID, chi.URLParam(r, "chatID"))
	if err != nil {
		writeDockChatError(w, err)
		return nil, false
	}
	if run == nil {
		writeError(w, http.StatusNotFound, "chat has no active run")
		return nil, false
	}
	return run, true
}

func (h *DockChatHandler) resolveOwnedChatRun(w http.ResponseWriter, r *http.Request) (*model.AgentRun, bool) {
	workspaceID := getWorkspaceID(r)
	userID := middleware.GetUserID(r.Context())
	run, err := h.dockChatService.OwnedActiveRunForChat(r.Context(), workspaceID, userID, chi.URLParam(r, "chatID"))
	if err != nil {
		writeDockChatError(w, err)
		return nil, false
	}
	if run == nil {
		writeError(w, http.StatusNotFound, "chat has no active run")
		return nil, false
	}
	return run, true
}

func (h *DockChatHandler) resolveDockRun(w http.ResponseWriter, r *http.Request) (*model.AgentRun, bool) {
	run, err := h.agentService.GetDockRunForActor(
		r.Context(),
		getWorkspaceID(r),
		middleware.GetUserID(r.Context()),
		chi.URLParam(r, "runID"),
	)
	if errors.Is(err, service.ErrDockRunNotFound) {
		writeError(w, http.StatusNotFound, "agent run not found")
		return nil, false
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to load agent run")
		return nil, false
	}
	return run, true
}

func writeDockChatError(w http.ResponseWriter, err error) {
	if errors.Is(err, service.ErrDockChatNotFound) {
		writeError(w, http.StatusNotFound, "chat not found")
		return
	}
	writeError(w, http.StatusBadRequest, err.Error())
}
