package handler

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/helpin-ai/helpin/server/internal/middleware"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/service"
)

// SupportRequestTypeHandler manages request types.
type SupportRequestTypeHandler struct {
	types *service.SupportRequestTypeService
}

// NewSupportRequestTypeHandler creates the handler.
func NewSupportRequestTypeHandler(types *service.SupportRequestTypeService) *SupportRequestTypeHandler {
	return &SupportRequestTypeHandler{types: types}
}

func writeRequestTypeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrSupportRequestTypeInvalid):
		writeError(w, http.StatusBadRequest, strings.TrimPrefix(err.Error(), service.ErrSupportRequestTypeInvalid.Error()+": "))
	case errors.Is(err, service.ErrSupportInvalidPriority):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, repository.ErrSupportRequestTypeNameTaken), errors.Is(err, repository.ErrSupportRequestTypeInUse):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, repository.ErrSupportRequestTypeNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "unable to save request types")
	}
}

// List handles GET /support/request-types.
func (h *SupportRequestTypeHandler) List(w http.ResponseWriter, r *http.Request) {
	types, err := h.types.List(r.Context(), getWorkspaceID(r))
	if err != nil {
		writeRequestTypeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, types)
}

// Create handles POST /support/request-types.
func (h *SupportRequestTypeHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req model.SupportRequestTypeRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	requestType, err := h.types.Create(r.Context(), getWorkspaceID(r), req)
	if err != nil {
		writeRequestTypeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, requestType)
}

// Update handles PUT /support/request-types/{typeId}.
func (h *SupportRequestTypeHandler) Update(w http.ResponseWriter, r *http.Request) {
	var req model.SupportRequestTypeRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	requestType, err := h.types.Update(r.Context(), getWorkspaceID(r), chi.URLParam(r, "typeId"), req)
	if err != nil {
		writeRequestTypeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, requestType)
}

// Delete handles DELETE /support/request-types/{typeId}.
func (h *SupportRequestTypeHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if err := h.types.Delete(r.Context(), getWorkspaceID(r), chi.URLParam(r, "typeId")); err != nil {
		writeRequestTypeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Reorder handles POST /support/request-types/reorder.
func (h *SupportRequestTypeHandler) Reorder(w http.ResponseWriter, r *http.Request) {
	var req model.ReorderSupportRequestTypesRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	types, err := h.types.Reorder(r.Context(), getWorkspaceID(r), req.IDs)
	if err != nil {
		writeRequestTypeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, types)
}

// SetConversationRequestTypeRequest sets or clears a conversation's type.
type SetConversationRequestTypeRequest struct {
	RequestTypeID *string `json:"request_type_id"`
}

// SetConversationRequestType handles PUT /inbox/conversations/{id}/request-type.
func (h *SupportInboxHandler) SetConversationRequestType(w http.ResponseWriter, r *http.Request) {
	var req SetConversationRequestTypeRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	conversation, err := h.supportService.SetConversationRequestType(r.Context(), getWorkspaceID(r), chi.URLParam(r, "id"), req.RequestTypeID, middleware.GetUserID(r.Context()))
	if err != nil {
		writeConversationFieldError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, conversation)
}

// SetConversationFields handles PATCH /inbox/conversations/{id}/fields:
// field values key by key; null clears a field.
func (h *SupportInboxHandler) SetConversationFields(w http.ResponseWriter, r *http.Request) {
	var req model.SetCustomFieldValuesRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	conversation, err := h.supportService.SetConversationFields(r.Context(), getWorkspaceID(r), chi.URLParam(r, "id"), req.Values, middleware.GetUserID(r.Context()))
	if err != nil {
		writeConversationFieldError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, conversation)
}

func writeConversationFieldError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrCustomFieldInvalid), errors.Is(err, service.ErrSupportRequestTypeInvalid):
		message := strings.TrimPrefix(err.Error(), service.ErrCustomFieldInvalid.Error()+": ")
		writeError(w, http.StatusBadRequest, strings.TrimPrefix(message, service.ErrSupportRequestTypeInvalid.Error()+": "))
	case errors.Is(err, repository.ErrSupportRequestTypeNotFound):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, service.ErrSupportFieldsUnavailable):
		writeError(w, http.StatusNotImplemented, err.Error())
	default:
		writeConversationControlError(w, err)
	}
}
