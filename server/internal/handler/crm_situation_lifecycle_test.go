package handler

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"

	f "github.com/helpin-ai/helpin/server/internal/crmsituationtest"
	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestCRMSituationLifecycleHTTPCommands(t *testing.T) {
	_, routes := situationHTTPFixture(t)
	creation := situationHTTPRequest(routes, http.MethodPost, "/situations", situationHTTPBody(t), f.Actor("member"), f.Workspace)
	if creation.Code != http.StatusCreated {
		t.Fatalf("creation: %d %s", creation.Code, creation.Body)
	}
	var situation model.CRMSituation
	if err := json.Unmarshal(creation.Body.Bytes(), &situation); err != nil {
		t.Fatal(err)
	}
	path := "/situations/" + situation.ID + "/commands"
	body := `{"command_key":"hold-1","expected_revision":1,"operation":"pause","reason":"Customer requested a hold"}`
	for _, tt := range []struct {
		name, role, body string
		want             int
	}{
		{name: "viewer", role: "viewer", body: body, want: http.StatusForbidden},
		{name: "missing revision", role: "member", body: `{"command_key":"hold-1","operation":"pause","reason":"Hold"}`, want: http.StatusBadRequest},
		{name: "null", role: "member", body: "null", want: http.StatusBadRequest},
		{name: "array", role: "member", body: "[]", want: http.StatusBadRequest},
		{name: "trailing JSON", role: "member", body: body + `{}`, want: http.StatusBadRequest},
		{name: "unknown field", role: "member", body: `{"command_key":"x","expected_revision":1,"operation":"pause","reason":"Hold","actor_member_id":"spoofed"}`, want: http.StatusBadRequest},
		{name: "nested unknown field", role: "member", body: `{"command_key":"x","expected_revision":1,"operation":"update","changes":{"owner":{"user_id":"spoofed"}}}`, want: http.StatusBadRequest},
		{name: "fake execution", role: "member", body: `{"command_key":"x","expected_revision":1,"operation":"update","changes":{"execution_status":"succeeded"}}`, want: http.StatusBadRequest},
		{name: "oversized", role: "member", body: `{"reason":"` + strings.Repeat("x", 65<<10) + `"}`, want: http.StatusBadRequest},
		{name: "pause", role: "member", body: body, want: http.StatusOK},
		{name: "identical retry", role: "member", body: body, want: http.StatusOK},
		{name: "stale update", role: "member", body: strings.Replace(body, "hold-1", "hold-2", 1), want: http.StatusConflict},
		{name: "conflicting retry", role: "member", body: strings.Replace(body, "Customer requested a hold", "Different reason", 1), want: http.StatusConflict},
	} {
		t.Run(tt.name, func(t *testing.T) {
			response := situationHTTPRequest(routes, http.MethodPost, path, tt.body, f.Actor(tt.role), f.Workspace)
			if response.Code != tt.want {
				t.Fatalf("got %d %s, want %d", response.Code, response.Body, tt.want)
			}
		})
	}
	for _, suffix := range []string{"/history", "/history?limit=1", "/history?limit=1&before_revision=2"} {
		response := situationHTTPRequest(routes, http.MethodGet, "/situations/"+situation.ID+suffix, "", f.Actor("viewer"), f.Workspace)
		if response.Code != http.StatusOK || strings.Contains(response.Body.String(), "command_key") || strings.Contains(response.Body.String(), "fingerprint") {
			t.Fatalf("history contract: %d %s", response.Code, response.Body)
		}
	}
	for _, query := range []string{"limit=0", "limit=101", "limit=bad", "before_revision=-1", "before_revision=bad", "before_revision=0"} {
		response := situationHTTPRequest(routes, http.MethodGet, "/situations/"+situation.ID+"/history?"+query, "", f.Actor("member"), f.Workspace)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("invalid history pagination: %d %s", response.Code, response.Body)
		}
	}
	for _, suffix := range []string{"/history", "/commands"} {
		method := http.MethodGet
		if suffix == "/commands" {
			method = http.MethodPost
		}
		response := situationHTTPRequest(routes, method, "/situations/"+uuid.NewString()+suffix, body, f.Actor("member"), f.Workspace)
		if response.Code != http.StatusNotFound {
			t.Fatalf("missing situation: %d %s", response.Code, response.Body)
		}
	}
}

func TestCRMSituationLifecycleHTTPNullClearsFields(t *testing.T) {
	db, routes := situationHTTPFixture(t)
	situation := f.Situation("conversion")
	if err := db.Create(&situation).Error; err != nil {
		t.Fatal(err)
	}
	body := `{"command_key":"clear","expected_revision":1,"operation":"update","changes":{"owner":{"member_id":null},"next_action_owner":{"member_id":null},"checkpoint":{"at":null},"next_step":""}}`
	response := situationHTTPRequest(routes, http.MethodPost, "/situations/"+situation.ID+"/commands", body, f.Actor("member"), f.Workspace)
	if response.Code != http.StatusOK {
		t.Fatalf("explicit clear: %d %s", response.Code, response.Body)
	}
	var result model.CRMSituationCommandResult
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Change.After.OwnerMemberID != nil || result.Change.After.NextActionOwnerMemberID != nil || result.Change.After.NextCheckpointAt != nil || result.Change.After.NextStep != "" {
		t.Fatal("explicit null/empty values did not clear work")
	}
}
