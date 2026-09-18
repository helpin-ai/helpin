# Reopen support conversations from email replies

This historical plan is for contributors investigating the original implementation. Use the source review below to distinguish the current behavior from the original proposal; the remaining checklist is historical, not a fresh execution instruction.

## Source review — 2026-09-18

- [The email service](../../server/internal/service/email_fallback.go) now accepts inbound replies to resolved conversations. Customer replies reopen both `resolved` and `waiting_on_customer` conversations; detected email notices do not trigger that transition. Teammate replies to resolved conversations instead move them to `waiting_on_customer`.
- The customer message, email log, state update, and resolved-only internal reopened message use transaction-scoped repositories. The update clears `resolved_at`, `closed_at`, `ai_resolved_at`, and `ai_resolution_type`. Attachments and post-commit notifications/events have separate side effects; this is not an atomic guarantee for all external work.
- Reopened flow defaults to human assignment when an assigned/opening user or human-assigned flow exists, otherwise waiting for a human. Eligible automatic AI processing can select `ai_handling`; the helper in the original proposal is no longer the exact implementation.
- Spam remains inbound-terminal; resolved and spam remain outbound-terminal for delayed fallback. Message and conversation WebSocket updates happen after the transaction. AI dispatch can return a retry error after the message is saved.
- [Coverage analysis](../../server/internal/service/support_coverage_daily_analyzer.go) already selects the latest lifecycle segment and includes segment boundaries in its transcript hash. The follow-up at the end is historical, not an unimplemented prerequisite.
- [Regression tests](../../server/internal/service/email_fallback_test.go) cover resolved/waiting reopen and spam behavior. They were inspected, not executed in this documentation review. The current [Go module](../../server/go.mod) declares Go 1.25.0; the Go 1.24 label and expected failures below describe the original plan.

## Original plan


**Goal:** Customer email replies to resolved support conversations should be accepted, appended to the same conversation, and reopen it instead of being silently ignored.

**Architecture:** Treat the current `resolved` state like Zendesk `solved` / Intercom replyable closed conversations. Keep `spam` terminal, but allow inbound email replies for `resolved`, then reuse the same reopen semantics already used by customer replies in `SupportInboxService.CreateConversationMessage`: status back to `open`, `resolved_at` cleared, flow state restored, notifications sent, and a `SystemEventReopened` audit/system message created.

**Tech Stack:** Go 1.24, GORM, existing support inbox repositories/services, Postmark inbound email webhook path, existing `support_messages` system event model.

---

## Scope

This plan fixes the critical email data-loss issue only.

In scope:
- Email reply to `resolved` conversation creates a support message.
- Conversation reopens to `open`.
- `resolved_at` and `closed_at` are cleared.
- A `SystemEventReopened` internal system message is recorded.
- Existing inbound email logs, webhook events, websocket publish, notifications, and route touch behavior continue to work.
- `spam` remains terminal and ignored.

Out of scope:
- Creating a new linked conversation after N days.
- Adding a true `closed`/`archived` lifecycle state.
- Coverage-gap segmentation by resolved/reopened boundaries.
- Frontend UI changes beyond existing system-message rendering.

## Files

- Modify: `server/internal/service/email_fallback.go`
  - Change terminal-status behavior for inbound replies.
  - Add a small helper to reopen resolved conversations from email replies.
  - Add a helper to create the internal `SystemEventReopened` system message.

- Modify: `server/internal/service/email_fallback_test.go`
  - Add regression coverage for resolved email replies.
  - Update terminal-status expectations.
  - Confirm spam remains ignored.

- Reference only: `server/internal/service/support_inbox.go`
  - Existing customer reply reopen behavior around `CreateConversationMessage`.
  - Existing system-message creation behavior in `UpdateConversationStatus`.

- Reference only: `server/internal/model/support_system_event.go`
  - Existing `SystemEventReopened` event type.

---

### Task 1: Add Regression Test For Resolved Email Reply Reopen

**Files:**
- Modify: `server/internal/service/email_fallback_test.go`

- [ ] **Step 1: Write a failing test**

Add a test near `TestEmailFallbackProcessInboundEmailCreatesMessageAndDedupes`:

```go
func TestEmailFallbackProcessInboundEmailReopensResolvedConversation(t *testing.T) {
	settings := model.DefaultSupportInboxSettings()
	env := setupEmailFallbackInboundTestEnv(t, settings)
	ctx := context.Background()

	workspaceID := "11111111-1111-1111-1111-111111111111"
	customerEmail := "customer@example.com"
	customerName := "Customer"
	resolvedAt := time.Date(2026, 4, 30, 8, 0, 0, 0, time.UTC)
	flowState := model.SupportConversationFlowStateResolvedByHuman
	conversationID := "33333333-3333-3333-3333-333333333333"
	conversation := &model.SupportConversation{
		ID:            conversationID,
		WorkspaceID:   workspaceID,
		Subject:       "Billing help",
		Status:        model.SupportConversationStatusResolved,
		FlowState:     &flowState,
		CustomerEmail: &customerEmail,
		CustomerName:  &customerName,
		Source:        "email",
		ResolvedAt:    &resolvedAt,
	}
	if err := env.convRepo.Create(ctx, conversation); err != nil {
		t.Fatalf("create resolved conversation: %v", err)
	}

	payload := model.PostmarkInboundPayload{
		MessageID:         "pm-reopen-1",
		MessageStream:     "inbound",
		OriginalRecipient: "conv-" + conversationID + "@replies.helpin.ai",
		To:                "conv-" + conversationID + "@replies.helpin.ai",
		From:              customerEmail,
		FromFull:          model.PostmarkAddress{Name: customerName, Email: customerEmail},
		Subject:           "Re: Billing help",
		StrippedTextReply: "I still need help with this invoice.",
	}
	rawPayload := `{"MessageStream":"inbound","MessageID":"pm-reopen-1"}`

	if err := env.service.ProcessInboundEmail(ctx, payload, rawPayload); err != nil {
		t.Fatalf("process inbound email: %v", err)
	}

	updated, err := env.convRepo.GetByID(ctx, workspaceID, conversationID, "", model.RoleOwner)
	if err != nil {
		t.Fatalf("load updated conversation: %v", err)
	}
	if updated.Status != model.SupportConversationStatusOpen {
		t.Fatalf("status = %q, want open", updated.Status)
	}
	if updated.ResolvedAt != nil {
		t.Fatalf("resolved_at = %v, want nil", updated.ResolvedAt)
	}
	if updated.ClosedAt != nil {
		t.Fatalf("closed_at = %v, want nil", updated.ClosedAt)
	}
	if updated.FlowState == nil || *updated.FlowState == model.SupportConversationFlowStateResolvedByHuman {
		t.Fatalf("flow_state was not restored: %#v", updated.FlowState)
	}

	messages, err := env.messageRepo.ListByConversation(ctx, workspaceID, conversationID, true)
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	var customerReplies, reopenedEvents int
	for _, msg := range messages {
		if msg.SenderType == "customer" && msg.MessageType == "reply" && strings.Contains(msg.Content, "invoice") {
			customerReplies++
		}
		if msg.MessageType == "system" && msg.SystemEventType != nil && *msg.SystemEventType == string(model.SystemEventReopened) {
			reopenedEvents++
			if !msg.IsInternal {
				t.Fatal("reopened system message should be internal")
			}
		}
	}
	if customerReplies != 1 {
		t.Fatalf("customer replies = %d, want 1", customerReplies)
	}
	if reopenedEvents != 1 {
		t.Fatalf("reopened system events = %d, want 1", reopenedEvents)
	}

	logs, err := env.emailLogRepo.ListByConversation(ctx, workspaceID, conversationID)
	if err != nil {
		t.Fatalf("list email logs: %v", err)
	}
	if len(logs) != 1 || logs[0].Direction != "inbound" {
		t.Fatalf("expected one inbound email log, got %+v", logs)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
cd server
go test ./internal/service -run TestEmailFallbackProcessInboundEmailReopensResolvedConversation -count=1
```

Expected: FAIL because resolved conversations are currently ignored by `isEmailFallbackTerminalStatus`.

---

### Task 2: Split Inbound And Outbound Terminal Status Semantics

**Files:**
- Modify: `server/internal/service/email_fallback.go`
- Modify: `server/internal/service/email_fallback_test.go`

- [ ] **Step 1: Add an inbound-specific terminal helper**

Do not change outbound fallback semantics globally. The existing `isEmailFallbackTerminalStatus` helper is used by outbound fallback enqueue/fire paths, where `resolved` should continue to suppress sending delayed emails.

Add an inbound-specific helper near the existing helper:

```go
func isEmailFallbackInboundTerminalStatus(status string) bool {
	switch model.NormalizeSupportConversationStatus(status) {
	case model.SupportConversationStatusSpam:
		return true
	default:
		return false
	}
}
```

Leave `isEmailFallbackTerminalStatus` unchanged for outbound code unless you rename it to `isEmailFallbackOutboundTerminalStatus` and update all outbound call sites. If renamed, keep its current behavior: `resolved` and `spam` are outbound-terminal.

- [ ] **Step 2: Use the inbound helper in `processInboundConversationReply`**

Change the guard at the top of `processInboundConversationReply`:

```go
if isEmailFallbackInboundTerminalStatus(conv.Status) {
	s.logger.InfoContext(ctx, "postmark inbound ignored for terminal conversation",
		"message_id", strings.TrimSpace(payload.MessageID),
		"conversation_id", conv.ID,
		"status", conv.Status,
	)
	return nil
}
```

- [ ] **Step 3: Add inbound terminal-status tests**

Add a focused test near `TestIsEmailFallbackTerminalStatus`:

```go
func TestIsEmailFallbackInboundTerminalStatus(t *testing.T) {
	if isEmailFallbackInboundTerminalStatus("resolved") {
		t.Fatal("resolved should accept inbound replies and reopen")
	}
	if !isEmailFallbackInboundTerminalStatus("spam") {
		t.Fatal("spam should be terminal")
	}
	if isEmailFallbackInboundTerminalStatus("open") {
		t.Fatal("open should not be terminal")
	}
}
```

Keep `TestIsEmailFallbackTerminalStatus` asserting the outbound behavior:

```go
if !isEmailFallbackTerminalStatus("resolved") {
	t.Fatal("resolved should remain terminal for outbound fallback")
}
if !isEmailFallbackTerminalStatus("spam") {
	t.Fatal("spam should be terminal")
}
```

- [ ] **Step 4: Run focused helper tests**

Run:

```bash
cd server
go test ./internal/service -run 'TestIsEmailFallbackTerminalStatus|TestIsEmailFallbackInboundTerminalStatus|TestEmailFallbackProcessInboundEmailReopensResolvedConversation' -count=1
```

Expected: the new reopen test may still fail until Task 3 adds the reopen update/system event; terminal helper expectations should pass.

---

### Task 3: Reopen Conversation In The Email Reply Transaction

**Files:**
- Modify: `server/internal/service/email_fallback.go`

- [ ] **Step 1: Add a helper to compute reopened flow state**

Near other email fallback helpers, add:

```go
func supportEmailReopenFlowState(conv *model.SupportConversation) string {
	if conv != nil && conv.HumanTakeover != nil && *conv.HumanTakeover {
		return model.SupportConversationFlowStateAssignedToHuman
	}
	if conv == nil {
		return model.SupportConversationFlowStateWaitingForHuman
	}
	return defaultConversationFlowState(conv.OpenedByUserID, conv.AssignedUserID, conv.AssignedAgentID)
}
```

This mirrors the existing customer reply path in `CreateConversationMessage`.

- [ ] **Step 2: Add a helper to create reopened system message**

Add:

```go
func createEmailReopenedSystemMessage(ctx context.Context, msgRepo *repository.SupportMessageRepository, conv *model.SupportConversation) error {
	if msgRepo == nil || conv == nil {
		return nil
	}
	sysMsg := &model.SupportMessage{
		WorkspaceID:     conv.WorkspaceID,
		ConversationID:  conv.ID,
		SenderType:      "user",
		Content:         "Reopened conversation",
		MessageType:     "system",
		SystemEventType: model.SupportSystemEventTypeStrPtr(model.SystemEventReopened),
		IsInternal:      true,
	}
	return msgRepo.Create(ctx, sysMsg)
}
```

Keep it internal so the customer-facing thread does not show an admin-only lifecycle marker.

- [ ] **Step 3: Update `processInboundConversationReply` transaction**

Inside the existing transaction, after creating the inbound `SupportMessage` and `SupportEmailLog`, add reopen behavior only when the original conversation status was resolved:

```go
wasResolved := model.NormalizeSupportConversationStatus(conv.Status) == model.SupportConversationStatusResolved
```

Inside the transaction:

```go
if wasResolved {
	reopenFlowState := supportEmailReopenFlowState(conv)
	if err := convRepoTx.UpdateFields(ctx, conv.WorkspaceID, conv.ID, map[string]any{
		"status":      model.SupportConversationStatusOpen,
		"flow_state":  reopenFlowState,
		"resolved_at": nil,
		"closed_at":   nil,
		"updated_at":  s.now(),
	}); err != nil {
		return err
	}
	conv.Status = model.SupportConversationStatusOpen
	conv.FlowState = strPtr(reopenFlowState)
	conv.ResolvedAt = nil
	conv.ClosedAt = nil
	if err := createEmailReopenedSystemMessage(ctx, msgRepoTx, conv); err != nil {
		return err
	}
}
```

Use the transaction-scoped repositories:

```go
convRepoTx := s.convRepo.WithTx(tx)
msgRepoTx := s.messageRepo.WithTx(tx)
emailLogRepoTx := s.emailLogRepo.WithTx(tx)
```

The existing code currently calls `s.messageRepo.WithTx(tx)` and `s.emailLogRepo.WithTx(tx)` inline. Refactor to local variables to avoid mixing transaction and non-transaction writes.

- [ ] **Step 4: Publish websocket updates after commit**

After the transaction succeeds, keep the existing message websocket publish. If `wasResolved`, also publish a conversation updated event:

```go
if wasResolved {
	s.wsPublisher.Publish(websocket.Event{
		Action:      "updated",
		Entity:      "support_conversation",
		EntityID:    conv.ID,
		WorkspaceID: conv.WorkspaceID,
	})
}
```

Do not publish the internal reopened system message unless existing status-change paths already expose internal system messages in admin UI through websocket. If needed later, add it deliberately with tests.

- [ ] **Step 5: Run focused reopen test**

Run:

```bash
cd server
go test ./internal/service -run TestEmailFallbackProcessInboundEmailReopensResolvedConversation -count=1
```

Expected: PASS.

---

### Task 4: Preserve Spam Ignore Behavior

**Files:**
- Modify: `server/internal/service/email_fallback_test.go`

- [ ] **Step 1: Add or update spam regression test**

Add a focused test:

```go
func TestEmailFallbackProcessInboundEmailIgnoresSpamConversation(t *testing.T) {
	settings := model.DefaultSupportInboxSettings()
	env := setupEmailFallbackInboundTestEnv(t, settings)
	ctx := context.Background()

	workspaceID := "22222222-2222-2222-2222-222222222222"
	customerEmail := "customer@example.com"
	conversationID := "44444444-4444-4444-4444-444444444444"
	conversation := &model.SupportConversation{
		ID:            conversationID,
		WorkspaceID:   workspaceID,
		Subject:       "Spam",
		Status:        model.SupportConversationStatusSpam,
		CustomerEmail: &customerEmail,
		Source:        "email",
	}
	if err := env.convRepo.Create(ctx, conversation); err != nil {
		t.Fatalf("create spam conversation: %v", err)
	}

	payload := model.PostmarkInboundPayload{
		MessageID:         "pm-spam-1",
		MessageStream:     "inbound",
		OriginalRecipient: "conv-" + conversationID + "@replies.helpin.ai",
		To:                "conv-" + conversationID + "@replies.helpin.ai",
		From:              customerEmail,
		FromFull:          model.PostmarkAddress{Email: customerEmail},
		Subject:           "Re: Spam",
		StrippedTextReply: "Why was this marked spam?",
	}

	if err := env.service.ProcessInboundEmail(ctx, payload, `{"MessageID":"pm-spam-1"}`); err != nil {
		t.Fatalf("process inbound spam reply: %v", err)
	}

	messages, err := env.messageRepo.ListByConversation(ctx, workspaceID, conversationID, true)
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	if len(messages) != 0 {
		t.Fatalf("messages = %d, want 0", len(messages))
	}
}
```

- [ ] **Step 2: Run spam regression test**

Run:

```bash
cd server
go test ./internal/service -run TestEmailFallbackProcessInboundEmailIgnoresSpamConversation -count=1
```

Expected: PASS.

---

### Task 5: Full Focused Verification

**Files:**
- Verify only.

- [ ] **Step 1: Run email fallback focused tests**

Run:

```bash
cd server
go test ./internal/service -run 'TestEmailFallbackProcessInboundEmail|TestIsEmailFallbackTerminalStatus|TestIsEmailFallbackInboundTerminalStatus' -count=1
```

Expected: PASS.

- [ ] **Step 2: Run broader support service tests if practical**

Run:

```bash
cd server
go test ./internal/service -run 'TestEmailFallback|TestSupportCoverageDailyAnalyzer|TestSupportInbox' -count=1
```

Expected: PASS for the selected tests. If unrelated existing tests fail, capture exact failures and do not claim broad pass.

- [ ] **Step 3: Run formatting**

Run:

```bash
cd server
gofmt -w internal/service/email_fallback.go internal/service/email_fallback_test.go
```

- [ ] **Step 4: Re-run focused tests after formatting**

Run:

```bash
cd server
go test ./internal/service -run 'TestEmailFallbackProcessInboundEmail|TestIsEmailFallbackTerminalStatus|TestIsEmailFallbackInboundTerminalStatus' -count=1
```

Expected: PASS.

---

## Follow-Up Plan: Coverage Segmentation

Do not implement this in the email-reopen patch.

After email replies stop being dropped, write a separate plan for coverage analysis segmentation:

- Load lifecycle boundary system messages or conversation status events.
- Split public transcript into analysis segments:
  - conversation start to resolved
  - reopened to next resolved
- Analyze only the latest eligible segment.
- Include segment start/end or boundary message IDs in the transcript hash.
- Keep gap evidence linked to the original conversation and optionally include segment metadata.

This keeps customer-facing conversation behavior simple while preventing coverage analysis from treating a long reopened thread as one mixed issue.
