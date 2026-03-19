package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

// SupportAIHandler handles AI-specific support endpoints.
type SupportAIHandler struct {
	aiService          *service.SupportAIService
	supportInboxSvc    *service.SupportInboxService
	knowledgeSourceSvc *service.AgentKnowledgeSourceService
}

// NewSupportAIHandler creates a new SupportAIHandler.
func NewSupportAIHandler(
	aiService *service.SupportAIService,
	supportInboxSvc *service.SupportInboxService,
	knowledgeSourceSvc *service.AgentKnowledgeSourceService,
) *SupportAIHandler {
	return &SupportAIHandler{
		aiService:          aiService,
		supportInboxSvc:    supportInboxSvc,
		knowledgeSourceSvc: knowledgeSourceSvc,
	}
}

// GetKnowledgeSources returns the knowledge sources linked to an agent.
// GET /pm/agents/{id}/knowledge-sources
func (h *SupportAIHandler) GetKnowledgeSources(w http.ResponseWriter, r *http.Request) {
	agentID := chi.URLParam(r, "id")
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}

	sources, err := h.knowledgeSourceSvc.List(r.Context(), workspaceID, agentID)
	if err != nil {
		slog.ErrorContext(r.Context(), "list knowledge sources failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list knowledge sources")
		return
	}
	writeJSON(w, http.StatusOK, sources)
}

// UpdateKnowledgeSources replaces all knowledge sources for an agent.
// PUT /pm/agents/{id}/knowledge-sources
func (h *SupportAIHandler) UpdateKnowledgeSources(w http.ResponseWriter, r *http.Request) {
	agentID := chi.URLParam(r, "id")
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}

	var req model.UpdateKnowledgeSourcesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.knowledgeSourceSvc.Set(r.Context(), workspaceID, agentID, req.SpaceIDs); err != nil {
		slog.ErrorContext(r.Context(), "update knowledge sources failed", "error", err)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	sources, err := h.knowledgeSourceSvc.List(r.Context(), workspaceID, agentID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list knowledge sources")
		return
	}
	writeJSON(w, http.StatusOK, sources)
}

// EscalateToHuman handles the widget "Talk to a human" button.
// POST /api/widget/support/{conversationId}/escalate
func (h *SupportAIHandler) EscalateToHuman(w http.ResponseWriter, r *http.Request) {
	conversationID := chi.URLParam(r, "conversationId")
	sessionToken := r.Header.Get("X-Session-Token")
	if sessionToken == "" {
		writeError(w, http.StatusUnauthorized, "session token required")
		return
	}

	session, err := h.supportInboxSvc.GetWidgetSession(r.Context(), sessionToken)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid session")
		return
	}

	if session.ConversationID == nil || *session.ConversationID != conversationID {
		writeError(w, http.StatusForbidden, "conversation not owned by session")
		return
	}

	if err := h.aiService.EscalateToHuman(r.Context(), session.WorkspaceID, conversationID, "customer_requested"); err != nil {
		slog.ErrorContext(r.Context(), "escalate to human failed", "error", err)
		writeError(w, http.StatusInternalServerError, "escalation failed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
