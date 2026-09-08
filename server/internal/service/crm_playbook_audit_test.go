package service

import (
	"encoding/json"
	f "github.com/helpin-ai/helpin/server/internal/crmsituationtest"
	"github.com/helpin-ai/helpin/server/internal/repository"
	"strings"
	"testing"
)

func TestCRMPlaybookAuditUsesExistingReceiptsWithoutPrivateRuntimeData(t *testing.T) {
	db, svc, binding, _, ctx := playbookActionFixture(t, 0)
	store := repository.NewCRMPlaybookExecutionRepository(db)
	book := binding.Input.CRMPlaybook.PlaybookID
	page, err := store.AutomationActivity(ctx, f.Workspace, book, 1)
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 4 || len(page.Data) != 4 {
		t.Fatalf("missing connection/settings/adoption/run history: %#v", page)
	}
	encoded, _ := json.Marshal(page)
	for _, forbidden := range []string{"system_prompt", "runtime_profile", "allowed_tools", "fingerprint", "remote-"} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("private data leaked: %s", forbidden)
		}
	}
	usage, err := store.AgentUsage(ctx, f.Workspace, binding.AgentID)
	if err != nil || len(usage) != 1 || usage[0].PlaybookID != book {
		t.Fatalf("missing dependency: %#v %v", usage, err)
	}
	foreign, err := store.AutomationActivity(ctx, f.ForeignWorkspace, book, 1)
	if err != nil || foreign.Total != 0 {
		t.Fatalf("cross-workspace history: %#v %v", foreign, err)
	}
	if _, err := svc.execution.AutomationActivity(ctx, f.Workspace, book, 0); err == nil {
		t.Fatal("invalid page accepted")
	}
}
