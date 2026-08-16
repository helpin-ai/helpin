package handler

import (
	"log/slog"
	"net/http"
	"net/url"

	"github.com/go-chi/chi/v5"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

// CRMEmailHandler handles CRM email HTTP endpoints.
type CRMEmailHandler struct {
	emailService *service.CRMEmailService
	appBaseURL   string
}

// NewCRMEmailHandler creates a new CRMEmailHandler.
func NewCRMEmailHandler(emailService *service.CRMEmailService, appBaseURL string) *CRMEmailHandler {
	return &CRMEmailHandler{emailService: emailService, appBaseURL: appBaseURL}
}

// ListAccounts handles GET /api/crm/email/accounts.
func (h *CRMEmailHandler) ListAccounts(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	filters := model.CRMEmailAccountListFilters{
		MemberID: queryStringPtr(r, "member_id"),
		Provider: queryStringPtr(r, "provider"),
	}

	// Non-admins can only see their own accounts.
	actor := authorization.GetActor(r.Context())
	if actor != nil && !authorization.NewRBACEngine().Can(actor.Role, authorization.PermSettingsManage) {
		userID := middleware.GetUserID(r.Context())
		filters.MemberID = &userID
	}

	accounts, err := h.emailService.ListAccounts(r.Context(), workspaceID, filters)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if accounts == nil {
		accounts = []model.CRMEmailAccount{}
	}
	writeJSON(w, http.StatusOK, accounts)
}

// GetAccount handles GET /api/crm/email/accounts/{id}.
func (h *CRMEmailHandler) GetAccount(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	id := chi.URLParam(r, "id")
	account, err := h.emailService.GetAccount(r.Context(), workspaceID, id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	actor := authorization.GetActor(r.Context())
	isAdmin := actor != nil && authorization.NewRBACEngine().Can(actor.Role, authorization.PermCRMAdmin)
	if !isAdmin && account.MemberID != middleware.GetUserID(r.Context()) {
		writeError(w, http.StatusForbidden, "not authorized to view this email account")
		return
	}
	writeJSON(w, http.StatusOK, account)
}

// CreateAccount handles POST /api/crm/email/accounts.
func (h *CRMEmailHandler) CreateAccount(w http.ResponseWriter, r *http.Request) {
	var req model.CreateCRMEmailAccountRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.WorkspaceID = getWorkspaceID(r)
	req.MemberID = middleware.GetUserID(r.Context())
	account, err := h.emailService.CreateAccount(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, account)
}

// DeleteAccount handles DELETE /api/crm/email/accounts/{id}.
func (h *CRMEmailHandler) DeleteAccount(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	id := chi.URLParam(r, "id")
	userID := middleware.GetUserID(r.Context())
	actor := authorization.GetActor(r.Context())
	isAdmin := actor != nil && authorization.NewRBACEngine().Can(actor.Role, authorization.PermSettingsManage)

	if err := h.emailService.DeleteAccount(r.Context(), workspaceID, id, userID, isAdmin); err != nil {
		if err.Error() == "not authorized to disconnect this email account" {
			writeError(w, http.StatusForbidden, err.Error())
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "email account disconnected; synced history preserved"})
}

// PurgeAccountData handles DELETE /api/crm/email/accounts/{id}/data.
func (h *CRMEmailHandler) PurgeAccountData(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	id := chi.URLParam(r, "id")
	actor := authorization.GetActor(r.Context())
	isAdmin := actor != nil && authorization.NewRBACEngine().Can(actor.Role, authorization.PermCRMAdmin)

	if err := h.emailService.PurgeAccountData(r.Context(), workspaceID, id, isAdmin); err != nil {
		if err.Error() == "not authorized to purge email account data" {
			writeError(w, http.StatusForbidden, err.Error())
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "email account and synced history permanently deleted"})
}

// GetAccountDiagnostics handles GET /api/crm/email/accounts/{id}/diagnostics.
func (h *CRMEmailHandler) GetAccountDiagnostics(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	id := chi.URLParam(r, "id")
	userID := middleware.GetUserID(r.Context())
	actor := authorization.GetActor(r.Context())
	isAdmin := actor != nil && authorization.NewRBACEngine().Can(actor.Role, authorization.PermCRMAdmin)

	diagnostics, err := h.emailService.GetAccountDiagnostics(r.Context(), workspaceID, id, userID, isAdmin)
	if err != nil {
		switch err.Error() {
		case "not authorized to view email account diagnostics":
			writeError(w, http.StatusForbidden, err.Error())
		case "email account not found":
			writeError(w, http.StatusNotFound, err.Error())
		default:
			writeError(w, http.StatusBadRequest, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, diagnostics)
}

// RebuildAssociations handles POST /api/crm/email/accounts/{id}/maintenance/rebuild-associations.
func (h *CRMEmailHandler) RebuildAssociations(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	id := chi.URLParam(r, "id")
	actor := authorization.GetActor(r.Context())
	isAdmin := actor != nil && authorization.NewRBACEngine().Can(actor.Role, authorization.PermCRMAdmin)

	result, err := h.emailService.RebuildAccountAssociations(r.Context(), workspaceID, id, isAdmin)
	if err != nil {
		switch err.Error() {
		case "not authorized to rebuild email associations":
			writeError(w, http.StatusForbidden, err.Error())
		case "email account not found":
			writeError(w, http.StatusNotFound, err.Error())
		default:
			writeError(w, http.StatusBadRequest, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// OAuthCallback handles POST /api/crm/email/accounts/{id}/oauth-callback (stub).
func (h *CRMEmailHandler) OAuthCallback(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	id := chi.URLParam(r, "id")
	code := r.URL.Query().Get("code")
	if err := h.emailService.OAuthCallback(r.Context(), workspaceID, id, code); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "OAuth callback processed"})
}

// InitiateOAuth handles GET /api/crm/email/oauth/initiate.
func (h *CRMEmailHandler) InitiateOAuth(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	memberID := middleware.GetUserID(r.Context())
	if memberID == "" {
		writeError(w, http.StatusUnauthorized, "user not authenticated")
		return
	}
	provider := r.URL.Query().Get("provider")
	if provider == "" {
		provider = "gmail"
	}

	redirectURL, err := h.emailService.InitiateOAuth(r.Context(), workspaceID, memberID, provider)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"redirect_url": redirectURL,
	})
}

// OAuthCallbackRedirect handles GET /api/crm/email/oauth/callback.
func (h *CRMEmailHandler) OAuthCallbackRedirect(w http.ResponseWriter, r *http.Request) {
	state := r.URL.Query().Get("state")
	code := r.URL.Query().Get("code")
	oauthError := r.URL.Query().Get("error")

	if oauthError != "" {
		slug, err := h.emailService.CancelOAuth(r.Context(), state)
		if err != nil {
			slog.WarnContext(r.Context(), "cancel Gmail OAuth callback", "error", err, "oauth_error", oauthError)
			writeError(w, http.StatusBadRequest, "Google connection was cancelled")
			return
		}
		h.redirectToEmailSettings(w, r, slug, "cancelled")
		return
	}

	if state == "" || code == "" {
		writeError(w, http.StatusBadRequest, "state and code are required")
		return
	}

	slug, err := h.emailService.CompleteOAuth(r.Context(), state, code)
	if err != nil {
		slog.ErrorContext(r.Context(), "complete Gmail OAuth callback", "error", err)
		if cancelSlug, cancelErr := h.emailService.CancelOAuth(r.Context(), state); cancelErr == nil {
			h.redirectToEmailSettings(w, r, cancelSlug, "error")
			return
		}
		writeError(w, http.StatusBadRequest, "Google connection could not be completed")
		return
	}

	h.redirectToEmailSettings(w, r, slug, "success")
}

func (h *CRMEmailHandler) redirectToEmailSettings(w http.ResponseWriter, r *http.Request, slug, status string) {
	redirectURL := h.appBaseURL + "/w/" + url.PathEscape(slug) + "/settings/crm-email?oauth=" + url.QueryEscape(status)
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

// SyncAccount handles POST /api/crm/email/accounts/{id}/sync.
func (h *CRMEmailHandler) SyncAccount(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	id := chi.URLParam(r, "id")
	userID := middleware.GetUserID(r.Context())
	actor := authorization.GetActor(r.Context())
	isAdmin := actor != nil && authorization.NewRBACEngine().Can(actor.Role, authorization.PermCRMAdmin)
	var req struct {
		Mode string `json:"mode"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	account, err := h.emailService.SyncAccount(r.Context(), workspaceID, id, userID, req.Mode, isAdmin)
	if err != nil {
		switch err.Error() {
		case "email account not found":
			writeError(w, http.StatusNotFound, err.Error())
		case "not authorized to sync this email account":
			writeError(w, http.StatusForbidden, err.Error())
		default:
			writeError(w, http.StatusBadRequest, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusAccepted, account)
}

// SendEmail handles POST /api/crm/email/send.
func (h *CRMEmailHandler) SendEmail(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	var req struct {
		AccountID string   `json:"account_id"`
		To        []string `json:"to"`
		CC        []string `json:"cc"`
		Subject   string   `json:"subject"`
		BodyHTML  string   `json:"body_html"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.AccountID == "" || len(req.To) == 0 || req.Subject == "" {
		writeError(w, http.StatusBadRequest, "account_id, to, and subject are required")
		return
	}

	actor := authorization.GetActor(r.Context())
	isAdmin := actor != nil && authorization.NewRBACEngine().Can(actor.Role, authorization.PermCRMAdmin)
	message, err := h.emailService.SendEmail(r.Context(), workspaceID, req.AccountID, middleware.GetUserID(r.Context()), isAdmin, req.To, req.CC, req.Subject, req.BodyHTML)
	if err != nil {
		if err.Error() == "not authorized to send from this email account" {
			writeError(w, http.StatusForbidden, err.Error())
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, message)
}

// ListThreads handles GET /api/crm/email/threads.
func (h *CRMEmailHandler) ListThreads(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	filters := model.CRMEmailThreadListFilters{
		EmailAccountID: queryStringPtr(r, "email_account_id"),
		ContactID:      queryStringPtr(r, "contact_id"),
		DealID:         queryStringPtr(r, "deal_id"),
		Search:         queryStringPtr(r, "search"),
	}
	pagination := queryPagination(r)

	threads, total, err := h.emailService.ListThreads(r.Context(), workspaceID, filters, pagination)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if threads == nil {
		threads = []model.CRMEmailThread{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data":  threads,
		"total": total,
		"page":  pagination.Page,
	})
}

// ListMessages handles GET /api/crm/email/messages.
func (h *CRMEmailHandler) ListMessages(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	filters := model.CRMEmailMessageListFilters{
		ThreadID:       queryStringPtr(r, "thread_id"),
		EmailAccountID: queryStringPtr(r, "email_account_id"),
		ContactID:      queryStringPtr(r, "contact_id"),
		DealID:         queryStringPtr(r, "deal_id"),
		Direction:      queryStringPtr(r, "direction"),
	}
	pagination := queryPagination(r)

	messages, total, err := h.emailService.ListMessages(r.Context(), workspaceID, filters, pagination)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if messages == nil {
		messages = []model.CRMEmailMessage{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data":  messages,
		"total": total,
		"page":  pagination.Page,
	})
}

// CreateMessage handles POST /api/crm/email/messages.
func (h *CRMEmailHandler) CreateMessage(w http.ResponseWriter, r *http.Request) {
	var req model.CreateCRMEmailMessageRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.WorkspaceID = getWorkspaceID(r)
	message, err := h.emailService.CreateMessage(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, message)
}

// ListByContact handles GET /api/crm/contacts/{id}/emails.
func (h *CRMEmailHandler) ListByContact(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	contactID := chi.URLParam(r, "id")
	pagination := queryPagination(r)

	filters := model.CRMEmailMessageListFilters{
		ContactID: &contactID,
	}
	messages, total, err := h.emailService.ListMessages(r.Context(), workspaceID, filters, pagination)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if messages == nil {
		messages = []model.CRMEmailMessage{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data":  messages,
		"total": total,
		"page":  pagination.Page,
	})
}

// ListByDeal handles GET /api/crm/deals/{id}/emails.
func (h *CRMEmailHandler) ListByDeal(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	dealID := chi.URLParam(r, "id")
	pagination := queryPagination(r)

	filters := model.CRMEmailMessageListFilters{
		DealID: &dealID,
	}
	messages, total, err := h.emailService.ListMessages(r.Context(), workspaceID, filters, pagination)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if messages == nil {
		messages = []model.CRMEmailMessage{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data":  messages,
		"total": total,
		"page":  pagination.Page,
	})
}

// GetEmailSyncSettings handles GET /api/crm/email/sync-settings.
func (h *CRMEmailHandler) GetEmailSyncSettings(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	settings, err := h.emailService.GetEmailSyncSettings(r.Context(), workspaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

// UpdateEmailSyncSettings handles PUT /api/crm/email/sync-settings.
func (h *CRMEmailHandler) UpdateEmailSyncSettings(w http.ResponseWriter, r *http.Request) {
	workspaceID := getWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	var req model.UpdateCRMEmailSyncSettingsRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	settings, err := h.emailService.UpdateEmailSyncSettings(r.Context(), workspaceID, req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

// GetDefaultBlockedPrefixes handles GET /api/crm/email/sync-settings/default-prefixes.
func (h *CRMEmailHandler) GetDefaultBlockedPrefixes(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, model.GetDefaultBlockedRecordPrefixes())
}
