package service

import "github.com/helpin-ai/helpin/server/internal/aiusage"

type AIUsageLifecycle = aiusage.AIUsageLifecycle
type MeteringRequest = aiusage.MeteringRequest
type MeteringContext = aiusage.MeteringContext
type PreflightRequest = aiusage.PreflightRequest
type CompletionUsage = aiusage.CompletionUsage
type UsageResult = aiusage.UsageResult

const AIUsageOperationMediaEnrichment = aiusage.AIUsageOperationMediaEnrichment

const (
	mediaEnrichmentProvider       = aiusage.MediaEnrichmentProvider
	mediaEnrichmentCanonicalModel = aiusage.MediaEnrichmentCanonicalModel
	mediaEnrichmentRoute          = aiusage.MediaEnrichmentRoute
)
