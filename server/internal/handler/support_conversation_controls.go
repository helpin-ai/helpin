package handler

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/service"
)

func writeConversationControlError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrSupportInvalidPriority), errors.Is(err, service.ErrSupportInvalidSnooze):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, service.ErrSupportSnoozeClosed), errors.Is(err, repository.ErrSupportMerged):
		writeError(w, http.StatusConflict, err.Error())
	case err != nil && strings.Contains(err.Error(), "not found"):
		writeError(w, http.StatusNotFound, "conversation not found")
	default:
		writeError(w, http.StatusInternalServerError, "unable to update the conversation")
	}
}

// UpdateConversationPriority handles PUT /api/support/inbox/conversations/{id}/priority.
func (h *SupportInboxHandler) UpdateConversationPriority(w http.ResponseWriter, r *http.Request) {
	var req model.UpdateConversationPriorityRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	conversation, err := h.supportService.UpdateConversationPriority(r.Context(), getWorkspaceID(r), chi.URLParam(r, "id"), req.Priority, middleware.GetUserID(r.Context()))
	if err != nil {
		writeConversationControlError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, conversation)
}

// SnoozeConversation handles POST /api/support/inbox/conversations/{id}/snooze.
func (h *SupportInboxHandler) SnoozeConversation(w http.ResponseWriter, r *http.Request) {
	var req model.SnoozeConversationRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "until must be an RFC 3339 time")
		return
	}
	conversation, err := h.supportService.SnoozeConversation(r.Context(), getWorkspaceID(r), chi.URLParam(r, "id"), req.Until, middleware.GetUserID(r.Context()))
	if err != nil {
		writeConversationControlError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, conversation)
}

// UnsnoozeConversation handles DELETE /api/support/inbox/conversations/{id}/snooze.
func (h *SupportInboxHandler) UnsnoozeConversation(w http.ResponseWriter, r *http.Request) {
	conversation, err := h.supportService.UnsnoozeConversation(r.Context(), getWorkspaceID(r), chi.URLParam(r, "id"), middleware.GetUserID(r.Context()))
	if err != nil {
		writeConversationControlError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, conversation)
}

// SupportSLAHandler manages SLA policies.
type SupportSLAHandler struct {
	sla *service.SupportSLAService
}

func NewSupportSLAHandler(sla *service.SupportSLAService) *SupportSLAHandler {
	return &SupportSLAHandler{sla: sla}
}

func writeSLAPolicyError(w http.ResponseWriter, err error) {
	var entitlementErr *service.EntitlementError
	switch {
	case errors.As(err, &entitlementErr):
		writeError(w, http.StatusPaymentRequired, entitlementErr.Error())
	case errors.Is(err, service.ErrSupportSLAPolicyInvalid):
		writeError(w, http.StatusBadRequest, strings.TrimPrefix(err.Error(), service.ErrSupportSLAPolicyInvalid.Error()+": "))
	case errors.Is(err, repository.ErrSupportSLAPolicyNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "unable to save SLA policies")
	}
}

// List handles GET /api/support/sla-policies.
func (h *SupportSLAHandler) List(w http.ResponseWriter, r *http.Request) {
	policies, err := h.sla.ListPolicies(r.Context(), getWorkspaceID(r))
	if err != nil {
		writeSLAPolicyError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, policies)
}

// Create handles POST /api/support/sla-policies.
func (h *SupportSLAHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req model.SupportSLAPolicyRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	policy, err := h.sla.CreatePolicy(r.Context(), getWorkspaceID(r), middleware.GetUserID(r.Context()), req)
	if err != nil {
		writeSLAPolicyError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, policy)
}

// Update handles PUT /api/support/sla-policies/{policyId}.
func (h *SupportSLAHandler) Update(w http.ResponseWriter, r *http.Request) {
	var req model.SupportSLAPolicyRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	policy, err := h.sla.UpdatePolicy(r.Context(), getWorkspaceID(r), chi.URLParam(r, "policyId"), req)
	if err != nil {
		writeSLAPolicyError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, policy)
}

// Delete handles DELETE /api/support/sla-policies/{policyId}.
func (h *SupportSLAHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if err := h.sla.DeletePolicy(r.Context(), getWorkspaceID(r), chi.URLParam(r, "policyId")); err != nil {
		writeSLAPolicyError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Reorder handles POST /api/support/sla-policies/reorder.
func (h *SupportSLAHandler) Reorder(w http.ResponseWriter, r *http.Request) {
	var req model.ReorderSupportSLAPoliciesRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	policies, err := h.sla.ReorderPolicies(r.Context(), getWorkspaceID(r), req.PolicyIDs)
	if err != nil {
		writeSLAPolicyError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, policies)
}

// SupportConversationActionsHandler applies macros and bulk actions.
type SupportConversationActionsHandler struct {
	actions *service.SupportConversationActionService
}

func NewSupportConversationActionsHandler(actions *service.SupportConversationActionService) *SupportConversationActionsHandler {
	return &SupportConversationActionsHandler{actions: actions}
}

func writeConversationActionsError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrSupportActionsInvalid):
		writeError(w, http.StatusBadRequest, strings.TrimPrefix(err.Error(), service.ErrSupportActionsInvalid.Error()+": "))
	case errors.Is(err, service.ErrSupportMacroWithoutActions):
		writeError(w, http.StatusBadRequest, err.Error())
	case err != nil && strings.Contains(err.Error(), "not found"):
		writeError(w, http.StatusNotFound, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "unable to apply the changes")
	}
}

// ApplyMacro handles POST /api/support/inbox/conversations/{id}/macros/{macroId}/apply.
func (h *SupportConversationActionsHandler) ApplyMacro(w http.ResponseWriter, r *http.Request) {
	if !isUUID(chi.URLParam(r, "id")) || !isUUID(chi.URLParam(r, "macroId")) {
		writeError(w, http.StatusNotFound, "conversation or shortcut not found")
		return
	}
	response, err := h.actions.ApplyMacro(r.Context(), getWorkspaceID(r), chi.URLParam(r, "id"), chi.URLParam(r, "macroId"), middleware.GetUserID(r.Context()))
	if err != nil {
		writeConversationActionsError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, response)
}

// Bulk handles POST /api/support/inbox/conversations/bulk.
func (h *SupportConversationActionsHandler) Bulk(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	var req model.SupportBulkActionRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	response, err := h.actions.ApplyBulk(r.Context(), getWorkspaceID(r), req, middleware.GetUserID(r.Context()))
	if err != nil {
		writeConversationActionsError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, response)
}

// isUUID reports whether value is a standard UUID, so malformed path IDs
// answer 404 instead of reaching the database.
func isUUID(value string) bool {
	if len(value) != 36 {
		return false
	}
	_, err := uuid.Parse(value)
	return err == nil
}
