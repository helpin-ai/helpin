package service

import (
	"github.com/helpin-ai/helpin/server/internal/aipolicy"
	"github.com/helpin-ai/helpin/server/internal/aiusage"
	"github.com/helpin-ai/helpin/server/internal/model"
)

type AIUsageLifecycle = aiusage.AIUsageLifecycle
type MeteringRequest = aiusage.MeteringRequest
type MeteringContext = aiusage.MeteringContext
type PreflightRequest = aiusage.PreflightRequest
type CompletionUsage = aiusage.CompletionUsage
type UsageResult = aiusage.UsageResult

const AIUsageOperationMediaEnrichment = aiusage.AIUsageOperationMediaEnrichment

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

type ProductAnalyticsEvent = model.ProductAnalyticsEvent

var usageFeatures = aipolicy.UsageFeatures(aipolicy.DefaultRegistry())

func AIUsageFeature(key string) (aipolicy.UsageFeatureDefinition, bool) {
	feature, ok := usageFeatures[key]
	return feature, ok
}
