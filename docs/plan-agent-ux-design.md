# Agent & Automation UX Design

## Current UX Problems

### 1. Technical language everywhere
Users see: "Runtime Kind: opencode/openclaw/zeroclaw", "Trigger Mode: auto_on_assignment", "Backing User ID", "System Prompt", "Token Budget", "Capability Profile". A marketing manager has no idea what any of this means.

### 2. Buried and fragmented
- Agent creation: PM > Agents page (only admins know it exists)
- Agent assignment to stories: inside Story detail > Delivery panel
- Agent assignment to epics: inside Epic detail > Planning section
- Agent assignment to support: inside Support page > conversation header
- CRM automation: CRM > Settings (completely separate, no agent concept)
- PM automation rules: Workspace Settings > somewhere

A user has to know 4-5 different locations to understand "what's automated in my workspace."

### 3. Class-first instead of goal-first
The create flow asks "Pick an agent class" first. Users don't think in classes — they think: "I want my support tickets triaged automatically" or "I want code reviewed before merge."

### 4. No discoverability
No empty states that suggest automation. No "you could automate this" hints. No presets. New workspace = blank agents page with a "Create Agent" button and no guidance.

---

## Design Principles

1. **Goal-first, not class-first** — users describe what they want, system picks the class
2. **Progressive disclosure** — simple by default, advanced settings hidden but accessible
3. **One home** — all automation lives in one place, with contextual entry points
4. **Team-owned** — each team sees and manages their own automations
5. **Show, don't tell** — preview what the agent will do before activating

---

## Navigation

### Where it lives

Move from `PM > Agents` to a **workspace-level section**:

```
Sidebar:
  My Work
  ─────────────
  Projects (PM)
    Stories / Sprints / Epics
    Objectives / Roadmap
  Support
  CRM
  Docs
  ─────────────
  Automations    ← NEW top-level section
  ─────────────
  Settings
```

**Why top-level?** Automations span modules. A support triage agent isn't a "PM" feature. A deal follow-up agent isn't a "CRM settings" feature. They're workspace capabilities.

### Contextual entry points (keep these)

Each module keeps lightweight entry points that link back to the main Automations page:

- **Story detail panel** — "Delivery" section stays, but "Manage agents" links to Automations
- **Epic detail panel** — "Planning" section stays, agent selector pulls from Automations
- **Support page** — agent assignment dropdown stays, "Set up automation" links to Automations
- **CRM settings** — "Signal detection" links to the same Automations page

---

## Automations Page (`/w/{slug}/automations`)

### Layout

```
┌─────────────────────────────────────────────────────────────────────┐
│ Automations                                          [+ New]       │
│                                                                     │
│ Filter: [All teams ▾]  [All types ▾]  [All statuses ▾]  [Search]  │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  ┌────────────────────────────────────────────────────────────────┐ │
│  │  ⚡ Story Implementation                    Engineering team   │ │
│  │  Implements assigned stories with code changes and tests       │ │
│  │                                                                │ │
│  │  Runs: on assignment  ·  Model: Claude Sonnet  ·  ● Active    │ │
│  │  Last run: 2h ago  ·  47 completed  ·  3 failed               │ │
│  └────────────────────────────────────────────────────────────────┘ │
│                                                                     │
│  ┌────────────────────────────────────────────────────────────────┐ │
│  │  ⚡ Support Triage                          Support team       │ │
│  │  Reads new conversations and drafts initial replies            │ │
│  │                                                                │ │
│  │  Runs: every 30 min  ·  Model: Claude Haiku  ·  ● Active      │ │
│  │  Last run: 12m ago  ·  203 completed  ·  Approval: required   │ │
│  └────────────────────────────────────────────────────────────────┘ │
│                                                                     │
│  ┌────────────────────────────────────────────────────────────────┐ │
│  │  ⚡ Sprint Auto-Create                      Engineering team   │ │
│  │  Creates next sprint when current one completes                │ │
│  │                                                                │ │
│  │  Runs: on event  ·  Rule-based  ·  ● Active                   │ │
│  └────────────────────────────────────────────────────────────────┘ │
│                                                                     │
│  ┌────────────────────────────────────────────────────────────────┐ │
│  │  ⚡ Deal Follow-up                          Sales team         │ │
│  │  Drafts follow-up notes for deals with no activity in 7 days  │ │
│  │                                                                │ │
│  │  Runs: daily at 9am  ·  Model: Claude Sonnet  ·  ○ Draft      │ │
│  └────────────────────────────────────────────────────────────────┘ │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

### Card anatomy

Each automation card shows:
- **Name** — human-readable, user-chosen
- **Description** — one line, what it does in plain language
- **Team** — which team owns it
- **When it runs** — "on assignment", "every 30 min", "daily at 9am", "manually", "on event"
- **Model** — friendly name (Claude Sonnet, not "anthropic/claude-sonnet-4-20250514")
- **Status** — Active (green), Paused (gray), Draft (amber), Error (red)
- **Stats** — last run time, completed count, failed count
- **Approval badge** — if approval is required, show it

### Empty state

First-time users see:

```
┌─────────────────────────────────────────────────────────────────┐
│                                                                 │
│                     ⚡ Automate your workflow                    │
│                                                                 │
│  Set up agents that handle repetitive work for your team —      │
│  triage support tickets, implement stories, follow up on        │
│  deals, or anything else you can describe.                      │
│                                                                 │
│                   [+ Create your first automation]              │
│                                                                 │
│  ── Quick start templates ──────────────────────────────────    │
│                                                                 │
│  ┌──────────────┐ ┌──────────────┐ ┌──────────────┐            │
│  │ 🔧 Story     │ │ 💬 Support   │ │ 📋 Epic      │            │
│  │ Engineer     │ │ Triage       │ │ Planner      │            │
│  │              │ │              │ │              │            │
│  │ Implement    │ │ Draft replies│ │ Break down   │            │
│  │ assigned     │ │ to new       │ │ epics into   │            │
│  │ stories      │ │ tickets      │ │ stories      │            │
│  └──────────────┘ └──────────────┘ └──────────────┘            │
│  ┌──────────────┐ ┌──────────────┐ ┌──────────────┐            │
│  │ 🔍 Code      │ │ 💼 Deal      │ │ 📝 Custom    │            │
│  │ Reviewer     │ │ Follow-up    │ │              │            │
│  │              │ │              │ │ Describe     │            │
│  │ Review code  │ │ Follow up on │ │ what you     │            │
│  │ before merge │ │ stale deals  │ │ need         │            │
│  └──────────────┘ └──────────────┘ └──────────────┘            │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

---

## Create Flow

### Step 1: What should this do?

Two paths:
- **Pick a template** (pre-fills everything, user just names it and picks a team)
- **Describe it** (free-text, system suggests class + tools)

```
┌─────────────────────────────────────────────────────────────────┐
│ New Automation                                           [X]    │
│                                                                 │
│ Start from a template or describe what you need.                │
│                                                                 │
│  Templates                                                      │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │ 🔧 Story Engineer     Implement stories with code       │    │
│  │ 🔍 Code Reviewer      Review code changes               │    │
│  │ 📋 Epic Planner       Break epics into stories           │    │
│  │ 💬 Support Triage     Draft replies to conversations     │    │
│  │ 💼 Deal Follow-up     Follow up on stale CRM deals      │    │
│  │ 📄 Docs Updater       Keep documentation current         │    │
│  └─────────────────────────────────────────────────────────┘    │
│                                                                 │
│  Or describe what you want:                                     │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │ "Triage new support tickets, tag them by category,      │    │
│  │  and create a PM story if it looks like a bug"          │    │
│  └─────────────────────────────────────────────────────────┘    │
│                                                                 │
│                                                  [Continue →]   │
└─────────────────────────────────────────────────────────────────┘
```

If user picks a template → skip to Step 3 (everything pre-filled).
If user types a description → system suggests tools and targets, go to Step 2.

### Step 2: Configure (only for custom / after template selection to review)

Single-page form with sensible defaults, no tabs:

```
┌─────────────────────────────────────────────────────────────────┐
│ Configure: Support Triage                                [X]    │
│                                                                 │
│ Name                                                            │
│ ┌─────────────────────────────────────────────────────────┐     │
│ │ Support Triage                                          │     │
│ └─────────────────────────────────────────────────────────┘     │
│                                                                 │
│ Team                                                            │
│ ┌─────────────────────────────────────────────────────────┐     │
│ │ Support ▾                                               │     │
│ └─────────────────────────────────────────────────────────┘     │
│                                                                 │
│ When should it run?                                             │
│ ┌────────────┐ ┌────────────┐ ┌────────────┐ ┌────────────┐   │
│ │ ○ Manually │ │ ● On event │ │ ○ Schedule │ │○ On assign │   │
│ └────────────┘ └────────────┘ └────────────┘ └────────────┘   │
│                                                                 │
│ Run when:  [New conversation is created ▾]                      │
│                                                                 │
│ What can it do?                                                 │
│ ☑ Read support conversations                                    │
│ ☑ Draft replies (requires approval)                             │
│ ☐ Update conversation status                                    │
│ ☐ Create PM stories                                             │
│ ☐ Read related docs                                             │
│ ☐ Read CRM contacts                                             │
│                                                                 │
│ Requires approval before acting?                                │
│ ● Yes, always    ○ Only for destructive actions    ○ Never     │
│                                                                 │
│ AI Model                                                        │
│ ┌─────────────────────────────────────────────────────────┐     │
│ │ Claude Sonnet (recommended) ▾                           │     │
│ └─────────────────────────────────────────────────────────┘     │
│                                                                 │
│ ▸ Advanced settings                                             │
│   (Additional instructions, monthly token limit, concurrency)   │
│                                                                 │
│                                       [Save as draft]  [Activate]│
└─────────────────────────────────────────────────────────────────┘
```

**Key UX decisions:**
- "When should it run?" replaces "Trigger Mode" — plain English options
- "What can it do?" replaces hidden class-locked tool sets — checkboxes with human labels
- "Requires approval?" replaces the hidden per-class boolean — user chooses
- "Additional instructions" replaces "System Prompt"
- "Monthly token limit" replaces "Token Budget" (with helper: "~1,000 tokens = 1 page of text")
- "Runtime Kind", "Backing User ID", "Skills", "Capability Profile" all hidden under Advanced or removed entirely
- Model shows friendly names: "Claude Sonnet (recommended)", "Claude Haiku (fast & cheap)", "Claude Opus (most capable)"

### Step 3: Review & Activate

```
┌─────────────────────────────────────────────────────────────────┐
│ Ready to activate: Support Triage                        [X]    │
│                                                                 │
│ Here's what this automation will do:                            │
│                                                                 │
│  When    A new support conversation is created                  │
│  It will Read the conversation and draft a reply                │
│  Then    Wait for your approval before sending                  │
│  Team    Support                                                │
│  Model   Claude Sonnet                                          │
│                                                                 │
│ ┌─────────────────────────────────────────────────────────┐     │
│ │ 💡 Try a dry run first?                                 │     │
│ │ Pick an existing conversation to see what the agent      │     │
│ │ would do — no changes will be made.                      │     │
│ │                                         [Run preview]    │     │
│ └─────────────────────────────────────────────────────────┘     │
│                                                                 │
│                                       [Save as draft]  [Activate]│
└─────────────────────────────────────────────────────────────────┘
```

---

## Automation Detail Page (`/w/{slug}/automations/{id}`)

After creation, clicking a card opens a detail page:

```
┌─────────────────────────────────────────────────────────────────┐
│ ← Automations                                                   │
│                                                                 │
│ Support Triage                              ● Active  [Edit]    │
│ Reads new conversations and drafts replies                      │
│ Team: Support · Runs: on new conversation · Approval: required  │
│                                                                 │
│ ── Stats (last 30 days) ────────────────────────────────────    │
│ ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────────────┐   │
│ │  203     │ │  194     │ │  9       │ │  ~2.3 min        │   │
│ │  Runs    │ │  Completed│ │  Failed  │ │  Avg duration    │   │
│ └──────────┘ └──────────┘ └──────────┘ └──────────────────┘   │
│                                                                 │
│ ── Recent runs ─────────────────────────────────────────────    │
│                                                                 │
│ ┌────────────────────────────────────────────────────────────┐  │
│ │ Conversation #482 "Can't login after..."   ✓ Completed     │  │
│ │ 12 min ago · 1,240 tokens · Approved by Sarah             │  │
│ ├────────────────────────────────────────────────────────────┤  │
│ │ Conversation #481 "Billing question about..."  ⏳ Pending  │  │
│ │ 28 min ago · 890 tokens · Awaiting approval                │  │
│ │                                    [Approve]  [Reject]     │  │
│ ├────────────────────────────────────────────────────────────┤  │
│ │ Conversation #479 "Integration not working"  ✗ Failed      │  │
│ │ 1h ago · Error: token budget exceeded                      │  │
│ └────────────────────────────────────────────────────────────┘  │
│                                                                 │
│ ── Configuration ───────────────────────────────────────────    │
│ (collapsible section showing current settings)                  │
│                                                                 │
│                                        [Pause]  [Delete]        │
└─────────────────────────────────────────────────────────────────┘
```

---

## Tool Labels (Human-readable mapping)

Users never see internal tool names. The UI shows:

| Internal Tool | User-Facing Label | Category |
|--------------|-------------------|----------|
| `read_file`, `read_file_range`, `list_directory`, `search_files` | Read project files | Code |
| `write_file` | Edit project files | Code |
| `ripgrep`, `grep` | Search code | Code |
| `list_symbols` | Analyze code structure | Code |
| `run_command` | Run terminal commands | Code |
| `create_branch`, `commit_and_push`, `open_pr` | Git operations (branch, commit, PR) | Code |
| `add_story_comment` | Comment on stories | Projects |
| `update_story_state` | Update story status | Projects |
| `list_story_checklist` | Read story checklists | Projects |
| `list_conversation_messages` | Read support conversations | Support |
| `draft_support_reply` | Draft support replies | Support |
| `update_conversation_status` | Update conversation status | Support |
| `web_search` | Search the web | Research |
| `list_deals`, `update_deal_stage`, `add_deal_note` | Manage CRM deals | CRM |
| `list_contacts`, `create_contact` | Manage CRM contacts | CRM |
| `list_buyer_signals` | Read buyer signals | CRM |
| `list_documents`, `read_document`, `search_documents` | Read documents | Docs |
| `create_document`, `update_document` | Write documents | Docs |
| `get_associations`, `create_association` | Link related items | Cross-module |

### Grouped in the UI as checkboxes:

```
What can this automation access?

  Code & Git
  ☑ Read project files
  ☑ Search code
  ☐ Edit project files
  ☐ Run terminal commands
  ☐ Git operations (branch, commit, PR)

  Projects
  ☐ Comment on stories
  ☐ Update story status

  Support
  ☑ Read support conversations
  ☑ Draft support replies

  CRM
  ☐ Manage CRM deals
  ☐ Read buyer signals

  Docs
  ☐ Read documents

  Other
  ☐ Search the web
  ☐ Link related items
```

Templates pre-check the right groups. Users can add/remove.

---

## Schedule UI

When user picks "Schedule" trigger:

```
Run every:
┌─────────┐   ┌──────────────────┐
│ 30 ▾    │   │ minutes ▾        │
└─────────┘   └──────────────────┘

  Or pick a preset:
  ○ Every 15 minutes    ○ Every hour    ○ Every 4 hours
  ○ Daily at [9:00 AM]  ○ Weekly on [Monday]

  Matching targets:
  ┌─────────────────────────────────────────────────────┐
  │ Module: [Support ▾]                                  │
  │ Filter: [Status is Open ▾] [Priority is Urgent ▾]   │
  │ Max per run: [10 ▾]                                  │
  │                                                      │
  │ Currently matches: 23 open conversations             │
  └─────────────────────────────────────────────────────┘
```

The "Currently matches: 23 open conversations" preview is critical — it shows users exactly what the agent will operate on before they activate it.

---

## Event Trigger UI

When user picks "On event":

```
Run when:
┌───────────────────────────────────────────┐
│ Select an event ▾                         │
│                                           │
│ Projects                                  │
│   Story is created                        │
│   Story is assigned                       │
│   Story status changes                    │
│   Story is blocked                        │
│   Sprint is completed                     │
│                                           │
│ Support                                   │
│   New conversation received               │
│   Conversation assigned                   │
│   Conversation reopened                   │
│                                           │
│ CRM                                       │
│   New deal created                        │
│   Deal stage changes                      │
│   Deal has no activity (7+ days)          │
│                                           │
│ Docs                                      │
│   Document updated                        │
│   Document not reviewed (30+ days)        │
└───────────────────────────────────────────┘
```

---

## Approval Inbox

If multiple automations require approval, users need one place to see pending items. Add to the header notification bell or as a badge on the Automations sidebar item:

```
Automations (3)   ← badge shows pending approvals
```

The Automations page shows a "Pending Approval" filter that surfaces all runs waiting for human review, across all automations.

---

## Permission Model

| Action | Required Permission |
|--------|-------------------|
| View automations | `pm.read` (any workspace member) |
| Create/edit automation | `pm.edit` + team membership (or admin) |
| Activate/pause automation | `pm.edit` + team membership (or admin) |
| Delete automation | `pm.edit` + team membership (or admin) |
| Approve/reject runs | Team member or admin |
| View run history | `pm.read` |

Admins see all automations. Team members see their team's automations.

---

## Migration from Current UI

### Phase 1 (backend flexibility sprint)
- No UI changes yet
- Backend adds per-agent tools, schedule, team ownership

### Phase 2 (automation UI sprint)
1. Add `/w/{slug}/automations` route and page
2. Move agents data to this page with new card layout
3. Keep PM > Agents link but redirect to `/w/{slug}/automations?type=agent`
4. Keep contextual entry points (story delivery, epic planning, support assignment)
5. Add sidebar "Automations" link under workspace nav

### Phase 3 (full migration)
1. Remove PM > Agents sidebar link (replaced by Automations)
2. Surface PM automation rules on Automations page as "Rule" type cards
3. Surface CRM autonomy settings on Automations page as read-only cards (link to CRM settings for config)
4. Add template gallery and empty state
5. Add approval inbox
