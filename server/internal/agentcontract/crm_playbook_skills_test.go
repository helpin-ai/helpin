package agentcontract

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestCRMPlaybookSpecializationSelectsOnlyRelevantJob(t *testing.T) {
	for _, test := range []struct{ journey, job string }{
		{journey: "buying_intent", job: "crm_buying_intent_follow_up"},
		{journey: "sales_handoff", job: "crm_sales_success_handoff"},
		{journey: "renewal_recovery", job: "crm_renewal_risk_recovery"},
	} {
		t.Run(test.journey, func(t *testing.T) {
			before, ok := BuiltInPresetSkillBundleForPreset(model.AgentPresetCRMOperator)
			if !ok {
				t.Fatal("Beacon preset missing")
			}
			snapshot, err := CaptureCRMPlaybookSpecialization(test.journey)
			if err != nil {
				t.Fatal(err)
			}
			if snapshot.SchemaVersion != 1 || snapshot.PresetKey != model.AgentPresetCRMOperator || snapshot.Journey != test.journey {
				t.Fatalf("incorrect specialization: %#v", snapshot)
			}
			if len(snapshot.Skills) != 2 || snapshot.Skills[0].Key != "crm_record_operations" || snapshot.Skills[0].Role != "core" || snapshot.Skills[1].Key != test.job || snapshot.Skills[1].Role != "job" {
				t.Fatalf("unexpected skill selection: %#v", snapshot.Skills)
			}
			if len(snapshot.Version) != 64 || snapshot.PresetPrompt == "" {
				t.Fatal("missing frozen prompt or full digest")
			}
			if err := ValidateCRMPlaybookSpecialization(snapshot); err != nil {
				t.Fatal(err)
			}
			again, err := CaptureCRMPlaybookSpecialization(test.journey)
			if err != nil || !reflect.DeepEqual(snapshot, again) {
				t.Fatalf("capture is not deterministic: %v", err)
			}
			after, _ := BuiltInPresetSkillBundleForPreset(model.AgentPresetCRMOperator)
			if !reflect.DeepEqual(before, after) {
				t.Fatal("capturing a job changed the shared Beacon preset")
			}
			if _, globallyAvailable := GetBuiltInSkill(test.job); globallyAvailable {
				t.Fatal("Playbook-only skill leaked into ordinary Agent discovery")
			}
		})
	}
}

func TestCRMPlaybookSpecializationRejectsUnknownJourneys(t *testing.T) {
	for _, journey := range []string{"", "custom", "retention", "../../system/crm_operator", "renewal_recovery "} {
		t.Run(journey, func(t *testing.T) {
			if _, err := CaptureCRMPlaybookSpecialization(journey); err == nil {
				t.Fatal("unsupported journey selected a default job")
			}
		})
	}
}

func TestCRMPlaybookSpecializationRejectsChangedContent(t *testing.T) {
	for _, test := range []struct {
		name string
		edit func(*model.CRMPlaybookSpecialization)
	}{
		{name: "prompt", edit: func(s *model.CRMPlaybookSpecialization) { s.PresetPrompt += " changed" }},
		{name: "journey", edit: func(s *model.CRMPlaybookSpecialization) { s.Journey = "buying_intent" }},
		{name: "schema", edit: func(s *model.CRMPlaybookSpecialization) { s.SchemaVersion++ }},
		{name: "file", edit: func(s *model.CRMPlaybookSpecialization) { s.Skills[1].Files[0].Data[0] ^= 1 }},
		{name: "path", edit: func(s *model.CRMPlaybookSpecialization) { s.Skills[1].Files[0].Path = "../SKILL.md" }},
		{name: "skill version", edit: func(s *model.CRMPlaybookSpecialization) { s.Skills[1].Version = strings.Repeat("0", 64) }},
		{name: "extra skill", edit: func(s *model.CRMPlaybookSpecialization) { s.Skills = append(s.Skills, s.Skills[1]) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			snapshot, err := CaptureCRMPlaybookSpecialization("renewal_recovery")
			if err != nil {
				t.Fatal(err)
			}
			test.edit(&snapshot)
			if err := ValidateCRMPlaybookSpecialization(snapshot); err == nil {
				t.Fatal("modified snapshot accepted")
			}
		})
	}
}

func TestCRMPlaybookSpecializationRoundTripRetainsSkillFiles(t *testing.T) {
	snapshot, err := CaptureCRMPlaybookSpecialization("sales_handoff")
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	var restored model.CRMPlaybookSpecialization
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatal(err)
	}
	if err := ValidateCRMPlaybookSpecialization(restored); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(snapshot, restored) {
		t.Fatal("serialization lost frozen content")
	}
	if len(restored.Skills[1].Files) < 2 {
		t.Fatal("skill references were not captured with SKILL.md")
	}
	restored.Skills[1].Files[0].Data[0] ^= 1
	fresh, err := CaptureCRMPlaybookSpecialization("sales_handoff")
	if err != nil || !reflect.DeepEqual(snapshot, fresh) {
		t.Fatalf("caller mutation changed future snapshots: %v", err)
	}
}

func TestCRMPlaybookSpecializationValidatesFrozenResourcesNotCurrentCatalogue(t *testing.T) {
	current, err := CaptureCRMPlaybookSpecialization("renewal_recovery")
	if err != nil {
		t.Fatal(err)
	}
	frozen, err := CaptureCRMPlaybookSpecialization("renewal_recovery")
	if err != nil {
		t.Fatal(err)
	}
	// Represent an earlier, trusted published resource without changing today's
	// embedded catalogue. Validation must check its own captured files, not reload
	// the newest package and silently replace or reject the earlier guidance.
	for i := range frozen.Skills[1].Files {
		if frozen.Skills[1].Files[i].Path == "references/outcomes.md" {
			frozen.Skills[1].Files[i].Data = append(frozen.Skills[1].Files[i].Data,
				[]byte("\nEarlier reviewed recovery guidance.\n")...)
		}
	}
	frozen.Skills[1].Version, err = crmContentVersion(frozen.Skills[1].Files)
	if err != nil {
		t.Fatal(err)
	}
	frozen.Version, err = crmSpecializationVersion(frozen)
	if err != nil {
		t.Fatal(err)
	}
	if frozen.Version == current.Version || frozen.Skills[1].Version == current.Skills[1].Version {
		t.Fatal("changed referenced guidance did not change snapshot identity")
	}
	if err := ValidateCRMPlaybookSpecialization(frozen); err != nil {
		t.Fatalf("valid prior snapshot depends on current catalogue: %v", err)
	}
}
