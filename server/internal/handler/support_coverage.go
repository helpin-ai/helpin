package handler

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

// SupportCoverageHandler handles docs coverage HTTP endpoints.
type SupportCoverageHandler struct {
	coverageSvc *service.SupportCoverageService
	eventSvc    *service.SupportEventService
	draftSvc    *service.SupportCoverageDraftService
	debounceMu  sync.Mutex
	debounce    map[string]time.Time
}

// NewSupportCoverageHandler creates a new SupportCoverageHandler.
func NewSupportCoverageHandler(
	coverageSvc *service.SupportCoverageService,
	eventSvc *service.SupportEventService,
	draftSvc *service.SupportCoverageDraftService,
) *SupportCoverageHandler {
	return &SupportCoverageHandler{
		coverageSvc: coverageSvc,
		eventSvc:    eventSvc,
		draftSvc:    draftSvc,
		debounce:    map[string]time.Time{},
	}
}

// RecordEvent handles POST /api/support/coverage/events.
// Accepts a raw support event for gap detection. Useful for testing
// and external integrations.
func (h *SupportCoverageHandler) RecordEvent(w http.ResponseWriter, r *http.Request) {
	wsID := middleware.GetWorkspaceID(r.Context())
	var req struct {
		EventType       string  `json:"event_type"`
		ConversationID  *string `json:"conversation_id"`
		MessageID       *string `json:"message_id"`
		WidgetSessionID *string `json:"widget_session_id"`
		DocumentID      *string `json:"document_id"`
		ArticlePublicID *string `json:"article_public_id"`
		IssueKey        string  `json:"issue_key"`
		IssueSummary    string  `json:"issue_summary"`
		FailureMode     string  `json:"failure_mode"`
		SourceSignal    string  `json:"source_signal"`
		CanAnswer       string  `json:"can_answer"`
		CanResolve      string  `json:"can_resolve"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.EventType == "" {
		writeError(w, http.StatusBadRequest, "event_type is required")
		return
	}

	err := h.eventSvc.RecordEvent(r.Context(), service.SupportEventInput{
		WorkspaceID:     wsID,
		EventType:       req.EventType,
		ConversationID:  req.ConversationID,
		MessageID:       req.MessageID,
		WidgetSessionID: req.WidgetSessionID,
		DocumentID:      req.DocumentID,
		ArticlePublicID: req.ArticlePublicID,
		IssueKey:        req.IssueKey,
		IssueSummary:    req.IssueSummary,
		FailureMode:     req.FailureMode,
		SourceSignal:    req.SourceSignal,
		CanAnswer:       req.CanAnswer,
		CanResolve:      req.CanResolve,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
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
		ShowRaw:   r.URL.Query().Get("show_raw") == "true",
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

// RegenerateGap handles POST /api/support/coverage/gaps/{gapId}/regenerate.
func (h *SupportCoverageHandler) RegenerateGap(w http.ResponseWriter, r *http.Request) {
	wsID := middleware.GetWorkspaceID(r.Context())
	userID := middleware.GetUserID(r.Context())
	gapID := chi.URLParam(r, "gapId")
	if wsID == "" || userID == "" {
		writeError(w, http.StatusUnauthorized, "missing workspace or user context")
		return
	}
	if !h.allowRegenerate(userID, gapID, 30*time.Second) {
		writeError(w, http.StatusTooManyRequests, "regenerating too frequently")
		return
	}
	if err := h.coverageSvc.RegenerateGap(r.Context(), wsID, gapID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "queued"})
}

func (h *SupportCoverageHandler) allowRegenerate(userID, gapID string, window time.Duration) bool {
	key := userID + ":" + gapID
	now := time.Now()
	h.debounceMu.Lock()
	defer h.debounceMu.Unlock()
	if last, ok := h.debounce[key]; ok && now.Sub(last) < window {
		return false
	}
	h.debounce[key] = now
	return true
}

// UpdateGapStatus handles POST /api/support/coverage/gaps/{gapId}/status.
func (h *SupportCoverageHandler) UpdateGapStatus(w http.ResponseWriter, r *http.Request) {
	wsID := middleware.GetWorkspaceID(r.Context())
	gapID := chi.URLParam(r, "gapId")
	userID := middleware.GetUserID(r.Context())
	var req struct {
		Status          string  `json:"status"`
		IssueResolved   *bool   `json:"issue_resolved"`
		RejectionReason *string `json:"rejection_reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Status == "" {
		writeError(w, http.StatusBadRequest, "status is required")
		return
	}
	var err error
	if req.Status == model.SupportCoverageGapStatusRejected {
		err = h.coverageSvc.RejectGap(r.Context(), wsID, gapID, userID, req.RejectionReason)
	} else {
		err = h.coverageSvc.UpdateGapStatus(r.Context(), wsID, gapID, req.Status, userID, req.IssueResolved)
	}
	if service.IsGapResolutionConflict(err) {
		writeError(w, http.StatusConflict, "gap is no longer open")
		return
	}
	if err != nil {
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

// AddDocumentToGap handles POST /api/support/coverage/gaps/{gapId}/add.
func (h *SupportCoverageHandler) AddDocumentToGap(w http.ResponseWriter, r *http.Request) {
	wsID := middleware.GetWorkspaceID(r.Context())
	gapID := chi.URLParam(r, "gapId")
	var req struct {
		Route            string `json:"route"`
		TargetDocumentID string `json:"target_document_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Route != "" && req.Route != model.SupportCoverageSuggestionUpdateArticle {
		writeError(w, http.StatusBadRequest, "unsupported route")
		return
	}
	if req.TargetDocumentID == "" {
		writeError(w, http.StatusBadRequest, "target_document_id is required")
		return
	}
	err := h.coverageSvc.AddDocumentToGap(r.Context(), wsID, gapID, req.TargetDocumentID)
	if service.IsGapResolutionConflict(err) {
		writeError(w, http.StatusConflict, "gap is no longer open")
		return
	}
	if err != nil {
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
func (h *SupportCoverageHandler) CreateArticleDraftSuggestion(w http.ResponseWriter, r *http.Request) {
	if h.draftSvc == nil {
		writeError(w, http.StatusNotImplemented, "draft service not configured")
		return
	}
	wsID := middleware.GetWorkspaceID(r.Context())
	gapID := chi.URLParam(r, "gapId")
	var req struct {
		TargetSpaceID      string  `json:"target_space_id"`
		TargetCollectionID *string `json:"target_collection_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.TargetSpaceID == "" {
		writeError(w, http.StatusBadRequest, "target_space_id is required")
		return
	}
	suggestion, err := h.draftSvc.GenerateArticleDraft(r.Context(), wsID, gapID, req.TargetSpaceID, req.TargetCollectionID)
	if err != nil {
		if service.IsGapResolutionConflict(err) {
			writeError(w, http.StatusConflict, "gap is no longer open")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, suggestion)
}

// CreateArticleUpdateSuggestion handles POST /api/support/coverage/gaps/{gapId}/suggestions/article-update.
func (h *SupportCoverageHandler) CreateArticleUpdateSuggestion(w http.ResponseWriter, r *http.Request) {
	if h.draftSvc == nil {
		writeError(w, http.StatusNotImplemented, "draft service not configured")
		return
	}
	wsID := middleware.GetWorkspaceID(r.Context())
	gapID := chi.URLParam(r, "gapId")
	var req struct {
		TargetDocumentID string `json:"target_document_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.TargetDocumentID == "" {
		writeError(w, http.StatusBadRequest, "target_document_id is required")
		return
	}
	suggestion, err := h.draftSvc.GenerateArticleUpdate(r.Context(), wsID, gapID, req.TargetDocumentID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, suggestion)
}

// ApplySuggestion handles POST /api/support/coverage/suggestions/{suggestionId}/apply.
func (h *SupportCoverageHandler) ApplySuggestion(w http.ResponseWriter, r *http.Request) {
	if h.draftSvc == nil {
		writeError(w, http.StatusNotImplemented, "draft service not configured")
		return
	}
	wsID := middleware.GetWorkspaceID(r.Context())
	suggestionID := chi.URLParam(r, "suggestionId")
	userID := middleware.GetUserID(r.Context())
	var req struct {
		Route            string `json:"route"`
		SuggestionType   string `json:"suggestion_type"`
		TargetDocumentID string `json:"target_document_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	overrideType := req.Route
	if overrideType == "" {
		overrideType = req.SuggestionType
	}

	var err error
	if overrideType != "" || req.TargetDocumentID != "" {
		err = h.draftSvc.ApplySuggestionWithOverride(r.Context(), wsID, suggestionID, userID, overrideType, req.TargetDocumentID)
	} else {
		err = h.draftSvc.ApplySuggestion(r.Context(), wsID, suggestionID, userID)
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// DiscardSuggestion handles POST /api/support/coverage/suggestions/{suggestionId}/discard.
// Rejects the suggestion and reverts the gap to open.
func (h *SupportCoverageHandler) DiscardSuggestion(w http.ResponseWriter, r *http.Request) {
	wsID := middleware.GetWorkspaceID(r.Context())
	suggestionID := chi.URLParam(r, "suggestionId")
	if err := h.coverageSvc.DiscardSuggestion(r.Context(), wsID, suggestionID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
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

// TriggerReanalysis starts a coverage reanalysis workflow for the workspace.
func (h *SupportCoverageHandler) TriggerReanalysis(w http.ResponseWriter, r *http.Request) {
	wsID := middleware.GetWorkspaceID(r.Context())
	if wsID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	if err := h.coverageSvc.TriggerReanalysis(r.Context(), wsID); err != nil {
		if errors.Is(err, service.ErrReanalysisAlreadyRunning) {
			writeError(w, http.StatusConflict, "reanalysis already in progress")
			return
		}
		slog.ErrorContext(r.Context(), "trigger coverage reanalysis", "error", err, "workspace_id", wsID)
		writeError(w, http.StatusInternalServerError, "failed to start reanalysis")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "started"})
}
