package service

import (
	"strings"
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func workspaceSetupStep(t *testing.T, guide model.WorkspaceSetupGuide, key string) model.SupportSetupStep {
	t.Helper()
	for _, section := range guide.Sections {
		for _, step := range section.Steps {
			if step.Key == key {
				return step
			}
		}
	}
	t.Fatalf("missing step %s", key)
	return model.SupportSetupStep{}
}

func TestWorkspaceSetupSelectedGoalsAndInvitationStates(t *testing.T) {
	guide := buildWorkspaceSetupGuide("ws-1", []string{model.SetupGoalCustomerSupport, model.SetupGoalHelpCenterDocs}, SetupEvidence{TeamCount: 1, ActiveMemberCount: 1}, 1, SetupAccess{Unrestricted: true})
	if len(guide.Sections) != 3 || guide.Sections[0].Key != model.SetupGoalFoundation {
		t.Fatalf("sections = %#v", guide.Sections)
	}
	if workspaceSetupStep(t, guide, "foundation.invitation_created").Status != model.SetupTaskCompleted {
		t.Fatal("valid pending invite not recognized")
	}
	if workspaceSetupStep(t, guide, "foundation.member_joined").Status == model.SetupTaskCompleted {
		t.Fatal("pending invitation treated as joined member")
	}
	if workspaceSetupStep(t, guide, "support.test_conversation").Status != model.SetupTaskUnableToVerify {
		t.Fatal("test inferred from settings")
	}
	for _, section := range guide.Sections {
		if section.Key == model.SetupGoalSalesCRM {
			t.Fatal("unselected CRM offered")
		}
	}
	for _, key := range []string{"foundation.company_context_ready", "foundation.team_ready", "foundation.invitation_created", "help_center.site_published"} {
		if workspaceSetupStep(t, guide, key).Path == "" {
			t.Fatalf("missing UI handoff %s", key)
		}
	}
}

func TestWorkspaceSetupPermissionsPrecedeEvidence(t *testing.T) {
	guide := buildWorkspaceSetupGuide("ws-1", []string{model.SetupGoalCustomerSupport, model.SetupGoalHelpCenterDocs, model.SetupGoalProductDelivery}, SetupEvidence{TeamCount: 5, InvitationCount: 4, PublicHelpDocCount: 10, PlannedProjectCount: 4}, 2, SetupAccess{})
	for _, section := range guide.Sections {
		for _, step := range section.Steps {
			if step.Status != model.SetupTaskBlocked || step.Path != "" {
				t.Fatalf("unauthorized evidence or link: %#v", step)
			}
		}
	}
}

func TestWorkspaceSetupCatalogLinksAndSafety(t *testing.T) {
	goals := []string{model.SetupGoalProductDelivery, model.SetupGoalCustomerSupport, model.SetupGoalHelpCenterDocs, model.SetupGoalInternalDocs, model.SetupGoalSalesCRM, model.SetupGoalAutomationMastery}
	guide := buildWorkspaceSetupGuide("ws-1", goals, SetupEvidence{}, 0, SetupAccess{Unrestricted: true})
	for _, section := range guide.Sections {
		for _, step := range section.Steps {
			if step.Path == "" || !strings.HasPrefix(step.Path, "/") || step.Verification == "" {
				t.Fatalf("incomplete handoff: %#v", step)
			}
		}
	}
	for _, text := range []string{"browser", "recipients", "roles", "credentials", "publishing", "get_workspace_setup", "not mandatory", "existing teams"} {
		if !strings.Contains(guide.Instructions, text) {
			t.Errorf("missing instruction %s", text)
		}
	}
}
