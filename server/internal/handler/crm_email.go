package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/service"
)

// CRMEmailHandler handles CRM email HTTP endpoints.
type CRMEmailHandler struct {
	emailService *service.CRMEmailService
}

// NewCRMEmailHandler creates a new CRMEmailHandler.
func NewCRMEmailHandler(emailService *service.CRMEmailService) *CRMEmailHandler {
	return &CRMEmailHandler{emailService: emailService}
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
	id := chi.URLParam(r, "id")
	account, err := h.emailService.GetAccount(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
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
	if req.WorkspaceID == "" {
		req.WorkspaceID = getWorkspaceID(r)
	}
	account, err := h.emailService.CreateAccount(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, account)
}

// DeleteAccount handles DELETE /api/crm/email/accounts/{id}.
func (h *CRMEmailHandler) DeleteAccount(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.emailService.DeleteAccount(r.Context(), id); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "email account deleted"})
}

// OAuthCallback handles POST /api/crm/email/accounts/{id}/oauth-callback (stub).
func (h *CRMEmailHandler) OAuthCallback(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	code := r.URL.Query().Get("code")
	if err := h.emailService.OAuthCallback(r.Context(), id, code); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, model.MessageResponse{Message: "OAuth callback processed"})
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
	if req.WorkspaceID == "" {
		req.WorkspaceID = getWorkspaceID(r)
	}
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
