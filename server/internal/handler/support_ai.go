package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

// SupportAIHandler handles AI-specific support endpoints.
type SupportAIHandler struct {
	aiService          *service.SupportAIService
	supportInboxSvc    *service.SupportInboxService
	knowledgeSourceSvc *service.AgentKnowledgeSourceService
	contentSourceSvc   *service.SupportContentSourceService
	agentContentSvc    *service.AgentContentSourceService
	curatedGuidanceSvc *service.CuratedGuidanceService
}

// NewSupportAIHandler creates a new SupportAIHandler.
func NewSupportAIHandler(
	aiService *service.SupportAIService,
	supportInboxSvc *service.SupportInboxService,
	knowledgeSourceSvc *service.AgentKnowledgeSourceService,
	contentSourceSvc *service.SupportContentSourceService,
	agentContentSvc *service.AgentContentSourceService,
	curatedGuidanceSvc *service.CuratedGuidanceService,
) *SupportAIHandler {
	return &SupportAIHandler{
		aiService:          aiService,
		supportInboxSvc:    supportInboxSvc,
		knowledgeSourceSvc: knowledgeSourceSvc,
		contentSourceSvc:   contentSourceSvc,
		agentContentSvc:    agentContentSvc,
		curatedGuidanceSvc: curatedGuidanceSvc,
	}
}

// ListCuratedGuidance returns pinned answers scoped to one support agent.
func (h *SupportAIHandler) ListCuratedGuidance(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	agentID := chi.URLParam(r, "id")
	items, err := h.curatedGuidanceSvc.List(r.Context(), workspaceID, agentID)
	if err != nil {
		slog.ErrorContext(r.Context(), "list curated guidance failed", "error", err)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, items)
}

// CreateCuratedGuidance creates and indexes a pinned support answer.
func (h *SupportAIHandler) CreateCuratedGuidance(w http.ResponseWriter, r *http.Request) {
	var req model.CreateCuratedGuidanceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	item, err := h.curatedGuidanceSvc.Create(
		r.Context(),
		getWorkspaceID(r),
		chi.URLParam(r, "id"),
		middleware.GetUserID(r.Context()),
		req,
	)
	if err != nil {
		slog.ErrorContext(r.Context(), "create curated guidance failed", "error", err)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

// UpdateCuratedGuidance updates scope, content, or status and reindexes when needed.
func (h *SupportAIHandler) UpdateCuratedGuidance(w http.ResponseWriter, r *http.Request) {
	var req model.UpdateCuratedGuidanceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	item, err := h.curatedGuidanceSvc.Update(
		r.Context(),
		getWorkspaceID(r),
		chi.URLParam(r, "id"),
		chi.URLParam(r, "guidanceId"),
		req,
	)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, service.ErrCuratedGuidanceNotFound) {
			status = http.StatusNotFound
		}
		writeError(w, status, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, item)
}

// DeleteCuratedGuidance permanently removes a pinned answer.
func (h *SupportAIHandler) DeleteCuratedGuidance(w http.ResponseWriter, r *http.Request) {
	err := h.curatedGuidanceSvc.Delete(
		r.Context(),
		getWorkspaceID(r),
		chi.URLParam(r, "id"),
		chi.URLParam(r, "guidanceId"),
	)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, service.ErrCuratedGuidanceNotFound) {
			status = http.StatusNotFound
		}
		writeError(w, status, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
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

	var err error
	if len(req.Sources) > 0 {
		err = h.knowledgeSourceSvc.SetScoped(r.Context(), workspaceID, agentID, req.Sources)
	} else {
		err = h.knowledgeSourceSvc.Set(r.Context(), workspaceID, agentID, req.SpaceIDs)
	}
	if err != nil {
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

// ReindexKnowledgeSource forces a re-index of one selected help-center space.
// POST /pm/agents/{id}/knowledge-sources/{spaceId}/reindex
func (h *SupportAIHandler) ReindexKnowledgeSource(w http.ResponseWriter, r *http.Request) {
	agentID := chi.URLParam(r, "id")
	spaceID := chi.URLParam(r, "spaceId")
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}

	if err := h.knowledgeSourceSvc.Reindex(r.Context(), workspaceID, agentID, spaceID); err != nil {
		slog.ErrorContext(r.Context(), "reindex knowledge source failed", "error", err)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "queued"})
}

// ListContentSources returns all workspace content sources for support AI.
func (h *SupportAIHandler) ListContentSources(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}

	sources, err := h.contentSourceSvc.List(r.Context(), workspaceID)
	if err != nil {
		slog.ErrorContext(r.Context(), "list content sources failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list content sources")
		return
	}
	writeJSON(w, http.StatusOK, sources)
}

// CreateContentSource creates a new workspace content source and queues indexing.
func (h *SupportAIHandler) CreateContentSource(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}

	var req model.CreateSupportContentSourceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	source, err := h.contentSourceSvc.Create(r.Context(), workspaceID, req)
	if err != nil {
		slog.ErrorContext(r.Context(), "create content source failed", "error", err)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, source)
}

// CreateContentSourceFileUpload creates a file source and returns a presigned upload URL.
func (h *SupportAIHandler) CreateContentSourceFileUpload(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}

	var req model.CreateSupportContentSourceFileUploadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	resp, err := h.contentSourceSvc.CreateFileUpload(r.Context(), workspaceID, req)
	if err != nil {
		slog.ErrorContext(r.Context(), "create content source file upload failed", "error", err)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, resp)
}

// ConfirmContentSourceFileUpload queues indexing for an uploaded file source.
func (h *SupportAIHandler) ConfirmContentSourceFileUpload(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}

	source, err := h.contentSourceSvc.ConfirmFileUpload(r.Context(), workspaceID, chi.URLParam(r, "contentSourceId"))
	if err != nil {
		slog.ErrorContext(r.Context(), "confirm content source file upload failed", "error", err)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, source)
}

// UpdateContentSource updates a content source and queues re-indexing.
func (h *SupportAIHandler) UpdateContentSource(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}

	var req model.UpdateSupportContentSourceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	source, err := h.contentSourceSvc.Update(r.Context(), workspaceID, chi.URLParam(r, "contentSourceId"), req)
	if err != nil {
		slog.ErrorContext(r.Context(), "update content source failed", "error", err)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, source)
}

// DeleteContentSource removes a workspace content source and all indexed content.
func (h *SupportAIHandler) DeleteContentSource(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}

	if err := h.contentSourceSvc.Delete(r.Context(), workspaceID, chi.URLParam(r, "contentSourceId")); err != nil {
		slog.ErrorContext(r.Context(), "delete content source failed", "error", err)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ReindexContentSource forces a re-index of a content source.
func (h *SupportAIHandler) ReindexContentSource(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}

	if err := h.contentSourceSvc.Reindex(r.Context(), workspaceID, chi.URLParam(r, "contentSourceId")); err != nil {
		slog.ErrorContext(r.Context(), "reindex content source failed", "error", err)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "queued"})
}

// ListContentSourcePages returns all synced pages for a content source.
// GET /pm/content-sources/{contentSourceId}/pages
func (h *SupportAIHandler) ListContentSourcePages(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}

	contentSourceID := chi.URLParam(r, "contentSourceId")
	pages, err := h.contentSourceSvc.ListPages(r.Context(), workspaceID, contentSourceID)
	if err != nil {
		slog.ErrorContext(r.Context(), "list content source pages failed", "error", err)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, pages)
}

// GetContentSourcePage returns a single synced page including its content.
// GET /pm/content-sources/{contentSourceId}/pages/{pageId}
func (h *SupportAIHandler) GetContentSourcePage(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}

	contentSourceID := chi.URLParam(r, "contentSourceId")
	pageID := chi.URLParam(r, "pageId")
	page, err := h.contentSourceSvc.GetPage(r.Context(), workspaceID, contentSourceID, pageID)
	if err != nil {
		slog.ErrorContext(r.Context(), "get content source page failed", "error", err)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, page)
}

// GetAgentContentSources returns selected workspace content source IDs for an agent.
func (h *SupportAIHandler) GetAgentContentSources(w http.ResponseWriter, r *http.Request) {
	agentID := chi.URLParam(r, "id")
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}

	ids, err := h.agentContentSvc.ListSelectedIDs(r.Context(), workspaceID, agentID)
	if err != nil {
		slog.ErrorContext(r.Context(), "list agent content sources failed", "error", err)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, ids)
}

// UpdateAgentContentSources replaces selected content source IDs for an agent.
func (h *SupportAIHandler) UpdateAgentContentSources(w http.ResponseWriter, r *http.Request) {
	agentID := chi.URLParam(r, "id")
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}

	var req model.UpdateAgentContentSourcesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.agentContentSvc.Set(r.Context(), workspaceID, agentID, req.ContentSourceIDs); err != nil {
		slog.ErrorContext(r.Context(), "update agent content sources failed", "error", err)
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	ids, err := h.agentContentSvc.ListSelectedIDs(r.Context(), workspaceID, agentID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list agent content sources")
		return
	}
	writeJSON(w, http.StatusOK, ids)
}

// PreviewSupportReply runs the support AI planner + RAG pipeline without side effects.
// POST /api/pm/agents/{id}/support-preview
func (h *SupportAIHandler) PreviewSupportReply(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	if h.aiService == nil {
		writeError(w, http.StatusServiceUnavailable, "support ai service unavailable")
		return
	}

	var req model.SupportAIPreviewRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	resp, err := h.aiService.PreviewSupportReply(r.Context(), workspaceID, chi.URLParam(r, "id"), req)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrSupportPreviewInvalidInput):
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, service.ErrSupportPreviewAgentNotFound), errors.Is(err, service.ErrSupportPreviewConversationNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		default:
			slog.ErrorContext(r.Context(), "support ai preview failed", "error", err)
			writeError(w, http.StatusInternalServerError, "failed to preview support response")
		}
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

// RewriteSupportDraft rewrites the current support draft with a targeted AI operation.
// POST /api/support/inbox/conversations/{id}/rewrite-draft
func (h *SupportAIHandler) RewriteSupportDraft(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	if h.aiService == nil {
		writeError(w, http.StatusServiceUnavailable, "support ai service unavailable")
		return
	}

	var req model.SupportAIRewriteDraftRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	resp, err := h.aiService.RewriteSupportDraft(r.Context(), workspaceID, chi.URLParam(r, "id"), req)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrSupportRewriteInvalidInput):
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, service.ErrSupportRewriteConversationNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		default:
			slog.ErrorContext(r.Context(), "support draft rewrite failed", "error", err)
			writeError(w, http.StatusInternalServerError, "failed to rewrite support draft")
		}
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

// RewriteNewSupportDraft rewrites a support draft before the conversation exists.
// POST /api/support/inbox/rewrite-draft
func (h *SupportAIHandler) RewriteNewSupportDraft(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	if h.aiService == nil {
		writeError(w, http.StatusServiceUnavailable, "support ai service unavailable")
		return
	}

	var req model.SupportAIRewriteDraftRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	resp, err := h.aiService.RewriteSupportDraftWithoutConversation(r.Context(), workspaceID, req)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrSupportRewriteInvalidInput):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			slog.ErrorContext(r.Context(), "new support draft rewrite failed", "error", err)
			writeError(w, http.StatusInternalServerError, "failed to rewrite support draft")
		}
		return
	}

	writeJSON(w, http.StatusOK, resp)
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
