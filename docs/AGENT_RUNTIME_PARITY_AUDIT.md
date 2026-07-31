# Agent Runtime Extraction — Feature Parity Audit

- **Date:** 2026-07-09
- **Author:** Azhar (azhar@d4interactive.io), with Claude
- **Question:** helpin originally executed agents in-process; the engine was later
  extracted into the standalone `agent-runtime` service. Did the extraction drop
  any capability?
- **Method:** three independent capability inventories, cross-referenced:
  1. **OLD embedded engine** — helpin `server/internal/temporalapp/` at the
     pre-cutover commit `77e30ab74^` (plus the execution logic retired from
     `server/internal/service/agent.go`).
  2. **NEW service** — `agent-runtime` (`internal/engine`, `internal/runtime`,
     `internal/tools`, `internal/skills`, `internal/durable`, `internal/store`,
     `internal/host`, `internal/mcp`, …) on SDK `agent-runtime-go@v0.1.3`.
  3. **Delegation contract** — what helpin now hands off (`agent_runtime_client.go`,
     `agent_runtime_host.go`, `agent_runtime_launch_context.go`,
     `agent_runtime_finalizers.go`, `agent_runtime_projection.go`).

Companion docs: `AGENT_RUNTIME_LOCAL.md` (per-surface parity rows — **now stale**,
see below), `AGENT_RUNTIME_FREEZE_AND_RETIREMENT.md`.

---

## 1. Bottom line

**Nothing was *silently* dropped at the infrastructure level.** The split is
well-engineered. Most capability did not disappear — it either **moved into
`agent-runtime`** (agent loop, tools, git, streaming, codex auth, MCP/skills —
some of it richer than the original) or **moved into helpin's projection +
finalizers** (billing, PR creation, support finalization, command-bar plan
advancement, automation triggers).

**The real problem is that the code outran the parity doc.** `AGENT_RUNTIME_LOCAL.md`
describes a gradual *per-surface* delegation ladder in which planner presets stay
on the local executor. Commit `77e30ab74` ("retire local Temporal agent execution;
agent-runtime is the only path") did a **hard cutover** instead:

- `AgentService.delegatesRunToAgentRuntime` (`agent.go:178-183`) now returns true
  for **every** agent + non-empty target when `AGENT_RUNTIME_LAUNCH_ENABLED` is on.
- There is **no local executor fallback** — a run either delegates or fails loudly.
- The per-surface predicate maps (`agentRuntimePresetDelegatedTargets`,
  `agentRuntimeCustomAgentDelegatedTargets`) were **deleted**.
- The old `ExecuteRunActivity` / `AgentRunWorkflow` were first unregistered and
  were removed from Helpin in the executor-demolition release. Their history is
  available in git and in the freeze document's inventory.

Net effect: surfaces the doc calls "not flipped" or "deliberately local" —
**planners (epic/task), custom coding agents, workspace-target coding, crm_company,
handoff** — now all delegate. That exposes gaps the ladder previously masked.

---

## 2. Genuine gaps (verified, prioritized)

### 🔴 HIGH

**G1 — Native loop caps at 25 tool steps; the old engine allowed 50, and 300 for planners.**
- Old: `defaultWorkflowMaxIterations = 50`, `plannerWorkflowMaxIterations = 300`
  (`worker/context.go`); overflow → "start another run to continue".
- New: `defaultNativeMaxToolSteps = 25`, hardcoded, and the provider factory
  always sets it (`native_eino_provider.go:93`). `MaxToolSteps` is plumbed
  (`native_exec.go:145-147`) but never overridden per-agent/preset. Overflow →
  `"native runtime reached max tool steps"` (`native_exec.go:244`).
- **Impact:** a planner or non-trivial native coding/review run needing >25 tool
  calls now truncates **12× sooner** than before. Applies to `native_sdk` runs
  (all planners; native coding/review). Codex/OpenCode have separate limits.

**G2 — Approved-preview application exists on *neither* side.**
- Old: helpin applied approved plans — `applyApprovedInteractivePreview`
  (`approved_preview_application.go`): apply approved PRD → write doc + approve
  epic spec; apply approved task plan → `pm.create_task_batch`; apply approved
  task-doc → persist to Docs.
- New service: no apply step — previews are only artifacts
  (`artifact_preview_tools.go`); on resume the model just continues.
- Delegation finalizer: **explicitly deferred** — "Deep temporalapp machinery
  (approved-preview application, planning repository prep, completion interaction
  policy, failing the run on invalid output) is intentionally deferred"
  (`agent_runtime_finalizers.go:351-356`; see
  `docs/plans/2026-07-02-delegated-run-finalizers.md`).
- New design instead expects the **agent itself** to call command tools
  (`pm.create_task_batch`, `docs.publish_prd_draft`) after approval.
- **Impact:** if the planner skill does not drive those commands post-approval,
  an approved epic plan **silently produces nothing** (no tasks, no persisted
  PRD). This is the doc's original "hard blocker," now reachable via the cutover.
  **Must be verified end-to-end with one live delegated planner run.**

### 🟡 MEDIUM

**G3 — Completion interaction policy defined but not enforced.**
- Old: `enforceCompletionInteractionPolicy` + `retryInvalidCompletionTurn` +
  `synthesizeCompletionInteractionFallback` (`completion_interaction_policy.go`)
  required the declared interaction kinds to exist for the terminal turn, retried
  an invalid completion once, and synthesized a review checkpoint for
  codex/opencode when policy required one.
- New: skills declare `CompletionRequiresInteractionKinds` and it is aggregated
  into `Policy` (`skills/policy.go:AggregatePolicy`), but **no engine/runtime
  code consumes it** at completion.
- **Impact:** an agent can complete a run without the required approval/review
  interaction and nothing catches it.

**G4 — Planner context is not assembled.**
- Old: built at execution time — spec/PRD snapshots, task/epic summaries, linked
  docs, comments, tickets, existing-tasks guardrails, repository code context
  (`planner_context_assembly.go`, `planning_domain_documents.go`,
  `planning_code_context.go`).
- New: agent-runtime does not assemble planner context. helpin's launch bridge
  `buildDelegatedTaskLaunchContext` stamps *coding-task* context (operator notes,
  task/epic background, branch, plan-doc) but **not planner context**; the host
  target-context endpoint returns only shallow epic/task rows.
- **Impact:** delegated planners start with essentially no planning context and
  must self-gather via read tools — into the G1 25-step cap.

**G5 — Phase-skill activation plumbing is broken.**
- Runtime activates planner phase skills from `preset_key` (read from
  `agent.ExecutionConfig`, else run `Metadata`/`Trigger`) + `planning_stage`
  (read from run `Metadata`/`Trigger`): `engine.go:nativeActiveSelectionContext`
  (`engine.go:368-386`), `skills/activation.go:SelectNativeActiveSkills`.
- helpin stamps `md["agent_preset_key"]` (`agent.go:109`) — the **wrong key** —
  and never stamps a derived `planning_stage`. `model.AgentExecutionConfig` does
  not carry `preset_key` either. The old engine *derived* the stage from epic/task
  state at execution time (`epicPlannerPhaseName`, `execution_contracts.go`); the
  runtime has no equivalent derivation.
- **Impact:** phase selection likely never fires for delegated planners → planner
  runs execute without stage-specific phase guidance.

**G6 — No failed-runtime salvage.**
- Old: `salvageFailedRuntimeStateFromSnapshotStore` (`failed_runtime_salvage.go`)
  recovered the plan and published previews from the stream snapshot into
  artifacts on failure.
- New: failures go straight to `MarkRunFailedActivity`; the execute activity is
  single-attempt (`MaximumAttempts: 1`), so a transient mid-run failure loses
  partial work with no recovery.

### 🟢 LOW–MEDIUM (execution-quality niceties)

**G7 — Missing read tools.**
- `list_task_checklist` is referenced by skills (`skills/builtin.go:205,240`) as a
  required tool but is **not registered** anywhere (not in agent-runtime
  `tools/`, not exposed as a helpin command) — agents cannot read task checklists.
- Also present in the old native tool set but not found in the new one:
  `find_tasks_for_git_changes`, docs `list_spaces` / `list_collections`.
- Launch context also omits checklist items and non-plan linked docs vs the
  Temporal path (documented at `agent_runtime_launch_context.go:77`).

**G8 — Lost execution helpers.**
- **Repair-guidance injection** — old classified tool failures into corrective
  instructions (`repair_instructions.go`); new returns the raw error to the model
  (no structured repair phase).
- **Flow-output instruction assembly** — old built the task-completion-followups /
  CRM deal-review JSON prompts (`flow_output_instructions.go`); new has no
  flow-output primitive by name (the finalizer only *validates* `flow_output_kind`).
- **Rolling transcript summarization** — old wrote transcript summary checkpoints
  to protect the context window (`execution_context_loading.go:ensureTranscriptSummaryCheckpoint`);
  new keeps the full `native_messages` transcript in `OutputSummary`.

---

## 3. Confirmed NOT gaps (moved, working)

Moved to helpin projection/finalizers (`agent_runtime_finalizers.go`,
`agent_runtime_projection.go`):

- Support draft finalization (pending-gate, run-ID idempotency, `sent_message_id`
  write-back, websocket publish, visitor refresh).
- Command-bar plan advancement for delegated child runs.
- `agent_run.completed` (finalizer) / `agent_run.approved` (approval path) automations.
- Billing: terminal usage consumption + overage preflight/auto-cancel.
- PR/MR creation off the pushed branch (`FinalizeDelegatedRunDelivery`), delivery
  target + git-link records, `pr_failed` bookkeeping.
- Agent-idle flip + monthly token rollup.

Present (and in some cases richer) in `agent-runtime`:

- Codex auth with a shared, encrypted `codex_auth_tokens` store + device-code manager.
- Git clone, base→work branch sync (conflict/recreate/unrelated-history handling),
  safe non-ff push, merge-conflict in-run handoff (codex/opencode),
  codex command guards blocking `git push` / `gh pr`.
- Security scanners (`scan_gitleaks`/`scan_semgrep`/`scan_trivy`), web tools
  (`web_search_brave`/`web_search_exa`/`fetch_url`/`crawl_url`).
- Resume-after-approval reconciliation (native) + codex resume fallback prompt.
- Inbound MCP gateway, stdio MCP bridge, outbound MCP providers, skill packages.
- Parallel read-only tool execution within a round.

---

## 4. Recommendations

1. **Re-add a planner delegation guardrail.** Do not delegate `epic_planner` /
   `task_planner` until G2/G4/G5 are resolved or verified E2E — the hard cutover
   removed the safety net that kept planners local. (A minimal targeted predicate
   check on the two planner presets, rather than restoring the whole map.)
2. **Lift / parameterize the 25-step cap (G1)** — thread a per-preset
   `MaxToolSteps` (planner ≈ 300) instead of the hardcoded default.
3. **Verify the command-driven planner path (G2)** with one live delegated
   planner run: publish plan → approve → confirm tasks/PRD actually materialize.
4. **Update `AGENT_RUNTIME_LOCAL.md`** to state that the per-surface ladder became
   a hard cutover, and link this audit for the current gap list.

---

## 5. Reference constants (old → new)

| Aspect | Old (temporalapp/worker) | New (agent-runtime) |
|---|---|---|
| Max tool steps (native) | 50 default / **300 planner** | **25 fixed** (`native_exec.go:26`) |
| Execute activity retry | single-attempt | single-attempt (`durable/workflow.go`) |
| Activity heartbeat | 15s ticker | 15s ticker (`durable/activities.go`) |
| Runtime modes | native_sdk / codex / opencode | native_sdk / codex / opencode |
| Codex activity timeout | 30 min | 30 min |
| Approved-preview application | helpin applies | **neither** (agent must self-drive commands) |
| Completion interaction policy | enforced + retry | **defined, not enforced** |
| Failed-runtime salvage | yes | **no** |
| Planner context assembly | execution-time | **not assembled** |
