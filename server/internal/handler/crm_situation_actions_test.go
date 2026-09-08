package handler

import (
	"encoding/json"
	"net/http"
	"testing"

	f "github.com/helpin-ai/helpin/server/internal/crmsituationtest"
	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestCRMSituationHTTPActionDecision(t *testing.T) {
	db, routes := situationHTTPFixture(t)
	situation := f.Situation("conversion")
	if err := db.Create(&situation).Error; err != nil {
		t.Fatal(err)
	}
	ref := model.CRMSituationReference{WorkspaceID: f.Workspace, SituationID: situation.ID, Kind: "suggestion", SourceID: f.Suggestion}
	if err := db.Create(&ref).Error; err != nil {
		t.Fatal(err)
	}
	f.Exec(t, db, "UPDATE crm_suggestions SET suggestion_type = 'follow_up' WHERE id = ?", f.Suggestion)
	detail := situationHTTPRequest(routes, http.MethodGet, "/situations/"+situation.ID, "", f.Actor("member"), f.Workspace)
	if detail.Code != http.StatusOK {
		t.Fatalf("detail: %d %s", detail.Code, detail.Body)
	}
	var item model.CRMSituationItem
	if err := json.Unmarshal(detail.Body.Bytes(), &item); err != nil {
		t.Fatal(err)
	}
	if len(item.Actions) != 1 || item.Actions[0].Revision == "" {
		t.Fatalf("missing canonical proposal revision: %#v", item)
	}
	path := "/situations/" + situation.ID + "/actions/" + f.Suggestion + "/accept"
	body := `{"revision":"` + item.Actions[0].Revision + `"}`
	for _, tt := range []struct {
		role, body string
		want       int
	}{
		{role: "viewer", body: body, want: http.StatusForbidden},
		{role: "member", body: `{"revision":"stale"}`, want: http.StatusConflict},
		{role: "member", body: body + "{}", want: http.StatusBadRequest},
		{role: "member", body: body, want: http.StatusOK},
		{role: "member", body: body, want: http.StatusConflict},
	} {
		response := situationHTTPRequest(routes, http.MethodPost, path, tt.body, f.Actor(tt.role), f.Workspace)
		if response.Code != tt.want {
			t.Fatalf("decision: %d %s, want %d", response.Code, response.Body, tt.want)
		}
		if response.Code == http.StatusOK {
			var action model.CRMSuggestion
			if err := json.Unmarshal(response.Body.Bytes(), &action); err != nil {
				t.Fatal(err)
			}
			if action.Status != "accepted" || action.ExecutionStatus != "manual_required" || action.ExecutedAt != nil {
				t.Fatalf("false execution success: %#v", action)
			}
		}
	}
}
