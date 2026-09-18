# Availability-Aware AI → Human Escalation — Implementation Plan

> Source review, 2026-09-17

Historical availability design. Current escalation resolves teammate presence and
office hours, chooses a handoff state, and renders state-specific reply-time and
next-opening text in [support_ai_escalate.go](../../server/internal/service/support_ai_escalate.go)
using [handoff helpers](../../server/internal/service/support_handoff_state.go).
The original missing-behavior diagnosis is not the current implementation.
Broader notification/assignment proposals below are not certified by that check.
For schema work, follow the [migration runbook](../ops/database-migrations.md):
Community disables AutoMigrate, so additive columns also require ledger migrations.

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the AI's escalation message reflect real availability (live / busy / after-hours), capture an email when nobody can respond, and close the loop so an offline customer still gets a reply.

**Architecture:** At escalation time the backend resolves a single `handoff_state` from the existing presence + business-hours resolver, selects the matching (new) per-state message, renders `{reply_time}`/`{next_open}` tokens server-side, and persists `handoff_state` on the conversation. The widget reads `conversation.handoffState` to render the honest copy and, for anonymous visitors in busy/after-hours states, an inline email-capture card reusing the existing transcript endpoint. Phase 2 adds a "joined" system line, reply-by-email, teammate queue visibility, and settings previews.

**Tech Stack:** Go 1.24 + GORM + Chi (backend), Preact widget in `packages/widget-core` + vanilla driver in `packages/sdk-js`, React 19 + TanStack Query settings UI in `frontend/`, Vitest (widget) + Go `testing`.

## Global Constraints

- Go module path: `github.com/helpin-ai/helpin/server`. Services hold DB as `db *gorm.DB` but `SupportAIService` works through repositories (`s.messageRepo`, `s.conversationRepo`, `s.handoffRepo`, `s.installationRepo`, `s.wsPublisher`).
- Settings are stored as a JSON string on `SupportWidgetInstallation.Settings` (`gorm:"type:jsonb"`), (un)marshaled via `parseSettings` / `json.Marshal` onto `DefaultSupportInboxSettings()`. No custom Value/Scan.
- Migrations that only ADD columns/fields are handled by GORM AutoMigrate on startup (struct changes). New JSON settings fields need NO migration (they live inside the JSONB blob). A new first-class column (`SupportConversation.HandoffState`) is picked up by AutoMigrate.
- `handoff_state` canonical values: `"live"`, `"busy"`, `"after_hours"`. Use these exact strings across Go, shared TS, widget, and settings.
- Escalation is backend-orchestrated; the LLM never decides handoff. Do NOT touch detection heuristics in `support_ai_escalation.go`.
- Backward compatibility: an existing custom `EscalationMessage` applies to the `live` state ONLY. Empty new fields fall back to their built-in defaults, never to the live message.
- Frontend uses `snake_case` JSON to the API; TS interfaces mirror backend JSON tags exactly.

---

## File Structure

**Backend (Go)**
- Modify `server/internal/model/support_inbox.go` — add settings fields, defaults, DTO fields, `SupportConversation.HandoffState`, handoff-state constants.
- Create `server/internal/service/support_handoff_state.go` — pure `resolveHandoffState` + `renderEscalationMessage` + `humanizeNextOpen` helpers.
- Create `server/internal/service/support_handoff_state_test.go` — unit tests for the above.
- Modify `server/internal/service/support_ai.go` — wire state resolution + message selection + token rendering + persist `handoff_state` + WS payload + AgentHandoff context.
- Modify `server/internal/service/support_inbox_settings.go` — apply new DTO fields to settings.
- Modify `server/internal/service/support_inbox_widget.go` — persist captured visitor email in the transcript path (Task 6).
- Modify `server/internal/service/support_availability_resolver.go` — combine presence AND hours for the widget snapshot `IsOnline` (Phase 2, Task 8).
- Modify `server/internal/service/support_notifications` path (located in Task 12) — notify team for busy/after_hours.

**Shared / Widget (TS)**
- No `packages/shared` changes — `teammate_joined` already exists in `SystemEventType`; handoff state rides on the conversation, not the availability config.
- Modify `packages/widget-core/src/types.ts` — add `handoffState?` to `Conversation`.
- Modify `packages/widget-core/src/components/ConversationView.tsx` — three-state copy + email-capture card.
- Modify `packages/sdk-js/src/core/widget.ts` — map `handoff_state` in `mapConversation` + `conversation:escalated` handler.
- Test `packages/widget-core/src/components/__tests__/ConversationView.test.tsx`.

**Settings UI (React)**
- Modify `frontend/src/components/settings/ChatGeneralTab.tsx` — three message fields, previews, nudge.
- Modify `frontend/src/lib/services/supportService.ts` types (`ChatSettingsDraft` / `SupportInboxSettings`) if needed for new fields.
- Modify `frontend/src/pages` inbox list component (located in Task 11) — queue badge.

---

# PHASE 1 — Honest Escalation

## Task 1: Handoff-state resolution helper (pure function)

**Files:**
- Create: `server/internal/service/support_handoff_state.go`
- Test: `server/internal/service/support_handoff_state_test.go`
- Modify: `server/internal/model/support_inbox.go` (add constants)

**Interfaces:**
- Produces: `model.HandoffStateLive/Busy/AfterHours` string constants; `service.resolveHandoffState(hasAvailableRecipient bool, withinOfficeHours bool) string`.

- [ ] **Step 1: Add handoff-state constants to the model**

In `server/internal/model/support_inbox.go`, next to the `SupportConversationFlowState*` constants (near L79-85), add:

```go
// Handoff states describe availability at the moment the AI hands off.
const (
	HandoffStateLive       = "live"
	HandoffStateBusy       = "busy"
	HandoffStateAfterHours = "after_hours"
)
```

- [ ] **Step 2: Write the failing test**

Create `server/internal/service/support_handoff_state_test.go`:

```go
package service

import (
	"testing"

	"github.com/helpin-ai/helpin/server/internal/model"
)

func TestResolveHandoffState(t *testing.T) {
	cases := []struct {
		name          string
		hasRecipient  bool
		withinHours   bool
		want          string
	}{
		{"online within hours -> live", true, true, model.HandoffStateLive},
		{"online outside hours -> live (presence trumps hours)", true, false, model.HandoffStateLive},
		{"nobody, within hours -> busy", false, true, model.HandoffStateBusy},
		{"nobody, outside hours -> after_hours", false, false, model.HandoffStateAfterHours},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := resolveHandoffState(c.hasRecipient, c.withinHours); got != c.want {
				t.Fatalf("resolveHandoffState(%v,%v) = %q, want %q", c.hasRecipient, c.withinHours, got, c.want)
			}
		})
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `cd server && go test ./internal/service/ -run TestResolveHandoffState -v`
Expected: FAIL — `undefined: resolveHandoffState`.

- [ ] **Step 4: Write minimal implementation**

Create `server/internal/service/support_handoff_state.go`:

```go
package service

import "github.com/helpin-ai/helpin/server/internal/model"

// resolveHandoffState maps availability at handoff time to a customer-facing state.
// Presence trumps business hours: an available recipient always yields "live".
func resolveHandoffState(hasAvailableRecipient bool, withinOfficeHours bool) string {
	if hasAvailableRecipient {
		return model.HandoffStateLive
	}
	if withinOfficeHours {
		return model.HandoffStateBusy
	}
	return model.HandoffStateAfterHours
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `cd server && go test ./internal/service/ -run TestResolveHandoffState -v`
Expected: PASS (4 subtests).

- [ ] **Step 6: Commit**

```bash
git add server/internal/model/support_inbox.go server/internal/service/support_handoff_state.go server/internal/service/support_handoff_state_test.go
git commit -m "feat(support): add handoff-state resolution helper"
```

---

## Task 2: Per-state escalation message settings

**Files:**
- Modify: `server/internal/model/support_inbox.go` (`SupportInboxSettings`, `DefaultSupportInboxSettings`, `UpdateInstallationSettingsRequest`)
- Modify: `server/internal/service/support_inbox_settings.go` (apply DTO → settings)

**Interfaces:**
- Produces: `SupportInboxSettings.EscalationMessageBusy string`, `.EscalationMessageAfterHours string`; DTO `UpdateInstallationSettingsRequest.EscalationMessageBusy *string`, `.EscalationMessageAfterHours *string`.

- [ ] **Step 1: Add the two settings fields**

In `SupportInboxSettings` (near the existing `EscalationMessage string` at L1042), add:

```go
	EscalationMessage           string `json:"escalation_message"`
	EscalationMessageBusy       string `json:"escalation_message_busy"`
	EscalationMessageAfterHours string `json:"escalation_message_after_hours"`
```

- [ ] **Step 2: Add defaults**

In `DefaultSupportInboxSettings()` (L1126+), alongside the existing `EscalationMessage` default, add:

```go
		EscalationMessage:           "Let me connect you with a team member — they typically reply in {reply_time}.",
		EscalationMessageBusy:       "I've notified the team. Everyone's helping other customers right now — expect a reply within {reply_time}.",
		EscalationMessageAfterHours: "I've passed this on to the team. We're away right now and back {next_open}.",
```

> Note: the live default now includes `{reply_time}`. This changes the shipped default copy intentionally (the old flat string had no expectation). Existing installations with a customized `EscalationMessage` are unaffected.

- [ ] **Step 3: Add DTO fields**

In `UpdateInstallationSettingsRequest` (near L1211), add:

```go
	EscalationMessage           *string `json:"escalation_message,omitempty"`
	EscalationMessageBusy       *string `json:"escalation_message_busy,omitempty"`
	EscalationMessageAfterHours *string `json:"escalation_message_after_hours,omitempty"`
```

(The first line may already exist — keep a single copy.)

- [ ] **Step 4: Apply the DTO fields to settings**

Locate the apply site (where `req.EscalationMessage` is copied onto settings):

Run: `cd server && grep -rn "req.EscalationMessage" internal/service/`

Mirror the existing pattern in that function, adding:

```go
	if req.EscalationMessage != nil {
		settings.EscalationMessage = *req.EscalationMessage
	}
	if req.EscalationMessageBusy != nil {
		settings.EscalationMessageBusy = *req.EscalationMessageBusy
	}
	if req.EscalationMessageAfterHours != nil {
		settings.EscalationMessageAfterHours = *req.EscalationMessageAfterHours
	}
```

If no existing `req.EscalationMessage != nil` block exists, add all three in the same apply function that handles `req.HandoffBehavior`.

- [ ] **Step 5: Verify build + JSON round-trip**

Run: `cd server && go build ./... && go test ./internal/service/ -run TestDefaultSupportInboxSettings -v`
Expected: build OK. (If no such test exists, skip the `-run` and just confirm build.)

- [ ] **Step 6: Commit**

```bash
git add server/internal/model/support_inbox.go server/internal/service/support_inbox_settings.go
git commit -m "feat(support): add busy/after-hours escalation message settings"
```

---

## Task 3: Escalation message token rendering

**Files:**
- Modify: `server/internal/service/support_handoff_state.go`
- Modify: `server/internal/service/support_handoff_state_test.go`

**Interfaces:**
- Consumes: `nextBusinessHoursStart(settings, localNow) *time.Time` (support_inbox_settings.go L468).
- Produces: `renderEscalationMessage(template, replyTime, nextOpen string) string`; `humanizeNextOpen(next *time.Time, localNow time.Time) string`; `shortReplyTimePhrase(preset string, minutes *int) string`.

- [ ] **Step 1: Write the failing tests**

Append to `support_handoff_state_test.go`:

```go
func TestRenderEscalationMessage(t *testing.T) {
	got := renderEscalationMessage("reply in {reply_time}, back {next_open}", "a few minutes", "tomorrow at 9:00 AM PST")
	want := "reply in a few minutes, back tomorrow at 9:00 AM PST"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	// Missing values degrade gracefully.
	got = renderEscalationMessage("back {next_open}", "", "")
	if got != "back as soon as possible" {
		t.Fatalf("degraded got %q", got)
	}
	// Unknown token rendered literally.
	got = renderEscalationMessage("hi {unknown}", "x", "y")
	if got != "hi {unknown}" {
		t.Fatalf("unknown token got %q", got)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd server && go test ./internal/service/ -run TestRenderEscalationMessage -v`
Expected: FAIL — `undefined: renderEscalationMessage`.

- [ ] **Step 3: Implement**

In `support_handoff_state.go`, add `"fmt"`, `"strings"`, and `"time"` to the file's existing import block (single import block per Go file — do NOT paste a second one), then append:

```go
const escalationFallbackTime = "as soon as possible"

// renderEscalationMessage substitutes {reply_time} and {next_open}. Empty
// values degrade to "as soon as possible". Unknown tokens are left literal.
func renderEscalationMessage(template, replyTime, nextOpen string) string {
	if strings.TrimSpace(replyTime) == "" {
		replyTime = escalationFallbackTime
	}
	if strings.TrimSpace(nextOpen) == "" {
		nextOpen = escalationFallbackTime
	}
	out := strings.ReplaceAll(template, "{reply_time}", replyTime)
	out = strings.ReplaceAll(out, "{next_open}", nextOpen)
	return out
}

// shortReplyTimePhrase converts the reply-time expectation into a short
// phrase for the {reply_time} token ("a few minutes", "about 45 minutes").
// Empty result degrades to the fallback inside renderEscalationMessage.
func shortReplyTimePhrase(preset string, minutes *int) string {
	switch preset {
	case "few_minutes":
		return "a few minutes"
	case "few_hours":
		return "a few hours"
	case "same_day":
		return "a day"
	case "custom":
		if minutes != nil && *minutes > 0 {
			return fmt.Sprintf("about %d minutes", *minutes)
		}
	}
	return ""
}

// humanizeNextOpen renders a next-open time relative to localNow in the
// business-hours timezone: "today at 5:00 PM PST", "tomorrow at 9:00 AM PST",
// or "on Monday at 9:00 AM PST". Returns "" when next is nil.
func humanizeNextOpen(next *time.Time, localNow time.Time) string {
	if next == nil {
		return ""
	}
	t := next.In(localNow.Location())
	zone, _ := t.Zone()
	clock := t.Format("3:04 PM") + " " + zone
	startOfDay := func(x time.Time) time.Time {
		y, m, d := x.Date()
		return time.Date(y, m, d, 0, 0, 0, 0, x.Location())
	}
	switch days := int(startOfDay(t).Sub(startOfDay(localNow)).Hours() / 24); {
	case days <= 0:
		return "today at " + clock
	case days == 1:
		return "tomorrow at " + clock
	default:
		return "on " + t.Format("Monday") + " at " + clock
	}
}
```

- [ ] **Step 4: Run the render test**

Run: `cd server && go test ./internal/service/ -run TestRenderEscalationMessage -v`
Expected: PASS.

- [ ] **Step 5: Add humanizeNextOpen + shortReplyTimePhrase tests**

```go
func TestHumanizeNextOpen(t *testing.T) {
	now := time.Date(2026, 7, 4, 20, 0, 0, 0, time.UTC) // Saturday evening
	if humanizeNextOpen(nil, now) != "" {
		t.Fatal("nil should be empty")
	}
	monday := time.Date(2026, 7, 6, 9, 0, 0, 0, time.UTC)
	if got := humanizeNextOpen(&monday, now); got != "on Monday at 9:00 AM UTC" {
		t.Fatalf("monday got %q", got)
	}
	sunday := time.Date(2026, 7, 5, 9, 0, 0, 0, time.UTC)
	if got := humanizeNextOpen(&sunday, now); got != "tomorrow at 9:00 AM UTC" {
		t.Fatalf("tomorrow got %q", got)
	}
}

func TestShortReplyTimePhrase(t *testing.T) {
	if got := shortReplyTimePhrase("few_minutes", nil); got != "a few minutes" {
		t.Fatalf("few_minutes got %q", got)
	}
	if got := shortReplyTimePhrase("same_day", nil); got != "a day" {
		t.Fatalf("same_day got %q", got)
	}
	m := 45
	if got := shortReplyTimePhrase("custom", &m); got != "about 45 minutes" {
		t.Fatalf("custom got %q", got)
	}
	if got := shortReplyTimePhrase("", nil); got != "" {
		t.Fatalf("unknown preset must degrade to empty, got %q", got)
	}
}
```

Run: `cd server && go test ./internal/service/ -run 'TestHumanizeNextOpen|TestShortReplyTimePhrase' -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add server/internal/service/support_handoff_state.go server/internal/service/support_handoff_state_test.go
git commit -m "feat(support): render escalation message tokens"
```

---

## Task 4: Wire state-aware escalation into escalateToHuman

**Files:**
- Modify: `server/internal/model/support_inbox.go` (add `SupportConversation.HandoffState *string`)
- Modify: `server/internal/service/support_ai.go` (escalateToHuman, ~L991-1192)

**Interfaces:**
- Consumes: `resolveHandoffState`, `renderEscalationMessage`, `humanizeNextOpen`, `model.HandoffState*`, existing `loadSupportAvailability`, existing recipient `selection`, `nextBusinessHoursStart`.
- Produces: escalation message content selected by state; `conversation.handoff_state` persisted; WS `escalated` payload carries `handoff_state`.

- [ ] **Step 1: Add HandoffState to SupportConversation**

In `SupportConversation` (support_inbox.go, near `FlowState *string` L21), add:

```go
	FlowState    *string `json:"flow_state" gorm:"column:flow_state"`
	HandoffState *string `json:"handoff_state" gorm:"column:handoff_state"`
```

AutoMigrate adds the `handoff_state` column on startup.

- [ ] **Step 2: Write the failing test (message selection matrix)**

Add to `support_handoff_state_test.go` a helper that mirrors the selection logic so it is unit-testable without the DB:

```go
func TestSelectEscalationTemplate(t *testing.T) {
	s := model.SupportInboxSettings{
		EscalationMessage:           "live {reply_time}",
		EscalationMessageBusy:       "busy {reply_time}",
		EscalationMessageAfterHours: "away {next_open}",
	}
	if got := selectEscalationTemplate(s, model.HandoffStateLive); got != "live {reply_time}" {
		t.Fatalf("live got %q", got)
	}
	if got := selectEscalationTemplate(s, model.HandoffStateBusy); got != "busy {reply_time}" {
		t.Fatalf("busy got %q", got)
	}
	if got := selectEscalationTemplate(s, model.HandoffStateAfterHours); got != "away {next_open}" {
		t.Fatalf("after_hours got %q", got)
	}
	// Empty busy field falls back to default, NOT to the live message.
	s2 := model.SupportInboxSettings{EscalationMessage: "custom live"}
	if got := selectEscalationTemplate(s2, model.HandoffStateBusy); got == "custom live" || got == "" {
		t.Fatalf("busy fallback must be the built-in default, got %q", got)
	}
}
```

- [ ] **Step 3: Run to verify it fails**

Run: `cd server && go test ./internal/service/ -run TestSelectEscalationTemplate -v`
Expected: FAIL — `undefined: selectEscalationTemplate`.

- [ ] **Step 4: Implement selectEscalationTemplate**

Append to `support_handoff_state.go`:

```go
func selectEscalationTemplate(s model.SupportInboxSettings, state string) string {
	defaults := model.DefaultSupportInboxSettings()
	pick := func(custom, def string) string {
		if strings.TrimSpace(custom) != "" {
			return custom
		}
		return def
	}
	switch state {
	case model.HandoffStateBusy:
		return pick(s.EscalationMessageBusy, defaults.EscalationMessageBusy)
	case model.HandoffStateAfterHours:
		return pick(s.EscalationMessageAfterHours, defaults.EscalationMessageAfterHours)
	default: // live
		return pick(s.EscalationMessage, defaults.EscalationMessage)
	}
}
```

Run: `cd server && go test ./internal/service/ -run TestSelectEscalationTemplate -v`
Expected: PASS.

- [ ] **Step 5: Reorder escalateToHuman, then wire state-aware content**

**Ordering caveat (verified against current code):** the function builds and persists the escalation message at L1018-1051 BEFORE it resolves the handoff mailbox (L1076) and recipient `selection` (L1079-1096). The state-aware message needs the recipient answer first, so reorder:

1. Move these two blocks up to just after the availability load (after L997, before the history/dedupe load at L1005):
   - `handoffMailboxID, mailboxSelectionSource := s.resolveEscalationMailbox(ctx, workspaceID, conversationID, messageID, conv, settings)`
   - the entire `selection, selectErr := selectSupportConversationRecipient(...)` block including its error log.
   Neither depends on the messages being created — they consume only `conv`, `settings`, `now`.

2. Immediately after the moved blocks, resolve the state and content (OUTSIDE the `if !escalationAlreadyMessaged` guard, so `handoffState` is persisted even when the message is deduped):

```go
	handoffState := resolveHandoffState(selection != nil, availability.IsWithinOfficeHours)

	// Short phrase for the {reply_time} token. Do NOT use
	// availability.WidgetAvailability.ReplyTimeText — that is full-sentence
	// copy (and the offline branch sets it to the outside-hours message).
	replyTime := shortReplyTimePhrase(
		availability.WidgetAvailability.ReplyTimePreset,
		availability.WidgetAvailability.ReplyTimeMinutes,
	)

	// Next-open text (only meaningful for after_hours).
	var nextOpenText string
	if loc, err := time.LoadLocation(settings.BusinessHoursTimezone); err == nil {
		localNow := now.In(loc)
		nextOpenText = humanizeNextOpen(nextBusinessHoursStart(settings, localNow), localNow)
	}

	escalationContent := renderEscalationMessage(
		selectEscalationTemplate(settings, handoffState),
		replyTime,
		nextOpenText,
	)
```

3. Delete the old `escalationContent` block inside the message-creation branch (L1019-1022):

```go
	escalationContent := "Let me connect you with a team member who can help further."
	if strings.TrimSpace(settings.EscalationMessage) != "" {
		escalationContent = settings.EscalationMessage
	}
```

The `replyMsg` at L1041-1051 keeps using `escalationContent` unchanged.

- [ ] **Step 6: Persist handoff_state on the conversation**

In the `UpdateFields` map (L1105-1113) add:

```go
		"handoff_state": handoffState,
```

- [ ] **Step 7: Include handoff_state in AgentHandoff context and WS payload**

Replace the `AgentHandoff` `Context: json.RawMessage("{}")` (L1128-1136) with a marshaled context:

```go
	handoffCtx, _ := json.Marshal(map[string]string{"handoff_state": handoffState})
	// ... in the &model.AgentHandoff{...}:
	Context: handoffCtx,
```

Extend the WS `escalated` event (L1187-1192). `websocket.Event.Data` is `json.RawMessage` (hub.go:27), so marshal the payload. Use `fields["flow_state"]` (not the `flowState` local) because L1120-1122 may have overridden it to `after_hours_queue`:

```go
	escalatedData, _ := json.Marshal(map[string]any{
		"conversation_id": conversationID,
		"flow_state":      fields["flow_state"],
		"handoff_state":   handoffState,
	})
	s.wsPublisher.Publish(websocket.Event{
		Action:      "escalated",
		Entity:      "support_conversation",
		EntityID:    conversationID,
		WorkspaceID: workspaceID,
		Data:        escalatedData,
	})
```

- [ ] **Step 8: Verify build + full service tests**

Run: `cd server && go build ./... && go test ./internal/service/ -v -run 'Handoff|Escalation|NextOpen'`
Expected: build OK, tests PASS.

- [ ] **Step 9: Commit**

```bash
git add server/internal/model/support_inbox.go server/internal/service/support_ai.go server/internal/service/support_handoff_state.go server/internal/service/support_handoff_state_test.go
git commit -m "feat(support): state-aware escalation message + persisted handoff_state"
```

---

## Task 5: Shared + widget TS types for handoff state

**Files:**
- Modify: `packages/widget-core/src/types.ts` (`Conversation`)
- Modify: `packages/sdk-js/src/core/widget.ts` (`mapConversation`, `conversation:escalated`)

(No `packages/shared` change: handoff state rides on the conversation, not the availability config, and `teammate_joined` already exists in `SystemEventType`.)

**Interfaces:**
- Produces: `Conversation.handoffState?: 'live' | 'busy' | 'after_hours'` in widget-core `types.ts`.

- [ ] **Step 1: Add handoffState to the Conversation interface**

In `packages/widget-core/src/types.ts` `Conversation` (L55-65), add:

```ts
  flowState?: string;
  aiState?: string;
  handoffState?: 'live' | 'busy' | 'after_hours';
```

- [ ] **Step 2: Map handoff_state from the server**

In `packages/sdk-js/src/core/widget.ts` `mapConversation` (L891-908), add to the returned object:

```ts
    flowState: raw.flow_state,
    aiState: raw.ai_state,
    handoffState: raw.handoff_state,
```

In the `conversation:escalated` handler (L1770-1789), when it sets `aiState: 'escalated'`, also read the new field:

```ts
      handoffState: data.data.handoff_state,
```

- [ ] **Step 3: Typecheck**

Run: `cd packages/widget-core && pnpm typecheck && cd ../sdk-js && pnpm typecheck`
Expected: no type errors.

- [ ] **Step 4: Commit**

```bash
git add packages/widget-core/src/types.ts packages/sdk-js/src/core/widget.ts
git commit -m "feat(widget): plumb handoff_state to conversation state"
```

---

## Task 6: Widget three-state rendering + email-capture card

**Files:**
- Modify: `packages/widget-core/src/components/ConversationView.tsx`
- Modify: `server/internal/service/support_inbox_widget.go` (persist captured email)
- Test: `packages/widget-core/src/components/__tests__/ConversationView.test.tsx`

**Interfaces:**
- Consumes: `conversation.handoffState`, existing `onRequestTranscript(email?) => Promise<{success; message}>`, existing `transcriptEmail`.
- Produces: `SupportConversation.CustomerEmail` persisted from the capture card — Task 10 (reply-by-email) depends on this.

- [ ] **Step 1: Write the failing test**

In `ConversationView.test.tsx` add (follow the file's existing render harness for props):

```tsx
it('shows email-capture card for anonymous visitor when busy', () => {
  const { getByText, getByPlaceholderText } = renderConversationView({
    conversation: { id: 'c1', status: 'open', aiState: 'escalated', handoffState: 'busy' },
    transcriptEmail: undefined,
  });
  expect(getByText(/reply there too/i)).toBeTruthy();
  expect(getByPlaceholderText(/you@/i)).toBeTruthy();
});

it('hides email-capture card when visitor email is known', () => {
  const { queryByText } = renderConversationView({
    conversation: { id: 'c1', status: 'open', aiState: 'escalated', handoffState: 'after_hours' },
    transcriptEmail: 'known@example.com',
  });
  expect(queryByText(/reply there too/i)).toBeNull();
});

it('does not show email-capture card in live state', () => {
  const { queryByText } = renderConversationView({
    conversation: { id: 'c1', status: 'open', aiState: 'escalated', handoffState: 'live' },
    transcriptEmail: undefined,
  });
  expect(queryByText(/reply there too/i)).toBeNull();
});
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd packages/widget-core && pnpm test -- ConversationView`
Expected: FAIL — card not rendered.

- [ ] **Step 3: Implement the email-capture card**

In `ConversationView.tsx`, derive the trigger near the existing `showHumanHandoffState` (L107-140):

```tsx
const handoffState = conversation?.handoffState;
const nobodyAvailable = handoffState === 'busy' || handoffState === 'after_hours';
const showEscalationEmailCapture =
  nobodyAvailable && !transcriptEmail && !hasHumanReply;
```

Render the card below the handoff subtitle (reuse the transcript form's email input + `handleTranscriptRequest`, which already calls `onRequestTranscript`). Add, near the L388-401 subtitle block:

```tsx
{showEscalationEmailCapture && (
  <div className="helpin-escalation-email-capture">
    <p className="helpin-escalation-email-capture__label">
      Leave your email and we'll reply there too.
    </p>
    <form onSubmit={handleTranscriptRequest} className="helpin-escalation-email-capture__form">
      <input
        type="email"
        required
        placeholder="you@example.com"
        value={transcriptEmailInput}
        onInput={(e) => setTranscriptEmailInput((e.target as HTMLInputElement).value)}
        className="helpin-transcript-email-input"
      />
      <button type="submit" disabled={isSendingTranscript}>
        {isSendingTranscript ? 'Sending…' : 'Notify me'}
      </button>
    </form>
    {transcriptStatus?.success && (
      <p className="helpin-escalation-email-capture__ok">
        We'll reply to you at {transcriptEmailInput}.
      </p>
    )}
  </div>
)}
```

(Reuse the existing state `transcriptEmailInput`, `setTranscriptEmailInput`, `transcriptStatus`, `isSendingTranscript`, and `handleTranscriptRequest` from L71-74 / L318-338. Do NOT duplicate them.)

- [ ] **Step 4: Add styles**

In `packages/widget-core/src/styles/widget.css`, add a minimal block near the transcript styles:

```css
.helpin-escalation-email-capture { margin-top: 8px; padding: 10px; border-radius: 10px; background: var(--helpin-surface-muted, #f4f4f5); }
.helpin-escalation-email-capture__label { font-size: 12px; margin: 0 0 6px; }
.helpin-escalation-email-capture__form { display: flex; gap: 6px; }
.helpin-escalation-email-capture__ok { font-size: 12px; margin: 6px 0 0; color: var(--helpin-success, #16a34a); }
```

- [ ] **Step 5: Persist the captured email on the conversation (backend)**

The card reuses the transcript endpoint, but `SendWidgetConversationTranscript` (`server/internal/service/support_inbox_widget.go:333`) only READS the email — it never stores it. Without this step, Task 10's reply-by-email has nothing to key off for anonymous visitors and the "we'll reply there too" promise silently fails.

In `SendWidgetConversationTranscript`, after the transcript email is sent successfully, persist the email when the conversation has none:

```go
	if strings.TrimSpace(derefString(conversation.CustomerEmail)) == "" && strings.TrimSpace(email) != "" {
		if err := s.conversationRepo.UpdateFields(ctx, conversation.WorkspaceID, conversationID, map[string]any{
			"customer_email": strings.TrimSpace(email),
		}); err != nil {
			slog.WarnContext(ctx, "persist captured visitor email failed", "error", err, "conversation_id", conversationID)
		}
	}
```

(Use whatever conversation-repository handle `SupportInboxService` already holds — confirm with `grep -n "conversationRepo" server/internal/service/support_inbox_widget.go` — and match the existing not-found/ownership checks already done earlier in the function.)

Add a service test: a transcript request carrying an email, against a conversation with no `customer_email`, sets `customer_email` after the call.

Run: `cd server && go test ./internal/service/ -run Transcript -v`
Expected: PASS (new test included).

- [ ] **Step 6: Run widget tests + typecheck**

Run: `cd packages/widget-core && pnpm test -- ConversationView && pnpm typecheck`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add packages/widget-core/src/components/ConversationView.tsx packages/widget-core/src/components/__tests__/ConversationView.test.tsx packages/widget-core/src/styles/widget.css server/internal/service/support_inbox_widget.go
git commit -m "feat(widget): email-capture card on busy/after-hours escalation"
```

---

## Task 7: Settings — three escalation message fields

**Files:**
- Modify: `frontend/src/components/settings/ChatGeneralTab.tsx`
- Modify: `frontend/src/lib/services/supportService.ts` (extend `SupportInboxSettings` / draft types)

**Interfaces:**
- Consumes: `supportService.updateInstallationSettings(workspaceId, settings)`; settings JSON keys `escalation_message`, `escalation_message_busy`, `escalation_message_after_hours`.

- [ ] **Step 1: Add the settings keys to the TS types**

In `frontend/src/lib/services/supportService.ts`, add to `SupportInboxSettings` (and `ChatSettingsDraft` if separate):

```ts
  escalation_message?: string;
  escalation_message_busy?: string;
  escalation_message_after_hours?: string;
```

- [ ] **Step 2: Add state + hydration**

In `ChatGeneralTab.tsx`, next to `escalationMessage` state (L73), add:

```tsx
const [escalationMessageBusy, setEscalationMessageBusy] = useState('');
const [escalationMessageAfterHours, setEscalationMessageAfterHours] = useState('');
```

In the hydration effect (near L130 where `escalationMessage` is set), add:

```tsx
setEscalationMessageBusy(s.escalation_message_busy ?? '');
setEscalationMessageAfterHours(s.escalation_message_after_hours ?? '');
```

- [ ] **Step 3: Add fields to the save payload**

In the `settingsDraft` construction (L157-199), next to `escalation_message`, add:

```ts
escalation_message_busy: escalationMessageBusy || undefined,
escalation_message_after_hours: escalationMessageAfterHours || undefined,
```

- [ ] **Step 4: Render the two new textareas**

Below the existing escalation field (L1117-1127), add:

```tsx
<div className="space-y-2">
  <Label htmlFor="escalation-msg-busy" className="text-sm">Escalation Message — Team Busy</Label>
  <Textarea id="escalation-msg-busy" value={escalationMessageBusy}
    onChange={(e) => setEscalationMessageBusy(e.target.value)}
    placeholder="I've notified the team. Everyone's helping other customers right now — expect a reply within {reply_time}." rows={2} />
  <p className="text-xs text-muted-foreground">Shown when nobody is online but you're within business hours. Tokens: <code>{'{reply_time}'}</code></p>
</div>
<div className="space-y-2">
  <Label htmlFor="escalation-msg-ah" className="text-sm">Escalation Message — After Hours</Label>
  <Textarea id="escalation-msg-ah" value={escalationMessageAfterHours}
    onChange={(e) => setEscalationMessageAfterHours(e.target.value)}
    placeholder="I've passed this on to the team. We're away right now and back {next_open}." rows={2} />
  <p className="text-xs text-muted-foreground">Shown when nobody is online and you're outside business hours. Tokens: <code>{'{next_open}'}</code></p>
</div>
```

- [ ] **Step 5: Verify build**

Run: `cd frontend && pnpm build`
Expected: `tsc -b` + vite build succeed.

- [ ] **Step 6: Commit**

```bash
git add frontend/src/components/settings/ChatGeneralTab.tsx frontend/src/lib/services/supportService.ts
git commit -m "feat(settings): busy/after-hours escalation message fields"
```

---

# PHASE 2 — First-Class Polish

## Task 8: Corrected pre-chat availability (presence AND hours)

**Files:**
- Modify: `server/internal/service/support_availability_resolver.go`
- Test: add to an availability resolver test (create `support_availability_resolver_test.go` if absent)

**Interfaces:**
- Consumes: presence statuses (`resolveSupportTeammatePresenceStatuses`) + `isWithinBusinessHours`.
- Produces: widget `WidgetConfigAvailability.IsOnline` reflects (someone online) — not hours alone.

- [ ] **Step 1: Write the failing test**

The widget snapshot must show `IsOnline=false` when nobody is online even if within hours, and `IsOnline=true` when a teammate is online even outside hours. Add a test that drives `GetPublicWidgetConfig`'s availability branch with a fake presence provider returning zero online agents while `BusinessHoursEnabled=true` and within schedule; assert `IsOnline == false`.

```go
func TestWidgetAvailabilityRequiresPresence(t *testing.T) {
	// within hours, nobody online -> not online
	// (build settings within hours; presence returns no online agents)
	// assert snapshot.IsOnline == false and StatusText mentions reply expectation, not "Online now"
}
```

Fill in using the existing resolver test harness / fakes (search for an existing presence fake: `grep -rn "PresenceProvider" server/internal/service`).

- [ ] **Step 2: Run to verify it fails**

Run: `cd server && go test ./internal/service/ -run TestWidgetAvailability -v`
Expected: FAIL (current code keys `IsOnline` off hours, not presence).

- [ ] **Step 3: Implement**

Where the widget snapshot's `IsOnline` is set (resolver online branch L32-40), gate it on actual presence: pass the count of online teammates into the snapshot builder and set `IsOnline = onlineCount > 0`. Keep `NextOnlineAt`/`OutsideHoursMessage` behavior for the offline branch. Ensure the caller in `GetPublicWidgetConfig` supplies presence to the resolver (thread the presence provider through if the resolver currently only takes settings + now).

> This may require the resolver to accept an `onlineCount int` (or the presence statuses) argument. Update `resolveSupportAvailability` / `...ForMailbox` signatures and their call sites accordingly; run `grep -rn "resolveSupportAvailability" server/internal` to update every caller.

- [ ] **Step 4: Run tests + build**

Run: `cd server && go build ./... && go test ./internal/service/ -run TestWidgetAvailability -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add server/internal/service/support_availability_resolver.go server/internal/service/support_availability_resolver_test.go
git commit -m "feat(support): widget availability reflects presence, not just hours"
```

---

## Task 9: "Teammate joined" system line

**Files:**
- Modify: backend assignment path (located in step 1)
- Modify: `packages/widget-core/src/components/ConversationView.tsx` (render `teammate_joined`)
- Test: `ConversationView.test.tsx`

**Interfaces:**
- Consumes: existing `SystemEventType` `'teammate_joined'` (shared message.ts) + `Message.systemEventType`.

- [ ] **Step 1: Locate assignment + emit a system message**

Run: `cd server && grep -rn "assigned_user_id\|AssignedUserID\|took\|assign" internal/service/support_*.go | grep -i assign`

In the service that assigns a conversation to a user (first human assignment), emit a `SupportMessage` with `MessageType:"system"`, `SystemEventType:"teammate_joined"`, `IsInternal:false`, `Content` like `<name> joined the conversation`. Mirror the existing internal system-message creation in `escalateToHuman` (L1026-1038) but with `IsInternal:false` so the widget shows it.

- [ ] **Step 2: Render the system line in the widget**

In `ConversationView.tsx`, where messages map to bubbles, add a branch: when `message.systemEventType === 'teammate_joined'` render a centered system line (reuse existing system-message styling if present; else a `helpin-system-line` span).

- [ ] **Step 3: Widget test**

```tsx
it('renders teammate_joined as a system line', () => {
  const { getByText } = renderConversationView({
    messages: [{ id: 'm1', role: 'system', systemEventType: 'teammate_joined', content: 'Sara joined the conversation', isInternal: false, createdAt: '', conversationId: 'c1' }],
  });
  expect(getByText(/joined the conversation/i)).toBeTruthy();
});
```

Run: `cd packages/widget-core && pnpm test -- ConversationView`
Expected: PASS.

- [ ] **Step 4: Build backend + commit**

```bash
cd server && go build ./...
git add server/internal/service packages/widget-core/src/components/ConversationView.tsx packages/widget-core/src/components/__tests__/ConversationView.test.tsx
git commit -m "feat(support): teammate-joined system line"
```

---

## Task 10: Reply-by-email when the visitor is offline

**Files:**
- Modify: backend human-reply path (located in step 1) + `email_fallback.go`

**Interfaces:**
- Consumes: existing email-fallback sender; `SupportConversation.CustomerEmail`; presence/connection state of the visitor.

- [ ] **Step 1: Locate the human-reply send path**

Run: `cd server && grep -rn "sender_type\|SenderType" internal/service/support_*.go | grep -i "agent\|human\|reply"`

Find where a teammate's reply message is created and broadcast to the widget.

- [ ] **Step 2: Add reply-by-email on offline visitor**

After the reply is persisted, if the conversation has a `CustomerEmail` AND the visitor is not currently connected (reuse whatever "widget session connected" signal exists; if none, gate on `handoff_state IN (busy, after_hours)` at escalation time OR presence of a captured email), call the existing email-fallback send with the reply content. Mirror the call already used by `email_fallback.go`.

- [ ] **Step 3: Test**

Add a service test: given a conversation with `CustomerEmail` set and visitor offline, a human reply triggers one email send (assert via a fake email client's call count).

Run: `cd server && go test ./internal/service/ -run TestReplyByEmail -v`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add server/internal/service
git commit -m "feat(support): reply-by-email when visitor is offline"
```

---

## Task 11: Teammate-side queue badge

**Files:**
- Modify: inbox list component (located in step 1) under `frontend/src/pages` or `frontend/src/components/support`

**Interfaces:**
- Consumes: `conversation.flow_state` in `('queued_for_human','after_hours_queue')`; conversation timestamp for wait time.

- [ ] **Step 1: Locate the inbox list row component**

Run: `cd frontend && grep -rn "flow_state\|flowState\|after_hours_queue\|queued_for_human" src/`

- [ ] **Step 2: Render the badge**

In the conversation row, when `flowState === 'queued_for_human' || flowState === 'after_hours_queue'`, render a small badge "Waiting for human" plus a relative wait time (use existing date-fns/dayjs helper already imported in that file, e.g. `formatDistanceToNow(new Date(escalatedAt))`). Use the timestamp already available on the row (e.g. `customer_requested_human_at` or `updated_at`).

- [ ] **Step 3: Verify build**

Run: `cd frontend && pnpm build`
Expected: succeeds.

- [ ] **Step 4: Commit**

```bash
git add frontend/src
git commit -m "feat(inbox): waiting-for-human queue badge"
```

---

## Task 12: Notify the team for busy/after-hours escalations

**Files:**
- Modify: the escalation notification path (located in step 1)

**Interfaces:**
- Consumes: `handoffState`; existing support notification/event emission.

- [ ] **Step 1: Locate the escalation notification emit**

Run: `cd server && grep -rn "SupportEventAIHandoffTriggered\|notif" internal/service/support_ai.go`

- [ ] **Step 2: Ensure notification fires for all states**

Confirm the mailbox-team notification is emitted even when `selection == nil` (busy/after_hours), not only on assignment. If it is currently gated on a recipient being chosen, move the emit so it always runs after escalation, including `handoff_state` in the payload.

- [ ] **Step 3: Test**

Add/extend a service test asserting the notification/event emit is invoked when `selection == nil` and `handoff_state == "after_hours"`.

Run: `cd server && go test ./internal/service/ -run TestEscalationNotifies -v`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add server/internal/service
git commit -m "feat(support): notify team on busy/after-hours escalations"
```

---

## Task 13: Settings previews + business-hours nudge

**Files:**
- Modify: `frontend/src/components/settings/ChatGeneralTab.tsx`

**Interfaces:**
- Consumes: `businessHoursEnabled`, `replyTimePreset`, the three escalation message states.

- [ ] **Step 1: Render a per-state preview**

Below the three escalation fields, add a read-only preview block that substitutes tokens client-side using the current reply-time selection and a placeholder next-open string:

```tsx
// Mirrors the backend shortReplyTimePhrase mapping (Task 3) so previews match reality.
const previewReply = replyTimePreset === 'few_minutes' ? 'a few minutes'
  : replyTimePreset === 'few_hours' ? 'a few hours'
  : replyTimePreset === 'same_day' ? 'a day'
  : `about ${replyTimeCustomMinutes ?? 0} minutes`;
const renderPreview = (t: string) =>
  t.replace('{reply_time}', previewReply).replace('{next_open}', 'on Monday at 9:00 AM');
```

Render `renderPreview(escalationMessage || defaultLive)`, etc., in muted preview cards.

- [ ] **Step 2: Business-hours nudge**

Under the After-hours field, when `!businessHoursEnabled`, render an inline note:

```tsx
{!businessHoursEnabled && (
  <p className="text-xs text-amber-600">
    Enable business hours below so customers see an accurate return time.
  </p>
)}
```

- [ ] **Step 3: Verify build**

Run: `cd frontend && pnpm build`
Expected: succeeds.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/components/settings/ChatGeneralTab.tsx
git commit -m "feat(settings): escalation message previews + business-hours nudge"
```

---

## Final verification

- [ ] Backend: `cd server && go build ./... && go test ./internal/service/ -v`
- [ ] Widget: `cd packages/widget-core && pnpm test && pnpm typecheck`
- [ ] SDK: `cd packages/sdk-js && pnpm typecheck`
- [ ] Frontend: `cd frontend && pnpm build`
- [ ] Manual smoke (per `verify` skill): trigger an escalation with (a) a teammate online → live copy, no email card; (b) nobody online within hours → busy copy + email card; (c) outside hours → after-hours copy with next-open time + email card.

## Spec coverage map

| Spec section | Task |
|---|---|
| §1 Handoff state resolution | 1, 4 |
| §2 Per-state messages + tokens | 2, 3, 4 |
| §3 Widget experience (3 states + email card) | 5, 6 |
| §3 Pre-chat transparency | 8 |
| §4 Re-engagement (joined line + reply-by-email) | 9, 10 |
| §5 Teammate visibility (badge + notifications) | 11, 12 |
| §6 Settings UX (fields + previews + nudge) | 7, 13 |
| Testing | per-task + Final verification |
