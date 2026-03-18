# PRD: Widget Identify → CRM Lead Creation & Conversation Identity Backfill

**Date**: 2026-03-18
**Status**: Draft
**Author**: Engineering
**Area**: Support Widget / CRM Integration

---

## 1. Problem Statement

When a visitor uses the help chat widget, they can optionally provide their email and name via the pre-chat form ("Collect Email" setting) or through the SDK `identify()` method. Today, this identification:

1. **Only updates the current session and conversation** — previous conversations by the same visitor remain anonymous in the inbox
2. **Creates a CRM contact as `subscriber`** with `source=support` — not reflecting that they came through the live chat widget and should be treated as leads
3. **Ignores workspace CRM settings** — the `AutoCreateCRMContact`, `DefaultLifecycleStage`, and `AutoPromoteToLead` settings in Support Inbox Settings are never checked during identification
4. **Does not backfill identity** — if a visitor had 3 anonymous conversations and then provides their email, only the active conversation shows their name; the other 2 remain "Anonymous" in the inbox

This creates a fragmented experience where support agents see a mix of anonymous and identified conversations from the same person, and the CRM doesn't properly reflect widget visitors as leads.

---

## 2. Goals

1. **Create CRM leads from widget identification** — when a visitor provides email/name via the widget, create (or match) a CRM contact as a **lead** with `source=live_chat`
2. **Backfill all previous conversations** — update ALL past conversations for the same visitor (`anonymous_id`) with their name, email, and CRM contact link
3. **Real-time inbox updates** — broadcast WebSocket events so the inbox immediately reflects the visitor's name on all their conversations without requiring a page refresh
4. **Respect workspace CRM settings** — honor `AutoCreateCRMContact`, `DefaultLifecycleStage`, and `AutoPromoteToLead` configuration

---

## 3. Non-Goals

- **Dedicated event table in Postgres** — identify events are persisted in ClickHouse via the events pipeline; no separate Postgres audit table is needed
- **Frontend changes** — the frontend already handles displaying `customer_name || customer_email || 'Anonymous'` and reacts to real-time WebSocket invalidation
- **SDK `identify()` method changes** — the SDK already sends `session:upgrade` over WebSocket; the backend changes handle both widget pre-chat form and SDK identify transparently

---

## 4. User Stories

### 4.1 Support Agent: See visitor identity across all conversations
> As a support agent, when a visitor provides their email in the chat widget, I want ALL of their previous conversations in the inbox to immediately show their name and email, so I have full context about who I'm talking to.

**Acceptance Criteria:**
- When visitor identifies via pre-chat form or SDK `identify()`, all their past conversations update from "Anonymous" to their name
- The inbox list view updates in real-time (no page refresh needed)
- Conversation detail sidebar shows email and "View CRM Contact" link
- Only conversations without an existing email are updated (no overwriting)

### 4.2 Sales/CRM User: Widget visitors become leads
> As a sales team member, when a chat widget visitor provides their email, I want them automatically created as a CRM lead, so I can follow up and track them in my sales pipeline.

**Acceptance Criteria:**
- A CRM contact is created with `lifecycle_stage=lead` (configurable via workspace settings)
- Contact source is set to `live_chat` (distinguishable from `support` or `email` sources)
- If a CRM contact with that email already exists, it is linked (not duplicated)
- If `AutoPromoteToLead` is enabled and the existing contact is a `subscriber`, promote to `lead`
- If existing contact is at a higher stage (opportunity, customer), do NOT downgrade

### 4.3 Workspace Admin: Control CRM integration behavior
> As a workspace admin, I want to configure whether widget visitors are auto-created as CRM contacts and at what lifecycle stage, so I can control how our CRM pipeline is populated.

**Acceptance Criteria:**
- `AutoCreateCRMContact` setting (existing) is respected — when `false`, no CRM contact is created
- `DefaultLifecycleStage` setting (existing) controls the lifecycle stage for new contacts (defaults to `lead`)
- `AutoPromoteToLead` setting (existing) controls whether existing `subscriber` contacts are promoted

---

## 5. Current Architecture

### 5.1 Identification Flow (session:upgrade)

```
Widget Pre-Chat Form / SDK identify()
  ↓
WebSocket message: session:upgrade { email, name }
  ↓
widget_handler.go → UpgradeWidgetSession(token, email, name)
  ↓
Current behavior:
  1. Update SupportWidgetSession (email, name, is_anonymous=false)
  2. Update ONLY current conversation (if exists)
  3. matchOrCreateCRMContact → hardcoded subscriber + source=support
```

### 5.2 Key Tables

| Table | Role |
|-------|------|
| `support_widget_sessions` | Active visitor sessions (30-day TTL) |
| `support_conversations` | Conversations with `customer_name`, `customer_email`, `anonymous_id`, `crm_contact_id` |
| `crm_contacts` | CRM contacts/leads (lifecycle_stage differentiates) |

### 5.3 Key Settings (SupportInboxSettings)

| Setting | Type | Current Default | Description |
|---------|------|----------------|-------------|
| `auto_create_crm_contact` | bool | `true` | Auto-create CRM contact from conversations |
| `default_lifecycle_stage` | string | `subscriber` | Lifecycle stage for auto-created contacts |
| `auto_promote_to_lead` | bool | `false` | Promote existing subscribers to lead on identify |

---

## 6. Proposed Changes

### 6.1 Backfill All Conversations on Identify

When `UpgradeWidgetSession` is called, instead of updating only the current conversation, perform a **batch update** on ALL conversations for the same `anonymous_id` within the workspace:

```sql
UPDATE support_conversations
SET customer_email = ?, customer_name = ?, crm_contact_id = COALESCE(?, crm_contact_id)
WHERE workspace_id = ? AND anonymous_id = ?
  AND (customer_email IS NULL OR customer_email = '')
```

**Guard clause**: `customer_email IS NULL OR customer_email = ''` ensures:
- Conversations already identified with a different email are NOT overwritten
- Idempotent — calling upgrade twice with the same email is safe

After the batch update, broadcast a WebSocket `updated` event for each affected conversation so the inbox UI refreshes in real-time.

### 6.2 CRM Lead Creation with Settings

Refactor `matchOrCreateCRMContact` to respect workspace settings:

| Behavior | Before | After |
|----------|--------|-------|
| Check `AutoCreateCRMContact` | Never checked | Skips creation when `false` |
| Email lookup | `List()` with search filter | `GetByEmail()` exact match |
| New contact lifecycle | Hardcoded `subscriber` | Uses `DefaultLifecycleStage` (default: `lead`) |
| New contact source | Hardcoded `support` | `live_chat` for widget-originated |
| Existing subscriber + `AutoPromoteToLead` | Never checked | Promotes to `lead` (never downgrades higher stages) |

### 6.3 Default Lifecycle Stage Change

Change the default `DefaultLifecycleStage` from `subscriber` to `lead` in `DefaultSupportInboxSettings()`.

This is non-breaking: existing workspaces with customized settings have the value stored in JSONB. The `parseSettings()` function unmarshals over defaults, so stored `subscriber` values are preserved.

### 6.4 Updated Flow

```
Widget Pre-Chat Form / SDK identify()
  ↓
WebSocket message: session:upgrade { email, name }
  ↓
widget_handler.go → UpgradeWidgetSession(token, email, name)
  ↓
New behavior:
  1. Update SupportWidgetSession (email, name, is_anonymous=false)
  2. Load workspace SupportInboxSettings
  3. matchOrCreateCRMContact with settings → lead + source=live_chat
  4. Batch-update ALL conversations for this anonymous_id (email, name, crm_contact_id)
  5. Broadcast WebSocket "updated" event for each affected conversation
  6. Log summary
```

---

## 7. Affected Files

| File | Change |
|------|--------|
| `server/internal/model/support_inbox.go` | Change `DefaultLifecycleStage` from `"subscriber"` to `"lead"` |
| `server/internal/repository/support_inbox.go` | Add `UpdateIdentityByAnonymousID` batch update method |
| `server/internal/service/support_inbox.go` | Refactor `matchOrCreateCRMContact` to accept settings; add `getWorkspaceInboxSettings` helper |
| `server/internal/service/support_inbox_widget.go` | Rewrite `UpgradeWidgetSession`; update `WidgetCreateConversation` and `WidgetCreateMessage` callers |

**No frontend changes required** — the existing `ConversationRow.tsx`, `useRealtimeSync`, and `ConversationDetailSidebar.tsx` already handle identity display and real-time cache invalidation.

---

## 8. Edge Cases

| Scenario | Behavior |
|----------|----------|
| **Same email provided twice** | `GetByEmail` returns existing contact (no duplicate). Batch update skips already-identified conversations. |
| **Different email on return visit** | Only fills conversations without email. New CRM contact created for new email. |
| **Settings not configured** | Falls back to defaults: `lifecycle_stage=lead`, `auto_create_crm_contact=true`. |
| **Existing contact at higher lifecycle** | `AutoPromoteToLead` only promotes `subscriber` → `lead`. Never downgrades `opportunity`, `customer`, etc. |
| **No conversation yet when identified** | Identity stored on session. Next message creates a conversation that inherits the identity. |
| **Multiple sessions, same anonymous_id** | Multiple tabs share `anonymous_id`. Upgrade from any session backfills all conversations. |

---

## 9. Testing Plan

1. **Unit test `UpdateIdentityByAnonymousID`**:
   - Create 3 conversations for same `anonymous_id` (2 anonymous, 1 already identified with different email)
   - Call batch update → verify only 2 anonymous conversations updated
   - Verify returned IDs match the 2 updated conversations

2. **Unit test `matchOrCreateCRMContact` with settings**:
   - `AutoCreateCRMContact=false` → returns nil
   - `AutoPromoteToLead=true` + existing subscriber → promoted to lead
   - `AutoPromoteToLead=true` + existing customer → NOT downgraded
   - No existing contact + `DefaultLifecycleStage=lead` → created as lead with `source=live_chat`

3. **Integration test `UpgradeWidgetSession`**:
   - Create anonymous session with 3 conversations
   - Call upgrade with email/name
   - Verify all conversations have email, name, crm_contact_id
   - Verify CRM contact created as lead

4. **Manual E2E test**:
   - Open widget → send messages anonymously across multiple conversations
   - Provide email/name via pre-chat form
   - Verify inbox shows name on ALL conversations in real-time
   - Verify CRM contact created with `lifecycle_stage=lead`, `source=live_chat`
   - Verify "View CRM Contact" link appears in conversation detail sidebar

5. **Build verification**:
   - `cd server && go build ./...`
   - `cd server && go vet ./...`

---

## 10. Future Considerations

- **Merge anonymous conversations**: When a visitor identifies, consider merging all their anonymous conversations into a single thread (out of scope for this PRD)
- **Webhook/event trigger**: Emit a webhook when a visitor identifies, enabling external integrations (handled by ClickHouse events pipeline)
- **Contact enrichment**: Auto-enrich CRM contact with company info, social profiles etc. from email domain (separate feature)
