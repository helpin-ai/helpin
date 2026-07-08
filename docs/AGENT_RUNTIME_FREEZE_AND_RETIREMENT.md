# Agent Runtime Freeze and Retirement Runbook

The final slice of the Helpin → agent-runtime migration: freeze the in-process
executor, graduate each remaining surface through the flag ladder, then
demolish `server/internal/worker/` and the Temporal agent-run machinery.

Companion docs:

- `docs/AGENT_RUNTIME_LOCAL.md` — parity rows per surface + planner blockers.
- `docs/AGENT_RUNTIME_STAGING.md` — staging enablement + rollback mechanics.
- `docs/AGENT_RUNTIME_SHADOW_VALIDATION.md` — canary window definition.
- `scripts/agent-runtime-canary/canary.sh` — the canary itself.
- `docs/AGENTS_AND_AUTOMATION.md` — agent taxonomy this migration preserves.

Terminology: a **surface** is a (preset-or-custom-agent, target-type) slice of
the delegation predicate — one entry in `agentRuntimePresetDelegatedTargets` /
`agentRuntimeCustomAgentDelegatedTargets` (`server/internal/service/agent.go`).
This runbook is deliberately surface-generic: it does not hardcode today's
delegated list, because that list is actively growing (the repository surface —
code_builder / review_agent — is being evaluated in parallel). Whatever the
list says on any given day, every surface goes through the same ladder below.

---

## 1. Freeze policy — effective immediately

`server/internal/worker/` (the in-process executor: eino/codex/opencode
runtimes, tool loop, workspace prep) and the agent-run portions of
`server/internal/temporalapp/` are **frozen: critical fixes only**.

Every new runtime capability — new tools, new model/provider support, new
interaction kinds, prompt/skill improvements, workspace features — lands in
the **agent-runtime repo (+ its SDK)** and reaches Helpin exclusively through
the existing contracts:

- launch/cancel/resume API (`internal/service/agent_runtime_client.go`),
- host adapter callbacks (`internal/service/agent_runtime_host.go`,
  `internal/service/agent_tool_gateway.go`, internal-command providers),
- the `AGENT_RUNTIME_EVENTS` NATS projection
  (`internal/service/agent_runtime_projection.go`) and terminal finalizers
  (`internal/service/agent_runtime_finalizers.go`).

### What counts as a "critical fix" (exhaustive)

A change to frozen code is critical if and only if it is one of:

1. **Data-integrity fix** — the local executor corrupts, loses, or
   double-writes run rows, messages, artifacts, git state, or billing usage.
2. **Security fix** — credential leakage, sandbox escape in the tool loop,
   command-guard bypass (`codex_command_guards.go`, `tools_security.go`),
   auth-token mishandling.
3. **Availability fix** — the executor crashes the Temporal worker, wedges a
   workflow so runs can never terminate, or leaks workspaces/disk
   (`git_grace_cleanup.go` class of problems).
4. **Migration-enabling change** — a change whose sole purpose is to advance
   this runbook: relocating a leaked helper (section 4), deleting dead code,
   tightening the delegation predicate, or adding parity instrumentation.

Everything else — including "small" tool improvements, prompt tweaks, and new
preset behavior — is **not critical** and must be built runtime-side. If a
frozen-path bug also exists runtime-side, fix it runtime-side first; only
backport if a still-local surface is actively bleeding.

### Enforcement

- **PR review rule (mandatory, active now):** any PR touching
  `server/internal/worker/**` or the MIGRATE-THEN-DELETE `temporalapp` files
  (section 5.2) must state in its description which of the four critical-fix
  categories it falls under. Reviewers reject PRs that add capability to
  frozen paths. Reviewer question to ask verbatim: *"why can't this land in
  agent-runtime?"*
- **CODEOWNERS suggestion (optional, no change made here):** add a
  `.github/CODEOWNERS` entry for `server/internal/worker/` routing to the
  migration owners so freeze review is automatic.
- **CI suggestion (optional, no change made here):** a PR check that flags
  (not blocks) diffs under `server/internal/worker/` lacking a
  `Freeze-Exception:` trailer in the PR body. Suggestion only — do not wire
  CI as part of this doc.

The freeze does **not** apply to: `internal/service/agent_runtime_*.go`, the
tool gateway, host adapters, the delegation predicate, or non-agent
`temporalapp` workflows (email sync, CRM crons, docs embedding, PM recurring,
etc.) — those are the permanent host-side surface and evolve freely.

---

## 2. Flag graduation ladder (per surface)

The only flag is `AGENT_RUNTIME_LAUNCH_ENABLED`
(`server/internal/config/config.go` → `AgentRuntimeLaunchEnabled`), gating
`AgentService.delegatesRunToAgentRuntime`. Per-surface granularity comes from
the predicate maps in `internal/service/agent.go`
(`agentRuntimePresetDelegatedTargets`, `agentRuntimeCustomAgentDelegatedTargets`):
"flipping a surface" = merging a predicate-map entry, which takes effect
wherever the flag is already on.

Each surface climbs this ladder independently:

```
parity rows green/amber-cleared (LOCAL.md)
        │
        ▼
[1] staging flag on for the surface  (predicate entry merged; staging has
        │                             AGENT_RUNTIME_LAUNCH_ENABLED=true)
        ▼
[2] shadow window on staging          per docs/AGENT_RUNTIME_SHADOW_VALIDATION.md:
        │                             20 canary runs over 5 business days
        │                             (≥3/day), all passing, ZERO projection
        │                             mismatches; any failure resets the window
        │                             after root-cause. Non-canary-scriptable
        │                             surfaces get an equivalent scripted run
        │                             checked against the same assertions
        │                             (external_runtime stamped, workflow_id
        │                             null, terminal status projected,
        │                             tokens_used > 0, usage consumed).
        ▼
[3] prod flag on                      predicate entry rides the normal
        │                             develop → main promotion; prod Doppler
        │                             gets AGENT_RUNTIME_LAUNCH_ENABLED=true
        │                             (first surface only — later surfaces
        │                             inherit it).
        ▼
[4] bake period on prod               declared over when ALL of:
        │                             • 4 weeks elapsed (N=4; the first-ever
        │                               surface and the repository surface use
        │                               N=6 — write path touches customer
        │                               repos);
        │                             • rollback flag never exercised for this
        │                               surface during the window;
        │                             • canary (or equivalent) green for the
        │                               whole window;
        │                             • zero projection-mismatch incidents
        │                               (status/transcript/usage divergence
        │                               between runtime and Helpin rows, or
        │                               `agent runtime terminal usage
        │                               consumption failed` / projection error
        │                               logs for the surface's runs).
        ▼
[5] surface declared migrated         recorded in LOCAL.md parity table;
                                      its local-executor code becomes
                                      demolition-eligible (section 4).
```

### Rollback semantics (identical at every rung)

Per `docs/AGENT_RUNTIME_STAGING.md` § Rollback:

- Flipping `AGENT_RUNTIME_LAUNCH_ENABLED=false` (or reverting a predicate
  entry) affects **NEW runs only** — they immediately route back to the
  in-process Temporal executor.
- **In-flight delegated runs are not orphaned**: the runtime keeps executing
  them, and the projection consumer — which runs regardless of the launch
  flag — finishes projecting their terminal state and firing finalizers.
- Consequence for demolition: the projection worker, finalizers, and host
  adapters must **never** be deleted; they are the permanent architecture, not
  migration scaffolding. Also never remove the `helpin` entry from
  `AGENT_RUNTIME_APP_CONFIG` while delegated runs are active.

### Surfaces still needing ladder entry

- **Planners (epic_planner / task_planner):** blocked; see "Planner parity
  blockers" in `docs/AGENT_RUNTIME_LOCAL.md` (execution-time context assembly,
  phase-selection metadata, approved-preview application). They enter rung 1
  only after those blockers close.
- **Repository surface (code_builder / review_agent):** under evaluation in a
  parallel effort; enters at rung 1 when its parity rows land.
- **Codex/opencode custom agents:** intentionally local (auth + interaction
  handling still live in-process); need runtime-side codex auth before rung 1.

---

## 3. Demolition preconditions

No file below is deleted until:

1. Every surface that exercises it has completed rung 5, **and**
2. No non-terminal `agent_runs` row has `external_runtime IS NULL` with a live
   local workflow (i.e., all in-flight local runs have drained), **and**
3. The leaked helpers it exports (section 4 RELOCATE list) have been moved to
   a neutral package and importers repointed, **and**
4. `go build ./... && go vet ./...` pass after the deletion commit.

---

## 4. Demolition inventory — `server/internal/worker/`

All 60 non-test files classified. Evidence: grep of
`internal/worker` imports across `internal/` and `cmd/` (importers are
`cmd/api/main.go`, `cmd/temporal-worker/main.go`, `internal/agentskills/*`,
`internal/llm/claude.go`, `internal/service/*`, `internal/temporalapp/*`) plus
per-file symbol tracing. Test files (`*_test.go`) follow their subjects.

### 4.1 RELOCATE first — helpers that leak out of the worker package (14 files)

These export symbols imported by host-side code that **survives** the
migration. Move each to a neutral package (suggested: `internal/agentskills`
for skill files — it already half-wraps them; `internal/agenttools` or
`internal/agentcontracts` for tool-name/artifact/interaction helpers;
`internal/llm` for the Claude client) **before** any DELETE group ships.

| Worker file | Leaked symbols (non-exhaustive) | Live importers |
| --- | --- | --- |
| `skill_loader.go` | `SkillDefinition`, `SkillPolicy`, `SkillInterface` | `internal/agentskills/activation.go`, `internal/agentskills/resolve.go`, `internal/service/agent_runtime_host.go` (skill host endpoints), `internal/service/agent_workspace_skills.go` |
| `skill_catalog.go` | `GetBuiltInSkill`, `ListSkillCatalog`, `BuiltInPresetSkillBundleForPreset`, `RuntimeSkillKeysForPresetBundle`, `CanonicalBuiltInSkillKey`, `BuiltInPresetPrompt`, `BuiltInPresetInstructionTemplateVersion`, `CompilePresetInstructionsWithAvailableSkills`, `InstructionTemplateVersionForPresetWithAvailableSkills` | `internal/agentskills/resolve.go`, `internal/agentskills/stage.go`, `internal/service/agent.go`, `internal/service/agent_presets.go`, `internal/service/agent_system_prompts.go`, `internal/service/agent_runtime_host.go` |
| `skill_archive.go` | `BuildSkillArchive`, `LoadSkillArchive`, `SkillVersionForBytes`, `ExtractSkillArchiveToDir`, `SortedUniqueStrings` | `internal/service/agent_runtime_host.go` (**skill host endpoints serve archives to the runtime**), `internal/service/agent_workspace_skills.go`, `internal/agentskills/stage.go` |
| `skill_package_fs.go` | `CopyBuiltInSkillPackageToDir` | `internal/agentskills/stage.go` |
| `tool_names.go` | `RenderRuntimeToolNamesInInstructions`, `RuntimeToolNameForPrompt`, `RuntimeToolNamesForPrompt` | `internal/agentskills/resolve.go`, `internal/agentskills/stage.go`, `internal/temporalapp` |
| `tools_interaction.go` | `NormalizeToolNames`, `CanonicalToolName`, `Tool*` name constants (`ToolRequestApproval`, `ToolUpdatePlan`, …), `ExtractLatestApprovalRequest`, `ExtractLatestHumanInputRequest`, `ExtractLatestReviewCheckpointRequest`, `UserInputRequest`, `UserInputSummary`, `HumanInputArtifactFromUserInputRequest` | `internal/service/agent.go`, `agent_policy.go`, `agent_presets.go`, `agent_tool_gateway.go`, `internal/agentskills/resolve.go` |
| `tools_preview.go` | `ExtractPublishedPreviews`, `PublishedPreview`, `RunPreviewArtifactType`, `PreviewFormatJSON/Markdown`, `ToolPublishTaskPlan`/`ToolPublishPRDDraft`/`ToolPublishTaskPlanDoc` constants | `internal/service/agent.go`, `agent_tool_gateway.go`, `coding_session.go`, **`internal/service/internal_command_docs_publish.go` (internal commands)** |
| `tools_plan.go` | `ExtractLatestRunPlan`, `RunPlanArtifact`, `RunPlanStep`, `PlanStep*` states | `internal/service/agent_tool_gateway.go`, `internal/temporalapp` |
| `interaction_contracts.go` | `NormalizeInteractionContracts`, `SkillInteractionContract`, `SkillInteractionTransport`, `InteractionKind*` | `internal/agentskills/resolve.go`, `internal/temporalapp` |
| `resolve.go` | `ResolveAgentProfile`, `ResolveApprovalState`, `ResolvedProfile` | `internal/service/agent.go`, `agent_policy.go`, `agent_skill_resolution.go` |
| `runtime_profiles.go` | `GetRuntimeProfile` | `internal/service/agent_presets.go` |
| `tool_catalog.go` | `ListToolCatalog` | `internal/service/agent.go` (tool catalog API) |
| `claude.go` | `ClaudeClient`, `NewClaudeClient`, `CreateMessageRequest/Response`, `ContentBlock`, `ToolDefinition`, `ToolChoice` | **`internal/llm/claude.go`** — the model-agnostic LLM provider (CRM signal detection etc.) wraps the worker's Claude client. Relocate into `internal/llm`. |
| `context.go` + `tools.go` (registry core) | `ExecutionContext`, `NewToolRegistry` | `internal/service/agent_tool_gateway.go` calls `worker.NewToolRegistry(nil)` + `worker.ExecutionContext` to execute artifact tools (preview/plan/interaction) host-side for delegated runs. **Narrow the gateway to an artifact-tool-only registry in a neutral package first**; the full registry drags in every `tools_*.go` (e.g. `NewWebFetchClient` from `tools_web.go`), which blocks Group E. |

Also relocate the single stray helper `NormalizeTaskPlanPreviewContent`
(defined in `orchestration.go`, imported by `internal/service/agent.go`) —
the rest of `orchestration.go` is Group A below.

### 4.2 DELETE groups (46 files) — with per-group preconditions

**Group A — native in-process executor (11 files).**
Precondition: **all** native_sdk surfaces migrated (incl. planners and any
custom-native targets), plus gateway narrowed per 4.1.
`eino_exec.go`, `eino_executor.go`, `runtime_adapter.go`,
`runtime_factory.go` (importer to clean: `cmd/temporal-worker/main.go`
`NewDefaultRuntimeRegistry`), `runtime_helpers.go`, `orchestration.go` (after
relocating `NormalizeTaskPlanPreviewContent`), `helpin_mcp.go` (importer:
`temporalapp` `HelpinMCPRuntimeToolName`/`HelpinMCPToolAliases` — falls with
Group F), `signal_prompts.go`, `planning_prompt_pack.go`,
`usage_metering.go` (usage preflight/recorder wrapper — **no importers outside
worker**, deletable with the executor), `tool_file_state.go` (no external
importers).

**Group B — codex executor + host codex glue (11 files).**
Precondition: codex-backed surfaces migrated **and** codex auth fully moved to
runtime endpoints (`/v1/runs/{id}/codex-auth/...` per LOCAL.md parity row);
delete the local-codex branches in `internal/service/agent.go` and the wiring
in `cmd/api/main.go` in the same change.
`codex.go`, `codex_appserver_client.go`, `codex_appserver_protocol.go`,
`codex_session_host.go`, `codex_event_mapper.go`, `codex_command_guards.go`,
`codex_auth_recovery.go`,
`codex_auth_manager.go` (importers: `cmd/api/main.go` `NewCodexAuthManager`,
`internal/service/agent.go` `CodexAuthManager`),
`codex_workspace_auth_store.go` (importers: `cmd/api/main.go`,
`cmd/temporal-worker/main.go`),
`codex_approval_bridge.go` (importer: `internal/service/agent.go`
`BuildCodexApprovalResponseFromPayload` / `BuildCodexUserInputResponseFromPayload`),
`codex_thread_store.go` (importer: `internal/service/agent.go`
`LoadCodexSessionSnapshot`).

**Group C — opencode executor (3 files).**
Precondition: same as Group B's surface condition (opencode custom agents).
`opencode.go`, `opencode_helpers.go`, `opencode_stream.go`. No importers
outside the worker package.

**Group D — workspace prep + git persistence (3 files).**
Precondition: repository surface migrated (workspace prep/push happens
runtime-side) and `PersistentWorkspacePathForRun` relocated for its importer
`internal/service/coding_session.go`.
`workspace.go` (`PrepareWorkspace*`/`CleanupWorkspaceForRun` importers are
`temporalapp` — Group F), `git_persistence.go` (`GitIdentity`,
`ResolveGitIdentity`, `PRMetadata`, `GitCommitMetadata` importers:
`temporalapp/task_delivery_git.go` — Group F), `git_grace_cleanup.go`
(importer to clean: `cmd/temporal-worker/main.go` `NewGitGraceCleanup`).

**Group E — tool loop implementations (16 files).**
Precondition: gateway narrowed (4.1 last row) + all surfaces migrated (each
tool's product behavior is served runtime-side via SDK tools or Helpin's
command provider — LOCAL.md parity rows show docs/CRM/support tools already
command-backed).
`tools.go`, `tools_cmd.go`, `tools_crm.go`, `tools_docs.go`,
`tools_edit_patch.go`, `tools_fs.go`, `tools_git.go`, `tools_github.go`,
`tools_internal_command.go`, `tools_planner.go`, `tools_release.go`,
`tools_security.go`, `tools_skills.go`, `tools_teampulse.go`, `tools_web.go`,
`tools_workspace.go`. None have importers outside worker/temporalapp (checked;
the leaked artifact/interaction helpers live in the 4.1 files, not here).

**Group F — worker files consumed only by the temporalapp agent machinery (2 files).**
Precondition: section 5.2 ships.
`prompt.go` (`BuildUserPrompt` — temporalapp), `transcript_summarizer.go`
(`BuildTranscriptSummaryCheckpoint`, `LatestTranscriptSummaryCheckpoint`,
`TranscriptSummaryArtifactType` — temporalapp).

Tally: **14 RELOCATE + 46 DELETE = 60 files** — the whole package. After
Groups A–F plus the relocations, `server/internal/worker/` is `rm -rf`-able
and both `workerpkg` imports in `cmd/*/main.go` disappear.

---

## 5. Temporalapp: MIGRATE-THEN-DELETE vs KEEP

### 5.1 KEEP (permanent, non-agent Temporal usage)

`engine.go`, `queues.go`, `client_options.go`, and the non-agent workflows:
`email_sync_workflow.go` + `email_sync_activities.go`,
`signal_detection_workflow.go`, `deal_management_workflow.go`,
`crm_summary_workflow.go`, `docs_asset_cleanup_workflow.go`,
`docs_embedding_workflow.go`, `pm_import_workflow.go`,
`pm_recurring_workflow.go`, `sprint_automation_workflow.go`,
`schedule_workflow.go`, `coverage_analysis_workflow.go`,
`coverage_gap_workflow.go`, `content_source_sync_workflow.go`. Temporal (the
product) stays; only agent-run execution leaves.

### 5.2 MIGRATE-THEN-DELETE — the agent-run machinery (24 files)

Deleted only after **planners + repository + codex/opencode + any remaining
surfaces** have all flipped (i.e., `AgentRunWorkflow` has no callers) and all
in-flight local runs have drained:

`workflow.go` (`AgentRunWorkflow`), `activities.go`,
`execution_contracts.go`, `execution_context_loading.go`,
`run_execution_context.go`, `initial_instruction_assembly.go`,
`task_execution_instructions.go`, `flow_output_instructions.go`,
`legacy_planner_fallback_instructions.go`, `repair_instructions.go`,
`planner_context_assembly.go`, `planner_phase_guidance.go`,
`planning_code_context.go`, `planning_domain_documents.go`,
`approved_preview_application.go`, `completion_interaction_policy.go`,
`run_interaction_persistence.go`, `run_message_persistence.go`,
`coding_session_persistence.go`, `failed_runtime_salvage.go`,
`task_delivery_git.go`, `durable_run_facts.go`,
`agent_contract_observability.go`, and `command_bar_workflow.go` (falls with
the command_agent surface specifically).

Planner caveat: per LOCAL.md, planner context assembly / phase guidance /
approved-preview application must be **reproduced runtime-side (or replaced by
the command-driven planner contract)** before their temporalapp files can go —
this is migration work, not deletion work; hence "migrate-then-delete."

---

## 6. Dependency-removal bonus (go.mod)

Verified against importer greps across `internal/` and `cmd/`:

| Dependency | Removable when | Evidence |
| --- | --- | --- |
| `github.com/cloudwego/eino-ext/components/model/agenticopenai` | Group A ships | imported only by `internal/worker/eino_exec.go` |
| `github.com/cloudwego/eino-ext/libs/acl/openai` | Group A ships | worker-only |
| `github.com/pelletier/go-toml/v2` | Group B ships | imported only by `internal/worker/codex.go` (+ its test) |
| `github.com/cloudwego/eino` (core) | Group A **+** migrating `internal/service/support_task_draft_eino.go` off eino | second consumer: the support task-draft LLM in service — not agent-run machinery; port it to `internal/llm` to unlock removal |
| `github.com/cloudwego/eino-ext/components/model/claude` | same as eino core | same two consumers |
| Indirect prunes (fall out of `go mod tidy` after the above) | — | `github.com/openai/openai-go/v3`, `github.com/meguminnnnnnnnn/go-openai` (via agenticopenai/acl), `github.com/anthropics/anthropic-sdk-go` (via eino-ext/claude) |

**Not removable:** `go.temporal.io/sdk` / `go.temporal.io/api` (non-agent
workflows keep them), `golang.org/x/net` (crawler, docsimport, email inbound
HTML, link previews all use `x/net/html`), `github.com/google/uuid`
(ubiquitous).

---

## 7. Ordering and estimated sequence

1. **Now — freeze.** Section 1 in force. Announce in the migration channel;
   reviewers start applying the critical-fix test.
2. **Now → surfaces graduate.** Each surface climbs the ladder (section 2)
   independently: repository surface enters when its parity evaluation lands;
   planners enter after LOCAL.md blockers close; codex/opencode custom agents
   enter after runtime-side codex auth. Expect several overlapping 4–6-week
   bake windows.
3. **In parallel with bakes — relocate leaked helpers** (section 4.1). Pure
   mechanical moves + import rewrites; zero behavior change; safe under the
   freeze (category 4 critical fix). Do these early so DELETE groups are
   unblocked the day their surface graduates. Narrow the tool gateway's
   registry dependency in this phase.
4. **Delete executor groups** as preconditions clear — expected order given
   current blockers: C (opencode) and E-partial rarely block anything;
   realistically A+E land together after planners flip, B after codex auth
   moves, D after the repository surface bakes, F last with step 5.
5. **Delete temporalapp agent machinery** (section 5.2 + Group F) once
   `AgentRunWorkflow` has no callers and local runs have drained. Remove the
   `workerpkg` wiring from `cmd/temporal-worker/main.go` and the local-run
   branches from `internal/service/agent.go`; the delegation predicate
   collapses to "always delegate" and the flag becomes a kill switch only.
6. **Dependency prune.** Apply section 6, `go mod tidy`, build, vet.
7. **Final state.** Helpin's entire agent execution footprint is host-side:
   - launch predicate + runtime client (`internal/service/agent.go`,
     `agent_runtime_client.go`) — launch/cancel/resume over the runtime API;
   - projection consumer (`agent_runtime_projection.go`) — NATS →
     runs/messages/artifacts/interactions/usage/billing;
   - terminal finalizers (`agent_runtime_finalizers.go`) — support draft send,
     repository delivery, planning output, completed-rules, agent status;
   - host adapters — target context + skills (`agent_runtime_host.go`), tool
     gateway (`agent_tool_gateway.go`), internal-command providers;
   - neutral helper packages from section 4.1 (skills, tool names, artifact
     extraction, contracts);
   - `server/internal/worker/` **does not exist**, and `temporalapp` contains
     only non-agent product workflows.
