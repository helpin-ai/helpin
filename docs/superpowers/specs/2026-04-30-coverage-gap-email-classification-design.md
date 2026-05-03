# Coverage Gap Email Classification

**Date**: 2026-04-30
**Status**: Draft

## Problem

The coverage gap daily analyzer processes nearly all non-spam conversations as possible support conversations. Newsletters, cold outreach, auto-replies, bounces, phishing, transactional notifications, and other sender-driven emails can become false-positive coverage gaps because the LLM analyzes them as if a customer needed help and the AI failed to resolve it.

The feature should reduce false-positive gaps without adding a new classifier service, new ingestion pipeline, or a broad inbox-routing system. It should also save LLM cost where that is easy and low-risk, but cost savings should not come at the expense of missing real customer/prospect needs.

## Goals

- Prevent obvious non-support emails from producing coverage gaps.
- Preserve genuine customer/prospect questions, including pre-purchase, pricing, migration, integration, security, account, billing, setup, and troubleshooting questions.
- Keep implementation inside the existing daily coverage analyzer.
- Avoid a second LLM call for classification.
- Add enough stored classification data to debug decisions and support future reporting.
- Use conservative deterministic skips to save some LLM calls where the signal is clear.

## Non-Goals

- Do not classify every inbound email at ingestion time.
- Do not add a new conversation-level routing or triage workflow.
- Do not build a separate LLM classifier.
- Do not add frontend reporting in this iteration.
- Do not automatically delete or close existing false-positive gaps.

## Approach

Use a two-layer filter in the daily analyzer:

1. **Conservative local prefilter before the LLM**
   Skip only conversations that are very likely non-support using deterministic signals. This saves LLM calls for the clearest cases and keeps ambiguous conversations in the existing analyzer path.

2. **LLM classification inside the existing `AnalyzeConversation` call**
   Add classification fields to the current JSON output. The same LLM response decides whether the conversation is a genuine support/customer need and, if it is, performs the existing gap analysis.

This avoids new infrastructure while improving both quality and cost. The prefilter is intentionally small; the LLM remains responsible for nuanced cases.

## Conversation Type Contract

Add these classification fields to the analyzer result and stored analysis row:

```go
IsSupportQuery      bool   `json:"is_support_query"`
ConversationType    string `json:"conversation_type"`
ClassificationReason string `json:"classification_reason"`
```

`conversation_type` values:

- `support_query` — genuine customer/prospect question, request, issue, buying evaluation, setup, troubleshooting, account, billing, or workflow need
- `newsletter` — marketing/promotional broadcast or digest
- `cold_outreach` — unsolicited sales, recruiting, agency, vendor, partnership, backlink, or service pitch
- `auto_reply` — out-of-office, vacation responder, auto-acknowledgement, delivery receipt, bounce, or mailbox notification
- `transactional` — order confirmation, receipt, shipping update, billing notice, invoice, password reset, or account notification with no support request
- `spam` — spam, phishing, malware, scam, or deceptive email
- `internal` — internal/team communication not from a customer/prospect
- `other` — not a support/customer need and not better described above

Important distinction: inbound sales or product-evaluation questions from a prospect are still `support_query`. A sender-driven vendor pitch to Helpin is `cold_outreach`.

## Deterministic Prefilter

Add a small helper in `server/internal/service/support_coverage_daily_analyzer.go`, called before `AnalyzeConversation`:

```go
func classifyCoverageConversationLocally(input CoverageConversationAnalysisInput) (CoverageLocalClassification, bool)
```

The second return value means "safe to skip without LLM." It should only return `true` for high-confidence non-support patterns.

Recommended initial rules:

- No non-internal customer messages: skip as `other`.
- Subject or first customer message starts with clear auto-reply/bounce patterns:
  - `out of office`
  - `automatic reply`
  - `auto-reply`
  - `delivery status notification`
  - `undeliverable`
  - `mail delivery failed`
  - `returned mail`
- Clear newsletter/promotional patterns with no question/request language:
  - unsubscribe/footer-heavy content
  - `view this email in your browser`
  - `manage your preferences`
  - marketing broadcast subjects such as `newsletter`, `digest`, `weekly update`
- Clear cold outreach patterns with no support/request context:
  - vendor pitch language such as `quick question`, `book a call`, `increase your leads`, `guest post`, `backlinks`, `SEO services`, `partnership opportunity`

Keep these rules conservative. If the email contains a concrete product/customer question, do not skip locally.

When skipped locally, record a `SupportCoverageConversationAnalysis` row with:

- `status = "skipped"`
- `has_gap = false`
- `is_support_query = false`
- `conversation_type` from the local classifier
- `classification_reason` explaining the rule
- `raw_output` containing a small JSON classification payload

## LLM Analysis Result

Update `CoverageConversationAnalysisResult` in `server/internal/service/support_coverage_daily_analyzer.go`:

```go
type CoverageConversationAnalysisResult struct {
    IsSupportQuery       bool   `json:"is_support_query"`
    ConversationType     string `json:"conversation_type"`
    ClassificationReason string `json:"classification_reason"`

    HasGap             bool                     `json:"has_gap"`
    GapKind            string                   `json:"gap_kind"`
    GapCategory        string                   `json:"gap_category"`
    CanonicalTitle     string                   `json:"canonical_title"`
    CustomerNeed       string                   `json:"customer_need"`
    AIFailure          string                   `json:"ai_failure"`
    HumanResolution    string                   `json:"human_resolution"`
    DecisionReason     string                   `json:"decision_reason"`
    SearchQuery        string                   `json:"search_query"`
    ShouldRunRetrieval bool                     `json:"should_run_retrieval"`
    RecommendedFixes   []CoverageRecommendedFix `json:"recommended_fixes"`
    Confidence         float64                  `json:"confidence"`
}
```

## JSON Schema Update

Add these fields to `coverageConversationAnalysisJSONSchema()` and include them in `required`:

- `is_support_query`: boolean
- `conversation_type`: enum using the values above
- `classification_reason`: string

The existing fields remain required. For non-support conversations, the LLM should return zero-value gap fields.

## Prompt Update

Prepend clear classification instructions to `coverageConversationAnalysisSystemPrompt()`:

```text
First classify whether the conversation contains a genuine customer or prospect need.
Set conversation_type=support_query for customer/prospect questions, support issues,
account or billing requests, setup/troubleshooting requests, and product-evaluation
questions such as pricing, migration, integration, security, or comparisons.

Set is_support_query=false for newsletters, cold outreach, auto-replies, bounces,
transactional notifications with no support request, spam/phishing, internal messages,
and emails that are not from someone seeking help or product information.

If is_support_query=false, set has_gap=false, should_run_retrieval=false,
recommended_fixes=[], and leave gap details empty. Explain the classification briefly
in classification_reason.
```

## Normalization And Guardrails

Update `normalizeCoverageConversationAnalysisResult` to enforce consistency:

- Trim `conversation_type` and `classification_reason`.
- Default empty `conversation_type` to `support_query` for backward compatibility in tests and defensive parsing.
- If `conversation_type` is not one of the known values, set it to `other`.
- Derive `IsSupportQuery` from `conversation_type == "support_query"` after normalization.
- If `IsSupportQuery` is false:
  - force `HasGap = false`
  - force `ShouldRunRetrieval = false`
  - clear `RecommendedFixes`
  - clear `GapKind`, `GapCategory`, `CanonicalTitle`, `CustomerNeed`, `AIFailure`, `HumanResolution`, and `SearchQuery`
- Clamp invalid confidence values to the existing expected range if not already handled elsewhere.

This protects the system from inconsistent LLM output such as `is_support_query=true` with `conversation_type=newsletter`.

## Analysis Flow Changes

In `runConversationCoverageAnalysis`:

1. Build `CoverageConversationAnalysisInput`.
2. Check `AlreadyAnalyzedConversation` as today.
3. Run the local prefilter.
4. If the local prefilter safely classifies the conversation as non-support:
   - record analysis with `status = "skipped"`
   - return without LLM
5. Otherwise call `AnalyzeConversation`.
6. Record classification fields on the analysis row.
7. If `!result.IsSupportQuery`, record `status = "skipped"` and return without gap creation.
8. If `result.IsSupportQuery && !result.HasGap`, record `status = "analyzed"` and return without gap creation.
9. Continue the existing gap creation path only for support queries with `has_gap=true`.

This keeps `skipped` meaningful: skipped means non-support, while analyzed/no-gap means a real support conversation did not expose a durable coverage gap.

## Model And Migration

Update `server/internal/model/support_coverage_analysis.go`:

```go
IsSupportQuery       bool   `json:"is_support_query" gorm:"not null;default:true"`
ConversationType     string `json:"conversation_type" gorm:"not null;default:'support_query'"`
ClassificationReason string `json:"classification_reason" gorm:"type:text;not null;default:''"`
```

Create a dbmigrate SQL migration in `server/internal/dbmigrate/sql/`:

```sql
ALTER TABLE support_coverage_conversation_analyses
  ADD COLUMN IF NOT EXISTS is_support_query boolean NOT NULL DEFAULT true,
  ADD COLUMN IF NOT EXISTS conversation_type text NOT NULL DEFAULT 'support_query',
  ADD COLUMN IF NOT EXISTS classification_reason text NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_support_coverage_conversation_analyses_type
  ON support_coverage_conversation_analyses(workspace_id, conversation_type, created_at DESC);
```

Use dbmigrate rather than relying only on AutoMigrate because this table was introduced by versioned SQL and the project uses dbmigrate for durable schema changes.

## Analyzer Version

Bump `coverageAnalyzerVersion` from `v1` to `v2`.

Reason: `AlreadyAnalyzedConversation` keys by workspace, conversation, transcript hash, and analyzer version. Without a version bump, previously analyzed false positives will not be reclassified.

Expected side effect: recent historical candidates may be reprocessed. That is acceptable, but implementation should not attempt a broad cleanup of old false-positive gaps in this iteration. If cleanup is needed later, handle it as a separate reviewed task.

## Cost Controls

This design saves LLM calls only for clear deterministic skips. It also avoids extra calls by keeping classification inside the existing analyzer call.

Do not add:

- a separate classifier prompt
- ingestion-time classification
- embeddings/vector classification
- background reclassification jobs
- frontend reporting

Those are future options only if the local prefilter plus single-call LLM classification is not enough.

## Testing

Add focused tests in `support_coverage_daily_analyzer_test.go`:

- Newsletter transcript is locally skipped or LLM-classified as non-support, with no gap.
- Cold outreach transcript is non-support, with no gap.
- Auto-reply/bounce transcript is locally skipped, with no LLM call.
- Real support query remains `support_query` and gap analysis proceeds normally.
- Prospect pricing/security/integration question remains `support_query`.
- Transactional notification with no request is non-support.
- Transactional email followed by a customer asking for help remains `support_query`.
- Inconsistent LLM output is normalized safely.

Add persistence tests in `support_coverage_analysis_test.go`:

- `RecordConversationAnalysis` persists `is_support_query`, `conversation_type`, and `classification_reason`.
- Duplicate transcript behavior remains idempotent.

Update any SQLite test table definitions that manually create `support_coverage_conversation_analyses`.

## Future Considerations

The stored classification fields can later support:

- reporting on non-support inbox volume
- tuning local prefilter rules from observed false positives
- promoting conversation type onto `SupportConversation`
- cleaning up existing false-positive gaps

Do not include those in this implementation unless the initial classifier proves insufficient.
