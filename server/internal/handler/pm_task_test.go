package handler

import (
	"errors"
	"net/http"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestPMTaskUpdateErrorStatusMapsForbidden(t *testing.T) {
	err := &model.ErrForbidden{Message: "you do not have access to this team's resources"}

	if got := pmTaskUpdateErrorStatus(err); got != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", got, http.StatusForbidden)
	}
}

func TestPMTaskUpdateErrorStatusDefaultsToBadRequest(t *testing.T) {
	if got := pmTaskUpdateErrorStatus(errors.New("invalid estimate")); got != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", got, http.StatusBadRequest)
	}
}
