package handler

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestPMTaskListFiltersIncludesSearchAndTeam(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/pm/tasks?search=HLP-42&team_id=team-a&state_type=started", nil)

	filters, err := taskListFilters(req)
	if err != nil {
		t.Fatalf("taskListFilters: %v", err)
	}
	if filters.Search == nil || *filters.Search != "HLP-42" {
		t.Fatalf("search = %v, want HLP-42", filters.Search)
	}
	if filters.TeamID == nil || *filters.TeamID != "team-a" {
		t.Fatalf("team_id = %v, want team-a", filters.TeamID)
	}
	if filters.StateType == nil || *filters.StateType != model.PMStateTypeStarted {
		t.Fatalf("state_type = %v, want started", filters.StateType)
	}
}

func TestPMTaskListFiltersRejectsInvalidStateType(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/pm/tasks?state_type=waiting", nil)

	if _, err := taskListFilters(req); err == nil {
		t.Fatal("taskListFilters should reject an invalid state_type")
	}
}

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
