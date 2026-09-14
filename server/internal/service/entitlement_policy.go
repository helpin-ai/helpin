package service

type EntitlementFeature string

const (
	EntitlementFeatureCustomAgents           EntitlementFeature = "custom_agents"
	EntitlementFeatureAutomationFlows        EntitlementFeature = "automation_flows"
	EntitlementFeatureAgentScheduling        EntitlementFeature = "agent_scheduling"
	EntitlementFeatureAIConversationRouting  EntitlementFeature = "ai_conversation_routing"
	EntitlementFeatureRoundRobinAssignment   EntitlementFeature = "round_robin_assignment"
	EntitlementFeatureSLAPolicies            EntitlementFeature = "sla_policies"
	EntitlementFeatureMultilingualHelpCenter EntitlementFeature = "multilingual_help_center"
	EntitlementFeatureAIArticleTranslation   EntitlementFeature = "ai_article_translation"
	EntitlementFeatureDealAutomation         EntitlementFeature = "deal_automation"
	EntitlementFeatureRemoveBranding         EntitlementFeature = "remove_branding"
)

type EntitlementLimit string

const (
	EntitlementLimitTeams     EntitlementLimit = "teams"
	EntitlementLimitDocuments EntitlementLimit = "documents"
	EntitlementLimitContacts  EntitlementLimit = "contacts"
)

type EntitlementError struct {
	Feature      string
	Limit        string
	RequiredPlan string
	CurrentPlan  string
	Message      string
}

func (e *EntitlementError) Error() string {
	if e == nil {
		return ""
	}
	return e.Message
}
