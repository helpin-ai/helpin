# Reply Expectations Plan — 2026-04-15

Ship Intercom-style reply-time expectations and a delay/outage notice for
the support widget, without cloning every Intercom surface. Phase 1 is a
hard-scoped MVP that delivers the configurable reply time, a per-mailbox
override, and a global special notice. Phase 2 covers dynamic timing,
assignment-scoped display, workflow steps, and send orchestration.

## Problem
Today every online-hours widget impression shows the same hardcoded line —
`We typically reply in a few minutes` (`support_availability_resolver.go:20`).
Admins can configure office hours and an outside-hours message, but cannot:

- set an in-hours reply time (`in a few minutes`, `in a few hours`, `in a day`, custom),
- give a busy team (e.g. engineering) a longer SLA than a fast one (e.g. sales),
- surface a temporary outage / backlog banner to all customers without changing copy in code,
- preview what the widget will show — the admin settings preview drifts from the production resolver (`frontend/src/components/settings/chat-widget/utils.ts:45`).

## Out of scope for Phase 1 (captured in Phase 2)
- `display_mode: after_assignment` — interacts with the `teammate_joined` pill (joined fires on first reply, not assignment), needs the UX window resolved first.
- `dynamic` reply-time mode (median from real conversation analytics) — requires a new aggregation pipeline.
- Intercom's "send immediately vs after 2 minutes" automation — that's orchestration behavior, not UI, and belongs alongside the automation/workflow controls.
- Workflow-step "show expected reply time" — no workflow builder exists yet for this kind of UX step.

## Recommendation: keep dynamic mode out of Phase 1
Dynamic reply expectations are not a settings toggle. They are a small analytics
product with real correctness risk, especially around business-hours timing.
Phase 1 should remain preset-only and ship the configurable reply expectation,
special notice, and widget/admin preview parity first.

Phase 1 should still shape the API so Phase 2 can plug in without another
contract rewrite:

- resolver returns both human-facing copy and structured metadata,
- widget consumes structured reply-expectation fields instead of parsing copy,
- settings model reserves a clear place for `preset` vs `dynamic` mode,
- preset fallback remains the default whenever dynamic data is unavailable.

This keeps the first release focused while avoiding rework when dynamic mode is
added later.

## Phase 1 Scope (single PR, two commits)

### Settings surface
Three knobs on the workspace support inbox settings, plus one override per mailbox.

| Setting (workspace) | Type | Default | Semantics |
|---------------------|------|---------|-----------|
| `reply_time_preset` | enum: `few_minutes` \| `few_hours` \| `same_day` \| `custom` | `few_minutes` | Drives in-hours copy and structured `reply_time_minutes`. |
| `reply_time_custom_minutes` | int, nullable | nil | Required when `reply_time_preset='custom'`. Clamped to `1..10080` (7 days). |
| `special_notice_text` | string, nullable | nil | When set (non-empty), renders as a slim banner at the top of conversation-capable views. Empty string = banner off. |

| Setting (per-mailbox) | Type | Default | Semantics |
|-----------------------|------|---------|-----------|
| `reply_time_preset` | enum \| null | null | Null = inherit workspace. Otherwise overrides for conversations routed into this mailbox. |
| `reply_time_custom_minutes` | int, nullable | nil | Same validation as workspace. |

Outside-hours behavior is unchanged (still the existing `next_online_at` + `outside_hours_message`).

### Copy rules (resolver)

```
in_hours, preset=few_minutes → "Usually replies in a few minutes"
in_hours, preset=few_hours   → "Usually replies in a few hours"
in_hours, preset=same_day    → "Usually replies within a day"
in_hours, preset=custom,  N  → "Usually replies in about {N} minutes" / "in about {N} hours" / "within {N} days" (unit chosen by magnitude)
out_of_hours                 → unchanged: outside_hours_message, or fallback "Back {day} {time}"
```

Mailbox override resolves first; workspace is the fallback.

## Data model

### Backend (Go, GORM AutoMigrate)

Additions to `model.SupportInboxSettings` (`server/internal/model/support_inbox.go`):

```go
// Reply-time expectations rendered on the widget during business hours.
ReplyTimePreset       string `json:"reply_time_preset" gorm:"size:20;default:'few_minutes'"`
ReplyTimeCustomMinutes *int  `json:"reply_time_custom_minutes" gorm:"default:null"`

// Optional global notice rendered as a slim banner above conversation
// surfaces (outages, backlog, maintenance). Empty = banner off.
SpecialNoticeText *string `json:"special_notice_text" gorm:"type:text;default:null"`
```

Additions to `model.SupportMailbox` (`server/internal/model/support_inbox.go`):

```go
// Optional reply-time override — null means inherit workspace setting.
ReplyTimePreset        *string `json:"reply_time_preset,omitempty" gorm:"size:20;default:null"`
ReplyTimeCustomMinutes *int    `json:"reply_time_custom_minutes,omitempty" gorm:"default:null"`
```

No dbmigrate SQL needed for the column adds — GORM AutoMigrate handles nullable additions. One defensive dbmigrate for **enum validation** at the DB layer:

```sql
-- 202604160001_support_reply_time_constraints.sql
ALTER TABLE support_inbox_settings
    ADD CONSTRAINT support_inbox_settings_reply_preset_chk
    CHECK (reply_time_preset IN ('few_minutes','few_hours','same_day','custom'))
    NOT VALID;
ALTER TABLE support_inbox_settings VALIDATE CONSTRAINT support_inbox_settings_reply_preset_chk;

ALTER TABLE support_mailboxes
    ADD CONSTRAINT support_mailboxes_reply_preset_chk
    CHECK (reply_time_preset IS NULL OR reply_time_preset IN ('few_minutes','few_hours','same_day','custom'))
    NOT VALID;
ALTER TABLE support_mailboxes VALIDATE CONSTRAINT support_mailboxes_reply_preset_chk;
```

### Widget config payload (public, customer-facing)

Structured additions to `WidgetConfig.availability` (`server/internal/service/support_inbox_widget.go:896`):

```go
type WidgetAvailability struct {
    IsOnline            bool    `json:"is_online"`
    StatusText          string  `json:"status_text"`           // kept for back-compat
    ReplyTimeText       string  `json:"reply_time_text"`       // kept for back-compat
    OutsideHoursMessage string  `json:"outside_hours_message,omitempty"`
    NextOnlineAt        *string `json:"next_online_at,omitempty"`

    // Phase 1 additions — structured so the widget can render per-surface.
    ReplyTimePreset   string  `json:"reply_time_preset,omitempty"`   // few_minutes|few_hours|same_day|custom
    ReplyTimeMinutes  *int    `json:"reply_time_minutes,omitempty"`  // resolved numeric, only set for custom
    SpecialNoticeText *string `json:"special_notice_text,omitempty"` // nil or non-empty
    MailboxID         *string `json:"mailbox_id,omitempty"`          // whose override (if any) was applied; null = workspace
}
```

Kept fields are unchanged so existing SDK builds don't break. New fields are additive; widget reads them when present.

## Backend implementation

### Resolver (`server/internal/service/support_availability_resolver.go`)
Extract a `resolveReplyExpectation(ctx, workspaceID, mailboxID) (preset string, minutes *int, text string)` helper. Order:

1. If `mailboxID` set and its `reply_time_preset` is non-null → use mailbox preset/custom.
2. Else → workspace `reply_time_preset` / `reply_time_custom_minutes`.
3. Format `text` via `formatReplyTimeCopy(preset, minutes)`.

`formatReplyTimeCopy` is pure and exported for test + frontend-mirror use. Table-driven unit test covering every preset × magnitude branch.

### Config loader (`support_inbox_widget.go`)
- Accept optional `mailbox_id` context — already present in some call sites via `SupportWidgetSession.ConversationID` → `Conversation.MailboxID`.
- Populate the structured fields on `WidgetAvailability`.
- For sessions without a resolved mailbox, `MailboxID` is `nil` and workspace preset applies.

### REST surface
- `PATCH /api/support/inbox/settings` accepts the new fields; validation rejects invalid preset or out-of-range custom minutes.
- `PATCH /api/support/mailboxes/{id}` accepts the override fields; null clears override.

## Frontend implementation

### Admin settings (`frontend/src/components/settings/ChatGeneralTab.tsx`)
One new section *inside the existing Availability block*, not a separate top-level block:

- **Reply expectations**
  - Preset select: `Usually a few minutes / Usually a few hours / Within a day / Custom`.
  - Custom-minutes input, shown only when preset = custom. `type="number"`, `min=1`, `max=10080`.
- **Special notice** (separate subsection, clearly labelled "Outage / maintenance banner"):
  - Textarea, max 500 chars, live character count.
  - Empty-string saves null.

Per-mailbox override lives on the existing mailbox edit form (`TeamInboxesTab`) with a "Inherit workspace default" switch + the same preset/custom inputs when overridden.

### Preview parity (`frontend/src/components/settings/chat-widget/utils.ts`)
Replace the simplified preview helper with a TS mirror of `formatReplyTimeCopy`. Same table, same inputs, same outputs — guard with a unit test that imports both the backend test cases and asserts the TS version matches string-for-string.

### Widget rendering

| Surface | File | New behavior |
|---------|------|--------------|
| Home primary-action subtitle | `packages/widget-core/src/components/HomeView.tsx:26` | Uses `reply_time_text` (same string, driven by new resolver). |
| Conversation header subtitle | `packages/widget-core/src/components/ConversationView.tsx:376` | Same. |
| Human-handoff banner | `packages/widget-core/src/components/ConversationView.tsx:486` | Unchanged. |
| Waiting-for-teammate row | `packages/widget-core/src/components/ConversationView.tsx:537` | Unchanged. |
| **NEW**: special notice banner | New slim banner at top of `ConversationView` and `MessagesView` | Renders when `config.availability.specialNoticeText` is non-empty. Dismissible per session (local-storage keyed on workspace + message hash). |

New CSS in `packages/widget-core/src/styles/widget.css`:
```
.helpin-special-notice { … subtle amber background, icon, dismiss button … }
```

## Phase 2: Dynamic Reply-Time Mode

### Goal
Add a dynamic reply expectation derived from recent real reply latency, while
keeping Phase 1 preset mode as the fallback and display contract.

### What dynamic mode means
Dynamic mode does **not** render raw latency values directly in the widget.
Instead, it computes a recent reply-time statistic, buckets it into the same
copy families as Phase 1 presets, and lets the resolver choose the final
display string.

That means the widget contract stays stable:

- `reply_time_mode` says whether the expectation came from `preset` or `dynamic`
- `reply_time_bucket` says which display family applies
- `reply_time_text` stays the actual rendered copy
- optional metadata is available for admin transparency only

### Recommended rollout order

#### Phase 2A — dynamic, workspace-level, business-hours aware, no holidays
First cut should be:

- workspace-level dynamic timing only,
- rolling-window statistic (p50 or p75),
- business-hours schedule aware,
- no holiday calendar support yet,
- preset fallback whenever sample size is too low or computed data is stale.

This avoids sparse mailbox-level datasets and keeps the first dynamic release
 operationally understandable.

#### Phase 2B — mailbox-level override
Per-mailbox dynamic timing can be added only after workspace-level dynamic mode
has proven useful and enough data density exists to avoid constant fallback.

#### Phase 2C — automation / send timing
Intercom-style "send immediately vs after 2 minutes" behavior belongs in the
automation/workflow layer, not the resolver. Keep it out of the dynamic
analytics implementation.

### Data pipeline requirements

#### 1. Capture latency samples at write time
Do not compute first-reply latency on reads.

Create a new table:

```sql
support_reply_latency_samples (
  workspace_id uuid not null,
  mailbox_id uuid null,
  conversation_id uuid not null,
  first_reply_business_minutes int not null,
  first_reply_user_id uuid null,
  created_at timestamptz not null default now()
)
```

Recommended indexes:

- `(workspace_id, created_at desc)`
- `(mailbox_id, created_at desc)`
- unique on `(conversation_id)` to prevent duplicate sample capture

Populate this when the first non-internal, non-AI human reply is created. The
existing first-teammate-reply branch used for `teammate_joined` is the right
place to hang this logic.

#### 2. Use a business-hours clock
Dynamic mode must measure **business minutes**, not elapsed wall-clock time.
Otherwise overnight replies distort the statistic for every non-24/7 workspace.

Add a helper like:

```go
businessMinutesBetween(settings model.SupportInboxSettings, start, end time.Time) (int, error)
```

Initial scope:

- use configured business-hours schedule only,
- honor timezone and DST transitions,
- do **not** block Phase 2A on holiday calendars.

If holiday support is needed later, add it as a separate enhancement after the
schedule-aware version is stable.

#### 3. Use a robust statistic
Do not use arithmetic mean.

Recommended:

- rolling 14- or 30-day window,
- p50 or p75,
- minimum sample size around `10` before trusting dynamic mode.

Below threshold, fall back to Phase 1 preset mode and say so in the admin UI.

#### 4. Recompute on a schedule
Do not recompute during widget requests.

Use existing Temporal/cron infrastructure to recompute at least hourly and
write results to persisted fields. Hourly is sufficient for a "usual reply
time" feature.

### Schema additions for Phase 2

#### Aggregation table
New table:

```sql
support_reply_latency_samples (
  workspace_id uuid not null,
  mailbox_id uuid null,
  conversation_id uuid not null,
  first_reply_business_minutes int not null,
  first_reply_user_id uuid null,
  created_at timestamptz not null default now()
)
```

#### Settings / mailbox state
Add explicit mode plus computed values to workspace settings first, and later to
mailboxes if Phase 2B ships:

```sql
reply_time_mode        enum('preset','dynamic') default 'preset'
computed_reply_minutes int null
computed_sample_size   int null
computed_updated_at    timestamptz null
```

Mailbox-level equivalents should be deferred until Phase 2B.

### Display bucketing
Dynamic mode should map computed minutes into the same display families as
Phase 1 presets:

- `<10m` → `few_minutes`
- `10–60m` → `under_an_hour` or folded into `few_minutes` if Phase 1 remains simpler
- `1–3h` → `few_hours`
- `3–24h` → `same_day` / `within_the_day`
- `>24h` → `within_n_days`

The exact bucket taxonomy can stay aligned with Phase 1 copy families so the
widget UI does not need a second rendering model.

### Resolver contract for Phase 2
The resolver should:

1. Prefer dynamic mode only when:
   - mode is enabled,
   - computed value exists,
   - sample size passes threshold,
   - computed value is fresh enough.
2. Otherwise fall back to preset mode deterministically.
3. Return both:
   - human-facing copy (`reply_time_text`)
   - structured metadata (`mode`, `bucket`, `minutes`, `sample_size`, `updated_at`)

### Admin UX for Phase 2
The settings screen should expose:

- mode selector: `Preset` / `Dynamic`
- currently computed reply time
- sample size
- last updated timestamp
- explicit fallback state:
  `Not enough data yet — using your preset`

Do not present dynamic mode as active when the resolver is still falling back.

### Explicit non-goals for Phase 2A

- holiday calendar support,
- mailbox-level dynamic timing,
- send-delay orchestration,
- workflow-step integration,
- real-time recomputation.

### Exit criteria for Phase 2A

- reply latency samples captured once per conversation,
- business-hours-aware computation implemented and tested,
- hourly recompute job persists workspace-level computed values,
- resolver can choose dynamic vs preset safely,
- widget continues to render only bucketed human-friendly copy,
- admin UI clearly shows when dynamic mode is active vs falling back.

## UI mockups

### Admin: ChatGeneralTab — Availability section
New subsections rendered inside the existing Availability block.

```
┌─ Availability ───────────────────────────────────────────────────┐
│                                                                  │
│  ☑  Show business hours on the widget                            │
│                                                                  │
│  Time zone          [ America/New_York                      ▾ ]  │
│                                                                  │
│  Schedule                                                        │
│   Mon  09:00 – 17:00     Fri  09:00 – 17:00                      │
│   Tue  09:00 – 17:00     Sat  closed                             │
│   Wed  09:00 – 17:00     Sun  closed                             │
│   Thu  09:00 – 17:00                                             │
│                                                                  │
│  Outside-hours message                                           │
│  ┌────────────────────────────────────────────────────────────┐  │
│  │ We're out — we'll get back first thing in the morning.     │  │
│  └────────────────────────────────────────────────────────────┘  │
│                                                                  │
│  ─── Reply expectations ────────────────────────────────────     │
│                                                                  │
│  During business hours, tell customers when to expect a reply.   │
│                                                                  │
│  Preset              [ Usually a few minutes              ▾ ]    │
│                      ├─ Usually a few minutes                    │
│                      ├─ Usually a few hours                      │
│                      ├─ Within a day                             │
│                      └─ Custom…                                  │
│                                                                  │
│  Custom time          [ 30 ] minutes        ← only when Custom   │
│                                                                  │
│  Preview: "Usually replies in a few minutes"                     │
│                                                                  │
│  ─── Outage / maintenance banner ───────────────────────────     │
│                                                                  │
│  Show a temporary notice across the widget when something is     │
│  off. Leave empty to hide.                                       │
│                                                                  │
│  ┌────────────────────────────────────────────────────────────┐  │
│  │                                                            │  │
│  │                                                            │  │
│  └────────────────────────────────────────────────────────────┘  │
│                                                0 / 500           │
│                                                                  │
│                                   [ Cancel ]   [ Save changes ]  │
└──────────────────────────────────────────────────────────────────┘
```

### Admin: TeamInboxesTab — per-mailbox override
Added to the existing mailbox edit drawer / dialog.

```
┌─ Edit inbox: Engineering ────────────────────────────────────────┐
│  Name       [ Engineering                                      ] │
│  Slug       [ engineering                                      ] │
│  …                                                               │
│                                                                  │
│  ─── Reply expectations ────────────────────────────────────     │
│                                                                  │
│  ◉  Inherit workspace default  (Usually a few minutes)           │
│  ○  Override for this inbox                                      │
│                                                                  │
│       Preset       [ Usually a few hours                  ▾ ]    │
│       Custom time  [ — ] minutes                                 │
│                                                                  │
│                                           [ Cancel ]  [ Save ]   │
└──────────────────────────────────────────────────────────────────┘
```

### Widget — Home view subtitle
`reply_time_text` drives the subtitle under the primary action.

```
┌─────────────────────────────────┐
│  ←       Acme Support       ⋮ ✕ │
│                                 │
│     Hi there 👋                 │
│     How can we help?            │
│                                 │
│  ┌───────────────────────────┐  │
│  │  Send us a message     →  │  │
│  │  Usually replies in a     │  │
│  │  few minutes              │  │  ← from reply_time_text
│  └───────────────────────────┘  │
│                                 │
│  ┌───────────────────────────┐  │
│  │  Search help articles  →  │  │
│  └───────────────────────────┘  │
│                                 │
│  [ 💬 ]   [ ? ]   [ ⚙ ]         │
└─────────────────────────────────┘
```

### Widget — Conversation header subtitle
Same string source, rendered as the header subline.

```
┌─────────────────────────────────┐
│  ← [logo] Acme Support      ⋮ ✕ │
│           Usually replies in a  │  ← reply_time_text
│           few minutes           │
├─────────────────────────────────┤
│                            Hi   │
│                      ───────    │
│  [H] Helpin AI                  │
│      How can I help you today?  │
│                                 │
│  ┌─────────────────────────┐    │
│  │  Ask a question…        │    │
│  └─────────────────────────┘    │
└─────────────────────────────────┘
```

### Widget — Special notice banner (Phase 1's one new surface)
Slim banner anchored above the conversation thread. Renders on both
`ConversationView` and `MessagesView` when `special_notice_text` is
non-empty. Dismissible per session (close button); dismissal state
lives in local storage keyed on `workspace_id + sha(text)` so a new
notice re-appears.

```
┌─────────────────────────────────┐
│  ← [logo] Acme Support      ⋮ ✕ │
│           Usually replies in a  │
│           few minutes           │
├─────────────────────────────────┤
│ ⚠  Our team is catching up on   │ ← .helpin-special-notice
│    a backlog — replies may be   │    amber bg, dismissible
│    slower today.          ✕     │
├─────────────────────────────────┤
│                            Hi   │
│                      ───────    │
│  [H] Helpin AI                  │
│      How can I help you today?  │
│                                 │
│  ┌─────────────────────────┐    │
│  │  Ask a question…        │    │
│  └─────────────────────────┘    │
└─────────────────────────────────┘
```

### Widget — preset copy examples
How the five supported shapes render in the widget:

```
preset=few_minutes       → "Usually replies in a few minutes"
preset=few_hours         → "Usually replies in a few hours"
preset=same_day          → "Usually replies within a day"
preset=custom, 30 min    → "Usually replies in about 30 minutes"
preset=custom, 180 min   → "Usually replies in about 3 hours"
preset=custom, 2880 min  → "Usually replies within 2 days"
```

Rollout of magnitudes is defined by `formatReplyTimeCopy` and mirrored
byte-for-byte on the frontend preview via the shared fixture.

## Tests

### Backend
- `support_availability_resolver_test.go` — table-driven `formatReplyTimeCopy` covering every preset × magnitude × locale-neutral edge case (1 min, 59 min, 60 min, 119 min, 120 min, 1440 min, 4320 min).
- Resolver test: mailbox override takes precedence over workspace default.
- Resolver test: invalid preset on mailbox falls back (defensive, even though DB constraint blocks write).
- Handler test: PATCH settings rejects invalid preset or out-of-range custom minutes with 400.
- WS / config test: widget payload includes structured `reply_time_preset`, `reply_time_minutes`, `special_notice_text`.

### Frontend
- `utils.ts` preview mirror: vitest case importing the same scenario table as the Go test JSON dump (checked in under `frontend/test-data/reply-time-cases.json`, generated by a small Go test hook). This keeps the two implementations literally in sync.
- Widget `ConversationView` test: special-notice banner renders when non-empty, dismissible.
- Widget `HomeView` test: `reply_time_text` from the structured payload flows through.
- Admin settings test: preset select + custom field toggles and saves correct payload.

## Rollout

1. Ship migration-only PR first (constraints, nullable columns) so schema lands before code that depends on it.
2. Ship backend resolver + config payload (keeps old string fields for existing clients).
3. Ship admin settings UI.
4. Ship widget rendering (home subtitle auto-picks up new copy; special notice banner is new).
5. Rebuild widget-core + sdk-js, redeploy.
6. Monitor: count of workspaces with non-default preset; count with special notice active. Flag if uptake is low — indicates settings discoverability problem.

## Phase 2 (captured — do not build now)

| Feature | Notes |
|---------|-------|
| `display_mode: after_assignment` | Needs UX resolution for the gap between assignment and first reply. The `teammate_joined` pill already fires on first reply; this mode would hide reply time until then. |
| `dynamic` reply time | Add `reply_time_mode: preset \| dynamic`. Dynamic requires a rolling-window aggregation over `support_messages` response latencies. New repo method + a periodic recompute worker. |
| Per-team (not per-mailbox) overrides | Depends on the team ↔ mailbox model evolving. Some workspaces pin multiple mailboxes to one team; decide the override level before building. |
| Send-timing orchestration (immediate vs 2-minute) | Lives in the AI/escalation path (`support_ai.go`), not in this plan. Likely a small rule engine on `support_inbox_settings`. |
| Workflow step "show expected reply time" | Requires the workflow builder to exist as a first-class concept. Out of scope until that ships. |

## Open Questions

- **Overriding at the team level later** — if we ship mailbox override in Phase 1, are we OK with workspaces that later want team-level override doing a data migration? I think yes; mailboxes are stable enough.
- **Locale / i18n** — current copy is English-only. Phase 1 keeps it that way; Phase 2 can thread locale through the resolver once we have translation infra.
- **Character limit on special notice** — 500 chars feels right for a banner; confirm with product.
- **Dismissal persistence** — per-session dismissal is the simplest. Never-remind-me-again probably isn't worth the extra state.
