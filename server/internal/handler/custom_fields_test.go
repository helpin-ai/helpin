package handler

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/service"
)

func TestWriteCustomFieldError(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want int
	}{
		{fmt.Errorf("%w: choose a field type", service.ErrCustomFieldInvalid), http.StatusBadRequest},
		{service.ErrCustomFieldExists, http.StatusConflict},
		{repository.ErrCustomFieldNotFound, http.StatusNotFound},
		{service.ErrCustomFieldRecordNotFound, http.StatusNotFound},
		{errors.New("database is down"), http.StatusInternalServerError},
	} {
		recorder := httptest.NewRecorder()
		writeCustomFieldError(recorder, tc.err)
		if recorder.Code != tc.want {
			t.Errorf("%v: status %d, want %d", tc.err, recorder.Code, tc.want)
		}
	}
}

func TestCRMFieldDefinitionHandlerRequiresObjectType(t *testing.T) {
	h := NewCRMFieldDefinitionHandler(nil)
	for _, target := range []string{"/crm/field-definitions", "/crm/field-definitions?object_type=conversation"} {
		recorder := httptest.NewRecorder()
		h.List(recorder, httptest.NewRequest(http.MethodGet, target, nil))
		if recorder.Code != http.StatusBadRequest {
			t.Errorf("%s: status %d, want 400", target, recorder.Code)
		}
	}
}
