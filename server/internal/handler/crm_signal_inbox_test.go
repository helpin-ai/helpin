package handler

import (
	"encoding/json"
	"net/http"
	"net/url"
	"testing"

	f "github.com/helpin-ai/helpin/server/internal/crmsituationtest"
	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestCRMSignalInboxHTTPDismissalReasons(t *testing.T) {
	for _, reason := range []string{"incorrect_evidence", "wrong_entity", "duplicate", "irrelevant", "handled", "bad_timing"} {
		t.Run(reason, func(t *testing.T) {
			_, routes := situationHTTPFixture(t)
			path := "/signal-inbox/recommendations/" + f.Suggestion
			detail := situationHTTPRequest(routes, http.MethodGet, path, "", f.Actor("member"), f.Workspace)
			var item model.CRMInboxRecommendation
			if err := json.Unmarshal(detail.Body.Bytes(), &item); err != nil {
				t.Fatal(err)
			}
			response := situationHTTPRequest(routes, http.MethodPost, path+"/dismiss", `{"revision":"`+item.Action.Revision+`","reason":"`+reason+`"}`, f.Actor("member"), f.Workspace)
			if response.Code != http.StatusOK {
				t.Fatalf("dismissal failed: %d %s", response.Code, response.Body)
			}
			var action model.CRMSuggestion
			if err := json.Unmarshal(response.Body.Bytes(), &action); err != nil {
				t.Fatal(err)
			}
			if action.Status != "dismissed" || action.DismissalReason == nil || *action.DismissalReason != reason {
				t.Fatalf("reason lost: %+v", action)
			}
		})
	}
}

func TestCRMSignalInboxHTTPValidationAndScope(t *testing.T) {
	db, routes := situationHTTPFixture(t)
	f.InboxTables(t, db)
	for _, test := range []struct {
		path string
		want int
	}{
		{path: "/signal-inbox?scope=all&state=needs_approval&sort=recommended", want: http.StatusOK},
		{path: "/signal-inbox?scope=all&sort=unsafe", want: http.StatusBadRequest},
		{path: "/signal-inbox?page=-1", want: http.StatusBadRequest},
		{path: "/signal-inbox?filter=" + url.QueryEscape(`{"logic":"and","rules":[{"field":"internal","operator":"is","value":"yes"}]}`), want: http.StatusBadRequest},
		{path: "/signal-inbox/recommendations/" + f.Suggestion, want: http.StatusOK},
		{path: "/signal-inbox/recommendations/" + f.ForeignSignal, want: http.StatusNotFound},
	} {
		response := situationHTTPRequest(routes, http.MethodGet, test.path, "", f.Actor("viewer"), f.Workspace)
		if response.Code != test.want {
			t.Fatalf("%s: %d %s", test.path, response.Code, response.Body)
		}
	}
	actor := f.Actor("viewer")
	actor.WorkspaceID = f.ForeignWorkspace
	actor.WorkspaceMemberID = f.ForeignMember
	response := situationHTTPRequest(routes, http.MethodGet, "/signal-inbox/recommendations/"+f.Suggestion, "", actor, f.ForeignWorkspace)
	if response.Code != http.StatusNotFound {
		t.Fatalf("cross-workspace detail: %d %s", response.Code, response.Body)
	}
}

func TestCRMSignalInboxHTTPDecisionUsesCanonicalRevision(t *testing.T) {
	db, routes := situationHTTPFixture(t)
	f.Exec(t, db, `UPDATE crm_suggestions SET suggestion_type = 'follow_up' WHERE id = ?`, f.Suggestion)
	path := "/signal-inbox/recommendations/" + f.Suggestion
	detail := situationHTTPRequest(routes, http.MethodGet, path, "", f.Actor("member"), f.Workspace)
	var item model.CRMInboxRecommendation
	if err := json.Unmarshal(detail.Body.Bytes(), &item); err != nil {
		t.Fatal(err)
	}
	if item.Action.Revision == "" {
		t.Fatal("missing displayed revision")
	}
	body := `{"revision":"` + item.Action.Revision + `"}`
	for _, test := range []struct {
		role, body string
		want       int
	}{
		{role: "viewer", body: body, want: http.StatusForbidden},
		{role: "member", body: `{"revision":"stale"}`, want: http.StatusConflict},
		{role: "member", body: body + `{}`, want: http.StatusBadRequest},
		{role: "member", body: `{"revision":"x","unsafe":true}`, want: http.StatusBadRequest},
		{role: "member", body: body, want: http.StatusOK},
		{role: "member", body: body, want: http.StatusConflict},
	} {
		response := situationHTTPRequest(routes, http.MethodPost, path+"/accept", test.body, f.Actor(test.role), f.Workspace)
		if response.Code != test.want {
			t.Fatalf("decision: %d %s, want %d", response.Code, response.Body, test.want)
		}
		if response.Code == http.StatusOK {
			var action model.CRMSuggestion
			if err := json.Unmarshal(response.Body.Bytes(), &action); err != nil {
				t.Fatal(err)
			}
			if action.ExecutionStatus != "manual_required" || action.ExecutedAt != nil {
				t.Fatal("approval falsely claimed execution")
			}
		}
	}
}

func TestCRMSignalInboxHTTPProjectionCannotBypassLifecycle(t *testing.T) {
	for _, decision := range []string{"accept", "dismiss"} {
		t.Run(decision, func(t *testing.T) {
			db, routes := situationHTTPFixture(t)
			f.Exec(t, db, `UPDATE crm_suggestions SET suggestion_type = 'follow_up' WHERE id = ?`, f.Suggestion)
			path := "/signal-inbox/recommendations/" + f.Suggestion
			detail := situationHTTPRequest(routes, http.MethodGet, path, "", f.Actor("member"), f.Workspace)
			var item model.CRMInboxRecommendation
			if err := json.Unmarshal(detail.Body.Bytes(), &item); err != nil {
				t.Fatal(err)
			}
			work := f.Situation("conversion")
			work.Lifecycle = "paused"
			if err := db.Create(&work).Error; err != nil {
				t.Fatal(err)
			}
			ref := model.CRMSituationReference{WorkspaceID: f.Workspace, SituationID: work.ID, Kind: "suggestion", SourceID: f.Suggestion}
			if err := db.Create(&ref).Error; err != nil {
				t.Fatal(err)
			}
			response := situationHTTPRequest(routes, http.MethodPost, path+"/"+decision, `{"revision":"`+item.Action.Revision+`","reason":"irrelevant"}`, f.Actor("member"), f.Workspace)
			if response.Code != http.StatusConflict {
				t.Fatalf("paused work bypassed: %d %s", response.Code, response.Body)
			}
		})
	}
}
