package service

import (
	"context"

	"github.com/helpin-ai/helpin/server/internal/model"
)

// SupportSetup reads current evidence without initializing goals, recording
// achievements, changing settings, or requiring the support goal to be selected.
func (s *SetupService) SupportSetup(ctx context.Context, workspaceID string, access SetupAccess) (model.SupportSetupGuide, error) {
	evidence, err := s.repo.GetSupportSetupEvidence(ctx, workspaceID)
	if err != nil {
		return model.SupportSetupGuide{}, err
	}
	return buildSupportSetupGuide(workspaceID, evidence, s.withEntitlements(ctx, workspaceID, access)), nil
}

const supportSetupInstructions = `Help the user configure customer support in the connected workspace. First inspect get_support_setup and reuse existing configuration. Ask only for missing decisions: email, website chat, or both; support ownership; knowledge sources; and whether AI should reply. Channels and AI are choices, not mandatory completion targets.
Use available, authorized MCP operations for supported work. For settings, open the returned browser links and use labelled UI controls. Browser access must be provided by your client; Helpin MCP does not provide a browser. Without browser access, give the user the exact link and next action, then resume after they finish.
Before browser changes, confirm the signed-in workspace and identity match the user's intended account. A read-only MCP grant or service token is not permission to write through a browser. Do not use the browser to bypass a denied operation or workspace policy; obtain the user's explicit authorization for UI changes and respect UI permissions.
Let the user complete sign-in, OAuth consent, secret entry, and external account approvals. Never ask them to paste credentials into chat. Website installation requires access to the website repository or a handoff to its owner. Save changes through the existing UI and re-read get_support_setup after each completed step; do not mark a step complete from a click or stale response.
Add only user-selected knowledge. Review AI behavior and human handoff before enabling customer-facing replies. Get explicit approval for publishing, sending test messages, and live activation. Keep existing live settings unchanged unless the user requested a change.
Verify a test conversation through each chosen channel, check its target inbox and human handoff, and inspect AI answers against the approved knowledge if AI is wanted. Configuration checks do not prove message delivery, answer quality, or successful handoff. Report what was observed, what still needs testing, and any blocked user actions. Never claim an end-to-end test passed from configuration alone.`

var supportSetupDetails = map[string]struct{ title, path, verification string }{
	"support.email_inbox_connected": {"Support email", "/settings/inboxes-routing?tab=email", "An active support email route exists. Send an approved test email and verify receipt and reply delivery separately."},
	"support.live_chat_installed":   {"Website chat", "/settings/chat-general", "An active widget installation and a visitor session exist in this workspace. Test the intended website and verify a new conversation separately."},
	"support.help_docs_ready":       {"Help articles", "/docs", "Public help content exists. Review the selected articles for accuracy and approve publishing separately."},
	"support.brand_knowledge_ready": {"AI knowledge", "/settings/knowledge", "A synced brand knowledge source exists. Check coverage and confirm it is appropriate for customer answers."},
	"support.ai_agent_activated":    {"AI support", "/settings/support-ai-assistant", "AI replies are enabled with an assigned agent. This does not verify answer quality; review behavior and handoff and obtain approval before activation."},
	"support.team_inbox_created":    {"Team inbox", "/settings/inboxes-routing?tab=inboxes", "A team inbox exists. Confirm its membership, ownership, and working hours in settings."},
	"support.routing_enabled":       {"Conversation routing", "/settings/inboxes-routing?tab=routing", "Automatic routing is enabled with an eligible configured inbox or rule. Verify the destination with a test conversation."},
}

func buildSupportSetupGuide(workspaceID string, evidence SetupEvidence, access SetupAccess) model.SupportSetupGuide {
	guide := model.SupportSetupGuide{WorkspaceID: workspaceID, Steps: []model.SupportSetupStep{}, Instructions: supportSetupInstructions}
	for _, definition := range setupJourneyCatalog[model.SetupGoalCustomerSupport].tasks {
		detail, ok := supportSetupDetails[definition.key]
		if !ok {
			continue
		}
		step := model.SupportSetupStep{Key: definition.key, Title: detail.title, Status: model.SetupTaskAvailable, Verification: detail.verification}
		allowed, reason := setupActionAllowed(definition.key, definition.actionKey, access)
		if !allowed {
			step.Status, step.BlockedReason = model.SetupTaskBlocked, reason
		} else {
			step.Path = detail.path
			if definition.completed(evidence) {
				step.Status = model.SetupTaskCompleted
			}
		}
		guide.Steps = append(guide.Steps, step)
	}
	test := model.SupportSetupStep{Key: "support.test_conversation", Title: "Test a conversation", Status: model.SetupTaskUnableToVerify, Verification: "With the user's approval, test each chosen channel, reply delivery, routing, and human handoff. Check AI answer quality if enabled. Report observed results; these are not inferred from configuration."}
	if allowed, reason := setupActionAllowed(test.Key, "support_inbox", access); allowed {
		test.Path = "/support"
	} else {
		test.Status, test.BlockedReason = model.SetupTaskBlocked, reason
	}
	guide.Steps = append(guide.Steps, test)
	return guide
}
