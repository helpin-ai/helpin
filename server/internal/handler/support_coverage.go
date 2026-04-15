package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

// SupportCoverageHandler handles docs coverage HTTP endpoints.
type SupportCoverageHandler struct {
	coverageSvc *service.SupportCoverageService
	eventSvc    *service.SupportEventService
}

// NewSupportCoverageHandler creates a new SupportCoverageHandler.
func NewSupportCoverageHandler(
	coverageSvc *service.SupportCoverageService,
	eventSvc *service.SupportEventService,
) *SupportCoverageHandler {
	return &SupportCoverageHandler{
		coverageSvc: coverageSvc,
		eventSvc:    eventSvc,
	}
}

// GetSummary handles GET /api/support/coverage/summary.
func (h *SupportCoverageHandler) GetSummary(w http.ResponseWriter, r *http.Request) {
	wsID := middleware.GetWorkspaceID(r.Context())
	summary, err := h.coverageSvc.GetSummary(r.Context(), wsID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

// ListGaps handles GET /api/support/coverage/gaps.
func (h *SupportCoverageHandler) ListGaps(w http.ResponseWriter, r *http.Request) {
	wsID := middleware.GetWorkspaceID(r.Context())
	filter := model.SupportCoverageGapFilter{
		Status:    r.URL.Query().Get("status"),
		V1GapType: r.URL.Query().Get("v1_gap_type"),
		IssueKey:  r.URL.Query().Get("issue_key"),
		Search:    r.URL.Query().Get("search"),
	}
	gaps, total, err := h.coverageSvc.ListGaps(r.Context(), wsID, filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": gaps,
		"total": total,
	})
}

// GetGap handles GET /api/support/coverage/gaps/{gapId}.
func (h *SupportCoverageHandler) GetGap(w http.ResponseWriter, r *http.Request) {
	wsID := middleware.GetWorkspaceID(r.Context())
	gapID := chi.URLParam(r, "gapId")
	detail, err := h.coverageSvc.GetGapDetail(r.Context(), wsID, gapID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if detail == nil {
		writeError(w, http.StatusNotFound, "gap not found")
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

// UpdateGapStatus handles POST /api/support/coverage/gaps/{gapId}/status.
func (h *SupportCoverageHandler) UpdateGapStatus(w http.ResponseWriter, r *http.Request) {
	wsID := middleware.GetWorkspaceID(r.Context())
	gapID := chi.URLParam(r, "gapId")
	var req struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Status == "" {
		writeError(w, http.StatusBadRequest, "status is required")
		return
	}
	if err := h.coverageSvc.UpdateGapStatus(r.Context(), wsID, gapID, req.Status); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ReclassifyGap handles POST /api/support/coverage/gaps/{gapId}/reclassify.
func (h *SupportCoverageHandler) ReclassifyGap(w http.ResponseWriter, r *http.Request) {
	wsID := middleware.GetWorkspaceID(r.Context())
	gapID := chi.URLParam(r, "gapId")
	var req struct {
		V1GapType string `json:"v1_gap_type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.V1GapType == "" {
		writeError(w, http.StatusBadRequest, "v1_gap_type is required")
		return
	}
	if err := h.coverageSvc.ReclassifyGap(r.Context(), wsID, gapID, req.V1GapType); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// MergeGap handles POST /api/support/coverage/gaps/{gapId}/merge.
func (h *SupportCoverageHandler) MergeGap(w http.ResponseWriter, r *http.Request) {
	wsID := middleware.GetWorkspaceID(r.Context())
	gapID := chi.URLParam(r, "gapId")
	var req struct {
		TargetGapID string `json:"target_gap_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.TargetGapID == "" {
		writeError(w, http.StatusBadRequest, "target_gap_id is required")
		return
	}
	if err := h.coverageSvc.MergeGaps(r.Context(), wsID, gapID, req.TargetGapID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// CreateArticleDraftSuggestion handles POST /api/support/coverage/gaps/{gapId}/suggestions/article-draft.
// Placeholder — full implementation in Task 6 (draft generation service).
func (h *SupportCoverageHandler) CreateArticleDraftSuggestion(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "article draft generation not yet implemented")
}

// CreateArticleUpdateSuggestion handles POST /api/support/coverage/gaps/{gapId}/suggestions/article-update.
// Placeholder — full implementation in Task 6.
func (h *SupportCoverageHandler) CreateArticleUpdateSuggestion(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "article update suggestion not yet implemented")
}

// ApplySuggestion handles POST /api/support/coverage/suggestions/{suggestionId}/apply.
// Placeholder — full implementation in Task 6.
func (h *SupportCoverageHandler) ApplySuggestion(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotImplemented, "suggestion apply not yet implemented")
}

// GetConversationState handles GET /api/support/coverage/conversations/{conversationId}/state.
func (h *SupportCoverageHandler) GetConversationState(w http.ResponseWriter, r *http.Request) {
	wsID := middleware.GetWorkspaceID(r.Context())
	convID := chi.URLParam(r, "conversationId")
	state, err := h.coverageSvc.GetConversationCoverageState(r.Context(), wsID, convID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, state)
}

// SubmitDocsIssueFeedback handles POST /api/support/coverage/conversations/{conversationId}/docs-issue.
func (h *SupportCoverageHandler) SubmitDocsIssueFeedback(w http.ResponseWriter, r *http.Request) {
	wsID := middleware.GetWorkspaceID(r.Context())
	convID := chi.URLParam(r, "conversationId")
	var req struct {
		DocsIssue bool `json:"docs_issue"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if h.eventSvc != nil {
		sourceSignal := ""
		if req.DocsIssue {
			sourceSignal = model.SupportCoverageSourceAgentFeedback
		}
		_ = h.eventSvc.RecordEvent(r.Context(), service.SupportEventInput{
			WorkspaceID:    wsID,
			EventType:      model.SupportEventDocsIssueFeedback,
			ConversationID: &convID,
			SourceSignal:   sourceSignal,
			ActorType:      model.SupportEventActorAgent,
		})
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
