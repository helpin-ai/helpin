package handler

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	f "github.com/helpin-ai/helpin/server/internal/crmsituationtest"
	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestCRMPlaybookConnectionHTTP(t *testing.T) {
	db, routes := playbookHTTPFixture(t)
	flow, agent := f.PlaybookConnectionStorage(t, db)
	create := situationHTTPRequest(routes, "POST", "/playbooks", playbookHTTPBody(t, routes), f.Actor("admin"), f.Workspace)
	if create.Code != http.StatusCreated {
		t.Fatalf("create: %s", create.Body)
	}
	var pb model.CRMPlaybook
	if err := json.Unmarshal(create.Body.Bytes(), &pb); err != nil {
		t.Fatal(err)
	}
	base := "/playbooks/" + pb.ID
	policy := situationHTTPRequest(routes, "POST", base+"/commands", `{"command_key":"publish","expected_revision":1,"operation":"publish"}`, f.Actor("admin"), f.Workspace)
	if policy.Code != http.StatusOK {
		t.Fatalf("publish policy: %s", policy.Body)
	}
	var result model.CRMPlaybookCommandResult
	if err := json.Unmarshal(policy.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	pb = result.Change.After
	selection := model.CRMPlaybookConnectionSelection{PlaybookVersionID: *pb.PublishedVersionID, ExpectedRevision: pb.Revision, FlowID: flow, AgentID: agent}
	body, err := json.Marshal(selection)
	if err != nil {
		t.Fatal(err)
	}
	preview := situationHTTPRequest(routes, "POST", base+"/automation/connection/preview", string(body), f.Actor("admin"), f.Workspace)
	if preview.Code != http.StatusOK {
		t.Fatalf("preview: %d %s", preview.Code, preview.Body)
	}
	var review model.CRMPlaybookConnectionReview
	if err := json.Unmarshal(preview.Body.Bytes(), &review); err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := db.Model(&model.CRMPlaybookConnection{}).Count(&count).Error; err != nil || count != 0 {
		t.Fatal("review wrote configuration")
	}
	req := model.PublishCRMPlaybookConnectionRequest{PlaybookVersionID: selection.PlaybookVersionID, ExpectedRevision: selection.ExpectedRevision, FlowID: selection.FlowID, AgentID: selection.AgentID, CommandKey: "approved-setup", ReviewFingerprint: review.ReviewFingerprint}
	body, err = json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	for _, role := range []string{"viewer", "member"} {
		response := situationHTTPRequest(routes, "POST", base+"/automation/connections", string(body), f.Actor(role), f.Workspace)
		if response.Code != http.StatusForbidden {
			t.Fatalf("%s publication: %d", role, response.Code)
		}
	}
	for _, suffix := range []string{`,"execution_enabled":true}`, `,"snapshot":{}}`, `,"system_prompt":"override"}`} {
		unsafe := strings.TrimSuffix(string(body), "}") + suffix
		response := situationHTTPRequest(routes, "POST", base+"/automation/connections", unsafe, f.Actor("admin"), f.Workspace)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("client supplied execution config: %d %s", response.Code, response.Body)
		}
	}
	response := situationHTTPRequest(routes, "POST", base+"/automation/connections", string(body), f.Actor("admin"), f.Workspace)
	if response.Code != http.StatusCreated {
		t.Fatalf("publish: %d %s", response.Code, response.Body)
	}
	var publication model.CRMPlaybookConnectionResult
	if err := json.Unmarshal(response.Body.Bytes(), &publication); err != nil {
		t.Fatal(err)
	}
	if publication.Connection.ExecutionEnabled || publication.Replayed {
		t.Fatal("publication claims activation or replay")
	}
	replay := situationHTTPRequest(routes, "POST", base+"/automation/connections", string(body), f.Actor("admin"), f.Workspace)
	if replay.Code != http.StatusOK || !strings.Contains(replay.Body.String(), `"replayed":true`) {
		t.Fatalf("replay: %s", replay.Body)
	}
	for _, path := range []string{base + "/automation/connections", base + "/automation/connections/" + publication.Connection.ID} {
		read := situationHTTPRequest(routes, "GET", path, "", f.Actor("admin"), f.Workspace)
		if read.Code != http.StatusOK {
			t.Fatalf("read: %d %s", read.Code, read.Body)
		}
		for _, private := range []string{"snapshot", "system_prompt", "runtime_agent", "command_fingerprint", "Workspace-reviewed"} {
			if strings.Contains(read.Body.String(), private) {
				t.Fatalf("private configuration leaked: %s", private)
			}
		}
	}
	for table, want := range map[string]int64{"crm_playbook_connections": 1, "crm_situations": 0, "crm_playbook_changes": 2, "crm_playbook_versions": 1} {
		if err := db.Table(table).Count(&count).Error; err != nil || count != want {
			t.Fatalf("unexpected %s writes: %d %v", table, count, err)
		}
	}
}
