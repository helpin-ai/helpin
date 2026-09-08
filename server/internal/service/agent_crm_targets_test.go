package service

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
	"github.com/helpin-ai/helpin/server/internal/repository"
)

func TestCustomAgentDraftPreservesCRMTargets(t *testing.T) {
	for _, target := range []string{"crm_deal", "crm_contact", "crm_company"} {
		t.Run(target, func(t *testing.T) {
			draft, warnings := validateCustomAgentDraft(model.CustomAgentDraft{
				Name: "Sales assistant", SystemPrompt: "Review the selected CRM record.",
				AllowedTargets: []string{target}, AllowedTools: []string{"list_crm_signals"},
			}, []model.ToolCatalogEntry{{Name: "list_crm_signals"}}, nil)
			if !slices.Equal(draft.AllowedTargets, []string{target}) {
				t.Fatalf("targets = %v, want only %s; warnings = %v", draft.AllowedTargets, target, warnings)
			}
			if !strings.Contains(customAgentDraftSystemPrompt(nil, nil), "- "+target+"\n") {
				t.Fatalf("draft prompt does not advertise supported target %s", target)
			}
			if draft.ApprovalMode != "mutating_tools" {
				t.Fatalf("approval mode = %s, want existing safe default", draft.ApprovalMode)
			}
		})
	}
}

func TestCRMTargetOptionsDoNotGrantAgentAccess(t *testing.T) {
	for _, target := range []string{"crm_deal", "crm_contact", "crm_company"} {
		t.Run(target, func(t *testing.T) {
			agent := &model.Agent{AllowedTargets: json.RawMessage(`["task"]`)}
			if err := validateAgentTarget(agent, target); err == nil {
				t.Fatal("task-only agent must not gain CRM targets")
			}
			agent.AllowedTargets = mustJSONStringSlice([]string{target})
			if err := validateAgentTarget(agent, target); err != nil {
				t.Fatalf("explicit CRM target should remain supported: %v", err)
			}
			if err := validateAgentTarget(agent, "task"); err == nil {
				t.Fatal("selecting CRM must not grant PM task access")
			}
		})
	}
}

func TestEnrichRunTargetsResolvesCRMCompaniesWithinWorkspace(t *testing.T) {
	db := newTestDB(t)
	if err := db.Create(&[]model.CRMCompany{
		{ID: "company-1", WorkspaceID: "ws-1", Name: "Acme", DisplayID: "COM-1"},
		{ID: "company-2", WorkspaceID: "ws-2", Name: "Private company", DisplayID: "COM-2"},
	}).Error; err != nil {
		t.Fatal(err)
	}
	repo := repository.NewCRMCompanyRepository(db)
	svc := &AgentService{crmCompanyRepo: repo}
	runs := []model.AgentRun{
		{TargetType: "crm_company", TargetID: "company-1"},
		{TargetType: "crm_company", TargetID: "company-1"},
		{TargetType: "crm_company", TargetID: "company-2"},
		{TargetType: "crm_company", TargetID: "missing"},
		{TargetType: "crm_company", TargetID: ""},
	}
	svc.enrichRunTargets(context.Background(), "ws-1", runs)
	for _, idx := range []int{0, 1} {
		if runs[idx].TargetInfo == nil || runs[idx].TargetInfo.Title != "Acme" {
			t.Fatalf("run %d target = %#v, want company name", idx, runs[idx].TargetInfo)
		}
	}
	for _, idx := range []int{2, 3, 4} {
		if runs[idx].TargetInfo != nil {
			t.Fatalf("run %d must not expose missing or cross-workspace company: %#v", idx, runs[idx].TargetInfo)
		}
	}
	if companies, err := repo.ListByIDs(context.Background(), "ws-1", nil); err != nil || len(companies) != 0 {
		t.Fatalf("empty IDs must not list workspace companies: companies=%v err=%v", companies, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := repo.ListByIDs(ctx, "ws-1", []string{"company-1"}); err == nil {
		t.Fatal("lookup must respect cancellation")
	}
}
