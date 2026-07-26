# Local Agent Runtime Setup

This runbook is for testing delegated Helpin runs against a local `agent-runtime`.
It is intentionally local-only; staging and production app config are managed as
runtime secrets and must merge Helpin with any existing app entries. For the
staging rollout (Doppler secrets, NATS sharing, flag order, rollback), see
`docs/AGENT_RUNTIME_STAGING.md`.

## Helpin Env

Use `127.0.0.1` for the runtime URL. On some machines Go resolves `localhost`
to IPv6 first while the runtime listens on IPv4.

```bash
AGENT_RUNTIME_BASE_URL=http://127.0.0.1:8090
AGENT_RUNTIME_SERVICE_TOKEN=dev-token
AGENT_RUNTIME_APP_ID=helpin
AGENT_RUNTIME_LAUNCH_ENABLED=true
```

`AGENT_RUNTIME_LAUNCH_ENABLED=true` sends every agent run with an agent and
target to Agent Runtime. Agent Runtime is the only execution path for native,
Codex, and OpenCode agents across every supported preset and target.

If the flag is false or unset, Helpin fails new run starts loudly. There is no
in-process Temporal fallback. Keep the flag enabled anywhere agent execution
is expected to work.

## Runtime App Config

Create a local app config whose callback URLs point at the running Helpin API
port. If Helpin is on `:8080`, use:

```json
{
  "apps": [
    {
      "app_id": "helpin",
      "context_endpoint": "http://127.0.0.1:8080/api/internal/agent-runtime/target-context",
      "context_token": "dev-token",
      "workspace_provider": {
        "transport": "repository",
        "base_url": "http://127.0.0.1:8080/api/internal/agent-runtime/workspace",
        "token": "dev-token",
        "root_dir": ".local/workspaces"
      },
      "command_provider": {
        "transport": "http",
        "base_url": "http://127.0.0.1:8080/api/internal/agent-runtime/commands",
        "token": "dev-token"
      },
      "skill_provider": {
        "transport": "http",
        "base_url": "http://127.0.0.1:8080/api/internal/agent-runtime/skills",
        "package_base_url": "http://127.0.0.1:8080/api/internal/agent-runtime/skill-packages",
        "token": "dev-token"
      }
    }
  ]
}
```

If Helpin is running on another port, update all three callback URLs. The runtime
must send the same bearer token as `AGENT_RUNTIME_SERVICE_TOKEN`.

## Runtime Env

Example `agent-runtime` local env:

```bash
AGENT_RUNTIME_ADDR=:8090
AGENT_RUNTIME_STORE_DRIVER=sqlite
AGENT_RUNTIME_SQLITE_DSN=.local/helpin-agent-runtime.sqlite3
AGENT_RUNTIME_SERVICE_TOKEN=dev-token
AGENT_RUNTIME_APP_CONFIG=@/tmp/helpin-app-config.json
TEMPORAL_ADDRESS=localhost:7233
TEMPORAL_NAMESPACE=default
TEMPORAL_TASK_QUEUE_PREFIX=helpin-
AGENT_RUNTIME_EVENT_SINK=log,nats
AGENT_RUNTIME_NATS_URL=nats://127.0.0.1:4222
CODEX_APP_SERVER=true
CODEX_PATH=codex
```

`CODEX_APP_SERVER=true` is required when using the normal Codex CLI binary. Raw
command mode starts the interactive TUI and fails in non-TTY worker execution.

Start the runtime from the `agent-runtime` checkout:

```bash
set -a
source .env.helpin-local
set +a
GOCACHE=/private/tmp/agent-runtime-go-cache go run ./cmd/agent-runtime
```

Start a local NATS server with JetStream before running the Helpin worker and
runtime. Live projection uses the `AGENT_RUNTIME_EVENTS` stream; the
reconciliation sweep is only a backstop.

```bash
nats-server -js
```

## Smoke Check

1. Start NATS with JetStream.
2. Start Helpin API and the Temporal worker with the Helpin env above.
3. Start `agent-runtime` with the app config above.
4. Launch a Mira workspace run.
5. Confirm the Helpin `agent_runs` row has:
   - `external_runtime = 'agent-runtime'`
   - `external_runtime_id` set
   - no Helpin `workflow_id`
6. Confirm projection updates the row from runtime events. The worker creates
   the `AGENT_RUNTIME_EVENTS` stream if it is missing.

For local Codex auth flows, a successful unauthenticated smoke can pause with
`pause_reason = authentication`; that still validates launch delegation,
runtime execution, and Helpin projection.

## Marketer Workspace Parity Row

Current pilot surface: marketer-preset workspace runs only.

| Capability | Status | Notes |
| --- | --- | --- |
| Launch routing | Green | `AGENT_RUNTIME_LAUNCH_ENABLED=true` delegates marketer-preset workspace runs to Agent Runtime and stamps `external_runtime` / `external_runtime_id` at launch. The predicate is preset-based, not name-based. |
| Runtime orchestration | Green | Helpin requests `execution_mode=durable`; Agent Runtime owns the Temporal workflow. Helpin does not create a local run workflow for delegated runs. |
| Projection | Green | NATS projection updates lifecycle, usage, artifacts, messages, and interactions back into Helpin. |
| Codex auth start/cancel | Green | Delegated Codex auth calls Agent Runtime's `/v1/runs/{id}/codex-auth/device-code/*` endpoints so auth is promoted into the runtime Codex home. |
| Codex auth completion | Yellow | Runtime emits `codex_auth.state_changed`; Helpin mirrors the state and forwards `auth_completed` when the runtime reports `connected`. Local smoke requires the active Helpin worker to be running this projection code. |
| Handoff | Deferred | `HandoffRun` still follows the local Temporal path. Attach it to Agent Runtime when the first delegated surface needs cross-agent/user handoff semantics. |
| Post-auth assistant turn | Blocked without browser login | Requires completing the OpenAI device-code login for the runtime-issued code. |
| Product finalizers | Not applicable | Workspace Mira run has no task/support/repository finalizer. |
| Repository delivery | Not applicable | Workspace Mira run does not prepare or push a repository workspace. |

## Delegated Surface Parity Rows

One row per surface delegated beyond the marketer pilot. Columns map to the
launch pipeline stages; evidence is the code path that exists today.

| Surface | Launch routing | Target context | Tools | Write path | Finalizers | Transcript |
| --- | --- | --- | --- | --- | --- | --- |
| documentation_agent / workspace | Green — preset→target map delegates; `startTargetRunWithOptions` workspace case stamps `workspace_id` metadata via `runtimeStartRunRequest`. | Green — host `workspace` case returns workspace context data. | Green — docs read tools cataloged; docs/publish product tools are command-backed via the commands provider endpoint. | Amber — document writes (`write_document_content`, `publish_document_change_proposal`) are command-backed but not yet E2E-smoked from a delegated run. | Green — agent-idle + automation completed-rules finalizers fire on terminal projection. | Green — NATS projection mirrors messages/interactions into Helpin. |
| documentation_agent / document | Green — predicate delegates; `document` launch case validates `doc.WorkspaceID == workspaceID` before `createRun`, so `metadata.workspace_id` is always set. | Green — host `document` case resolves the doc and stamps `workspace_id`; type string `document` matches what Helpin stamps. | Green — same command-backed docs tool set as workspace runs. | Amber — document-target write flows not yet E2E-smoked against a live runtime. | Green — agent-idle + completed-rules; no doc-specific finalizer exists (none needed yet). | Green — same projection path as marketer pilot. |
| crm_operator / workspace | Green — preset→target map delegates workspace runs. | Green — host `workspace` case. | Green — CRM product tools (`add_deal_note`, `update_deal_stage`, `ensure_crm_contact_company`, `enrich_crm_*`) are command-backed. | Amber — CRM command writes exist but no delegated-run smoke has exercised them yet. | Green — agent-idle + completed-rules; `crm.deal_review_actions` flow output is validated by the planning finalizer. | Green — same projection path. |
| crm_operator / crm_contact + crm_deal | Green — predicate delegates; `crm_contact` / `crm_deal` launch cases validate workspace ownership before `createRun`. | Green — host `crm_contact` / `crm_deal` cases; type strings match Helpin's stamps exactly. | Green — command-backed CRM tools with target-aware defaults (e.g. enrich uses run target ID). | Amber — same as above: command-backed, not E2E-smoked. | Green — agent-idle + completed-rules. | Green — same projection path. |
| crm_operator / crm_company | Amber — predicate delegates `crm_company`, but `startTargetRunWithOptions` has no `crm_company` launch case yet (`unsupported target type`), so no Helpin path can create such a run today; predicate is forward-ready only. | Green — host `crm_company` case already resolves company context. | Green — `enrich_crm_company` command supports `crm_company` targets. | Amber — unreachable until the launch case lands. | Green — generic finalizers would apply once reachable. | Green — projection is target-agnostic. |
| custom native_sdk agents / workspace + document + crm_* | Green — custom agents with no preset and `runtime_kind = native_sdk` delegate on these targets; codex/opencode custom agents intentionally stay local. | Green — same host target-context cases as preset runs. | Amber — tool surface is whatever the agent's `allowed_tools` grants; scanners/preview/github built-ins plus command-backed product tools exist, but per-agent tool grants are not preset-curated. | Amber — write commands available but no custom-agent delegated smoke yet. | Green — agent-idle + completed-rules are agent-agnostic. | Green — same projection path. |
| support_agent / support_conversation | Green — predicate delegates; every trigger path funnels through `createRun`'s delegation branch: manual (`RunConversationAgent`, `startTargetRunWithOptions` support case), inbox auto (`RunConversationAgentAuto`), and widget auto-run (`maybeAutoRunConversationAgent` → `conversationAgentRunner`). Conversation-specific input assembly (target + `conversation_id` + manual/`support.auto` trigger context) happens before `createRun`, so it is identical for delegated runs. | Green — host `support_conversation` case resolves the conversation; requires `workspace_id` metadata, which `runtimeStartRunRequest` stamps into both run metadata and target metadata. | Green — support tools are command-backed with aliases matching the native names: `support.list_conversation_messages` / `support.draft_reply` / `support.update_conversation_status`. `support.draft_reply` stages `output_summary.draft_reply` on the Helpin run row via the external-runtime-ID run lookup; the terminal summary merge preserves local-only keys. | Green — reply / approve / request_changes forward through `resumeAgentRuntimeRunWithIntent` (unit-tested for support runs in `agent_runtime_signal_test.go`); approval pauses mirror as `human_approval` + pending, and `finalizeSupportDraft` skips pending runs so an unapproved draft is never sent. | Green — `finalizeSupportDraft` (terminal completed transition) is a faithful port of `finalizeSupportConversationRun`: same pending-gate, run-ID message idempotency, `sent_message_id` write-back, websocket publish, and visitor refresh. Turn policy: autonomous (preset default) maps to `complete_on_finish` so the terminal transition fires; interactive-configured support agents map to `pause_after_assistant`. | Green — same projection path; interaction mirroring covers the approval checkpoint on the draft. |
| code_builder / task (+ story, normalized to task at launch) | Green — predicate delegates; task launch case resolves the delivery target (`ResolveTaskDeliveryTargetForRun`, repo required) before `createRun`, which stamps `repository_id` / `repo_full_name` / `base_branch` / `working_branch` / `delivery_target_id` on the run row for delegated runs exactly as for Temporal runs. | Amber — host `task` case returns the task row (name, description, type, priority, state, labels) but adapters inject only the shallow summary ("Task N: name") into prompts; bridged at launch by `buildDelegatedTaskLaunchContext`, which stamps operator notes, task name/description, parent-epic background, branch values, and truncated plan-doc content into `additional_context` → runtime `Instructions`. Known gaps vs Temporal: checklist items and non-plan linked docs are not stamped (no repos wired for them on the launch path). | Amber — repo workspace tools (`read_file`/`write_file`/`run_command`/git tools) exist runtime-side and the repository clone is prepared via the host repository-spec endpoint with the work branch checked out (`workspace.mode=repository` is injected into the runtime agent record at delegation because Helpin's `AgentExecutionConfig` does not model it). `add_task_comment` / `update_task_state` are command-backed with native aliases; `list_task_checklist` and `open_pr` have no runtime counterpart (PR opening is deliberately backend-managed). | Green — runtime `push_branch` finalize policy commits/pushes and reports `{"repository": {pushed, branch, commit}}` in the output summary. | Green — `finalizeRepositoryDelivery` → `FinalizeDelegatedRunDelivery` ensures the PR/MR off the pushed branch (run fields with delivery-target fallback), records the delivery target, git link, activity, and websocket events; `pr_failed` bookkeeping on provider errors. Codex auth pause/mirroring is the same machinery as the marketer pilot. | Green — same NATS projection; summary merge preserves the local finalizer markers. E2E smoke vs a live runtime: **ops-pending**. |
| code_builder / repository | Green — predicate delegates; repository launch case stamps `repository_id` / `repo_full_name` / `base_branch` (default-branch fallback) on the run. | Green-by-parity — host has no `repository` target-context case (generic fallback), but the Temporal path also assembles no repository-target instructions (`additional_context` passthrough), so delegated ≥ Temporal here; the clone spec resolver has a first-class `repository` case. | Amber — same tool notes as the task row. | Green — same push_branch write-back. | Green — `FinalizeDelegatedRunDelivery` resolves the repository directly from the run/target for repository targets. | Green — same projection. E2E smoke: **ops-pending**. |
| review_agent / task + repository | Green — same launch paths as code_builder; interactive default invocation maps to `pause_after_assistant` so the review loop stays interactive. | Amber — same launch-context bridge; the runtime `review_agent` skill expects "repository base and working branch context" for task reviews, which the bridge stamps. | Amber — review tools (`read_file`, `edit_file`, `apply_patch`, `run_command`, `request_review_checkpoint`, `request_user_input`) exist runtime-side; `list_task_checklist` gap as above. | Green — `review_checkpoint` / `request_user_input` interactions mirror through the projection; approve / request-changes / reply forward via `resumeAgentRuntimeRunWithIntent` (generic, unit-tested). | Green — repository delivery finalizer fires only on the terminal completed transition, so review-only runs with nothing pushed skip PR creation and fix-applying runs get their PR. | Green — same projection. E2E smoke: **ops-pending**. |
| support_agent / support_coverage_gap | Deliberately local — `support_coverage_gap` is not an allowed support-agent target (runtime profile allows `support_conversation` only), so there is no support-agent path to flip. Coverage-gap runs belong to the documentation-agent surface (`support_gap_to_docs`), which currently delegates workspace/document only; additionally the host `ResolveTargetContext` has no `support_coverage_gap` case (falls through to the generic default), so flipping it belongs to the docs-agent slice together with a dedicated target-context resolver. | — | — | — | — | — |

## Coding preset parity (code_builder / review_agent — flipped for task + repository)

Flipped combos: `code_builder × {task, repository}`, `review_agent × {task,
repository}` (story launches normalize to task before the predicate). Three
launch-path bridges made the flip honest instead of lobotomized:

1. **Repository workspace mode** — Agent Runtime prepares a repository clone
   only when the runtime agent record's `execution_config` carries
   `workspace.mode = "repository"` (`engine.ensureWorkspace` →
   `workspace.WorkspaceMode`). Helpin's `AgentExecutionConfig` does not model
   that field, so `runtimeAgentFromHelpinAgent` injects it for repo-requiring
   presets (`agentRequiresRepositoryWorkspace`, keyed strictly off the preset
   runtime profile's `RequiresRepo`; custom agents never match). The host-side
   clone spec (`GitService.ResolveAgentRuntimeRepositorySpec`) already handled
   task/story/repository/epic targets with delivery-target and branch
   resolution plus `push_branch` finalize policy.
2. **Launch-time task context** — Temporal assembles task instructions at
   execution time (`worker.BuildUserPrompt` + `buildTaskExecutionInstructions`:
   operator notes, task description, epic background, branch values, plan-doc
   content, linked docs, checklist). Runtime adapters inject only the shallow
   target summary ("Task N: name"), and the coding presets' tool grants include
   no PM read tools to self-gather the rest. `buildDelegatedTaskLaunchContext`
   stamps the equivalent context into `additional_context` at launch for
   delegated task runs (best-effort; enrichment failures log and skip).
   Remaining gaps vs Temporal: checklist items and non-plan linked docs.
3. **Branch stamping** — no bridge needed: `createRun` stamps
   `repository_id` / `repo_full_name` / `base_branch` / `working_branch` from
   the resolved delivery target (task) or repository row (repository) *before*
   the delegation branch, so delegated runs get the same run-row fields the
   repository-delivery finalizer reads (with delivery-target fallback).

Deliberately not flipped:

- `code_builder / review_agent × workspace` — the host repository-spec
  resolver has no `workspace` case, and the runtime agent record now demands a
  repository workspace, so `PrepareWorkspace` would fail. Temporal keeps this
  exploratory surface.
- **custom agents × task/repository** — custom agents carry no
  requires-repo signal, so the workspace-mode injection (preset-keyed) does not
  apply; without it a delegated custom coding run would execute with no
  workspace at all. Revisit when custom-agent execution profiles express
  workspace needs.
- Flow-output runs on task targets (`pm.task_completion_followups`) delegate
  with the rest of the surface; like the already-delegated CRM flow outputs
  they rely on the delegated planning finalizer's validate-and-log path rather
  than Temporal's fail-on-invalid retry.

E2E smoke against a live Agent Runtime (clone → edit → commit → push → PR) is
**ops-pending**; unit coverage lives in
`agent_runtime_coding_delegation_test.go` (launch chain, run-row stamping,
instruction stamping, workspace-mode injection, interactive review turn
policy) and the updated predicate matrix in
`agent_runtime_delegation_test.go`.

## Historical planner parity blockers (obsolete after hard cutover)

This section records the blockers that existed before Helpin retired local
agent execution. It does not describe current routing: planner runs now use
Agent Runtime, and the old Temporal planner implementation has been removed
from Helpin. The pre-cutover blockers were:

1. **Execution-time context assembly** — `temporalapp/planner_context_assembly.go`
   (+ `planning_domain_documents.go`, `planning_code_context.go`) builds the
   initial instructions at execution time: epic/task summaries, approved spec
   versions, spec-document drafts, linked docs/tickets, task comments,
   existing epic tasks, operator notes, and repository code context. For
   delegated runs `runtimeStartRunRequest.Instructions` only carries
   `input.AdditionalContext` (raw user text from launch), and the host target
   context returns shallow epic/task rows. A delegated planner would start
   with essentially no planning context; the read tools/commands to
   self-gather an equivalent context set are not yet curated for planners.
2. **Phase selection metadata** — the runtime can select planner phase skills
   (`prd_authorship`/`task_decomposition`/`task_planner_context` +
   `approval_protocol`, keyed off `preset_key` + `planning_stage` in
   `skills.SelectNativeActiveSkills`), but Helpin stamps neither `preset_key`
   (agent `execution_config` passthrough does not include it) nor
   `planning_stage` into run metadata/trigger, so phase selection never
   activates. Even with the metadata, the runtime has no equivalent of
   `epicPlannerPhaseName` derived-state facts (has-spec / has-tasks
   derivation in `planner_phase_guidance.go`).
3. **Approved-preview application** — `temporalapp/approved_preview_application.go`
   (`applyApprovedInteractivePreview`: create_tasks, persist_prd,
   persist_task_doc) is explicitly deferred from the delegated finalizer set
   (see `docs/plans/2026-07-02-delegated-run-finalizers.md`).
   `maybePersistApprovedInteractivePreview` on the write path only persists
   the approved-preview artifact; nothing applies it for delegated runs, so
   an approved delegated epic plan would never create tasks or persist the
   PRD. This is the hard blocker: a flipped planner would look functional and
   silently drop its output.
4. **Completion interaction policy** — `enforceCompletionInteractionPolicy` /
   `retryInvalidCompletionTurn` / `synthesizeCompletionInteractionFallback`
   and transcript planning-artifact capture are Temporal-execution machinery
   with no runtime counterpart yet.
5. **Fail-on-invalid flow output** — delegated flow-output runs cannot be
   failed by the host (run status is runtime-owned); the delegated planning
   finalizer only validates and logs. Acceptable for the currently delegated
   flow outputs, not for planners whose output *is* the product.

What already works (would carry over on flip): epic planning pointer
finalizer (`epic.last_planning_run_id`), flow-output summary validation,
generic agent-idle/automation finalizers, interaction mirroring, and
`pm.*` / `docs.*` commands (`pm.create_task_batch`, `pm.approve_epic_spec`,
`docs.ensure_spec_doc`, `docs.ensure_task_plan_doc`) that a future
command-driven planner contract could target instead of preview application.
