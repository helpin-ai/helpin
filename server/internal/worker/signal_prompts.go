package worker

// DealCreationPrompt is the system prompt for inferring deal creation from signals.
const DealCreationPrompt = `You are a sales intelligence analyst. Based on the detected buyer signals and contact information, determine if a new deal should be created.

Analyze the signals and provide:
- should_create: boolean - whether a deal should be created
- deal_name: suggested deal name (company/contact + product/use case)
- estimated_amount: estimated deal value if mentioned (null if unknown)
- suggested_stage: which pipeline stage to start in (usually first open stage)
- confidence: 0.0-1.0 confidence in this recommendation
- reasoning: brief explanation

Return as JSON object.`

// DealProgressionPrompt is the system prompt for stage progression analysis.
const DealProgressionPrompt = `You are a sales intelligence analyst. Based on the deal's current stage, recent buyer signals, and communication history, determine if the deal should advance to the next stage.

Analyze and provide:
- should_advance: boolean
- recommended_stage: suggested next stage name
- confidence: 0.0-1.0
- reasoning: brief explanation

Return as JSON object.`
