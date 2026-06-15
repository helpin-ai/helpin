# Coding Agent Runtime Flow

This document explains the current repository-execution flow for coding agents, with emphasis on:

- Forge / Code Builder runs
- `codex` vs `opencode` runtime behavior
- repository checkout and branch preparation
- base-branch sync before execution
- commit / push / PR ownership
- recovery and failure paths

For the broader generic agent executor model, see `docs/AGENTS_AND_AUTOMATION.md`.
This document is specifically about repository-backed coding delivery.

Use this doc when changing:

- `server/internal/temporalapp/activities.go`
- `server/internal/worker/codex.go`
- `server/internal/worker/opencode.go`
- Forge / Code Builder presets

## Scope

This document covers coding-agent runs that operate on a repository-backed target such as a task with a delivery target.

It does not try to explain:

- generic planner-only runs
- support-agent reply drafting
- non-repository custom-agent flows

## Main components

The coding-agent runtime flow spans four layers:

1. Product / configuration layer
   - agent preset
   - runtime kind
   - task delivery target
2. Temporal orchestration layer
   - workspace prep
   - branch selection
   - branch sync
   - backend delivery steps
3. Runtime layer
   - Codex app-server runtime
   - OpenCode runtime
4. Repository delivery layer
   - local commit
   - backend push
   - PR metadata / task git link updates

Key files:

- `server/internal/temporalapp/activities.go`
- `server/internal/worker/codex.go`
- `server/internal/worker/codex_session_host.go`
- `server/internal/worker/codex_command_guards.go`
- `server/internal/worker/git_persistence.go`
- `server/internal/worker/opencode.go`
- `server/internal/service/agent_presets.go`
- `server/internal/service/agent_system_prompts.go`

## Runtime kinds

There are three important runtime families in the backend:

- `native_sdk`
  - Eino/model-loop backend runtime
  - used for generic agents where the product chooses the native backend
  - can consume the same Helpin MCP product tools and planner contracts as Codex
- `codex`
  - Codex CLI / app-server runtime
  - used by Forge / Code Builder system agents
  - current product direction for repository coding runs
- `opencode`
  - older coding runtime still supported in code
  - still valid for some legacy or custom agents

Current Code Builder default:

- preset family: `code_builder`
- default preset version: `code_builder_local_commit_delivery`
- runtime kind: `codex`

Runtime kind selects the backend adapter, not a separate product behavior path.
System agents, custom agents, and one-shot command agents still execute as
normal `agent_run` records. Planner/review/support behavior is expressed
through prompts, skills, allowed tools, targets, and artifact contracts, not a
separate native planner controller.

Helpin product and interaction tools are model-facing through the run-scoped
Helpin MCP bridge using names such as `mcp__helpin__update_plan` and
`mcp__helpin__request_user_input`. Backend policy and persistence normalize
those calls back to canonical bare aliases.

## High-level lifecycle

For a repository-backed Code Builder run, the lifecycle is:

1. Resolve target and delivery target.
2. Prepare local workspace clone.
3. Checkout the effective working ref.
4. Sync base branch into the working branch before execution.
5. Start the runtime.
6. Persist runtime changes.
7. Push / record delivery state.
8. Persist assistant message, artifacts, and run state.

In simplified form:

```text
start run
  -> resolve task delivery target
  -> prepare workspace
  -> checkout working branch
  -> sync base into working
  -> run Codex / OpenCode
  -> persist local commit or pushed state
  -> backend delivery bookkeeping
  -> finalize run
```

ASCII view:

```text
                  +----------------------+
                  | start agent_run      |
                  +----------+-----------+
                             |
                             v
                  +----------------------+
                  | resolve delivery     |
                  | base + working refs  |
                  +----------+-----------+
                             |
                             v
                  +----------------------+
                  | prepare workspace    |
                  | clone / reuse repo   |
                  +----------+-----------+
                             |
                             v
                  +----------------------+
                  | checkout working ref |
                  +----------+-----------+
                             |
                             v
                  +----------------------+
                  | sync base into work  |
                  +----------+-----------+
                             |
               +-------------+--------------+
               |                            |
               v                            v
     +--------------------+      +----------------------+
     | runtime executes   |      | fail / recover first |
     | codex or opencode  |      | depending on state   |
     +----------+---------+      +----------------------+
                |
                v
     +---------------------------+
     | persist changes + deliver |
     +-------------+-------------+
                   |
                   v
     +---------------------------+
     | artifacts + run finalize  |
     +---------------------------+
```

## Delivery target resolution

For task-backed repo runs, Temporal resolves a `task_delivery_target`.

The effective values come from:

- the saved delivery target
- any explicit run-time branch overrides
- workspace/team repo defaults
- repository default branch

Important outcomes:

- `BaseBranch`
  - usually the team default base branch or repository default branch
- `WorkingBranch`
  - either an explicitly stored branch or a generated task branch

If the run requires a repo and a working branch is known, Temporal ensures the remote branch exists before execution.

## Checkout behavior

Checkout logic is:

1. If remote `working_branch` exists:
   - fetch explicit remote-tracking ref for that branch
   - check out local working branch from `origin/<working_branch>`
2. Otherwise:
   - fetch `origin/<base_branch>`
   - create local working branch from base
3. If needed, bootstrap the remote working branch from base

This means an existing remote working branch wins over base branch for checkout.

ASCII checkout path:

```text
                   +------------------------------+
                   | do we already have a remote  |
                   | working branch?              |
                   +---------------+--------------+
                                   |
                    +--------------+--------------+
                    |                             |
                  yes                            no
                    |                             |
                    v                             v
      +-----------------------------+   +-----------------------------+
      | fetch origin/<working>      |   | fetch origin/<base>         |
      | checkout local working from |   | create local working from   |
      | origin/<working>            |   | origin/<base>               |
      +-----------------------------+   +-----------------------------+
                    |                             |
                    +--------------+--------------+
                                   |
                                   v
                      +--------------------------+
                      | continue into base sync  |
                      +--------------------------+
```

## Base sync before execution

After checkout, Temporal always evaluates whether the configured base branch needs to be brought into the working branch.

This is a backend-owned repository-preparation step, not a model decision.

Normal path:

- fetch `origin/<base_branch>`
- if base is already contained in working branch:
  - no-op
- otherwise:
  - merge `origin/<base_branch>` into the checked-out working branch

Why it exists:

- Codex should see the actual repo state it needs to edit
- branch freshness is execution-environment state, not prompt-only context

ASCII base-sync decision flow:

```text
                +---------------------------------+
                | base_branch and working_branch  |
                | both present?                   |
                +----------------+----------------+
                                 |
                     +-----------+-----------+
                     |                       |
                    no                      yes
                     |                       |
                     v                       v
             +---------------+    +--------------------------+
             | no-op         |    | same branch name?        |
             +---------------+    +-------------+------------+
                                               |
                                 +-------------+-------------+
                                 |                           |
                                yes                          no
                                 |                           |
                                 v                           v
                        +----------------+       +-------------------------+
                        | no-op          |       | fetch origin/<base>     |
                        | same_branch    |       | check merge relation    |
                        +----------------+       +------------+------------+
                                                             |
                                         +-------------------+-------------------+
                                         |                                       |
                                  already contains base                    needs sync
                                         |                                       |
                                         v                                       v
                               +--------------------+               +----------------------+
                               | no-op              |               | try merge base into  |
                               | up_to_date         |               | working              |
                               +--------------------+               +----------+-----------+
                                                                                 |
                                              +----------------------------------+----------------------------------+
                                              |                                  |                                  |
                                         clean merge                      merge conflicts                    unrelated history
                                              |                                  |                                  |
                                              v                                  v                                  v
                                   +--------------------+      +----------------------------+      +-----------------------------+
                                   | status=merged      |      | Codex: continue conflicted |      | backup old tip             |
                                   | continue           |      | Non-Codex: fail           |      | rebuild working from base  |
                                   +--------------------+      +----------------------------+      | or fail if active PR       |
                                                                                                    +-----------------------------+
```

## Branch sync outcomes

`branchSync.Status` can fall into several states:

- `not_applicable`
  - no base/working branch pair to sync
- `same_branch`
  - base branch and working branch are the same
- `up_to_date`
  - working already contains base
- `merged`
  - base merged cleanly into working before runtime execution
- `conflicted`
  - merge produced real conflicts
- `recreated_from_base`
  - working branch had unrelated history and was rebuilt from base
- `unrelated_history`
  - working branch had unrelated history and could not be safely auto-rewritten

### Shared history, clean merge

Behavior:

- merge base into working
- continue into runtime

### Shared history, merge conflicts

Behavior:

- leave repo in merge-conflict state
- set `branchSync.Status = "conflicted"`
- if runtime is Codex:
  - continue
  - Codex is instructed to resolve conflicts before further implementation
- if runtime is not Codex:
  - fail run

### Unrelated histories

This means the checked-out working branch and the configured base branch do not share a merge base.

This can happen if:

- the branch was created from the wrong repository history
- the repo was rewritten
- stale branch metadata points at an unrelated branch

Plain `git merge origin/<base>` is not valid in this state.

Current behavior for system-managed task branches without an active PR:

1. Create a backup branch from the old unrelated tip.
2. Push that backup branch to origin.
3. Recreate the working branch from `origin/<base_branch>`.
4. Rewrite the remote working branch using explicit `--force-with-lease=<old_sha>`.
5. Continue the run on the repaired working branch.

Backup branch naming:

- `helpin-backup/unrelated-history/<timestamp>-<sha>`

Important property:

- the old divergent history is preserved remotely on the backup branch
- the canonical task branch is repaired to descend from the configured base branch

Fail-closed exception:

- if the delivery target already has an active PR on that working branch, Forge does not auto-rewrite it
- in that case the run fails with an explicit error

ASCII unrelated-history recovery path:

```text
              +--------------------------------------+
              | working branch has no merge-base     |
              | with configured base branch          |
              +-------------------+------------------+
                                  |
                    +-------------+-------------+
                    |                           |
                 active PR                    no active PR
                    |                           |
                    v                           v
      +-----------------------------+   +----------------------------------+
      | fail closed                 |   | create backup branch from old tip|
      | do not rewrite branch       |   | push backup branch               |
      +-----------------------------+   | recreate working from base       |
                                        | push with explicit lease         |
                                        | continue run                     |
                                        +----------------------------------+
```

## Codex runtime behavior

Forge / Code Builder now runs on `codex`.

### What Codex is responsible for

Codex is responsible for:

- inspecting the repository
- editing files
- resolving merge conflicts when branch sync leaves the repo conflicted
- running validation
- producing a local commit

### What Codex is not responsible for

Codex is not supposed to:

- push the branch
- open a PR
- manage backend delivery state

That is stated in two places:

- preset / system prompt
- runtime-specific Codex instructions

### Hard guardrails

Prompt instructions are not enough, so Codex sessions also install command guards.

Current guards:

- block `git push`
- block `gh pr ...`

This is done by prepending wrapper scripts to `PATH` inside the Codex session.

### Local commit only

At post-run persistence time, Codex:

- stages changes
- validates the staged result
- creates a local commit
- records `LocalGitCommit` metadata on the execution context

It does not push from inside the runtime.

### Merge-resolution sanity checks

If branch sync entered the `conflicted` state and Codex is expected to resolve it, finalization validates that the merge is truly resolved before creating the local commit.

Current checks:

- no unmerged paths remain
- no staged conflict markers remain

If those checks fail:

- no commit is created
- no backend push happens
- the run fails closed

## Backend-owned Codex delivery

After successful Codex execution:

1. Temporal reads `execCtx.LocalGitCommit`.
2. Temporal pushes the branch from backend orchestration.
3. Temporal records branch / commit metadata.
4. Temporal saves the `git_delivery_result` artifact.

This makes final remote delivery deterministic and retryable at the orchestration layer.

Why this split exists:

- model writes code
- backend owns repository state transitions

That boundary is intentional.

ASCII delivery ownership:

```text
           CODEX FLOW                              OPENCODE FLOW

   edit files in runtime                    edit files in runtime
   -> stage changes                         -> stage changes
   -> validate merge state                  -> commit
   -> local commit                          -> push from runtime
   -> return LocalGitCommit                 -> record pushed branch
   -> backend push
   -> backend record delivery
```

## OpenCode runtime behavior

`opencode` still exists and behaves differently today.

OpenCode currently:

- edits files
- stages changes
- creates a commit
- pushes the branch from inside the runtime

That means:

- Codex delivery is backend-owned after local commit
- OpenCode delivery is still runtime-owned

This asymmetry is acceptable for legacy support, but Forge / Code Builder system agents are now migrated toward Codex so that the modern default path uses backend-owned delivery.

## Forge preset and migration behavior

The Code Builder / Forge preset now assumes:

- runtime kind `codex`
- local commit only
- backend-managed delivery

Existing built-in Forge agents are migrated to:

- preset version `code_builder_local_commit_delivery`
- runtime kind `codex`

Service-side normalization also realigns legacy system Forge agents to the preset runtime, so stale `opencode` runtime values do not survive on built-in Code Builder agents.

## Why branch sync is backend-owned

The branch sync step is intentionally not left to the model because it is:

- environment preparation
- deterministic repo orchestration
- safety-sensitive

The model should operate on the repo state it is given, not be asked to infer and repair repo freshness from prompt text alone.

## Failure modes and intended responses

### Clean merge failure due to real conflicts

Expected behavior:

- Codex receives conflicted repo
- Codex resolves conflicts
- finalization verifies the result before committing

### Unrelated history with no active PR

Expected behavior:

- backup old tip
- recreate task branch from base
- continue run

### Unrelated history with active PR

Expected behavior:

- fail closed
- do not rewrite branch under an open PR

### Remote rewrite lease fails

Expected behavior:

- fail closed
- this means the remote branch changed after the old SHA was observed
- a later run can re-evaluate with fresh branch state

### Codex tries to push directly

Expected behavior:

- command guard blocks it
- backend remains the only delivery path for Codex runs

## Practical mental model

The easiest way to reason about the current system is:

- Temporal owns repo preparation and repo delivery
- Codex owns code changes and merge resolution
- Forge is a Codex-backed system agent with backend-owned delivery
- OpenCode is still supported but is a legacy-style runtime with in-runtime push behavior

## Future cleanup opportunities

Likely follow-up improvements:

- surface unrelated-history backup branch names in run facts and UI
- surface branch-sync status more explicitly in run detail views
- converge OpenCode onto the same local-commit / backend-push model if legacy support remains necessary
- add operator-facing remediation guidance when active PR prevents unrelated-history repair
