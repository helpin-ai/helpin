package handler

import (
	"github.com/helpin-ai/helpin/server/internal/deployment"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/service"
)

// AdminEmailQueueHandler serves admin endpoints for the email fallback queue.
type AdminEmailQueueHandler struct {
	emailFallbackService *service.EmailFallbackService
	emailLogRepo         *repository.SupportEmailLogRepository
	webhookRepo          *repository.SupportEmailWebhookEventRepository
	config               model.EmailDiagnosticsConfig
}

// NewAdminEmailQueueHandler creates a new AdminEmailQueueHandler.
func NewAdminEmailQueueHandler(
	emailFallbackService *service.EmailFallbackService,
	emailLogRepo *repository.SupportEmailLogRepository,
	webhookRepo *repository.SupportEmailWebhookEventRepository,
	config model.EmailDiagnosticsConfig,
) *AdminEmailQueueHandler {
	config.SupportEmailReplyDomain = strings.TrimSpace(config.SupportEmailReplyDomain)
	if config.SupportEmailReplyDomain == "" {
		config.SupportEmailReplyDomain = deployment.DefaultReplyDomain
	}
	config.SupportEmailRouteDomain = strings.TrimSpace(config.SupportEmailRouteDomain)
	if config.SupportEmailRouteDomain == "" {
		config.SupportEmailRouteDomain = deployment.DefaultRouteDomain
	}
	config.VerifiedFallbackFromEmail = strings.TrimSpace(config.VerifiedFallbackFromEmail)
	if config.VerifiedFallbackFromEmail == "" {
		config.VerifiedFallbackFromEmail = strings.TrimSpace(config.ReplyFromEmail)
	}
	config.ExpectedBrandedFromShape = "{agent_name} - {workspace_name} <sender-address>"
	config.ExpectedFallbackFromShape = config.VerifiedFallbackFromEmail
	config.ExpectedReplyToShape = "conv-{conversation_id}@" + config.SupportEmailReplyDomain
	config.OutboundFromBehavior = "use verified mailbox default sender, then workspace default sender, then active sender domain, then generated route sender; retry with verified fallback sender on Postmark sender-signature rejection while preserving Reply-To"

	return &AdminEmailQueueHandler{
		emailFallbackService: emailFallbackService,
		emailLogRepo:         emailLogRepo,
		webhookRepo:          webhookRepo,
		config:               config,
	}
}

// List handles GET /api/admin/email-queue.
func (h *AdminEmailQueueHandler) List(w http.ResponseWriter, r *http.Request) {
	if h.emailFallbackService == nil {
		writeJSON(w, http.StatusOK, map[string]any{"entries": []any{}, "total": 0})
		return
	}

	result, err := h.emailFallbackService.ListQueue(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list email queue")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// Diagnostics handles GET /api/admin/email-diagnostics.
func (h *AdminEmailQueueHandler) Diagnostics(w http.ResponseWriter, r *http.Request) {
	var queue *model.EmailQueueResponse
	if h.emailFallbackService != nil {
		result, err := h.emailFallbackService.ListQueue(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "list email queue")
			return
		}
		queue = result
	} else {
		queue = &model.EmailQueueResponse{}
	}

	var recentLogs []model.SupportEmailLog
	var logCounts []model.EmailLogCount
	if h.emailLogRepo != nil {
		logs, err := h.emailLogRepo.ListRecent(r.Context(), 50)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "list email logs")
			return
		}
		recentLogs = logs

		counts, err := h.emailLogRepo.CountByDirectionStatus(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "count email logs")
			return
		}
		logCounts = counts
	}

	var recentWebhooks []model.SupportEmailWebhookEvent
	if h.webhookRepo != nil {
		result, err := h.webhookRepo.ListPaginated(r.Context(), 1, 20, "", "")
		if err != nil {
			writeError(w, http.StatusInternalServerError, "list webhook events")
			return
		}
		recentWebhooks = result.Data
	}

	writeJSON(w, http.StatusOK, model.EmailDiagnosticsResponse{
		Config:         h.config,
		Queue:          queue,
		RecentLogs:     recentLogs,
		LogCounts:      logCounts,
		RecentWebhooks: recentWebhooks,
	})
}

// ConversationDiagnostics handles GET /api/admin/email-diagnostics/conversations/{conversationID}.
func (h *AdminEmailQueueHandler) ConversationDiagnostics(w http.ResponseWriter, r *http.Request) {
	if h.emailFallbackService == nil {
		writeError(w, http.StatusServiceUnavailable, "email fallback service unavailable")
		return
	}
	conversationID := strings.TrimSpace(chi.URLParam(r, "conversationID"))
	result, err := h.emailFallbackService.DiagnoseConversation(r.Context(), conversationID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "diagnose email fallback conversation")
		return
	}
	if result == nil {
		writeError(w, http.StatusNotFound, "conversation not found")
		return
	}
	writeJSON(w, http.StatusOK, result)
}
