package model

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestCRMPlaybookContextKeepsOrdinaryRunJSONUnchanged(t *testing.T) {
	payload := AgentRunInputPayload{
		Trigger: &AgentRunTriggerContext{Source: "manual"},
		Target:  &AgentRunTargetContext{TargetType: "crm_deal", TargetID: "existing-deal"},
	}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"trigger":{"source":"manual"},"target":{"target_type":"crm_deal","target_id":"existing-deal"}}`
	if string(data) != want || strings.Contains(string(data), "crm_playbook") {
		t.Fatalf("ordinary Agent run changed: %s", data)
	}
}

func TestCRMPlaybookContextRoundTripsInSharedAgentInput(t *testing.T) {
	context := &CRMPlaybookRunContext{
		SchemaVersion: 1, WorkspaceID: "workspace", PlaybookID: "playbook",
		PlaybookVersionID: "policy-version", SituationID: "signal", SituationRevision: 3,
		SpecializationVersion: "skills-version", Journey: "renewal_recovery",
		Target: AgentRunTargetContext{TargetType: "crm_deal", TargetID: "deal"},
	}
	input := AgentRunInputPayload{
		CRMPlaybook: context, Target: &context.Target,
		Trigger: &AgentRunTriggerContext{Source: "automation_rule"},
	}
	data, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	var restored AgentRunInputPayload
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(input, restored) {
		t.Fatal("shared Agent input lost Playbook context or existing trigger metadata")
	}
}
