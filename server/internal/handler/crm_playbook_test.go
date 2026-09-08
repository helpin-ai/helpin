package handler

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/helpin-ai/helpin/server/internal/authorization"
	f "github.com/helpin-ai/helpin/server/internal/crmsituationtest"
	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"github.com/helpin-ai/helpin/server/internal/service"
)

func playbookHTTPFixture(t *testing.T) (*gorm.DB, http.Handler) {
	t.Helper()
	db := f.Open(t)
	authz := authorization.NewAuthzService(db, nil, nil)
	crm := service.NewCRMSituationService(repository.NewCRMSituationRepository(db), authz)
	h := NewCRMPlaybookHandler(service.NewCRMPlaybookService(repository.NewCRMPlaybookRepository(db), authz, crm))
	r := chi.NewRouter()
	r.Get("/playbooks/templates", h.Templates)
	r.Get("/playbooks", h.List)
	r.Post("/playbooks", h.Create)
	r.Get("/playbooks/{id}", h.Get)
	r.Post("/playbooks/{id}/commands", h.Command)
	r.Get("/playbooks/{id}/preview", h.Preview)
	r.Get("/playbooks/{id}/automation/preview", h.AutomationPreview)
	r.Post("/playbooks/{id}/automation/connection/preview", h.ReviewConnection)
	r.Post("/playbooks/{id}/automation/connections", h.PublishConnection)
	r.Get("/playbooks/{id}/automation/connections", h.Connections)
	r.Get("/playbooks/{id}/automation/connections/{connection_id}", h.Connection)
	r.Get("/playbooks/{id}/versions", h.Versions)
	r.Get("/playbooks/{id}/history", h.History)
	r.Get("/playbooks/{id}/participants", h.Participants)
	r.Post("/playbooks/{id}/apply", h.Apply)
	r.Post("/playbooks/{id}/participants/{situation_id}/milestones", h.AssessMilestone)
	return db, r
}

func playbookHTTPBody(t *testing.T, routes http.Handler) string {
	t.Helper()
	response := situationHTTPRequest(routes, "GET", "/playbooks/templates", "", f.Actor("viewer"), f.Workspace)
	if response.Code != http.StatusOK {
		t.Fatalf("templates: %d %s", response.Code, response.Body)
	}
	var templates []model.CRMPlaybookDefinition
	if err := json.Unmarshal(response.Body.Bytes(), &templates); err != nil {
		t.Fatal(err)
	}
	if len(templates) != 3 {
		t.Fatalf("missing journey defaults: %d", len(templates))
	}
	templates[0].Responsibilities.EscalationMemberID = f.Ptr(f.Success)
	body, err := json.Marshal(model.CreateCRMPlaybookRequest{CreationKey: uuid.NewString(), Definition: templates[0]})
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

func TestCRMPlaybookHTTPLifecycleAndIdempotency(t *testing.T) {
	_, routes := playbookHTTPFixture(t)
	body := playbookHTTPBody(t, routes)
	first := situationHTTPRequest(routes, "POST", "/playbooks", body, f.Actor("admin"), f.Workspace)
	if first.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", first.Code, first.Body)
	}
	var pb model.CRMPlaybook
	if err := json.Unmarshal(first.Body.Bytes(), &pb); err != nil {
		t.Fatal(err)
	}
	if pb.AcceptingCustomers || pb.PublishedVersionID != nil {
		t.Fatal("draft activated")
	}
	publish := `{"command_key":"publish","expected_revision":1,"operation":"publish"}`
	response := situationHTTPRequest(routes, "POST", "/playbooks/"+pb.ID+"/commands", publish, f.Actor("admin"), f.Workspace)
	if response.Code != http.StatusOK {
		t.Fatalf("publish: %d %s", response.Code, response.Body)
	}
	replay := situationHTTPRequest(routes, "POST", "/playbooks", body, f.Actor("admin"), f.Workspace)
	if replay.Code != http.StatusOK || replay.Body.String() != first.Body.String() {
		t.Fatalf("creation replay differs: %d %s", replay.Code, replay.Body)
	}
	for _, path := range []string{"/playbooks", "/playbooks/" + pb.ID, "/playbooks/" + pb.ID + "/versions", "/playbooks/" + pb.ID + "/history", "/playbooks/" + pb.ID + "/participants", "/playbooks/" + pb.ID + "/preview?revision=2"} {
		response := situationHTTPRequest(routes, "GET", path, "", f.Actor("viewer"), f.Workspace)
		if response.Code != http.StatusOK {
			t.Fatalf("%s: %d %s", path, response.Code, response.Body)
		}
		for _, secret := range []string{"creation_key", "creation_fingerprint", "command_key", "command_fingerprint"} {
			if strings.Contains(response.Body.String(), `"`+secret+`"`) {
				t.Fatalf("internal %s exposed", secret)
			}
		}
	}
	stale := situationHTTPRequest(routes, "GET", "/playbooks/"+pb.ID+"/preview?revision=1", "", f.Actor("viewer"), f.Workspace)
	if stale.Code != http.StatusConflict {
		t.Fatalf("stale preview: %d", stale.Code)
	}
}

func TestCRMPlaybookHTTPRejectsUnauthorizedAndMalformedInput(t *testing.T) {
	_, routes := playbookHTTPFixture(t)
	body := playbookHTTPBody(t, routes)
	for _, role := range []string{"viewer", "member"} {
		response := situationHTTPRequest(routes, "POST", "/playbooks", body, f.Actor(role), f.Workspace)
		if response.Code != http.StatusForbidden {
			t.Fatalf("%s create: %d", role, response.Code)
		}
	}
	for _, invalid := range []string{body + ` {}`, strings.Replace(body, `"definition":{`, `"definition":{"flow_id":"spoofed",`, 1), strings.Replace(body, `"creation_key":`, `"accepting_customers":true,"creation_key":`, 1), `null`, "{"} {
		response := situationHTTPRequest(routes, "POST", "/playbooks", invalid, f.Actor("admin"), f.Workspace)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("invalid body accepted: %d %s", response.Code, response.Body)
		}
	}
	for _, path := range []string{"/playbooks?page=0", "/playbooks?page_size=101", "/playbooks?state=banana", "/playbooks/not-an-id", "/playbooks/" + uuid.NewString() + "/preview?revision=0"} {
		response := situationHTTPRequest(routes, "GET", path, "", f.Actor("viewer"), f.Workspace)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("invalid query: %s %d", path, response.Code)
		}
	}
	for _, path := range []string{"/playbooks", "/playbooks/templates"} {
		response := situationHTTPRequest(routes, "GET", path, "", nil, f.Workspace)
		if response.Code != http.StatusForbidden {
			t.Fatalf("anonymous read: %d", response.Code)
		}
	}
}

func TestCRMPlaybookHTTPHidesStorageErrors(t *testing.T) {
	db, routes := playbookHTTPFixture(t)
	f.Exec(t, db, "ALTER TABLE crm_playbooks RENAME TO unavailable_playbooks")
	response := situationHTTPRequest(routes, "GET", "/playbooks", "", f.Actor("viewer"), f.Workspace)
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("storage error status: %d", response.Code)
	}
	for _, detail := range []string{"SELECT", "crm_playbooks", "no such table", "SQLSTATE"} {
		if strings.Contains(response.Body.String(), detail) {
			t.Fatalf("storage details exposed: %s", response.Body)
		}
	}
}
