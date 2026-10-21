package service

import (
	"context"
	"encoding/json"
	"github.com/helpin-ai/helpin/server/internal/authorization"
	"github.com/helpin-ai/helpin/server/internal/commandtools"
	"github.com/helpin-ai/helpin/server/internal/model"
	"strings"
	"testing"
)

func TestSkillManagementPagedRead(t *testing.T) {
	s := NewInternalCommandService(&AgentService{}, nil, nil, nil, nil, nil, nil, nil)
	meta := model.InternalCommandContext{WorkspaceID: "ws", ActorID: "user", ActorRole: "admin"}
	catalog, _ := s.agentService.ListSkillCatalog(context.Background(), "ws")
	var original model.SkillCatalogEntry
	for _, entry := range catalog.Skills {
		if entry.Key == "support_triage_response" {
			original = entry
		}
	}
	var reconstructed string
	offset := 0
	for {
		input, _ := json.Marshal(map[string]any{"key": original.Key, "offset": offset, "limit": 1000, "expected_version": original.VersionKey})
		out, err := s.Execute(context.Background(), meta, "skills.list_workspace", input)
		if err != nil {
			t.Fatal(err)
		}
		var page struct {
			Version      string `json:"version_key"`
			Instructions string `json:"instructions"`
			Next         *int   `json:"next_offset"`
			Complete     bool   `json:"complete"`
		}
		if err = json.Unmarshal(out, &page); err != nil {
			t.Fatal(err)
		}
		if len([]rune(page.Instructions)) > 1000 || strings.Contains(string(out), "default_content") {
			t.Fatal("read is unbounded or duplicates defaults")
		}
		if page.Version != original.VersionKey {
			t.Fatal("missing version")
		}
		reconstructed += page.Instructions
		if page.Complete {
			break
		}
		if page.Next == nil || *page.Next <= offset {
			t.Fatal("missing continuation")
		}
		offset = *page.Next
	}
	if reconstructed != original.Instructions {
		t.Fatal("paged read lost instructions")
	}
}
func TestSkillManagementRejectsInvalidRead(t *testing.T) {
	s := NewInternalCommandService(&AgentService{}, nil, nil, nil, nil, nil, nil, nil)
	for _, input := range []string{`{"key":"support_triage_response","name":"ignored"}`, `{"key":"support_triage_response","offset":1}`, `{"key":"support_triage_response","offset":1,"expected_version":"stale"}`, `{"offset":-1}`} {
		if _, err := s.Execute(context.Background(), model.InternalCommandContext{WorkspaceID: "ws"}, "skills.list_workspace", json.RawMessage(input)); err == nil {
			t.Errorf("accepted %s", input)
		}
	}
}
func TestSkillManagementValidateEdit(t *testing.T) {
	s := NewInternalCommandService(&AgentService{}, nil, nil, nil, nil, nil, nil, nil)
	meta := model.InternalCommandContext{WorkspaceID: "ws", ActorID: "user", ActorRole: "admin"}
	catalog, _ := s.agentService.ListSkillCatalog(context.Background(), "ws")
	skill := catalog.Skills[0]
	for _, tc := range []struct {
		name   string
		fields map[string]any
		valid  bool
	}{
		{"missing version", map[string]any{"skill_id": skill.Key, "title": "New title"}, false},
		{"probe version", map[string]any{"skill_id": skill.Key, "expected_version": "probe", "instructions": "probe"}, false},
		{"no changes", map[string]any{"skill_id": skill.Key, "expected_version": skill.VersionKey, "title": skill.Title}, false},
		{"empty instructions", map[string]any{"skill_id": skill.Key, "expected_version": skill.VersionKey, "instructions": " "}, false},
		{"actual edit", map[string]any{"skill_id": skill.Key, "expected_version": skill.VersionKey, "title": "New title"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input, _ := json.Marshal(tc.fields)
			_, err := s.Execute(context.Background(), meta, "skills.validate_update", input)
			if (err == nil) != tc.valid {
				t.Fatalf("validation: %v", err)
			}
		})
	}
}

func TestSkillManagementValidationBeforeHostApproval(t *testing.T) {
	s, db, meta := setupDockExecutionGuardTest(t)
	s.agentService = &AgentService{}
	s.registerSkillCommands()
	meta.ActorRole = "admin"
	catalog, _ := s.agentService.ListSkillCatalog(context.Background(), meta.WorkspaceID)
	skill := catalog.Skills[0]
	input, _ := json.Marshal(map[string]any{"skill_id": skill.Key, "expected_version": "probe", "instructions": "probe"})
	if _, err := s.Execute(context.Background(), meta, "skills.update", input); err == nil || !strings.Contains(err.Error(), "expected_version is invalid") {
		t.Fatalf("invalid edit should fail before approval: %v", err)
	}
	var count int64
	if err := db.Model(&model.DockActionProposal{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("invalid edit created an approval proposal")
	}
	input, _ = json.Marshal(map[string]any{"skill_id": skill.Key, "expected_version": skill.VersionKey, "title": "An actual new title"})
	if _, err := s.Execute(context.Background(), meta, "skills.validate_update", input); err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&model.DockActionProposal{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("read-only validation created an approval proposal")
	}
	if _, err := s.Execute(context.Background(), meta, "skills.update", input); err == nil || !strings.Contains(err.Error(), "dock_execution_approval_required") {
		t.Fatalf("valid edit bypassed approval: %v", err)
	}
}

func TestSkillManagementEditRoundTrip(t *testing.T) {
	agent, _, _ := newWorkspaceSkillTestService(t)
	s := NewInternalCommandService(agent, nil, nil, nil, nil, nil, nil, nil)
	meta := model.InternalCommandContext{WorkspaceID: "ws-1", ActorID: "user-1", ActorRole: "admin"}
	created, err := agent.CreateWorkspaceSkill(context.Background(), meta.WorkspaceID, meta.ActorID, model.CreateWorkspaceSkillRequest{Key: "unicode_test", Title: stringPtr("Unicode test"), Description: "Test paginated editing.", Instructions: strings.Repeat("こんにちは 🌍\n", 800)})
	if err != nil {
		t.Fatal(err)
	}
	offset := 0
	full := ""
	version := ""
	for {
		input, _ := json.Marshal(map[string]any{"skill_id": created.ID, "offset": offset, "limit": 997, "expected_version": version})
		out, err := s.Execute(context.Background(), meta, "skills.list_workspace", input)
		if err != nil {
			t.Fatal(err)
		}
		var page struct {
			Instructions string         `json:"instructions"`
			Version      string         `json:"version_key"`
			Next         *int           `json:"next_offset"`
			Hint         map[string]any `json:"_agent_runtime_compaction"`
		}
		if err := json.Unmarshal(out, &page); err != nil {
			t.Fatal(err)
		}
		if page.Hint["exempt"] != true {
			t.Fatal("bounded page is not protected from compaction")
		}
		full += page.Instructions
		version = page.Version
		if page.Next == nil {
			break
		}
		offset = *page.Next
	}
	if full != created.Instructions {
		t.Fatal("Unicode instructions corrupted")
	}
	updated := full + "\nCheck publishing logs only for publishing issues."
	input, _ := json.Marshal(map[string]any{"skill_id": created.ID, "expected_version": version, "instructions": updated})
	if _, err := s.Execute(context.Background(), meta, "skills.validate_update", input); err != nil {
		t.Fatal(err)
	}
	out, err := s.Execute(context.Background(), meta, "skills.update", input)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) > 1000 || !strings.Contains(string(out), `"saved":true`) {
		t.Fatalf("unexpected save receipt: %s", out)
	}
	catalog, err := agent.ListSkillCatalog(context.Background(), meta.WorkspaceID)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, entry := range catalog.Skills {
		if entry.ID != nil && *entry.ID == created.ID {
			found = true
			if entry.Instructions != updated || entry.Title != created.Title {
				t.Fatal("saved edit lost unrelated content")
			}
		}
	}
	if !found {
		t.Fatal("updated skill absent")
	}
	if _, err := s.Execute(context.Background(), meta, "skills.validate_update", input); err == nil {
		t.Fatal("stale version accepted after save")
	}
}

func TestSkillManagementCatalogPages(t *testing.T) {
	s := NewInternalCommandService(&AgentService{}, nil, nil, nil, nil, nil, nil, nil)
	meta := model.InternalCommandContext{WorkspaceID: "ws", ActorID: "user", ActorRole: "admin"}
	catalog, _ := s.agentService.ListSkillCatalog(context.Background(), meta.WorkspaceID)
	seen := map[string]bool{}
	offset := 0
	for {
		input, _ := json.Marshal(map[string]any{"offset": offset, "limit": 2})
		out, err := s.Execute(context.Background(), meta, "skills.list_workspace", input)
		if err != nil {
			t.Fatal(err)
		}
		var page struct {
			Skills []skillToolMetadata `json:"skills"`
			Next   *int                `json:"next_offset"`
		}
		if err := json.Unmarshal(out, &page); err != nil {
			t.Fatal(err)
		}
		if len(page.Skills) > 2 || strings.Contains(string(out), `"instructions":`) || strings.Contains(string(out), `"default_content":`) {
			t.Fatal("catalog is not compact")
		}
		for _, skill := range page.Skills {
			if seen[skill.SkillID] || skill.VersionKey == "" {
				t.Fatal("duplicate skill or missing version")
			}
			seen[skill.SkillID] = true
		}
		if page.Next == nil {
			break
		}
		offset = *page.Next
	}
	if len(seen) != len(catalog.Skills) {
		t.Fatal("catalog pagination omitted skills")
	}
}

func TestSkillManagementValidationRequiresEditPermission(t *testing.T) {
	s := NewInternalCommandService(&AgentService{}, nil, nil, nil, nil, nil, nil, nil)
	s.authz = authorization.NewAuthzService(nil, nil, nil)
	catalog, _ := s.agentService.ListSkillCatalog(context.Background(), "ws")
	skill := catalog.Skills[0]
	input, _ := json.Marshal(map[string]any{"skill_id": skill.Key, "expected_version": skill.VersionKey, "title": "Actual change"})
	if _, err := s.Execute(context.Background(), model.InternalCommandContext{WorkspaceID: "ws", ActorID: "viewer", ActorRole: "viewer"}, "skills.validate_update", input); err == nil {
		t.Fatal("viewer validated an edit without edit permission")
	}
	read, _ := s.Definition("skills.validate_update")
	write, _ := s.Definition("skills.update")
	if read.Mutating || read.RiskLevel() != commandtools.RiskLevelRead || !write.Mutating || write.RiskLevel() != commandtools.RiskLevelSensitive {
		t.Fatalf("incorrect skill tool approval classification: %s, %s", read.RiskLevel(), write.RiskLevel())
	}
}
