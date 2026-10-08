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

const supportSetupInstructions = `Help the customer get their chosen support channels working in the connected workspace. Start with get_support_setup, confirm the intended workspace, and reuse existing configuration. Choose email, website chat, or both; support ownership; knowledge sources; and whether AI should reply.
` + setupAssistantExecutionInstructions + `
Configure the intended inbox, ownership, working hours and routing before enabling live handling. Add approved help content and review AI behavior and human handoff if AI is wanted. Configure through supported MCP tools or the returned browser links, and re-read get_support_setup after each saved step. Complete publishing, agreed test sends and live activation within the approved plan without asking again for the same action. Keep human support usable while resolving optional AI setup.
Verify a test conversation through each chosen channel, its target inbox, reply delivery and human handoff. If AI is enabled, inspect answers against approved knowledge and test escalation when it cannot answer. Configuration checks do not prove message delivery, answer quality, or successful handoff. Report observed results and the exact next action for any remaining test; never claim an end-to-end test passed from configuration alone.`

var supportSetupDetails = map[string]struct{ title, path, verification string }{
	"support.email_inbox_connected": {"Support email", "/settings/inboxes-routing?tab=email", "An active support email route exists. Send an approved test email and verify receipt and reply delivery separately."},
	"support.live_chat_installed":   {"Website chat", "/settings/chat-general", "An active widget installation and a visitor session exist in this workspace. Test the intended website and verify a new conversation separately."},
	"support.help_docs_ready":       {"Help articles", "/docs", "Public help content exists. Review the selected articles for accuracy and intended public visibility."},
	"support.brand_knowledge_ready": {"AI knowledge", "/settings/knowledge", "A synced brand knowledge source exists. Check coverage and confirm it is appropriate for customer answers."},
	"support.ai_agent_activated":    {"AI support", "/settings/support-ai-assistant", "AI replies are enabled with an assigned agent. Verify answer quality and human handoff against the approved setup plan."},
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
