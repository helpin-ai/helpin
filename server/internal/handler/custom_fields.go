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

// CustomFieldHandler manages custom field definitions for one area: support
// conversations or CRM records. objectType picks the object from the request.
type CustomFieldHandler struct {
	fields     *service.CustomFieldService
	objectType func(r *http.Request) (string, bool)
}

// NewConversationFieldHandler serves the conversation fields.
func NewConversationFieldHandler(fields *service.CustomFieldService) *CustomFieldHandler {
	return &CustomFieldHandler{fields: fields, objectType: func(*http.Request) (string, bool) {
		return model.CustomFieldObjectConversation, true
	}}
}

// NewCRMFieldDefinitionHandler serves contact and company fields, chosen by
// the object_type query parameter.
func NewCRMFieldDefinitionHandler(fields *service.CustomFieldService) *CustomFieldHandler {
	return &CustomFieldHandler{fields: fields, objectType: func(r *http.Request) (string, bool) {
		objectType := strings.TrimSpace(r.URL.Query().Get("object_type"))
		return objectType, objectType == model.CustomFieldObjectCRMContact || objectType == model.CustomFieldObjectCRMCompany
	}}
}

func writeCustomFieldError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrCustomFieldInvalid):
		writeError(w, http.StatusBadRequest, strings.TrimPrefix(err.Error(), service.ErrCustomFieldInvalid.Error()+": "))
	case errors.Is(err, service.ErrCustomFieldExists):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, repository.ErrCustomFieldNotFound), errors.Is(err, service.ErrCustomFieldRecordNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "unable to save custom fields")
	}
}

func (h *CustomFieldHandler) object(w http.ResponseWriter, r *http.Request) (string, bool) {
	objectType, ok := h.objectType(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "object_type must be crm_contact or crm_company")
	}
	return objectType, ok
}

// List handles GET: fields in order, with usage when usage=true.
func (h *CustomFieldHandler) List(w http.ResponseWriter, r *http.Request) {
	objectType, ok := h.object(w, r)
	if !ok {
		return
	}
	query := r.URL.Query()
	definitions, err := h.fields.List(r.Context(), getWorkspaceID(r), objectType, query.Get("archived") == "true", query.Get("usage") == "true")
	if err != nil {
		writeCustomFieldError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, definitions)
}

// Create handles POST.
func (h *CustomFieldHandler) Create(w http.ResponseWriter, r *http.Request) {
	objectType, ok := h.object(w, r)
	if !ok {
		return
	}
	var req model.CustomFieldDefinitionRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	definition, err := h.fields.Create(r.Context(), getWorkspaceID(r), middleware.GetUserID(r.Context()), objectType, req)
	if err != nil {
		writeCustomFieldError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, definition)
}

// Update handles PUT /{fieldId}.
func (h *CustomFieldHandler) Update(w http.ResponseWriter, r *http.Request) {
	objectType, ok := h.object(w, r)
	if !ok {
		return
	}
	var req model.CustomFieldDefinitionRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	definition, err := h.fields.Update(r.Context(), getWorkspaceID(r), objectType, chi.URLParam(r, "fieldId"), req)
	if err != nil {
		writeCustomFieldError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, definition)
}

// Archive handles POST /{fieldId}/archive.
func (h *CustomFieldHandler) Archive(w http.ResponseWriter, r *http.Request) {
	h.setArchived(w, r, true)
}

// Restore handles POST /{fieldId}/restore.
func (h *CustomFieldHandler) Restore(w http.ResponseWriter, r *http.Request) {
	h.setArchived(w, r, false)
}

func (h *CustomFieldHandler) setArchived(w http.ResponseWriter, r *http.Request, archived bool) {
	objectType, ok := h.object(w, r)
	if !ok {
		return
	}
	if err := h.fields.SetArchived(r.Context(), getWorkspaceID(r), objectType, chi.URLParam(r, "fieldId"), archived); err != nil {
		writeCustomFieldError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Reorder handles POST /reorder.
func (h *CustomFieldHandler) Reorder(w http.ResponseWriter, r *http.Request) {
	objectType, ok := h.object(w, r)
	if !ok {
		return
	}
	var req model.ReorderCustomFieldDefinitionsRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	definitions, err := h.fields.Reorder(r.Context(), getWorkspaceID(r), objectType, req.IDs)
	if err != nil {
		writeCustomFieldError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, definitions)
}
