# Agent pipeline and planning improvement proposal

This undated historical proposal records six possible improvements to agent planning and review. Use it to understand the original ideas, then compare each item with the current task model and Agent Runtime contract before implementing follow-up work.

## Source review — 2026-09-18

- The old `PMStory`, `worker/prompt.go` and `worker/planning_prompt_pack.go` design no longer describes the runtime boundary. Current [agent models](../../server/internal/model/agent.go) use `AgentRunInputPayload` with target, event, workspace, attached-context and task fields. The proposed `StoryRunContext` model and standalone `run_context` field were not found; do not treat the sample persistence code as current or replayable context guarantees.
- Vertical slicing is partially represented in current source: [ProposedTask and orchestration models](../../server/internal/model/agent_planning.go) contain `slice_type` and `vertical_coverage`; [planning service](../../server/internal/service/agent_planning.go) handles task briefs and warns about enabler ratios. Tasks replace stories. These fields do not mechanically prove a slice delivers an end-to-end behavior.
- The proposed four-action approval request and approval audit fields were not found. `ApproveAgentRunRequest` currently accepts `content` and `send_message`; request-changes is a separate resume intent handled through current run interactions in the [agent service](../../server/internal/service/agent.go). Copying the proposed switch would bypass that newer lifecycle.
- Repository searches did not find `PipelineStatusStrip`, `RunContextView`, or `planning_output_format`. Those UI/format concepts remain proposals, not options users can assume are available.
- `merge_branch` remains an action in the [automation rule engine](../../server/internal/service/automation_rule_engine.go), which calls the Git service after resolving the configured target. The suggested auto-created done-state rule and alternative `merge_on_done` migration are not established by this proposal.
- The “no SQL migration needed” and “all independent” statements below are historical assumptions. Deployed schema changes must follow current versioned migration ownership, and any run/review change must integrate with current Runtime authorization and interaction contracts. No run, merge, model call or test suite was executed for this review.

## Original proposal

Six changes to the agent pipeline and planning system, ordered by priority. Each is independent and can be implemented separately.

---

## 1. Structured RunContext on Agent Runs

**Priority: High | Effort: Medium**

### Problem

When `RunAgent` fires for a story, the input is `{"story_id": "..."}`. The Temporal worker in `ExecuteRunActivity` assembles context ad-hoc at execution time — loading the story, delivery target, epic, checklist, linked docs. There's no record of what context the agent actually received. You can't audit it, reproduce it, or let a human review/edit it before the agent starts.

The current context assembly lives across two places:
- `worker/prompt.go:BuildUserPrompt()` — story name, description, epic context, checklist, additional instructions
- `service/agent_planning.go:buildStoryExecutionBrief()` — approved spec snapshot (up to 12K chars), acceptance criteria, dependencies, source refs, resolved clarifications

### Solution

Add a `run_context` JSONB field to `AgentRun`. Build and persist it when the run is created (in `AgentService.RunAgent`), not at execution time. The executor reads from this field instead of assembling context from scratch.

### Data Model

Add to `AgentRun` in `server/internal/model/agent.go`:

```go
RunContext json.RawMessage `json:"run_context,omitempty" gorm:"type:jsonb;not null;default:'{}'"`
```

Define the structured context:

```go
// In server/internal/model/agent_run_context.go

type StoryRunContext struct {
    // Story
    StoryID            string   `json:"story_id"`
    StoryName          string   `json:"story_name"`
    StoryDescription   string   `json:"story_description"`
    StoryType          string   `json:"story_type"`
    AcceptanceCriteria []string `json:"acceptance_criteria,omitempty"`

    // Epic origin (if story was planned from an epic)
    EpicID             *string  `json:"epic_id,omitempty"`
    EpicName           *string  `json:"epic_name,omitempty"`

    // Spec snapshot (from approved spec, if available)
    SpecSnapshot       *string  `json:"spec_snapshot,omitempty"`
    SpecVersionID      *string  `json:"spec_version_id,omitempty"`

    // Dependencies
    BlockedByStories   []StoryRef `json:"blocked_by_stories,omitempty"`
    BlocksStories      []StoryRef `json:"blocks_stories,omitempty"`

    // Delivery
    RepositoryFullName *string  `json:"repository_full_name,omitempty"`
    BaseBranch         *string  `json:"base_branch,omitempty"`
    WorkingBranch      *string  `json:"working_branch,omitempty"`

    // Additional
    Checklist          []ChecklistItem `json:"checklist,omitempty"`
    AdditionalContext  *string         `json:"additional_context,omitempty"`
    SourceRefs         []PlanningSourceRef `json:"source_refs,omitempty"`
}

type StoryRef struct {
    StoryID   string `json:"story_id"`
    StoryName string `json:"story_name"`
    DisplayID int    `json:"display_id"`
}

type ChecklistItem struct {
    Text      string `json:"text"`
    Completed bool   `json:"completed"`
}
```

### Backend Changes

**`server/internal/service/agent.go` — `RunAgent()`:**

After creating the run record and before starting the Temporal workflow, build and persist the context:

```go
func (s *AgentService) buildStoryRunContext(ctx context.Context, story *model.PMStory) (*model.StoryRunContext, error) {
    rc := &model.StoryRunContext{
        StoryID:          story.ID,
        StoryName:        story.Name,
        StoryDescription: deref(story.Description),
        StoryType:        story.StoryType,
    }

    // Acceptance criteria from description (parsed) or from planning origin
    rc.AcceptanceCriteria = extractAcceptanceCriteria(story)

    // Epic context
    if story.EpicID != nil {
        epic, _ := s.epicRepo.GetByID(ctx, *story.EpicID)
        if epic != nil {
            rc.EpicID = &epic.ID
            rc.EpicName = &epic.Name
            // Load approved spec snapshot
            if epic.ApprovedSpecVersionID != nil {
                version, _ := s.docsVersionRepo.GetByID(ctx, *epic.ApprovedSpecVersionID)
                if version != nil {
                    text := truncate(version.ContentText, 12000)
                    rc.SpecSnapshot = &text
                    rc.SpecVersionID = &version.ID
                }
            }
        }
    }

    // Dependencies
    links, _ := s.storyLinkRepo.GetByStoryID(ctx, story.ID)
    for _, link := range links {
        ref := model.StoryRef{StoryID: link.TargetStoryID, StoryName: link.TargetStoryName, DisplayID: link.TargetDisplayID}
        if link.LinkType == "blocked_by" {
            rc.BlockedByStories = append(rc.BlockedByStories, ref)
        } else if link.LinkType == "blocks" {
            rc.BlocksStories = append(rc.BlocksStories, ref)
        }
    }

    // Delivery target
    dt, _ := s.deliveryTargetRepo.GetByStoryID(ctx, story.ID)
    if dt != nil {
        rc.RepositoryFullName = dt.RepoFullName
        rc.BaseBranch = dt.BaseBranch
        rc.WorkingBranch = dt.WorkingBranch
    }

    // Checklist
    items, _ := s.checklistRepo.ListByStory(ctx, story.ID)
    for _, item := range items {
        rc.Checklist = append(rc.Checklist, model.ChecklistItem{Text: item.Text, Completed: item.Completed})
    }

    return rc, nil
}
```

Persist on run creation:

```go
runContext, _ := s.buildStoryRunContext(ctx, story)
contextJSON, _ := json.Marshal(runContext)
run.RunContext = contextJSON
```

**`server/internal/worker/prompt.go` — `BuildUserPrompt()`:**

Add an alternative path that builds the prompt from `RunContext` when available:

```go
func BuildUserPromptFromContext(rc *model.StoryRunContext) string {
    // Build structured prompt from the persisted context
    // Same content as current BuildUserPrompt, but reads from rc instead of loading from DB
}
```

The executor checks: if `run.RunContext` is non-empty, use `BuildUserPromptFromContext`. Otherwise fall back to existing `BuildUserPrompt` for backward compatibility.

### Frontend Changes

**`frontend/src/lib/pmTypes.ts`:**

Add `run_context` to `AgentRun` interface:

```typescript
interface AgentRun {
    // ... existing fields ...
    run_context?: StoryRunContext;
}

interface StoryRunContext {
    story_id: string;
    story_name: string;
    story_description: string;
    story_type: string;
    acceptance_criteria?: string[];
    epic_id?: string;
    epic_name?: string;
    spec_snapshot?: string;
    spec_version_id?: string;
    blocked_by_stories?: StoryRef[];
    blocks_stories?: StoryRef[];
    repository_full_name?: string;
    base_branch?: string;
    working_branch?: string;
    checklist?: { text: string; completed: boolean }[];
    additional_context?: string;
    source_refs?: { type: string; title?: string; id?: string }[];
}
```

**`frontend/src/components/pm/AgentRunDetail.tsx`:**

Add a collapsible "Context" section below the run metadata:

```tsx
{run.run_context && (
    <Collapsible>
        <CollapsibleTrigger className="flex items-center gap-1.5 text-xs text-muted-foreground">
            <FileText className="h-3 w-3" />
            Agent Context
            <ChevronRight className="h-3 w-3" />
        </CollapsibleTrigger>
        <CollapsibleContent>
            <RunContextView context={run.run_context} />
        </CollapsibleContent>
    </Collapsible>
)}
```

`RunContextView` renders: story info, acceptance criteria list, spec snapshot (truncated with expand), dependencies as links, delivery target info.

### Migration

No SQL migration needed — GORM AutoMigrate adds the column. Default `'{}'` ensures backward compatibility with existing runs.

### Implementation Order

1. Add `RunContext` field to `AgentRun` model
2. Create `StoryRunContext` type in `server/internal/model/agent_run_context.go`
3. Add `buildStoryRunContext()` to agent service
4. Wire context building into `RunAgent()` — persist on run creation
5. Add `BuildUserPromptFromContext()` to prompt builder
6. Update executor to prefer `RunContext` when available
7. Add TypeScript types
8. Add `RunContextView` component to `AgentRunDetail`

---

## 2. Vertical Slice Enforcement in Story Planning

**Priority: Medium | Effort: Small**

### Problem

`plan_stories` produces stories with `acceptance_criteria`, `dependency_refs`, and `source_refs`. But there's no structural signal telling the human reviewer whether a proposed story is a vertical slice (spans DB → API → UI for one behavior) or a horizontal layer (just the DB schema, just the API). The prompt hints at vertical slicing but the model doesn't enforce or surface it.

### Solution

Two additions:
1. A `slice_type` field on `ProposedStory` — `vertical`, `enabler`, or `spike`
2. A `vertical_coverage` summary on `OrchestrationProposal` — maps user-facing behaviors to the stories that deliver them

### Data Model Changes

**`server/internal/model/agent_planning.go`:**

Add to `ProposedStory`:

```go
type ProposedStory struct {
    Ref                string              `json:"ref"`
    Name               string              `json:"name"`
    Description        string              `json:"description"`
    StoryType          string              `json:"story_type"`
    Estimate           *int                `json:"estimate,omitempty"`
    Priority           *string             `json:"priority,omitempty"`
    AcceptanceCriteria []string            `json:"acceptance_criteria,omitempty"`
    DependencyRefs     []string            `json:"dependency_refs,omitempty"`
    SourceRefs         []PlanningSourceRef `json:"source_refs,omitempty"`
    AssignAgentID      *string             `json:"assign_agent_id,omitempty"`
    SliceType          string              `json:"slice_type,omitempty"`  // NEW: "vertical", "enabler", "spike"
}
```

Add to `OrchestrationProposal`:

```go
type OrchestrationProposal struct {
    EpicID            string                `json:"epic_id"`
    Summary           string                `json:"summary"`
    SpecVersionID     string                `json:"spec_version_id,omitempty"`
    ProposedStories   []ProposedStory       `json:"proposed_stories"`
    OpenQuestions     []string              `json:"open_questions,omitempty"`
    Risks             []string              `json:"risks,omitempty"`
    TokensUsed        int                   `json:"tokens_used"`
    VerticalCoverage  []VerticalCoverageEntry `json:"vertical_coverage,omitempty"` // NEW
}

// NEW
type VerticalCoverageEntry struct {
    Behavior   string   `json:"behavior"`    // User-facing behavior description
    StoryRefs  []string `json:"story_refs"`  // Which stories deliver this behavior
    FullSlice  bool     `json:"full_slice"`  // True if covered by a single vertical story
}
```

### Prompt Pack Changes

**`server/internal/worker/planning_prompt_pack.go`:**

Update the `plan_stories` JSON schema instruction to include the new fields:

In the structured_v1 `plan_stories` guidance, add:

```
Each proposed story must include a "slice_type" field:
- "vertical": The story delivers a complete user-visible behavior from database to UI. This is the default and strongly preferred.
- "enabler": The story sets up shared infrastructure needed by multiple vertical stories (e.g., database migration, auth middleware). Use sparingly.
- "spike": A timeboxed investigation to resolve uncertainty before committing to implementation. Use only when a decision cannot be made from the spec alone.

Prefer vertical slices. If you find yourself creating more than 1-2 enablers, reconsider whether those can be folded into the first vertical story that needs them.

After the proposed_stories array, include a "vertical_coverage" array that maps each user-facing behavior in the spec to the story or stories that deliver it:
[
  { "behavior": "User can create a pipeline from email signal", "story_refs": ["story_1"], "full_slice": true },
  { "behavior": "Admin can configure OAuth providers", "story_refs": ["story_2", "story_3"], "full_slice": false }
]

If a behavior requires multiple stories (full_slice=false), explain why in the story descriptions and ensure the stories have explicit dependency_refs.
```

### Frontend Changes

**`frontend/src/lib/pmTypes.ts`:**

Add `slice_type` to `ProposedStory`:

```typescript
interface ProposedStory {
    // ... existing fields ...
    slice_type?: 'vertical' | 'enabler' | 'spike';
}

interface VerticalCoverageEntry {
    behavior: string;
    story_refs: string[];
    full_slice: boolean;
}

interface OrchestrationProposal {
    // ... existing fields ...
    vertical_coverage?: VerticalCoverageEntry[];
}
```

**Story review UI (in `EpicOrchestrationPanel` review step):**

Add a badge next to each proposed story name:

```tsx
<Badge variant={
    story.slice_type === 'vertical' ? 'default' :
    story.slice_type === 'enabler' ? 'secondary' : 'outline'
}>
    {story.slice_type ?? 'vertical'}
</Badge>
```

Add a coverage summary panel below the story list (collapsible):

```tsx
{proposal.vertical_coverage && (
    <div className="space-y-1">
        <Label>Behavior Coverage</Label>
        {proposal.vertical_coverage.map(entry => (
            <div key={entry.behavior} className="flex items-center gap-2 text-xs">
                <span className={entry.full_slice ? 'text-emerald-600' : 'text-amber-600'}>
                    {entry.full_slice ? '●' : '◐'}
                </span>
                <span>{entry.behavior}</span>
                <span className="text-muted-foreground">→ {entry.story_refs.join(', ')}</span>
            </div>
        ))}
    </div>
)}
```

Behaviors with `full_slice=false` are highlighted as warnings — the human should consider whether those stories should be merged.

### Validation

In `validatePlanningStories()` (in `agent_planning.go`), add a soft validation:

```go
// Count slice types
enablerCount := 0
for _, s := range stories {
    if s.SliceType == "enabler" {
        enablerCount++
    }
}
// Warn (not error) if enablers exceed 30% of total stories
if enablerCount > 0 && float64(enablerCount)/float64(len(stories)) > 0.3 {
    slog.Warn("high enabler ratio in story plan",
        "enabler_count", enablerCount,
        "total_count", len(stories),
        "epic_id", epicID)
}
```

### Persisting slice_type on created stories

When stories are created from a confirmed proposal (`ConfirmEpicRun`), persist `slice_type` in the story metadata. Add to `PMStory`:

```go
SliceType *string `json:"slice_type,omitempty" gorm:"type:text"`
```

This allows the kanban board or story list to show slice type badges, making it visible during execution too.

### Implementation Order

1. Add `SliceType` to `ProposedStory` and `VerticalCoverage` to `OrchestrationProposal` models
2. Add `SliceType` field to `PMStory` model
3. Update prompt pack with slice_type guidance and vertical_coverage schema
4. Update `ConfirmEpicRun` to persist `slice_type` on created stories
5. Add TypeScript types
6. Add badges to story review UI
7. Add coverage summary panel

---

## 3. Richer Approval Semantics

**Priority: Medium | Effort: Small**

### Problem

Agent run approval is binary: approve or leave pending. The `ApproveAgentRunRequest` only has `send_message` (for support runs). There's no way to:
- Approve with a note ("looks good, I fixed one typo in the PR")
- Request changes ("the auth middleware needs a rate limiter, re-run with this feedback")
- Reject outright ("wrong approach, abandon this")

In practice, engineers approve runs after making manual edits to the PR, but there's no record of that. Reviewers want to say "mostly good but add X" without fully re-running.

### Solution

Expand the approval model with:
- `approval_action` enum: `approve`, `approve_with_changes`, `request_changes`, `reject`
- `approval_note` text field for human feedback
- On `request_changes`: optionally re-run the agent with the note as additional context

### Data Model Changes

**`server/internal/model/agent.go` — `AgentRun`:**

Add fields:

```go
ApprovalAction *string `json:"approval_action,omitempty" gorm:"type:text"`
    // "approve", "approve_with_changes", "request_changes", "reject"
ApprovalNote   *string `json:"approval_note,omitempty" gorm:"type:text"`
ApprovalBy     *string `json:"approval_by,omitempty" gorm:"type:uuid"`
ApprovalAt     *time.Time `json:"approval_at,omitempty"`
```

Update `ApproveAgentRunRequest`:

```go
type ApproveAgentRunRequest struct {
    SendMessage bool    `json:"send_message"`
    Action      string  `json:"action"`       // NEW: "approve" (default), "approve_with_changes", "request_changes", "reject"
    Note        *string `json:"note,omitempty"` // NEW: human feedback
}
```

### Backend Changes

**`server/internal/service/agent.go` — `ApproveRun()`:**

Expand the approval handling:

```go
func (s *AgentService) ApproveRun(ctx context.Context, workspaceID, runID, actorID string, req model.ApproveAgentRunRequest) (*model.AgentRun, error) {
    // ... existing validation ...

    action := req.Action
    if action == "" {
        action = "approve" // backward compat
    }

    run.ApprovalAction = &action
    run.ApprovalNote = req.Note
    run.ApprovalBy = &actorID
    now := time.Now()
    run.ApprovalAt = &now

    switch action {
    case "approve", "approve_with_changes":
        run.ApprovalState = model.ApprovalStateApproved
        if run.Status == model.RunStatusAwaitingApproval {
            run.Status = model.RunStatusCompleted
            run.CompletedAt = &now
            stage := "approved"
            run.ExecutionStage = &stage
        }
        // Signal Temporal, evaluate automation rules (existing logic)
        s.signalApprove(ctx, run)
        if run.StoryID != nil {
            s.ruleEngine.EvaluateEvent(ctx, model.AutomationEvent{
                TriggerType: model.TriggerAgentRunApproved,
                StoryID:     *run.StoryID,
                StateID:     storyStateID,
                AgentID:     run.AgentID,
                RunID:       run.ID,
            }, nil)
        }

    case "request_changes":
        // Don't complete the run — re-run the agent with feedback
        run.ApprovalState = model.ApprovalStateNotRequired // reset for the new run
        run.Status = model.RunStatusCompleted
        run.CompletedAt = &now

        // Create a follow-up run with the feedback as additional context
        if run.StoryID != nil {
            story, _ := s.storyRepo.GetByID(ctx, *run.StoryID)
            if story != nil {
                feedback := "Previous run feedback from reviewer:\n" + deref(req.Note)
                newRun, err := s.createAndStartRun(ctx, workspaceID, story, run.AgentID, actorID, feedback)
                if err != nil {
                    slog.Error("failed to start follow-up run", "error", err, "run_id", runID)
                }
                // Link the runs
                if newRun != nil {
                    newRun.ParentRunID = &run.ID
                    s.runRepo.Update(ctx, newRun)
                }
            }
        }

    case "reject":
        run.ApprovalState = "rejected"
        run.Status = model.RunStatusCompleted
        run.CompletedAt = &now
        // Reset agent to idle
        s.resetAgentStatus(ctx, run.AgentID)
        // Do NOT trigger automation rules — pipeline stops here
    }

    s.runRepo.Update(ctx, run)
    s.publishRunEvent(run, actorID)
    return run, nil
}
```

### Frontend Changes

**`frontend/src/components/pm/AgentRunDetail.tsx`:**

Replace the single "Approve" button with an action menu:

```tsx
{run.approval_state === 'pending' && (
    <div className="flex items-center gap-1">
        <Button size="sm" variant="outline" className="h-7 gap-1 text-[11px]"
            onClick={() => onApprove(run.id, 'approve')}>
            <Check className="h-3 w-3" /> Approve
        </Button>
        <DropdownMenu>
            <DropdownMenuTrigger asChild>
                <Button size="sm" variant="ghost" className="h-7 w-7 p-0">
                    <ChevronDown className="h-3 w-3" />
                </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
                <DropdownMenuItem onClick={() => setApprovalDialog({ runId: run.id, action: 'approve_with_changes' })}>
                    <CheckCheck className="h-3.5 w-3.5 mr-2" />
                    Approve with changes
                </DropdownMenuItem>
                <DropdownMenuItem onClick={() => setApprovalDialog({ runId: run.id, action: 'request_changes' })}>
                    <MessageSquare className="h-3.5 w-3.5 mr-2" />
                    Request changes
                </DropdownMenuItem>
                <DropdownMenuSeparator />
                <DropdownMenuItem className="text-destructive" onClick={() => setApprovalDialog({ runId: run.id, action: 'reject' })}>
                    <X className="h-3.5 w-3.5 mr-2" />
                    Reject
                </DropdownMenuItem>
            </DropdownMenuContent>
        </DropdownMenu>
    </div>
)}
```

When `approve_with_changes`, `request_changes`, or `reject` is selected, show a dialog with a textarea for the note:

```tsx
<Dialog open={!!approvalDialog} onOpenChange={() => setApprovalDialog(null)}>
    <DialogContent>
        <DialogHeader>
            <DialogTitle>
                {approvalDialog?.action === 'approve_with_changes' && 'Approve with Changes'}
                {approvalDialog?.action === 'request_changes' && 'Request Changes'}
                {approvalDialog?.action === 'reject' && 'Reject Run'}
            </DialogTitle>
        </DialogHeader>
        <Textarea
            placeholder={
                approvalDialog?.action === 'request_changes'
                    ? 'Describe what needs to change. The agent will re-run with this feedback...'
                    : 'Add a note (optional)...'
            }
            value={approvalNote}
            onChange={e => setApprovalNote(e.target.value)}
        />
        <DialogFooter>
            <Button variant="outline" onClick={() => setApprovalDialog(null)}>Cancel</Button>
            <Button
                variant={approvalDialog?.action === 'reject' ? 'destructive' : 'default'}
                onClick={() => handleApproval(approvalDialog!.runId, approvalDialog!.action, approvalNote)}
            >
                Confirm
            </Button>
        </DialogFooter>
    </DialogContent>
</Dialog>
```

**Display approval metadata on completed runs:**

```tsx
{run.approval_action && (
    <div className="flex items-center gap-1.5 text-xs">
        <Badge variant={
            run.approval_action === 'approve' ? 'default' :
            run.approval_action === 'approve_with_changes' ? 'secondary' :
            run.approval_action === 'request_changes' ? 'outline' : 'destructive'
        }>
            {run.approval_action.replace(/_/g, ' ')}
        </Badge>
        {run.approval_note && (
            <span className="text-muted-foreground truncate max-w-[200px]">{run.approval_note}</span>
        )}
    </div>
)}
```

### Automation Rule Implications

- `approve` and `approve_with_changes` both trigger `agent_run.approved` rules → pipeline advances
- `request_changes` does NOT trigger rules → agent re-runs, then produces a new run that goes through approval again
- `reject` does NOT trigger rules → pipeline stops

This means the existing rule engine works unchanged. The distinction between `approve` and `approve_with_changes` is purely for audit — both advance the pipeline.

### Implementation Order

1. Add `ApprovalAction`, `ApprovalNote`, `ApprovalBy`, `ApprovalAt` fields to `AgentRun` model
2. Update `ApproveAgentRunRequest` with `Action` and `Note` fields
3. Expand `ApproveRun` service method with action-based branching
4. Add follow-up run creation logic for `request_changes`
5. Update TypeScript types
6. Replace single approve button with action menu + dialog
7. Add approval metadata display on completed runs

---

## 4. Pipeline Status Strip on Kanban Board

**Priority: Medium | Effort: Medium**

### Problem

Pipeline configuration lives in Settings > Teams > Workflow. During day-to-day operation, users see bot icons on kanban columns but can't see the overall pipeline flow, which agents are assigned where, or what's currently running. They have to navigate to settings to understand their automation setup.

### Solution

A read-only horizontal pipeline strip at the top of the kanban board. Think of it as a compact, non-editable PipelineBuilder embedded in the board view. It shows: states left-to-right, which have agents assigned, auto-advance arrows, and real-time indicators of active agent runs.

### Frontend Implementation

**New component: `frontend/src/components/pm/PipelineStatusStrip.tsx`**

```tsx
function PipelineStatusStrip({
    states,
    automationRules,
    agents,
    activeRuns,
}: {
    states: WorkflowState[];
    automationRules: AutomationRule[];
    agents: Agent[];
    activeRuns: AgentRun[];  // Currently running/queued runs
}) {
    // Build the same stateRuleMap as PipelineBuilder (reuse logic)
    const stateRuleMap = useMemo(() => {
        // ... same mapping logic as PipelineBuilder.tsx lines 33-51 ...
    }, [states, automationRules]);

    // Map active runs to states
    const activeRunsByState = useMemo(() => {
        const map = new Map<string, AgentRun[]>();
        for (const run of activeRuns) {
            // Match run to state via the story's current workflow_state_id
            // This requires activeRuns to carry story state info
        }
        return map;
    }, [activeRuns]);

    return (
        <div className="flex items-center gap-0 overflow-x-auto px-4 py-2 border-b bg-muted/30">
            {states.map((state, idx) => {
                const entry = stateRuleMap.get(state.id);
                const agentId = entry?.agentRule?.action_config?.agent_id;
                const agent = agentId ? agents.find(a => a.id === agentId) : null;
                const hasAdvance = !!entry?.advanceRule;
                const isLast = idx === states.length - 1;
                const stateRuns = activeRunsByState.get(state.id) || [];
                const isActive = stateRuns.length > 0;

                return (
                    <div key={state.id} className="flex items-center">
                        {/* Compact state pill */}
                        <div className={cn(
                            'flex items-center gap-1 rounded-full px-2.5 py-1 text-[11px] font-medium border',
                            isActive && 'border-violet-300 bg-violet-50 dark:border-violet-800 dark:bg-violet-950/30',
                            !isActive && agent && 'border-violet-200 dark:border-violet-900',
                            !agent && 'border-border',
                        )}>
                            {state.color && (
                                <span className="h-2 w-2 rounded-full shrink-0" style={{ backgroundColor: state.color }} />
                            )}
                            <span className="truncate max-w-[80px]">{state.name}</span>
                            {agent && (
                                <QuickTooltip label={agent.name}>
                                    <Bot className="h-3 w-3 text-violet-500" />
                                </QuickTooltip>
                            )}
                            {isActive && (
                                <span className="h-1.5 w-1.5 rounded-full bg-violet-500 animate-pulse" />
                            )}
                        </div>

                        {/* Arrow connector */}
                        {!isLast && (
                            <div className="flex items-center mx-0.5">
                                <div className={cn(
                                    'h-[1.5px] w-3',
                                    hasAdvance ? 'bg-violet-400' : 'bg-border',
                                )} />
                                <ChevronRight className={cn(
                                    'h-3 w-3 -ml-1',
                                    hasAdvance ? 'text-violet-400' : 'text-border',
                                )} />
                            </div>
                        )}
                    </div>
                );
            })}
        </div>
    );
}
```

### Integration into KanbanBoard

**`frontend/src/components/pm/KanbanBoard.tsx`:**

The kanban board already receives `automatedStateIds`. Extend it to also receive the data needed for the strip:

```tsx
// In the parent component that renders KanbanBoard (likely the stories page route):
const { data: rules } = useAutomationRulesByWorkflow(workspaceId, workflowId);
const { data: agents } = useAgents(workspaceId);
const { data: activeRuns } = useActiveAgentRuns(workspaceId);  // NEW query hook

// In KanbanBoard, above the columns:
{showPipelineStrip && rules && rules.length > 0 && (
    <PipelineStatusStrip
        states={sortedStates}
        automationRules={rules}
        agents={agents ?? []}
        activeRuns={activeRuns ?? []}
    />
)}
```

Add a toggle for the strip (persist in localStorage or board display store):

```tsx
// In board toolbar:
<QuickTooltip label="Toggle pipeline view">
    <Button variant="ghost" size="sm"
        className={cn('h-7 w-7 p-0', showPipelineStrip && 'bg-muted')}
        onClick={() => setShowPipelineStrip(v => !v)}>
        <Workflow className="h-3.5 w-3.5" />
    </Button>
</QuickTooltip>
```

### New query hook: `useActiveAgentRuns`

```typescript
// In frontend/src/hooks/queries/useAgentRuns.ts (or add to existing)
export function useActiveAgentRuns(workspaceId?: string) {
    return useQuery({
        queryKey: queryKeys.agentRuns.active(workspaceId),
        queryFn: () => unwrap(agentService.listActiveRuns(workspaceId!)),
        enabled: !!workspaceId,
        refetchInterval: 15_000,  // Poll every 15s for active run status
    });
}
```

Backend endpoint: `GET /pm/agent-runs?workspace_id=X&status=queued,running,awaiting_approval` — filter existing list endpoint by status.

### Implementation Order

1. Add status filter to agent runs list endpoint (backend)
2. Create `useActiveAgentRuns` query hook
3. Build `PipelineStatusStrip` component
4. Add strip toggle to kanban board toolbar
5. Integrate into `KanbanBoard` above columns
6. Add board display store setting for strip visibility

---

## 5. OpenSpec Output Format in Planning Methodology

**Priority: Low | Effort: Small**

### Problem

The spec document format is whatever the prompt pack produces. If a team wants OpenSpec-style requirements (SHALL/MUST statements, GIVEN/WHEN/THEN scenarios), they have to manually instruct the planner via `planning_notes`. There's no first-class way to select a spec output format.

### Solution

Add a `planning_output_format` setting to workspace AI settings. The prompt pack produces specs in the selected format. The acceptance criteria on planned stories naturally inherit the format from the spec.

### Data Model Changes

**`server/internal/model/settings.go` — `WorkspaceSettings`:**

Add field:

```go
PlanningOutputFormat string `json:"planning_output_format" gorm:"not null;default:'default'"`
    // "default": current freeform format
    // "openspec": OpenSpec-style (SHALL/MUST + GIVEN/WHEN/THEN scenarios)
```

Add to `UpdateSystemSettingsRequest`:

```go
PlanningOutputFormat *string `json:"planning_output_format,omitempty"`
```

### Prompt Pack Changes

**`server/internal/worker/planning_prompt_pack.go`:**

In the `draft_spec` stage guidance, branch on output format:

```go
func specOutputFormatGuidance(format string) string {
    switch format {
    case "openspec":
        return `
Write the spec using OpenSpec conventions:

- Requirements use RFC 2119 keywords: SHALL, MUST, SHOULD, MAY
- Each requirement is a clear, testable statement
- Behavioral requirements include GIVEN/WHEN/THEN scenarios:

  GIVEN a user with OAuth configured
  WHEN they click "Sign in with Google"
  THEN the system SHALL redirect to Google's OAuth consent screen
  AND on successful auth, create or update the user record

- Group requirements by capability area
- Each section should be independently implementable
- Mark non-functional requirements (performance, security) separately

When this spec is used for story planning, the GIVEN/WHEN/THEN scenarios will become the acceptance criteria for each story.`

    default:
        return "" // Current behavior: freeform structured markdown
    }
}
```

In the `plan_stories` stage guidance, if the spec was written in OpenSpec format:

```go
func storyPlanningFormatGuidance(format string) string {
    switch format {
    case "openspec":
        return `
The spec uses OpenSpec format with GIVEN/WHEN/THEN scenarios.

For each proposed story:
- acceptance_criteria MUST be the relevant GIVEN/WHEN/THEN scenarios from the spec, copied verbatim
- Each story should cover one or more complete scenarios
- Do not split a single GIVEN/WHEN/THEN scenario across multiple stories`

    default:
        return ""
    }
}
```

### Frontend Changes

**Settings > AI page:**

Add a format selector below the methodology dropdown:

```tsx
<div className="space-y-1.5">
    <Label>Spec Output Format</Label>
    <Select value={settings.planning_output_format} onValueChange={v => updateSetting('planning_output_format', v)}>
        <SelectTrigger><SelectValue /></SelectTrigger>
        <SelectContent>
            <SelectItem value="default">Default (structured markdown)</SelectItem>
            <SelectItem value="openspec">OpenSpec (SHALL/MUST + GIVEN/WHEN/THEN)</SelectItem>
        </SelectContent>
    </Select>
    <p className="text-xs text-muted-foreground">
        Controls the format of AI-generated product specs. OpenSpec uses formal requirements language and testable scenarios.
    </p>
</div>
```

### Pass-through

The format setting flows through the existing pipeline:
1. `resolvePlanningWorkspaceAISettings()` reads it from `WorkspaceSettings`
2. Added to `planningRunInput` as `PlanningOutputFormat`
3. `BuildSystemPrompt` passes it to `planningPackSections`
4. Prompt pack includes format-specific guidance
5. Claude generates spec in the chosen format
6. Acceptance criteria on planned stories naturally use GIVEN/WHEN/THEN (from the spec)

No database schema changes for specs or stories — the format is a rendering concern handled entirely by the prompt.

### Implementation Order

1. Add `PlanningOutputFormat` to `WorkspaceSettings` model + update request
2. Add format guidance functions to prompt pack
3. Wire format through `planningRunInput` and `BuildSystemPrompt`
4. Add format selector to Settings > AI frontend page
5. Update TypeScript types for settings

---

## 6. Evaluate merge_branch — Keep or Move to Delivery Layer

**Priority: Low | Effort: Design decision**

### Current State

`merge_branch` is one of three action types in the automation rules engine. It's configured per-state in the pipeline builder. When a story enters that state, the rule engine calls `GitService.MergeBranch()` directly.

### Arguments for Moving to Delivery Layer

The delivery system already has the right primitives:
- `PMTeamRepoDefault` has `DoneStateID` (PR-merged state mapping)
- `StoryDeliveryTarget` tracks working branch, base branch, PR metadata
- GitHub webhook handler processes `pull_request.merged` events and updates story state

If merge happened as part of the done-state transition:
1. Story enters done state → delivery system merges working branch to base branch (from team defaults)
2. No need to configure `merge_branch` in pipeline builder
3. One less action type to explain to users
4. Git operations stay in the delivery layer, not the rule engine

### Arguments for Keeping

Mid-pipeline merges are a real use case:
- Merge to `staging` after review state (before production deploy)
- Merge to `develop` after engineer approval (before QA)
- Multiple merge targets at different pipeline stages

The team defaults only support one `DoneStateID` mapping. The rule engine's `merge_branch` supports any state as a trigger and any branch as a target. This flexibility is genuinely useful for teams with multi-environment workflows.

### Recommendation

**Keep `merge_branch` but clarify the mental model.** The pipeline builder already makes it intuitive — it's a per-state configuration, not a raw rule. The key change is documentation and defaults:

1. For the common case (merge to base branch on done), auto-create a merge rule when a team sets up their workflow defaults. This is a one-time seed, not ongoing automation.
2. For the advanced case (mid-pipeline merges), the pipeline builder already supports it via the `MergeBranchInput` component.
3. Document the distinction: team defaults control *what* gets merged (base branch). Pipeline rules control *when* it happens (which state triggers the merge).

### If You Do Want to Move It Later

The migration path would be:
1. Add a `merge_on_done` boolean to `PMTeamRepoDefault` (default true)
2. When a story transitions to a state matching `DoneStateID`, the delivery service auto-merges
3. Remove `merge_branch` as an action type from automation rules
4. Migrate existing `merge_branch` rules: if the trigger state matches `DoneStateID`, delete the rule and enable `merge_on_done`. If it's a mid-pipeline merge, log a warning and keep it as a custom rule (or create a new action type like `deploy_stage`).

This is a non-trivial migration with edge cases. Recommend deferring unless the current model causes active confusion.

### No Code Changes Required Now

This item is a design decision, not an implementation. The current model works. Revisit if users report confusion about where merge configuration lives.

---

## Summary: Implementation Dependencies

```
                        ┌──────────────────────────────────┐
                        │ 1. RunContext (High, Medium)     │
                        │    - model + service + executor  │
                        │    - frontend context viewer     │
                        └──────────────────────────────────┘

  ┌───────────────────────────┐    ┌──────────────────────────────┐
  │ 2. Vertical Slicing       │    │ 3. Approval Semantics        │
  │    (Medium, Small)         │    │    (Medium, Small)            │
  │    - model + prompt        │    │    - model + service + UI     │
  │    - frontend badges       │    │    - request_changes re-run   │
  └───────────────────────────┘    └──────────────────────────────┘

  ┌───────────────────────────┐    ┌──────────────────────────────┐
  │ 4. Pipeline Strip          │    │ 5. OpenSpec Format            │
  │    (Medium, Medium)        │    │    (Low, Small)               │
  │    - new component         │    │    - settings + prompt pack   │
  │    - active runs query     │    │    - format selector UI       │
  └───────────────────────────┘    └──────────────────────────────┘

  ┌───────────────────────────┐
  │ 6. merge_branch eval       │
  │    (Low, Design decision)  │
  │    - no code changes now   │
  └───────────────────────────┘
```

All six items are independent — no cross-dependencies. They can be implemented in any order or in parallel.
