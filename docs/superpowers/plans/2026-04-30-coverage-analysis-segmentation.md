# Coverage Analysis Segmentation Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make daily coverage-gap analysis operate on the latest lifecycle segment of a support conversation instead of the entire reopened thread, so unrelated issues in the same conversation do not contaminate gap detection.

**Architecture:** Use existing `support_messages` lifecycle system events (`resolved`, `reopened`) and public message chronology to derive conversation segments in memory. Analyze only the latest segment, filter retrieval traces to messages in that segment, and compute a segment-aware transcript hash so the existing `support_coverage_conversation_analyses` table can remain idempotent without a new schema table.

**Tech Stack:** Go 1.24, existing support inbox repositories, GORM models, current daily coverage analyzer, existing `SupportSystemEventType` constants.

---

## Design Summary

### Why Segment

Today the analyzer receives a whole conversation/thread. If a customer reopens a resolved thread with a different issue, the LLM can see unrelated historical messages and infer the wrong coverage gap.

We should keep the support UX as one conversation, but analyze only the latest support lifecycle segment:

- Segment 1: first public message -> resolved
- Segment 2: first public message after that resolution -> next resolved
- Segment 3: next reopened issue -> next resolved or current open state

### Important Edge Case

The email reopen fix creates the customer reply first and the internal `SystemEventReopened` after it. Therefore, segmentation must **not** define a segment as "messages after reopened event" only. It must treat the first public message after a resolved boundary as the start of the new segment, even if the `reopened` system event is written after that customer message.

### Storage Choice

Do **not** add a new table in this iteration.

Use:

- existing `support_coverage_conversation_analyses.conversation_id`
- existing `transcript_hash`, but make it segment-aware
- existing `analyzer_version`, bumped for the new semantics
- optional segment metadata in LLM input and evidence metadata

This lets one conversation have multiple analysis rows across different lifecycle segments because the transcript hash changes by segment.

### Scope

In scope:

- Build latest lifecycle segment from all conversation messages, including internal system events.
- Exclude internal messages from LLM transcript content as today.
- Filter retrieval traces to messages inside the selected segment.
- Compute segment-aware transcript hash.
- Add segment metadata to `CoverageConversationAnalysisInput`.
- Tell the LLM prompt that the provided transcript is the selected lifecycle segment, not necessarily the whole conversation.
- Bump analyzer version.
- Add focused tests.

Out of scope:

- Analyzing multiple historical segments in one run.
- Creating a new `support_coverage_conversation_segments` table.
- Frontend display of segments.
- Splitting support conversations.
- Backfilling old resolved/reopened system events.

---

## Files

- Modify: `server/internal/service/support_coverage_daily_analyzer.go`
  - Add segment types and segment selection helpers.
  - Update `runConversationCoverageAnalysis` to load internal messages.
  - Filter retrieval traces to selected segment.
  - Add segment metadata to analyzer input.
  - Change transcript hash generation to segment-aware.
  - Update prompt wording to respect segment boundaries.
  - Bump `coverageAnalyzerVersion`.

- Modify: `server/internal/service/support_coverage_daily_analyzer_test.go`
  - Add unit tests for segment selection and segment hash behavior.
  - Update manual SQLite table fixtures only if new code touches existing schema assumptions.
  - Add `slices` import if using `slices.Equal` for ID assertions.

- Modify: `server/internal/repository/support_coverage_analysis.go`
  - Optional: add `ListRetrievalTracesByConversationMessages` if filtering in repository is cleaner.
  - Preferred first pass: filter traces in service to avoid repository API churn.

---

### Task 1: Add Segment Model And Selection Tests

**Files:**
- Modify: `server/internal/service/support_coverage_daily_analyzer.go`
- Modify: `server/internal/service/support_coverage_daily_analyzer_test.go`

- [ ] **Step 1: Add segment types**

Add near the existing coverage analyzer input structs:

```go
type CoverageConversationSegment struct {
	ID              string
	StartMessageID  string
	EndMessageID    string
	StartAt         time.Time
	EndAt           *time.Time
	Resolved        bool
	PublicMessages  []model.SupportMessage
	PublicMessageID map[string]bool
}
```

Use a map for fast retrieval trace filtering.

- [ ] **Step 2: Write segment selection test: no lifecycle events**

Add:

```go
func TestBuildLatestCoverageConversationSegment_NoLifecycleEventsUsesAllPublicMessages(t *testing.T) {
	base := time.Date(2026, 4, 30, 9, 0, 0, 0, time.UTC)
	messages := []model.SupportMessage{
		{ID: "internal", SenderType: "user", MessageType: "reply", Content: "Internal", IsInternal: true, CreatedAt: base},
		{ID: "m-1", SenderType: "customer", MessageType: "reply", Content: "How do I reset password?", CreatedAt: base.Add(time.Minute)},
		{ID: "m-2", SenderType: "ai", MessageType: "reply", Content: "Try settings.", CreatedAt: base.Add(2 * time.Minute)},
	}

	segment := BuildLatestCoverageConversationSegment(messages)
	if segment == nil {
		t.Fatal("expected segment")
	}
	if segment.StartMessageID != "m-1" || segment.EndMessageID != "m-2" {
		t.Fatalf("unexpected segment bounds: %+v", segment)
	}
	if segment.Resolved {
		t.Fatal("segment should not be resolved")
	}
	if len(segment.PublicMessages) != 2 {
		t.Fatalf("public messages = %d, want 2", len(segment.PublicMessages))
	}
}
```

- [ ] **Step 3: Write segment selection test: resolved then reopened**

This catches the critical ordering edge case where the customer reply can occur before the `reopened` system event.

```go
func TestBuildLatestCoverageConversationSegment_ReopenedThreadStartsAtFirstPublicMessageAfterResolved(t *testing.T) {
	base := time.Date(2026, 4, 30, 9, 0, 0, 0, time.UTC)
	resolved := string(model.SystemEventResolved)
	reopened := string(model.SystemEventReopened)
	messages := []model.SupportMessage{
		{ID: "old-customer", SenderType: "customer", MessageType: "reply", Content: "Refund question", CreatedAt: base},
		{ID: "old-agent", SenderType: "user", MessageType: "reply", Content: "Refunded", CreatedAt: base.Add(time.Minute)},
		{ID: "resolved-1", SenderType: "user", MessageType: "system", SystemEventType: &resolved, IsInternal: true, Content: "Resolved conversation", CreatedAt: base.Add(2 * time.Minute)},
		{ID: "new-customer", SenderType: "customer", MessageType: "reply", Content: "Now I need SSO help", CreatedAt: base.Add(3 * time.Minute)},
		{ID: "reopened-1", SenderType: "user", MessageType: "system", SystemEventType: &reopened, IsInternal: true, Content: "Reopened conversation", CreatedAt: base.Add(4 * time.Minute)},
		{ID: "new-ai", SenderType: "ai", MessageType: "reply", Content: "Let me check SSO docs.", CreatedAt: base.Add(5 * time.Minute)},
	}

	segment := BuildLatestCoverageConversationSegment(messages)
	if segment == nil {
		t.Fatal("expected segment")
	}
	gotIDs := coverageTestMessageIDs(segment.PublicMessages)
	wantIDs := []string{"new-customer", "new-ai"}
	if !slices.Equal(gotIDs, wantIDs) {
		t.Fatalf("segment messages = %v, want %v", gotIDs, wantIDs)
	}
	if segment.StartMessageID != "new-customer" {
		t.Fatalf("start = %q, want new-customer", segment.StartMessageID)
	}
}
```

Add small test helper:

```go
func coverageTestMessageIDs(messages []model.SupportMessage) []string {
	ids := make([]string, 0, len(messages))
	for _, message := range messages {
		ids = append(ids, message.ID)
	}
	return ids
}
```

- [ ] **Step 4: Write segment selection test: latest closed segment**

When the current conversation is resolved and no newer public messages exist, the latest segment should be the just-resolved segment, not empty.

```go
func TestBuildLatestCoverageConversationSegment_ResolvedConversationUsesLatestClosedSegment(t *testing.T) {
	base := time.Date(2026, 4, 30, 9, 0, 0, 0, time.UTC)
	resolved := string(model.SystemEventResolved)
	messages := []model.SupportMessage{
		{ID: "m-1", SenderType: "customer", MessageType: "reply", Content: "How do I invite a user?", CreatedAt: base},
		{ID: "m-2", SenderType: "user", MessageType: "reply", Content: "Use Settings > Members.", CreatedAt: base.Add(time.Minute)},
		{ID: "resolved-1", SenderType: "user", MessageType: "system", SystemEventType: &resolved, IsInternal: true, Content: "Resolved conversation", CreatedAt: base.Add(2 * time.Minute)},
	}

	segment := BuildLatestCoverageConversationSegment(messages)
	if segment == nil {
		t.Fatal("expected segment")
	}
	if !segment.Resolved {
		t.Fatal("expected latest segment to be marked resolved")
	}
	gotIDs := coverageTestMessageIDs(segment.PublicMessages)
	wantIDs := []string{"m-1", "m-2"}
	if !slices.Equal(gotIDs, wantIDs) {
		t.Fatalf("segment messages = %v, want %v", gotIDs, wantIDs)
	}
}
```

- [ ] **Step 5: Run tests and confirm they fail**

Run:

```bash
cd server
go test ./internal/service -run 'TestBuildLatestCoverageConversationSegment' -count=1
```

Expected: FAIL because `BuildLatestCoverageConversationSegment` does not exist.

---

### Task 2: Implement Latest Segment Builder

**Files:**
- Modify: `server/internal/service/support_coverage_daily_analyzer.go`

- [ ] **Step 1: Implement `BuildLatestCoverageConversationSegment`**

Add:

```go
func BuildLatestCoverageConversationSegment(messages []model.SupportMessage) *CoverageConversationSegment {
	ordered := sortedCoverageMessages(messages)
	var segments []CoverageConversationSegment
	var current *CoverageConversationSegment

	ensureCurrent := func(message model.SupportMessage) {
		if current != nil {
			return
		}
		current = &CoverageConversationSegment{
			ID:              message.ID,
			StartMessageID:  message.ID,
			StartAt:         message.CreatedAt.UTC(),
			PublicMessageID: map[string]bool{},
		}
	}

	closeCurrent := func(event model.SupportMessage) {
		if current == nil || len(current.PublicMessages) == 0 {
			return
		}
		endAt := event.CreatedAt.UTC()
		current.EndAt = &endAt
		current.EndMessageID = event.ID
		current.Resolved = true
		segments = append(segments, *current)
		current = nil
	}

	for _, message := range ordered {
		if isCoverageResolvedSystemMessage(message) {
			closeCurrent(message)
			continue
		}
		if message.IsInternal || strings.TrimSpace(message.MessageType) == "system" {
			continue
		}
		ensureCurrent(message)
		current.PublicMessages = append(current.PublicMessages, message)
		current.PublicMessageID[message.ID] = true
		current.EndMessageID = message.ID
	}

	if current != nil && len(current.PublicMessages) > 0 {
		segments = append(segments, *current)
	}
	if len(segments) == 0 {
		return nil
	}
	segment := segments[len(segments)-1]
	if segment.PublicMessageID == nil {
		segment.PublicMessageID = map[string]bool{}
		for _, message := range segment.PublicMessages {
			segment.PublicMessageID[message.ID] = true
		}
	}
	segment.ID = coverageSegmentID(segment)
	return &segment
}
```

- [ ] **Step 2: Implement helper predicates**

Add:

```go
func isCoverageResolvedSystemMessage(message model.SupportMessage) bool {
	if strings.TrimSpace(message.MessageType) != "system" || message.SystemEventType == nil {
		return false
	}
	return strings.TrimSpace(*message.SystemEventType) == model.SystemEventResolved
}

func coverageSegmentID(segment CoverageConversationSegment) string {
	start := strings.TrimSpace(segment.StartMessageID)
	end := strings.TrimSpace(segment.EndMessageID)
	if start == "" && len(segment.PublicMessages) > 0 {
		start = segment.PublicMessages[0].ID
	}
	if end == "" && len(segment.PublicMessages) > 0 {
		end = segment.PublicMessages[len(segment.PublicMessages)-1].ID
	}
	if end == "" {
		end = "open"
	}
	return start + ":" + end
}
```

Do not use `SystemEventReopened` as a hard start boundary. It is useful metadata, but customer replies may be written before the reopened event.

- [ ] **Step 3: Run segment tests**

Run:

```bash
cd server
go test ./internal/service -run 'TestBuildLatestCoverageConversationSegment' -count=1
```

Expected: PASS.

---

### Task 3: Add Segment Metadata To Analyzer Input And Hash

**Files:**
- Modify: `server/internal/service/support_coverage_daily_analyzer.go`
- Modify: `server/internal/service/support_coverage_daily_analyzer_test.go`

- [ ] **Step 1: Extend `CoverageConversationAnalysisInput`**

Add fields:

```go
SegmentID             string     `json:"segment_id"`
SegmentStartMessageID string     `json:"segment_start_message_id"`
SegmentEndMessageID   string     `json:"segment_end_message_id"`
SegmentStartAt        *time.Time `json:"segment_start_at,omitempty"`
SegmentEndAt          *time.Time `json:"segment_end_at,omitempty"`
SegmentResolved       bool       `json:"segment_resolved"`
```

These tell the LLM the transcript is a deliberate segment, not the whole conversation.

- [ ] **Step 2: Add segment hash helper test**

Add:

```go
func TestCoverageSegmentTranscriptHashIncludesSegmentBoundary(t *testing.T) {
	base := time.Date(2026, 4, 30, 9, 0, 0, 0, time.UTC)
	messages := []model.SupportMessage{
		{ID: "m-1", SenderType: "customer", MessageType: "reply", Content: "Question", CreatedAt: base},
	}
	first := CoverageConversationSegment{ID: "m-1:resolved-1", StartMessageID: "m-1", EndMessageID: "resolved-1", PublicMessages: messages}
	second := CoverageConversationSegment{ID: "m-1:m-1", StartMessageID: "m-1", EndMessageID: "m-1", PublicMessages: messages}

	if CoverageSegmentTranscriptHash(first) == CoverageSegmentTranscriptHash(second) {
		t.Fatal("segment boundary should affect transcript hash")
	}
}
```

- [ ] **Step 3: Implement `CoverageSegmentTranscriptHash`**

Add:

```go
func CoverageSegmentTranscriptHash(segment CoverageConversationSegment) string {
	baseHash := CoverageTranscriptHash(segment.PublicMessages)
	fields := []string{
		strings.TrimSpace(segment.ID),
		strings.TrimSpace(segment.StartMessageID),
		strings.TrimSpace(segment.EndMessageID),
		baseHash,
	}
	if !segment.StartAt.IsZero() {
		fields = append(fields, segment.StartAt.UTC().Format(time.RFC3339Nano))
	}
	if segment.EndAt != nil && !segment.EndAt.IsZero() {
		fields = append(fields, segment.EndAt.UTC().Format(time.RFC3339Nano))
	}
	sum := sha256.Sum256([]byte(strings.Join(fields, "\x1f")))
	return hex.EncodeToString(sum[:])
}
```

- [ ] **Step 4: Replace `BuildCoverageConversationAnalysisInput` internals**

Keep the public function name, but have it select the latest segment:

```go
func BuildCoverageConversationAnalysisInput(conversation model.SupportConversation, messages []model.SupportMessage, traces []model.SupportAIRetrievalTrace) (CoverageConversationAnalysisInput, error) {
	segment := BuildLatestCoverageConversationSegment(messages)
	if segment == nil {
		return CoverageConversationAnalysisInput{}, fmt.Errorf("conversation has no public messages to analyze")
	}
	return BuildCoverageConversationAnalysisInputForSegment(conversation, *segment, traces)
}
```

Add:

```go
func BuildCoverageConversationAnalysisInputForSegment(conversation model.SupportConversation, segment CoverageConversationSegment, traces []model.SupportAIRetrievalTrace) (CoverageConversationAnalysisInput, error) {
	orderedMessages := sortedCoverageMessages(segment.PublicMessages)
	hash := CoverageSegmentTranscriptHash(segment)
	analysisMessages := make([]CoverageConversationMessage, 0, len(orderedMessages))
	// existing message conversion loop, using orderedMessages
	// keep the IsInternal/system-message guard as defense-in-depth even though segment.PublicMessages should already be public-only
	// existing max-message truncation
	// filter traces to the final analysisMessages IDs before coverageRetrievalTraceInputs
}
```

When moving the existing conversion loop, keep this guard in the segment variant:

```go
for _, message := range orderedMessages {
	if message.IsInternal || strings.TrimSpace(message.MessageType) == "system" {
		continue
	}
	// existing CoverageConversationMessage construction
}
```

This is intentionally redundant with `BuildLatestCoverageConversationSegment`; it protects the LLM input if a future caller builds a segment manually.

In the result, set the new segment fields:

```go
input := CoverageConversationAnalysisInput{
	WorkspaceID:             conversation.WorkspaceID,
	ConversationID:          conversation.ID,
	Subject:                 strings.TrimSpace(conversation.Subject),
	Status:                  strings.TrimSpace(conversation.Status),
	FlowState:               flowState,
	AITurnCount:             conversation.AITurnCount,
	TranscriptHash:          hash,
	SegmentID:               segment.ID,
	SegmentStartMessageID:   segment.StartMessageID,
	SegmentEndMessageID:     segment.EndMessageID,
	SegmentStartAt:          timePtrIfNonZero(segment.StartAt),
	SegmentEndAt:            segment.EndAt,
	SegmentResolved:         segment.Resolved,
	Messages:                analysisMessages,
	RetrievalTraces:         traceInputs,
}
```

Add `timePtrIfNonZero` if no equivalent helper exists:

```go
func timePtrIfNonZero(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	utc := t.UTC()
	return &utc
}
```

- [ ] **Step 5: Filter retrieval traces to messages actually sent to the LLM**

Do not keep traces for segment messages that are later dropped by `coverageAnalysisMaxMessages`. Retrieval traces should match the final `analysisMessages` slice to avoid old search context leaking into the segment analysis.

After truncating `analysisMessages`, build a sent-message map:

```go
analysisMessageIDs := map[string]bool{}
for _, message := range analysisMessages {
	analysisMessageIDs[strings.TrimSpace(message.ID)] = true
}
```

Add:

```go
func filterCoverageRetrievalTracesByMessageIDs(traces []model.SupportAIRetrievalTrace, messageIDs map[string]bool) []model.SupportAIRetrievalTrace {
	if len(traces) == 0 || len(messageIDs) == 0 {
		return nil
	}
	filtered := make([]model.SupportAIRetrievalTrace, 0, len(traces))
	for _, trace := range traces {
		if messageIDs[strings.TrimSpace(trace.MessageID)] {
			filtered = append(filtered, trace)
		}
	}
	return filtered
}
```

Use this before `coverageRetrievalTraceInputs`.

- [ ] **Step 6: Update prompt wording**

In `coverageConversationAnalysisSystemPrompt`, add a short instruction near the top:

```text
The provided messages may be only the latest lifecycle segment of a longer conversation.
Do not infer gaps from earlier issues that are not present in the provided segment.
Use segment_id and segment boundary fields only as analysis context.
```

Do not ask the LLM to segment. Segmentation is deterministic server-side behavior.

- [ ] **Step 7: Run input/hash tests**

Run:

```bash
cd server
go test ./internal/service -run 'TestCoverageConversationAnalysisInput|TestCoverageSegmentTranscriptHash|TestBuildLatestCoverageConversationSegment' -count=1
```

Expected: update existing assertions as needed, then PASS.

---

### Task 4: Update Analyzer Runtime Path

**Files:**
- Modify: `server/internal/service/support_coverage_daily_analyzer.go`

- [ ] **Step 1: Load internal messages for boundary detection**

In `runConversationCoverageAnalysis`, change:

```go
messages, err := s.messageRepo.ListByConversation(ctx, workspaceID, conversation.ID, false)
```

to:

```go
messages, err := s.messageRepo.ListByConversation(ctx, workspaceID, conversation.ID, true)
```

`BuildCoverageConversationAnalysisInput` still excludes internal/system messages from LLM content, but it needs them to find lifecycle boundaries.

- [ ] **Step 2: Handle no public segment gracefully**

If `BuildCoverageConversationAnalysisInput` returns the no-public-messages error, record a skipped analysis if possible or return `false, nil`.

Preferred first pass:

```go
input, err := BuildCoverageConversationAnalysisInput(conversation, messages, traces)
if err != nil {
	if errors.Is(err, errCoverageNoPublicSegment) {
		return false, nil
	}
	return false, err
}
```

Define:

```go
var errCoverageNoPublicSegment = errors.New("coverage conversation has no public segment")
```

Use `fmt.Errorf("%w", errCoverageNoPublicSegment)` if wrapping.

- [ ] **Step 3: Add segment metadata to gap evidence**

In `UpsertFinding`, merge these fields into the existing `evidenceMetadata` map that currently includes `customer_need`, `ai_failure`, `human_resolution`, `decision_reason`, `recommended_fixes`, `conversation_analysis_id`, and `matched_knowledge_candidates`:

```go
"segment_id": input.SegmentID,
"segment_start_message_id": input.SegmentStartMessageID,
"segment_end_message_id": input.SegmentEndMessageID,
"segment_resolved": input.SegmentResolved,
```

This helps explain why evidence may include only a later portion of a long conversation.

- [ ] **Step 4: Bump analyzer version**

Change:

```go
coverageAnalyzerVersion = "v2"
```

to:

```go
coverageAnalyzerVersion = "v3"
```

Reason: transcript hash semantics and LLM input changed. Previously analyzed conversations need to be eligible for segment-aware reanalysis.

- [ ] **Step 5: Run focused analyzer tests**

Run:

```bash
cd server
go test ./internal/service -run 'TestCoverageConversationAnalysisInput|TestSupportCoverageDailyAnalyzer_|TestClassifyCoverageConversationLocally|TestNormalizeCoverageConversationAnalysisResult|TestBuildLatestCoverageConversationSegment|TestCoverageSegmentTranscriptHash' -count=1
```

Expected: PASS after updating expected hashes/counts.

---

### Task 5: Add End-To-End Regression Around Reopened Thread

**Files:**
- Modify: `server/internal/service/support_coverage_daily_analyzer_test.go`

- [ ] **Step 1: Add input-building regression**

Add a test that proves an old issue is excluded from LLM input:

```go
func TestCoverageConversationAnalysisInputUsesLatestReopenedSegment(t *testing.T) {
	base := time.Date(2026, 4, 30, 9, 0, 0, 0, time.UTC)
	resolved := string(model.SystemEventResolved)
	reopened := string(model.SystemEventReopened)
	conversation := model.SupportConversation{
		ID:          "conversation-1",
		WorkspaceID: "ws-1",
		Subject:     "Mixed thread",
		Status:      model.SupportConversationStatusOpen,
	}
	messages := []model.SupportMessage{
		{ID: "old-customer", WorkspaceID: "ws-1", ConversationID: "conversation-1", SenderType: "customer", MessageType: "reply", Content: "Refund issue", CreatedAt: base},
		{ID: "old-agent", WorkspaceID: "ws-1", ConversationID: "conversation-1", SenderType: "user", MessageType: "reply", Content: "Refunded", CreatedAt: base.Add(time.Minute)},
		{ID: "resolved-1", WorkspaceID: "ws-1", ConversationID: "conversation-1", SenderType: "user", MessageType: "system", SystemEventType: &resolved, IsInternal: true, Content: "Resolved conversation", CreatedAt: base.Add(2 * time.Minute)},
		{ID: "new-customer", WorkspaceID: "ws-1", ConversationID: "conversation-1", SenderType: "customer", MessageType: "reply", Content: "SSO setup is failing", CreatedAt: base.Add(3 * time.Minute)},
		{ID: "reopened-1", WorkspaceID: "ws-1", ConversationID: "conversation-1", SenderType: "user", MessageType: "system", SystemEventType: &reopened, IsInternal: true, Content: "Reopened conversation", CreatedAt: base.Add(4 * time.Minute)},
		{ID: "new-ai", WorkspaceID: "ws-1", ConversationID: "conversation-1", SenderType: "ai", MessageType: "reply", Content: "Try SAML settings.", CreatedAt: base.Add(5 * time.Minute)},
	}

	input, err := BuildCoverageConversationAnalysisInput(conversation, messages, nil)
	if err != nil {
		t.Fatalf("BuildCoverageConversationAnalysisInput: %v", err)
	}
	gotIDs := make([]string, 0, len(input.Messages))
	for _, message := range input.Messages {
		gotIDs = append(gotIDs, message.ID)
		if strings.Contains(message.Content, "Refund") {
			t.Fatalf("old segment leaked into analyzer input: %+v", input.Messages)
		}
	}
	wantIDs := []string{"new-customer", "new-ai"}
	if !slices.Equal(gotIDs, wantIDs) {
		t.Fatalf("input message ids = %v, want %v", gotIDs, wantIDs)
	}
	if input.SegmentID == "" || input.SegmentStartMessageID != "new-customer" {
		t.Fatalf("missing segment metadata: %+v", input)
	}
}
```

- [ ] **Step 2: Add retrieval-trace filtering regression**

Add:

```go
func TestCoverageConversationAnalysisInputFiltersRetrievalTracesToSentSegmentMessages(t *testing.T) {
	base := time.Date(2026, 4, 30, 9, 0, 0, 0, time.UTC)
	resolved := string(model.SystemEventResolved)
	conversation := model.SupportConversation{ID: "conversation-1", WorkspaceID: "ws-1"}
	messages := []model.SupportMessage{
		{ID: "old-ai", WorkspaceID: "ws-1", ConversationID: "conversation-1", SenderType: "ai", MessageType: "reply", Content: "Old answer", CreatedAt: base},
		{ID: "resolved-1", WorkspaceID: "ws-1", ConversationID: "conversation-1", SenderType: "user", MessageType: "system", SystemEventType: &resolved, IsInternal: true, Content: "Resolved", CreatedAt: base.Add(time.Minute)},
		{ID: "new-customer", WorkspaceID: "ws-1", ConversationID: "conversation-1", SenderType: "customer", MessageType: "reply", Content: "New issue", CreatedAt: base.Add(2 * time.Minute)},
		{ID: "new-ai", WorkspaceID: "ws-1", ConversationID: "conversation-1", SenderType: "ai", MessageType: "reply", Content: "New answer", CreatedAt: base.Add(3 * time.Minute)},
	}
	traces := []model.SupportAIRetrievalTrace{
		{MessageID: "old-ai", SearchQueries: json.RawMessage(`["refund"]`), Results: json.RawMessage(`[]`), CitedSourceIDs: json.RawMessage(`[]`)},
		{MessageID: "new-ai", SearchQueries: json.RawMessage(`["sso"]`), Results: json.RawMessage(`[]`), CitedSourceIDs: json.RawMessage(`[]`)},
	}

	input, err := BuildCoverageConversationAnalysisInput(conversation, messages, traces)
	if err != nil {
		t.Fatalf("BuildCoverageConversationAnalysisInput: %v", err)
	}
	if len(input.RetrievalTraces) != 1 || input.RetrievalTraces[0].MessageID != "new-ai" {
		t.Fatalf("retrieval traces were not filtered to sent segment messages: %+v", input.RetrievalTraces)
	}
}
```

- [ ] **Step 3: Run regressions**

Run:

```bash
cd server
go test ./internal/service -run 'TestCoverageConversationAnalysisInputUsesLatestReopenedSegment|TestCoverageConversationAnalysisInputFiltersRetrievalTracesToSentSegmentMessages' -count=1
```

Expected: PASS.

---

### Task 6: Verification

**Files:**
- Verify only.

- [ ] **Step 1: Format changed files**

Run:

```bash
cd server
gofmt -w internal/service/support_coverage_daily_analyzer.go internal/service/support_coverage_daily_analyzer_test.go
```

- [ ] **Step 2: Run focused analyzer tests**

Run:

```bash
cd server
go test ./internal/service -run 'TestCoverageConversationAnalysisInput|TestSupportCoverageDailyAnalyzer_|TestClassifyCoverageConversationLocally|TestNormalizeCoverageConversationAnalysisResult|TestBuildLatestCoverageConversationSegment|TestCoverageSegmentTranscriptHash' -count=1
```

Expected: PASS.

- [ ] **Step 3: Run repository tests only if modified**

If `server/internal/repository/support_coverage_analysis.go` was modified:

```bash
cd server
go test ./internal/repository -run 'TestSupportCoverageAnalysisRepository' -count=1
```

Expected: PASS.

- [ ] **Step 4: Run migration validation**

No migration should be required. Still run:

```bash
cd server
go test ./internal/dbmigrate -count=1
```

Expected: PASS.

---

## Risks And Decisions

- **Why not a new segment table?** Not needed for first value. Existing `transcript_hash` plus analyzer version gives idempotency for the latest segment.
- **Why not analyze every historical segment?** That would multiply LLM cost and complicate gap dedupe. Daily analyzer should focus on the latest eligible segment.
- **Why not use `reopened` as the segment start?** Email reopen writes the customer message before the reopened system event. The first public message after a resolved event is the reliable start.
- **What about old conversations without system events?** They remain one segment, preserving current behavior.
- **What about resolved conversations?** The latest closed segment is analyzed, so human-resolved issues are still eligible for gap detection.
- **What about reopened open conversations?** The current active segment is analyzed without prior resolved segments.
