package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestSkillEditRequiresPreparedChange(t *testing.T) {
	agent, _, _ := newWorkspaceSkillTestService(t)
	s, _, meta := setupDockExecutionGuardTest(t)
	s.agentService = agent
	s.registerSkillCommands()
	meta.ActorRole = "admin"
	catalog, _ := agent.ListSkillCatalog(context.Background(), meta.WorkspaceID)
	skill := catalog.Skills[0]
	input, _ := json.Marshal(map[string]any{"skill_id": skill.Key, "expected_version": skill.VersionKey, "title": "Changed title"})
	_, err := s.Execute(context.Background(), meta, "skills.update", input)
	if err == nil || strings.Contains(err.Error(), "dock_execution_approval_required") {
		t.Fatalf("unprepared edit reached approval: %v", err)
	}
}

func TestSkillEditPreparesExactReplacement(t *testing.T) {
	agent, _, _ := newWorkspaceSkillTestService(t)
	s, _, meta := setupDockExecutionGuardTest(t)
	s.agentService = agent
	s.registerSkillCommands()
	meta.ActorRole = "admin"
	skill, err := agent.CreateWorkspaceSkill(context.Background(), meta.WorkspaceID, meta.ActorID, model.CreateWorkspaceSkillRequest{Key: "scoped_edit", Title: stringPtr("Scoped edit"), Description: "Support guidance.", Instructions: "## Context\nKeep this.\n\n## Investigation\nOld instruction.\n\n## Handoff\nKeep this too."})
	if err != nil {
		t.Fatal(err)
	}
	req := map[string]any{"skill_id": skill.ID, "expected_version": skill.VersionKey, "replacements": []map[string]string{{"old_text": "Old instruction.", "new_text": "New instruction."}}}
	input, _ := json.Marshal(req)
	out, err := s.Execute(context.Background(), meta, "skills.validate_update", input)
	if err != nil {
		t.Fatal(err)
	}
	var prepared struct {
		SaveInput json.RawMessage `json:"save_input"`
	}
	if err := json.Unmarshal(out, &prepared); err != nil {
		t.Fatal(err)
	}
	if len(prepared.SaveInput) == 0 {
		t.Fatal("no prepared save arguments")
	}
	// The preparation must not edit the skill or grant mutation approval.
	if _, err := s.Execute(context.Background(), meta, "skills.update", prepared.SaveInput); err == nil || !strings.Contains(err.Error(), "dock_execution_approval_required") {
		t.Fatalf("save skipped approval: %v", err)
	}
	// Exercise the approved execution path without an external runtime in this test.
	s.agentRunRepo = nil
	if _, err := s.Execute(context.Background(), meta, "skills.update", prepared.SaveInput); err != nil {
		t.Fatal(err)
	}
	catalog, _ := agent.ListSkillCatalog(context.Background(), meta.WorkspaceID)
	for _, entry := range catalog.Skills {
		if entry.ID != nil && *entry.ID == skill.ID && entry.Instructions != strings.Replace(skill.Instructions, "Old instruction.", "New instruction.", 1) {
			t.Fatal("replacement damaged unrelated instructions")
		}
	}
	if _, err := s.Execute(context.Background(), meta, "skills.update", prepared.SaveInput); err == nil {
		t.Fatal("prepared edit was replayed")
	}
}

func TestSkillSectionReadIncludesExactContinuation(t *testing.T) {
	s := NewInternalCommandService(&AgentService{}, nil, nil, nil, nil, nil, nil, nil)
	meta := model.InternalCommandContext{WorkspaceID: "ws", ActorID: "user", ActorRole: "admin"}
	out, err := s.Execute(context.Background(), meta, "skills.list_workspace", json.RawMessage(`{"key":"support_triage_response","section_heading":"## Investigation workflow","limit":100}`))
	if err != nil {
		t.Fatal(err)
	}
	var page struct {
		Instructions string          `json:"instructions"`
		Next         json.RawMessage `json:"next_read"`
		Contract     string          `json:"contract_version"`
	}
	if err := json.Unmarshal(out, &page); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(page.Instructions, "## Investigation workflow") || len(page.Next) == 0 || page.Contract == "" {
		t.Fatalf("missing precise section/continuation: %s", out)
	}
	if _, err := s.Execute(context.Background(), meta, "skills.list_workspace", page.Next); err != nil {
		t.Fatal(err)
	}
}

func TestSkillReplacementRejectsAmbiguousAndOverlappingEdits(t *testing.T) {
	for _, edits := range [][]skillTextReplacement{
		{{OldText: "", NewText: "new"}},
		{{OldText: "missing", NewText: "new"}},
		{{OldText: "repeat", NewText: "new"}},
		{{OldText: "unique passage", NewText: "new"}, {OldText: "passage", NewText: "other"}},
	} {
		if _, err := applySkillReplacements("repeat repeat unique passage", edits); err == nil {
			t.Fatalf("accepted ambiguous edits: %#v", edits)
		}
	}
	got, err := applySkillReplacements("first and second", []skillTextReplacement{{OldText: "first", NewText: "second"}, {OldText: "second", NewText: "last"}})
	if err != nil || got != "second and last" {
		t.Fatalf("edits did not use the original text: %q %v", got, err)
	}
}

func TestPreparedSkillEditScopeAndVersion(t *testing.T) {
	agent, _, _ := newWorkspaceSkillTestService(t)
	s, db, meta := setupDockExecutionGuardTest(t)
	s.agentService = agent
	s.registerSkillCommands()
	meta.ActorRole = "admin"
	catalog, _ := agent.ListSkillCatalog(context.Background(), meta.WorkspaceID)
	skill := catalog.Skills[0]
	input, _ := json.Marshal(map[string]any{"skill_id": skill.Key, "expected_version": skill.VersionKey, "title": "Prepared title"})
	out, err := s.Execute(context.Background(), meta, "skills.validate_update", input)
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		SaveInput preparedSkillSaveInput `json:"save_input"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatal(err)
	}
	save, _ := json.Marshal(result.SaveInput)
	for _, change := range []func(*model.InternalCommandContext){
		func(m *model.InternalCommandContext) { m.WorkspaceID = "other" },
		func(m *model.InternalCommandContext) { m.ActorID = "other" },
		func(m *model.InternalCommandContext) { m.RunID = "other" },
		func(m *model.InternalCommandContext) { m.ActorRole = "" },
	} {
		different := meta
		change(&different)
		if err := s.validatePreparedSkillUpdate(context.Background(), different, save); err == nil {
			t.Fatal("proposal crossed actor/workspace/run scope")
		}
	}
	tampered := result.SaveInput
	tampered.Summary = "Save something else"
	bad, _ := json.Marshal(tampered)
	if err := s.validatePreparedSkillUpdate(context.Background(), meta, bad); err == nil {
		t.Fatal("tampered summary accepted")
	}
	// Preparing an edit does not alter the skill. A later edit invalidates preparation.
	current, _ := agent.ListSkillCatalog(context.Background(), meta.WorkspaceID)
	if current.Skills[0].Title != skill.Title {
		t.Fatal("preparation mutated skill")
	}
	if _, err := agent.UpdateWorkspaceSkill(context.Background(), meta.WorkspaceID, skill.Key, model.UpdateWorkspaceSkillRequest{Title: stringPtr("Intervening edit")}); err != nil {
		t.Fatal(err)
	}
	if err := s.validatePreparedSkillUpdate(context.Background(), meta, save); err == nil {
		t.Fatal("stale prepared edit accepted")
	}
	if err := db.Model(&model.DockActionProposal{}).Where("id = ?", result.SaveInput.ProposalID).Update("expires_at", "2000-01-01T00:00:00Z").Error; err != nil {
		t.Fatal(err)
	}
	if err := s.validatePreparedSkillUpdate(context.Background(), meta, save); err == nil {
		t.Fatal("expired proposal accepted")
	}
}

func TestSkillSectionPreservesSubsectionsAndIgnoresFences(t *testing.T) {
	text := "## One\nfirst\n### Child\nkeep\n```md\n## Example\n```\n## Two\nsecond"
	got, _, err := skillInstructionSection(text, "## One")
	if err != nil || got != "## One\nfirst\n### Child\nkeep\n```md\n## Example\n```\n" {
		t.Fatalf("wrong section: %q %v", got, err)
	}
	if _, _, err := skillInstructionSection("## Same\none\n## Same\ntwo", "## Same"); err == nil {
		t.Fatal("ambiguous heading accepted")
	}
}
