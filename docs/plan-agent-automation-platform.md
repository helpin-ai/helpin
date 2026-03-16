# Agent & Automation Platform — Architecture Plan

## Current State Assessment

### Three Separate Automation Systems

The codebase has **three independent automation systems** that don't talk to each other:

| System | Location | Targets | Triggers | Tools |
|--------|----------|---------|----------|-------|
| **PM Agents** | `worker/`, `model/agent.go` | story, epic, support_conversation | manual, auto_on_assignment (stories only) | 20 tools in registry |
| **CRM Autonomy** | `service/crm_signal_detection.go`, `crm_deal_automation.go` | deals, contacts, signals | Temporal cron (hourly deal eval, 5min email poll) | LLM calls, no tool registry |
| **PM Automations** | `service/pm_automation.go`, `model/pm_automation.go` | epics, sprints | Event-driven (state change) | Hard-coded actions (auto-start, auto-complete, move items) |

### Agent Classes Are Hard-Coded Silos

Five classes defined in `worker/runtime_profiles.go`, each locked to specific targets and tools:

| Class | Targets | Tools | Approval | Repo Required |
|-------|---------|-------|----------|---------------|
| `engineer` | story only | 15 (filesystem, git, commands, story mgmt) | No | Yes |
| `product_planner` | epic only | 11 (filesystem, search, web_search) | Yes | No |
| `reviewer` | story only | 11 (filesystem, search, story mgmt) | No | Yes |
| `support` | support_conversation only | 3 (list messages, draft reply, update status) | Yes | No |
| `human` | none | 0 | No | No |

**Cross-functional agents are impossible by design:**
- `agent_class` is a single-select enum
- Each domain UI hard-codes which classes show (Support.tsx filters `agent_class === 'support'`, EpicDetail filters `product_planner`)
- No agent can operate on both a story AND a deal
- No agent can read support conversations AND update CRM contacts
- CRM has zero agent integration — it runs its own LLM calls

### What Works Well (Keep These)

- **Temporal orchestration** — reliable execution, heartbeats, cancellation, queues
- **Tool registry** — pluggable, well-structured (`tools.go`)
- **UUID-polymorphic targets** — `target_type` + `target_id` requires no schema changes for new types
- **Approval states** — `not_required | pending | approved | rejected`
- **Token budget enforcement** — monthly cap per agent
- **Artifact system** — 10 artifact types, good for audit trail
- **Association system** — cross-object linking already exists
- **ExecutionContext** — composable, easy to extend with new fields

### What's Broken

1. **No cron/schedule triggers** — manual works, auto_on_assignment works for stories (`agent.go#L404`), but auto_on_event and scheduled runs are not implemented
2. **No CRM tools** — agents can't update deals, create contacts, or read signals
3. **No docs tools** — docs are editable and linkable but NOT a runnable target type in the runtime (only story/epic/support_conversation are)
4. **No cross-module awareness** — an engineer agent can't see related support tickets or CRM deals for context
5. **Three automation UIs** — agents in PM settings, CRM autonomy in CRM settings, PM automations in workspace settings
6. **Class-locked tool sets** — tools are per-class, not per-agent. Can't give an engineer web_search or a support agent story management
7. **No event system** — auto_on_event is defined but there's no event bus to trigger it
8. **No pipeline/chaining** — agents run in isolation, can't trigger other agents on completion

---

## Design Principles

1. **Agents are the execution identity** — keep them. Don't replace with "automations."
2. **Classes become templates, not constraints** — class defines defaults, not hard limits.
3. **Tools are per-agent, not per-class** — each agent gets a custom tool set.
4. **Triggers are first-class** — manual, schedule, event, webhook.
5. **Targets are cross-module** — any agent can target any object type it has tools for.
6. **Teams own their agents** — agents belong to teams, not just workspaces.
7. **Safety scales with autonomy** — more autonomous = more guardrails.
8. **One UI surface** — single "Automations" page, module-specific entry points.

---

## Architecture

### Phase 1: Unlock the Foundation (Current sprint)

**Goal:** Make existing agents flexible enough for cross-functional use without new tables.

#### 1A. Agent model changes

```go
// model/agent.go — extend existing model

type Agent struct {
    // ... existing fields ...

    // NEW: Replace class-locked tools with per-agent config
    AllowedTools    []string  `json:"allowed_tools" gorm:"type:jsonb;default:'[]'"`
    AllowedCommands []string  `json:"allowed_commands" gorm:"type:jsonb;default:'[]'"`

    // NEW: Multiple target types per agent
    AllowedTargets  []string  `json:"allowed_targets" gorm:"type:jsonb;default:'[]'"`
    // e.g. ["story", "support_conversation", "crm_deal"]

    // NEW: Team ownership (nullable for workspace-level agents)
    TeamID          *string   `json:"team_id" gorm:"type:uuid;index"`

    // NEW: Schedule (Temporal cron expression)
    Schedule        *string   `json:"schedule" gorm:"type:text"`
    // e.g. "0 */2 * * *" (every 2 hours)

    // NEW: Target selector for scheduled/event runs
    TargetSelector  *JSONMap  `json:"target_selector" gorm:"type:jsonb"`
    // e.g. {"module": "support", "status": "open", "priority": "urgent", "max_items": 10}

    // NEW: Event-driven trigger configuration
    TriggerEvents   []string  `json:"trigger_events" gorm:"type:jsonb;default:'[]'"`
    // e.g. ["story.assigned", "conversation.created"]

    // NEW: Approval mode (replaces class-level boolean)
    ApprovalMode    string    `json:"approval_mode" gorm:"not null;default:'class_default'"`
    // Phase 1: "never", "always", "class_default"
    // Phase 2: adds "destructive_only" (requires tool-level tagging)

    // NEW: Concurrency control
    MaxConcurrentRuns int     `json:"max_concurrent_runs" gorm:"not null;default:1"`
}
```

**Migration:** `042_agent_flexibility.sql`
- Add columns with defaults (empty arrays, nulls)
- Backfill `allowed_tools` and `allowed_targets` from existing class profiles
- `agent_class` remains for backward compatibility and defaults

#### 1B. ResolveAgentProfile — decouple runtime from class

The runtime is class-coupled in **4 specific places** that all need to route through a single resolution layer:

| Coupling Point | File | What it does today |
|----------------|------|-------------------|
| `normalizeAgentRecord()` | `service/agent_policy.go:147,156` | Overwrites `CapabilityProfile` from `AgentClass` on every call — ignores any per-agent override |
| Target validation | `service/agent_policy.go:214-224` | Validates `target_type` against class profile's `AllowedTargetTypes` via `GetRuntimeProfile()` |
| Tool resolution | `temporalapp/activities.go:371,2185` | Derives profile from `GetRuntimeProfile(agent.CapabilityProfile)`, then builds tool set from `profile.AllowedTools` |
| Queue selection | `temporalapp/queues.go:31-44` | `QueueForProfile()` maps profile string to queue via switch on class name |

**Without fixing all 4, per-agent tools are a database illusion.**

```go
// worker/resolve.go — NEW file, single resolution boundary

type ResolvedProfile struct {
    OriginalClass   string   // The agent's agent_class, retained for class_default fallback
    Tools           []string
    Commands        []string
    TargetTypes     []string
    ApprovalMode    string   // "never", "always", "destructive_only", "class_default"
    RequiresRepo    bool
    Queue           string
}

func ResolveAgentProfile(agent *model.Agent) ResolvedProfile {
    // Start with class defaults
    classProfile := GetDefaultProfile(agent.AgentClass)

    resolved := ResolvedProfile{
        Tools:        classProfile.AllowedTools,
        Commands:     classProfile.AllowedCommands,
        TargetTypes:  classProfile.AllowedTargetTypes,
        ApprovalMode: "class_default",
        RequiresRepo: classProfile.RequiresRepo,
        Queue:        QueueForClass(agent.AgentClass),
    }

    // Per-agent overrides (non-empty = explicit override)
    if len(agent.AllowedTools) > 0 {
        resolved.Tools = agent.AllowedTools
    }
    if len(agent.AllowedCommands) > 0 {
        resolved.Commands = agent.AllowedCommands
    }
    if len(agent.AllowedTargets) > 0 {
        resolved.TargetTypes = agent.AllowedTargets
    }
    if agent.ApprovalMode != "" && agent.ApprovalMode != "class_default" {
        resolved.ApprovalMode = agent.ApprovalMode
    }

    // Queue: if agent has cross-module tools, use general queue
    if hasCrossModuleTools(resolved.Tools) {
        resolved.Queue = "automation-default"
    }

    // Repo: required if agent has any filesystem/git tools
    resolved.RequiresRepo = hasRepoTools(resolved.Tools)

    return resolved
}
```

**Changes to the 4 coupling points:**

1. **`agent_policy.go` `normalizeAgentRecord()`** — stop overwriting `CapabilityProfile`. Class sets defaults only on create, not on every load.

2. **`agent_policy.go` target validation** — replace `classProfile.AllowedTargetTypes` check with `ResolveAgentProfile(agent).TargetTypes`.

3. **`temporalapp/activities.go:371`** — replace `GetRuntimeProfile(agent.CapabilityProfile)` with `ResolveAgentProfile(agent)`. Update `workerAllowedToolSet()` at L2185 to use resolved tools.

4. **`temporalapp/queues.go:31-44`** — `QueueForProfile()` must accept a `ResolvedProfile` or the agent itself, not just the class string.

#### 1C. Add CRM and Docs tools to registry

```go
// worker/tools.go — register new tools

// CRM tools
"list_deals"              // List deals with filters
"update_deal_stage"       // Move deal to new stage
"add_deal_note"           // Add note to deal
"list_contacts"           // List contacts with filters
"create_contact"          // Create new contact
"list_buyer_signals"      // Read CRM signals for context

// Docs tools
"list_documents"          // List docs in collection
"read_document"           // Read document content
"create_document"         // Create new document
"update_document"         // Update document content
"search_documents"        // Full-text search across docs

// Cross-module context tools
"get_associations"        // Get linked objects (story↔ticket↔deal↔doc)
"create_association"      // Link two objects
```

#### 1D. Target hydration and run paths for non-PM targets

The current execution has three hard-coded entry points:
- `service/agent.go:467` — story execution
- `service/agent.go:515` — support conversation execution
- `service/agent_planning.go:60` — epic planning

The worker hydrates state only for these three in `temporalapp/activities.go:351+`, and `ServiceBridge` in `worker/context.go:69` only exposes story/support actions.

**Without new run paths, CRM/docs tools are dead code.**

Phase 1 approach: add specific handlers alongside the existing ones (don't abstract to generic yet — prove the pattern first with 2 new targets, then generalize in Phase 2).

CRM deals and docs have no `assigned_agent_id` field today, and adding one would be premature — these targets are primarily reached via manual or scheduled runs where the agent is already known. So unlike stories (which have `assigned_agent_id` and use auto_on_assignment), CRM/docs run paths accept `agentID` directly as a parameter.

```go
// service/agent.go — add alongside RunAgent() and RunConversationAgent()

func (s *AgentService) RunDealAgent(ctx context.Context, workspaceID, agentID, dealID, actorID string) (*model.AgentRun, error) {
    // 1. Load agent by agentID (not looked up from deal — deals have no assigned_agent_id)
    // 2. Validate target via ResolveAgentProfile(agent).TargetTypes includes "crm_deal"
    // 3. Create AgentRun{TargetType: "crm_deal", TargetID: dealID}
    // 4. Signal Temporal workflow
}

func (s *AgentService) RunDocumentAgent(ctx context.Context, workspaceID, agentID, docID, actorID string) (*model.AgentRun, error) {
    // Same pattern — agent passed explicitly, not looked up from document
}
```

If later we want auto_on_assignment for deals/docs, we add `assigned_agent_id` to those models then. Not needed for Phase 1.

```go
// temporalapp/activities.go — extend loadRunState() at L351+

case "crm_deal":
    deal, err := a.crmRepo.GetDeal(ctx, run.TargetID)
    if err != nil { return nil, err }
    state.deal = deal

case "document":
    doc, err := a.docsRepo.GetDocument(ctx, run.TargetID)
    if err != nil { return nil, err }
    state.document = doc
```

```go
// worker/context.go — extend ServiceBridge

type ServiceBridge struct {
    // ... existing story/support methods ...

    // NEW: CRM actions
    GetDeal          func(ctx context.Context, id string) (*model.CRMDeal, error)
    UpdateDealStage  func(ctx context.Context, id, stage string) error
    AddDealNote      func(ctx context.Context, dealID, note string) error
    ListContacts     func(ctx context.Context, wsID string, filters map[string]string) ([]model.CRMContact, error)

    // NEW: Docs actions
    GetDocument      func(ctx context.Context, id string) (*model.Document, error)
    UpdateDocument   func(ctx context.Context, id string, content string) error
    SearchDocuments  func(ctx context.Context, wsID, query string) ([]model.Document, error)
}
```

**Phase 2 generalization:** Once we have 5 target types (story, epic, conversation, deal, document), extract a `TargetLoader` interface:
```go
type TargetLoader interface {
    Load(ctx context.Context, targetType, targetID string) (TargetContext, error)
}
```

#### 1E. Approval mode plumbing

Current approval logic in `agent.go:720` derives from a boolean on the runtime profile plus a support special-case. Phase 1 replaces this with `approval_mode` resolved through `ResolveAgentProfile()`.

```go
// In run creation (agent.go), replace:
//   if profile.ApprovalRequired { run.ApprovalState = "pending" }
// With:

func resolveApprovalState(resolved ResolvedProfile) string {
    switch resolved.ApprovalMode {
    case "never":
        return "not_required"
    case "always":
        return "pending"
    case "class_default":
        // Fall back to current class behavior
        classProfile := GetDefaultProfile(resolved.OriginalClass)
        if classProfile.ApprovalRequired {
            return "pending"
        }
        return "not_required"
    default:
        return "not_required"
    }
}
```

**`destructive_only` deferred to Phase 2.** It requires:
1. Tool-level metadata: tag each tool as `readOnly` or `mutating`
2. Mid-run pause logic: execution loop checks approval before each mutating tool call
3. UI for inline approval (approve specific action, not entire run)

This is meaningful work — don't half-implement it in Phase 1.

#### 1F. Schedule trigger via Temporal

```go
// worker/workflows.go

func ScheduledAgentWorkflow(ctx workflow.Context, agentID string) error {
    // 1. Load agent + target selector
    // 2. Query matching targets (e.g. open support conversations)
    // 3. For each target, create AgentRun
    // 4. Execute runs (respecting concurrency limit)
}

// Registration in main.go:
// When agent.Schedule is set, register Temporal cron workflow
```

#### 1G. Fix existing bugs and inconsistencies

- `support_inbox.go:280` — change `support_ticket` validation to `support_conversation`
- `runtime_profiles.go` — `product_planner` approval now driven by `approval_mode`, not hard-coded boolean
- `service/agent_policy.go:156` — `normalizeAgentRecord()` must stop overwriting `CapabilityProfile` from class on every call
- Sync docs with actual runtime kinds (only `opencode` is implemented; `native_claude`, `claude_code`, `openclaw`, `zeroclaw` are in frontend types but unused)
- Generalize auto_on_assignment beyond stories — currently only story assignment triggers runs (`agent.go#L404`); support conversations and future CRM targets need the same path

---

### Phase 2: Unified Automation Surface (Next sprint)

**Goal:** One place to see and manage all automations across modules.

#### 2A. New "Automations" page

Route: `/w/{slug}/automations`

Replaces fragmented surfaces:
- PM Agents page → moves here
- PM Automations (sprint/epic auto rules) → moves here as "Rule" type
- CRM Autonomy settings → moves here as "CRM Intelligence" section

**Layout:**
```
┌─────────────────────────────────────────────────────┐
│ Automations                              [+ Create] │
├─────────────────────────────────────────────────────┤
│ Tabs: All | Agents | Rules | Schedules              │
├─────────────────────────────────────────────────────┤
│ ┌─────────────────────────────────────────────────┐ │
│ │ 🤖 Story Engineer          team:Engineering     │ │
│ │    Trigger: auto on assignment                   │ │
│ │    Targets: stories · Last run: 2h ago · ✓ 47   │ │
│ ├─────────────────────────────────────────────────┤ │
│ │ 🤖 Support Triage          team:Support         │ │
│ │    Trigger: every 30min                          │ │
│ │    Targets: open conversations · ✓ 203           │ │
│ ├─────────────────────────────────────────────────┤ │
│ │ 🤖 Deal Follow-up          team:Sales           │ │
│ │    Trigger: daily 9am                            │ │
│ │    Targets: stale deals (7d) · ✓ 12             │ │
│ ├─────────────────────────────────────────────────┤ │
│ │ ⚡ Sprint Auto-Create       team:Engineering     │ │
│ │    Trigger: on sprint complete                   │ │
│ │    Rule: create next sprint, move unfinished     │ │
│ ├─────────────────────────────────────────────────┤ │
│ │ 🧠 CRM Signal Detection    workspace-wide       │ │
│ │    Trigger: every 5min (email poll)              │ │
│ │    Auto-execute: confidence ≥ 0.9               │ │
│ └─────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────┘
```

#### 2B. Create Agent flow redesign

Replace class dropdown with goal-oriented flow:

```
Step 1: What should this agent do?
  ┌──────────────┐ ┌──────────────┐ ┌──────────────┐
  │  Implement   │ │   Review     │ │   Triage     │
  │  stories     │ │   code       │ │   support    │
  └──────────────┘ └──────────────┘ └──────────────┘
  ┌──────────────┐ ┌──────────────┐ ┌──────────────┐
  │  Plan epics  │ │  Follow up   │ │   Custom     │
  │              │ │  on deals    │ │              │
  └──────────────┘ └──────────────┘ └──────────────┘

Step 2: When should it run?
  ○ Manually          (I'll trigger it)
  ○ On assignment     (when work is assigned)
  ○ On event          (when state changes)
  ○ On schedule       (recurring: every __, at __)

Step 3: What can it access?
  [Pre-filled based on Step 1, editable]
  ☑ Read files        ☑ Search code       ☐ Write files
  ☑ Git operations    ☐ Web search        ☑ Story management
  ☐ Support tools     ☐ CRM tools         ☐ Docs tools

Step 4: Safety
  Approval required:  ○ Never  ○ Always  ○ On destructive actions
  Monthly token cap:  [______] tokens
  Concurrent runs:    [__3__] max
```

#### 2C. Event bus for auto triggers

```go
// internal/events/bus.go

type Event struct {
    Module      string    // "pm", "crm", "support", "docs"
    Type        string    // "story.state_changed", "deal.stage_changed", "conversation.created"
    ObjectType  string    // "story", "deal", "support_conversation"
    ObjectID    string
    WorkspaceID string
    TeamID      *string
    Payload     map[string]any
    Timestamp   time.Time
}

type EventBus interface {
    Publish(ctx context.Context, event Event) error
    Subscribe(pattern string, handler EventHandler) error
}
```

Events emitted from existing service methods:
- `pm_story.go` → `story.state_changed`, `story.assigned`, `story.created`
- `crm_deal.go` → `deal.stage_changed`, `deal.created`, `deal.stale`
- `support_inbox.go` → `conversation.created`, `conversation.assigned`
- `docs.go` → `document.created`, `document.updated`

Agent trigger matcher:
```go
// When agent.TriggerMode == "auto_on_event"
// Match agent.TriggerEvents against incoming events
// e.g. agent.TriggerEvents = ["story.state_changed:in_progress", "story.assigned"]
```

---

### Phase 3: Cross-Functional Intelligence (Future)

**Goal:** Agents that understand the full context across modules.

#### 3A. Cross-module context injection

When an agent runs on a story, automatically inject:
- Related support conversations (via associations)
- Related CRM deal context (if customer-facing)
- Related docs (specs, runbooks)

```go
// worker/context.go — extend BuildCrossModuleContext

func (b *ContextBuilder) BuildCrossModuleContext(ctx context.Context, target TargetRef) string {
    var sections []string

    // Get associations
    assocs := b.assocRepo.GetAssociations(ctx, target.Type, target.ID)

    for _, a := range assocs {
        switch a.LinkedType {
        case "support_conversation":
            conv := b.supportRepo.Get(ctx, a.LinkedID)
            sections = append(sections, formatConversationContext(conv))
        case "crm_deal":
            deal := b.crmRepo.GetDeal(ctx, a.LinkedID)
            sections = append(sections, formatDealContext(deal))
        case "document":
            doc := b.docsRepo.Get(ctx, a.LinkedID)
            sections = append(sections, formatDocContext(doc))
        }
    }

    return strings.Join(sections, "\n\n")
}
```

#### 3B. Agent pipelines

```go
// model/agent.go

type AgentPipeline struct {
    ID          string          `json:"id" gorm:"type:uuid;primaryKey"`
    WorkspaceID string          `json:"workspace_id"`
    Name        string          `json:"name"`
    Steps       []PipelineStep  `json:"steps" gorm:"type:jsonb"`
}

type PipelineStep struct {
    AgentID     string            `json:"agent_id"`
    Condition   string            `json:"condition"`    // "previous.status == 'completed'"
    InputMap    map[string]string  `json:"input_map"`    // Map previous outputs to inputs
}
```

Example pipeline: **Support Escalation to PM**
1. Support agent triages conversation → extracts bug report
2. PM agent creates story from bug details → links to conversation
3. Engineer agent picks up story if auto-assign enabled

#### 3C. Team automation presets

When creating a team, offer starter automation packs:

| Team Type | Preset Automations |
|-----------|-------------------|
| Engineering | Story engineer, code reviewer, sprint auto-create |
| Support | Conversation triage, escalation to PM, SLA monitor |
| Sales | Deal follow-up, signal detection, stale deal alerts |
| Marketing | Content review, campaign tracking |
| Product | Epic planner, story decomposition, roadmap updates |

These create pre-configured agents that teams can customize or disable.

---

## Implementation Priority

### Must-do (Phase 1 — enables everything else)
1. **Per-agent tool configuration** — unlock cross-functional agents
2. **Schedule triggers** — enable cron-based automation
3. **CRM + Docs tools** — expand what agents can do
4. **Team ownership** — agents belong to teams
5. **Fix support target bug** — `support_ticket` → `support_conversation`

### Should-do (Phase 2 — makes it usable)
6. **Unified Automations page** — single surface for all automation
7. **Goal-oriented create flow** — replace class dropdown
8. **Event bus** — enable auto_on_event triggers
9. **Migrate PM automations** — sprint/epic rules become automation entries
10. **Run history & observability** — success rate, time saved, actions taken

### Nice-to-have (Phase 3 — makes it powerful)
11. **Cross-module context injection** — agents see the full picture
12. **Agent pipelines** — chained execution
13. **Team presets** — one-click automation setup
14. **Dry-run mode** — simulate before activating
15. **Webhook triggers** — external systems can invoke agents

---

## Migration Strategy

### Backward Compatibility
- `agent_class` stays — it becomes the default template, not a constraint
- Existing agent configs keep working — new fields default to empty (fall back to class profile)
- PM Automations page stays until Automations page is ready
- CRM autonomy settings stay — they become viewable from Automations page

### Data Migration
```sql
-- 042_agent_flexibility.sql

ALTER TABLE agents ADD COLUMN IF NOT EXISTS allowed_tools jsonb DEFAULT '[]';
ALTER TABLE agents ADD COLUMN IF NOT EXISTS allowed_commands jsonb DEFAULT '[]';
ALTER TABLE agents ADD COLUMN IF NOT EXISTS allowed_targets jsonb DEFAULT '[]';
ALTER TABLE agents ADD COLUMN IF NOT EXISTS team_id uuid REFERENCES workspace_teams(id) ON DELETE SET NULL;
ALTER TABLE agents ADD COLUMN IF NOT EXISTS schedule text;
ALTER TABLE agents ADD COLUMN IF NOT EXISTS target_selector jsonb;
ALTER TABLE agents ADD COLUMN IF NOT EXISTS trigger_events jsonb DEFAULT '[]';
ALTER TABLE agents ADD COLUMN IF NOT EXISTS max_concurrent_runs int DEFAULT 1;
ALTER TABLE agents ADD COLUMN IF NOT EXISTS approval_mode text DEFAULT 'class_default';
-- approval_mode: 'never', 'always', 'destructive_only', 'class_default'

CREATE INDEX IF NOT EXISTS idx_agents_team_id ON agents(team_id);
CREATE INDEX IF NOT EXISTS idx_agents_schedule ON agents(schedule) WHERE schedule IS NOT NULL;

-- Backfill allowed_tools from class profiles
UPDATE agents SET allowed_tools = '["read_file","read_file_range","write_file","list_directory","search_files","ripgrep","grep","list_symbols","run_command","create_branch","commit_and_push","open_pr","add_story_comment","update_story_state","list_story_checklist"]'
WHERE agent_class = 'engineer' AND allowed_tools = '[]';

UPDATE agents SET allowed_tools = '["read_file","read_file_range","list_directory","search_files","ripgrep","grep","list_symbols","run_command","web_search","add_story_comment","list_story_checklist"]'
WHERE agent_class = 'product_planner' AND allowed_tools = '[]';

UPDATE agents SET allowed_tools = '["list_conversation_messages","draft_support_reply","update_conversation_status"]'
WHERE agent_class = 'support' AND allowed_tools = '[]';

-- Backfill allowed_targets
UPDATE agents SET allowed_targets = '["story"]'
WHERE agent_class IN ('engineer', 'reviewer') AND allowed_targets = '[]';

UPDATE agents SET allowed_targets = '["epic"]'
WHERE agent_class = 'product_planner' AND allowed_targets = '[]';

UPDATE agents SET allowed_targets = '["support_conversation"]'
WHERE agent_class = 'support' AND allowed_targets = '[]';
```

---

## What This Enables

### Before (today)
- Sales team: "We can't use agents, they only do engineering stuff"
- Support lead: "I want the triage agent to also create PM stories, but it can't"
- Product manager: "CRM signals are great, but I can't connect them to epics automatically"
- Admin: "I have to check three different places to see what's automated"

### After (Phase 2)
- Sales team creates a "Deal Follow-up" agent that runs daily, emails stale deals, logs notes
- Support lead's triage agent reads conversations AND creates PM stories AND links them
- Product manager sets up a pipeline: CRM signal → auto-create story → assign to engineer
- Admin sees all automations in one page with run history and success rates
- Each team owns and configures their own agents from team settings

---

## Risk Assessment

| Risk | Likelihood | Mitigation |
|------|-----------|------------|
| Per-agent tools bypass safety (e.g. giving support agent write_file) | Medium | Tool categories with warnings: "This tool can modify code — requires repo access" |
| Schedule triggers create runaway costs | Medium | Monthly token budget (already exists) + max_concurrent_runs + daily run cap |
| Cross-module tools leak data between teams | Low | Tools respect existing RBAC — agent runs as workspace member, not god mode |
| Migration breaks existing agents | Low | All new columns are nullable/defaulted, class profiles remain as fallback |
| Event bus creates feedback loops (agent A triggers agent B triggers agent A) | Medium | Max pipeline depth (5), cycle detection, per-agent cooldown period |
