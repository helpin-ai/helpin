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

func TestCRMPlaybookAutomationPreviewHTTPIsReadOnly(t *testing.T) {
	db, routes := playbookHTTPFixture(t)
	created := situationHTTPRequest(routes, "POST", "/playbooks", playbookHTTPBody(t, routes), f.Actor("admin"), f.Workspace)
	if created.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", created.Code, created.Body)
	}
	var pb model.CRMPlaybook
	if err := json.Unmarshal(created.Body.Bytes(), &pb); err != nil {
		t.Fatal(err)
	}
	path := "/playbooks/" + pb.ID + "/automation/preview?revision=1"
	for range 2 {
		response := situationHTTPRequest(routes, "GET", path, "", f.Actor("viewer"), f.Workspace)
		if response.Code != http.StatusOK {
			t.Fatalf("preview: %d %s", response.Code, response.Body)
		}
		var preview model.CRMPlaybookAutomationPreview
		if err := json.Unmarshal(response.Body.Bytes(), &preview); err != nil {
			t.Fatal(err)
		}
		if preview.ExecutionEnabled || preview.Status != "not_connected" || preview.Scope != "draft" || len(preview.Skills) != 2 {
			t.Fatalf("preview claims a runnable connection: %#v", preview)
		}
		for _, field := range []string{"files", "data", "preset_prompt", "system_prompt", "flow_id", "agent_id"} {
			if strings.Contains(response.Body.String(), `"`+field+`":`) {
				t.Fatalf("preview exposed or invented %q", field)
			}
		}
	}
	for table, want := range map[string]int64{"crm_playbook_changes": 1, "crm_playbook_versions": 0, "crm_situations": 0} {
		var count int64
		if err := db.Table(table).Count(&count).Error; err != nil || count != want {
			t.Fatalf("preview wrote %s: count=%d want=%d err=%v", table, count, want, err)
		}
	}
}

func TestCRMPlaybookAutomationPreviewHTTPRejectsInvalidRequests(t *testing.T) {
	_, routes := playbookHTTPFixture(t)
	created := situationHTTPRequest(routes, "POST", "/playbooks", playbookHTTPBody(t, routes), f.Actor("admin"), f.Workspace)
	if created.Code != http.StatusCreated {
		t.Fatal(created.Body)
	}
	var pb model.CRMPlaybook
	if err := json.Unmarshal(created.Body.Bytes(), &pb); err != nil {
		t.Fatal(err)
	}
	base := "/playbooks/" + pb.ID + "/automation/preview"
	for _, test := range []struct {
		query  string
		status int
	}{
		{query: "", status: http.StatusBadRequest},
		{query: "?revision=0", status: http.StatusBadRequest},
		{query: "?revision=abc", status: http.StatusBadRequest},
		{query: "?revision=2", status: http.StatusConflict},
		{query: "?revision=1&version_id=bad", status: http.StatusBadRequest},
		{query: "?revision=1&version_id=" + uuid.NewString(), status: http.StatusConflict},
	} {
		response := situationHTTPRequest(routes, "GET", base+test.query, "", f.Actor("viewer"), f.Workspace)
		if response.Code != test.status {
			t.Fatalf("%s: got %d want %d: %s", test.query, response.Code, test.status, response.Body)
		}
	}
	response := situationHTTPRequest(routes, "GET", base+"?revision=1", "", nil, f.Workspace)
	if response.Code != http.StatusForbidden {
		t.Fatalf("anonymous preview: %d", response.Code)
	}
}
