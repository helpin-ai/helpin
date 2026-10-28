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

func TestWriteSLAPolicyError(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want int
	}{
		{fmt.Errorf("%w: tier name must be 1 to 60 characters", service.ErrSupportSLAPolicyInvalid), http.StatusBadRequest},
		{repository.ErrSupportSLAPolicyNotFound, http.StatusNotFound},
		{repository.ErrSupportSLATierNotFound, http.StatusNotFound},
		{service.ErrSupportSLACompanyNotFound, http.StatusNotFound},
		{service.ErrSupportSLACustomExists, http.StatusConflict},
		{&service.EntitlementError{Message: "upgrade"}, http.StatusPaymentRequired},
		{errors.New("database is down"), http.StatusInternalServerError},
	} {
		recorder := httptest.NewRecorder()
		writeSLAPolicyError(recorder, tc.err)
		if recorder.Code != tc.want {
			t.Errorf("%v: status %d, want %d", tc.err, recorder.Code, tc.want)
		}
	}
}
