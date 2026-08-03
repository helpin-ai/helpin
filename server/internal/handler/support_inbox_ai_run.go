package handler

// AI-run interaction endpoints for the support inbox: teammates review and
// resolve interactions raised by a conversation's AI chat run — most notably
// support_plan_confirm approvals for child launches that are not read-only.

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/model"
)

// resolveConversationAIRunID loads the conversation (access-checked) and
// returns its active AI chat run ID, writing the error response itself when
// there is nothing to resolve.
func (h *SupportInboxHandler) resolveConversationAIRunID(w http.ResponseWriter, r *http.Request) (string, bool) {
	conversation, err := h.supportService.GetConversation(r.Context(), getWorkspaceID(r), chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return "", false
	}
	if conversation.AIActiveRunID == nil || strings.TrimSpace(*conversation.AIActiveRunID) == "" {
		writeError(w, http.StatusNotFound, "conversation has no active AI run")
		return "", false
	}
	return strings.TrimSpace(*conversation.AIActiveRunID), true
}

// ListAIRunInteractions handles
// GET /support/inbox/conversations/{id}/ai-run/interactions.
func (h *SupportInboxHandler) ListAIRunInteractions(w http.ResponseWriter, r *http.Request) {
	runID, ok := h.resolveConversationAIRunID(w, r)
	if !ok {
		return
	}
	interactions, err := h.agentService.ListRunInteractions(r.Context(), getWorkspaceID(r), runID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"run_id": runID, "interactions": interactions})
}

// ResolveAIRunInteraction handles
// POST /support/inbox/conversations/{id}/ai-run/interactions/{interactionID}/resolve.
func (h *SupportInboxHandler) ResolveAIRunInteraction(w http.ResponseWriter, r *http.Request) {
	runID, ok := h.resolveConversationAIRunID(w, r)
	if !ok {
		return
	}
	var req model.ResolveAgentRunInteractionRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	interaction, err := h.agentService.ResolveCodingSessionInteraction(
		r.Context(), getWorkspaceID(r), runID, chi.URLParam(r, "interactionID"),
		middleware.GetUserID(r.Context()), req,
	)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, interaction)
}
