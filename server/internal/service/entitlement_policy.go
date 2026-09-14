package service

import "github.com/helpin-ai/helpin/server/internal/model"

type EntitlementFeature = model.EntitlementFeature
type EntitlementLimit = model.EntitlementLimit
type EntitlementError = model.EntitlementError

const (
	EntitlementFeatureCustomAgents           = model.EntitlementFeatureCustomAgents
	EntitlementFeatureAutomationFlows        = model.EntitlementFeatureAutomationFlows
	EntitlementFeatureAgentScheduling        = model.EntitlementFeatureAgentScheduling
	EntitlementFeatureAIConversationRouting  = model.EntitlementFeatureAIConversationRouting
	EntitlementFeatureRoundRobinAssignment   = model.EntitlementFeatureRoundRobinAssignment
	EntitlementFeatureSLAPolicies            = model.EntitlementFeatureSLAPolicies
	EntitlementFeatureMultilingualHelpCenter = model.EntitlementFeatureMultilingualHelpCenter
	EntitlementFeatureAIArticleTranslation   = model.EntitlementFeatureAIArticleTranslation
	EntitlementFeatureDealAutomation         = model.EntitlementFeatureDealAutomation
	EntitlementFeatureRemoveBranding         = model.EntitlementFeatureRemoveBranding
	EntitlementLimitTeams                    = model.EntitlementLimitTeams
	EntitlementLimitDocuments                = model.EntitlementLimitDocuments
	EntitlementLimitContacts                 = model.EntitlementLimitContacts
)
