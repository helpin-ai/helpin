package service

import (
	"context"
	"strings"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// WorkspaceSetup deliberately avoids Get: that method initializes goals and
// records achievements. External inspection must not write onboarding state.
func (s *SetupService) WorkspaceSetup(ctx context.Context, workspaceID string, access SetupAccess) (model.WorkspaceSetupGuide, error) {
	goals, err := s.repo.ListGoalKeys(ctx, workspaceID)
	if err != nil {
		return model.WorkspaceSetupGuide{}, err
	}
	if len(goals) == 0 {
		goals, err = s.repo.PendingGoalKeys(ctx, workspaceID)
		if err != nil {
			return model.WorkspaceSetupGuide{}, err
		}
	}
	goals, err = NormalizeSetupGoals(goals)
	if err != nil {
		return model.WorkspaceSetupGuide{}, err
	}
	evidence, err := s.repo.GetEvidence(ctx, workspaceID)
	if err != nil {
		return model.WorkspaceSetupGuide{}, err
	}
	invitations, err := s.repo.UsableSetupInvitationCount(ctx, workspaceID)
	if err != nil {
		return model.WorkspaceSetupGuide{}, err
	}
	return buildWorkspaceSetupGuide(workspaceID, goals, evidence, invitations, s.withEntitlements(ctx, workspaceID, access)), nil
}

const workspaceSetupInstructions = `Help the user set up the connected Helpin workspace. Start with get_workspace_setup and confirm the workspace and identity. Reuse existing configuration, existing teams and pending invitations. Cover workspace essentials and only the user's selected goals, in their chosen order. The listed steps are options, not mandatory completion targets: ask which teams, people, channels and automations are actually wanted. Do not invent a requirement to enable AI or every channel.
Use available authorized MCP operations for supported work. For settings, use the returned browser links and the existing labelled Helpin UI. Your client must provide browser access; Helpin MCP does not provide a browser. Without it, give the user the exact link and next action and resume after they finish. Read-only MCP access is inspection permission, not authorization to change settings through a browser. Confirm the signed-in identity/workspace, obtain authorization for UI changes, and never bypass denied actions, module access, or workspace policy.
Company context can be entered manually. Connecting AI is optional for this external-assistant workflow; Helpin's own AI generation and agent runs still require a configured provider and the normal usage checks. Do not replace or erase existing context without approval.
Create teams through Settings → Teams, preserving its team type, workflows, estimates and field defaults. A workspace team and a support team inbox are different things. Inspect existing teams before creating more. For invitations, confirm the exact recipients, roles, team assignments and module access before sending. Use Settings → Members; reuse pending invitations and honor the UI's available roles. If email is unavailable, Helpin can return invitation links for the user to share; do not expose these links to other recipients. An invitation created or sent does not mean the member joined or the email was delivered.
Let the user complete sign-in, OAuth consent, credentials and external account approvals. Never request secrets in chat. Website/widget installation and custom-domain DNS need access to the website or DNS owner. Explain external handoffs. Use only user-approved knowledge and imports. Obtain explicit approval before publishing, sending invitations or test messages, enabling live AI replies, activating automations, loading sample data, or starting paid agent runs. Keep existing live settings unchanged unless the user requested a change.
For support, choose email, chat or both, configure the intended inbox and routing, review knowledge and human handoff, and enable AI replies only if requested. For help centers, distinguish draft content, published articles and the published site; verify intended public visibility. Internal documents must not become public. Respect module availability and feature limits for projects, CRM and automation.
After each saved step, re-read get_workspace_setup. Completed means the recorded configuration check passed, not that the entire setup or a real-world test succeeded. Test chosen support channels and help-center access separately with approval. Report observed results, remaining user actions and unavailable steps. Users can continue in Helpin at any time; do not create a second workspace or mark skipped tasks complete.`

// Keep these action destinations aligned with frontend/src/lib/setupActions.ts.
// A frontend parity test checks the shared keys against these exact paths.
var workspaceSetupPaths = map[string]string{
	"workspace_context": "/settings/knowledge#company-context",
	"workspace_teams":   "/settings/teams",
	"workspace_members": "/settings/members",
	"pm_epics":          "/pm/epics", "pm_tasks": "/pm/tasks", "pm_sprints": "/pm/sprints",
	"product_agent": "/automation/agents", "git_settings": "/settings/repositories",
	"support_email_inbox": "/settings/inboxes-routing?tab=email",
	"support_live_chat":   "/settings/chat-general", "support_help_docs": "/docs",
	"support_brand_knowledge": "/settings/knowledge", "support_team_inboxes": "/settings/inboxes-routing?tab=inboxes",
	"support_routing": "/settings/inboxes-routing?tab=routing", "support_inbox": "/support",
	"support_ai": "/settings/support-ai-assistant", "support_coverage": "/support/coverage",
	"docs_home": "/docs", "help_center_article_publish": "/docs",
	"help_center_settings": "/settings/helpcenter", "help_center_widget": "/settings/chat-general",
	"internal_docs_agent_knowledge": "/automation/agents", "internal_docs_agent_run": "/automation/agents",
	"crm_contacts": "/crm/contacts", "crm_companies": "/crm/companies", "crm_deals": "/crm/deals",
	"crm_email": "/settings/crm-email", "crm_autonomy": "/settings/crm-autonomy", "crm_review": "/crm/review",
	"automation_flows": "/automation/flows", "automation_agents": "/automation/agents",
	"automation_custom_agent": "/automation/agents", "automation_approval_guard": "/automation/agents",
	"product_required_flow":       "/automation/flows?template=stale_task_escalation",
	"help_center_required_flow":   "/automation/flows?template=public_help_freshness_sweep",
	"internal_docs_required_flow": "/automation/flows?template=docs_freshness_sweep",
	"crm_required_flow":           "/automation/flows?template=buying_signal_to_task",
}

func buildWorkspaceSetupGuide(workspaceID string, goals []string, evidence SetupEvidence, invitations int64, access SetupAccess) model.WorkspaceSetupGuide {
	guide := model.WorkspaceSetupGuide{WorkspaceID: workspaceID, Goals: goals, Sections: []model.WorkspaceSetupSection{}, Instructions: workspaceSetupInstructions}
	if guide.Goals == nil {
		guide.Goals = []string{}
	}
	keys := append([]string{model.SetupGoalFoundation}, goals...)
	for _, key := range keys {
		definition, ok := setupJourneyCatalog[key]
		if !ok {
			continue
		}
		section := model.WorkspaceSetupSection{Key: key, Title: definition.title, Steps: []model.SupportSetupStep{}}
		for _, task := range definition.tasks {
			// Initial setup only. Longer-term adoption milestones remain in the Setup guide.
			if !isSetupReadinessTask(task.key) && key != model.SetupGoalAutomationMastery {
				continue
			}
			step := model.SupportSetupStep{Key: task.key, Title: task.title, Status: model.SetupTaskAvailable, Verification: "The current configuration matches this Setup guide check. Review the saved values in Helpin; this does not prove a real-world outcome or authorize changes."}
			switch task.key {
			case "foundation.company_context_ready":
				step.Title = "Company context"
				step.Verification = "Non-empty company context is saved. Review accuracy and the website in Settings → Knowledge."
			case "foundation.team_ready":
				step.Title = "Workspace teams"
				step.Verification = "At least one non-sample team exists. Check team type, membership and defaults in Settings → Teams; do not create duplicates."
			case "foundation.member_joined":
				step.Title = "Teammate joined"
				step.Verification = "More than one active workspace member exists. An invitation alone is not a joined member."
			}
			if detail, ok := supportSetupDetails[task.key]; ok {
				step.Title = detail.title
				step.Verification = detail.verification
			}
			allowed, reason := workspaceSetupActionAllowed(task.key, task.actionKey, access)
			if !allowed {
				step.Status = model.SetupTaskBlocked
				step.BlockedReason = reason
			} else {
				step.Path = workspaceSetupPaths[task.actionKey]
				if task.completed(evidence) {
					step.Status = model.SetupTaskCompleted
				}
			}
			section.Steps = append(section.Steps, step)
			if task.key == "foundation.team_ready" {
				invite := model.SupportSetupStep{Key: "foundation.invitation_created", Title: "Invite members", Status: model.SetupTaskAvailable, Verification: "A non-expired pending or accepted invitation exists. Check intended recipients, roles, team assignments and module access in Members. Delivery and acceptance are separate checks."}
				if allowed, reason := workspaceSetupActionAllowed(invite.Key, "workspace_members", access); !allowed {
					invite.Status = model.SetupTaskBlocked
					invite.BlockedReason = reason
				} else {
					invite.Path = workspaceSetupPaths["workspace_members"]
					if invitations > 0 {
						invite.Status = model.SetupTaskCompleted
					}
				}
				section.Steps = append(section.Steps, invite)
			}
		}
		if key == model.SetupGoalCustomerSupport {
			support := buildSupportSetupGuide(workspaceID, evidence, access)
			section.Steps = append(section.Steps, support.Steps[len(support.Steps)-1])
		}
		guide.Sections = append(guide.Sections, section)
	}
	return guide
}

func workspaceSetupActionAllowed(taskKey, actionKey string, access SetupAccess) (bool, string) {
	if !access.Unrestricted {
		// Some older Setup guide actions check permission but not module access.
		// Inspection must also honor the MCP connection's module projection.
		module := ""
		if strings.HasPrefix(taskKey, "product.") {
			module = "pm"
		}
		if strings.HasPrefix(taskKey, "crm.") {
			module = "crm"
		}
		if strings.HasPrefix(taskKey, "help_center.") || strings.HasPrefix(taskKey, "internal_docs.") {
			module = "docs"
		}
		if strings.HasPrefix(taskKey, "automation.") {
			module = "automation"
		}
		if module != "" && !access.Modules[module] {
			return false, "This module is unavailable to you or is not included in this connection's access."
		}
	}
	return setupActionAllowed(taskKey, actionKey, access)
}
