# PM Agent Tool Surface Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give Helpin agents a safe, executable PM tool surface for discovering and operating on tasks, epics, sprints, objectives, comments, checklists, labels, dependencies, and key results, including direct sprint/objective runs.

**Architecture:** Add typed metadata in `commandtools`, command executors in a focused PM internal-command file, and dependency setters that reuse the existing PM services. Extend generic agent targeting and target-context projection for sprint/objective entities, propagate the resolved authorization actor into service calls, and keep the frozen catalog synchronized through parity tests.

**Tech Stack:** Go 1.24, GORM test database, Chi service wiring, embedded JSON tool catalog, React 19/TypeScript, Vitest, Go test.

**Spec:** `docs/superpowers/specs/2026-08-04-pm-agent-tool-surface-design.md`

---

## File map

- Create `server/internal/service/internal_command_pm_tools.go`: PM operational command registration, target defaulting, parent-child validation, compact projections, and date parsing.
- Create `server/internal/service/internal_command_pm_tools_test.go`: database-backed red/green tests for discovery and PM entity tools.
- Modify `server/internal/service/internal_command_service.go`: PM service/repository dependencies, setters, and registration call.
- Modify `server/internal/commandtools/metadata.go`: canonical metadata and JSON schemas for all new aliases and extended task tools.
- Modify `server/internal/agentcontract/tool_catalog.json`: selectable tool entries and PM categories.
- Modify `server/internal/agentcontract/tool_catalog_test.go`: schema and catalog/executor contract assertions that do not require service wiring.
- Modify `server/internal/service/internal_command_service_test.go`: executor parity, target support, task schema, and authorization tests.
- Modify `server/internal/service/agent_runtime_host.go`: resolved actor propagation plus sprint/objective target context.
- Modify `server/internal/service/agent_runtime_host_test.go`: actor, target isolation, and target projection tests.
- Modify `server/internal/service/agent.go`: sprint/objective service dependencies and direct run branches.
- Modify `server/internal/service/agent_runtime_launch_context.go`: compact sprint/objective context helpers if shared by direct-run and host paths.
- Modify `server/internal/service/agent_runtime_host_test.go` and `server/internal/service/agent_policy_test.go`: supported-target and multi-team objective launch coverage.
- Modify `server/internal/service/agent_presets.go` and tests: exact preset alias expansion from the spec.
- Modify `server/cmd/api/main.go`: inject PM services/repositories into internal-command, agent, and runtime-host services.
- Modify `frontend/src/lib/pm-types/agents.ts`: add `sprint` and `objective` target types.
- Modify `frontend/src/pages/automation/Agents.tsx`: expose target choices and run-now support.
- Modify `frontend/src/pages/automation/__tests__/Agents.test.tsx`: target option coverage.

---

### Task 1: Preserve runtime actor/team roles and establish PM target types

**Files:**
- Modify: `server/internal/service/agent_runtime_host.go`
- Modify: `server/internal/service/agent_runtime_host_test.go`
- Modify: `frontend/src/lib/pm-types/agents.ts`
- Modify: `frontend/src/pages/automation/Agents.tsx`
- Test: `frontend/src/pages/automation/__tests__/Agents.test.tsx`

- [ ] **Step 1: Write failing runtime actor-propagation test**

Register a test-only internal command whose executor captures `authorization.GetActor(ctx)`. Execute it through `AgentRuntimeHostService.ExecuteCommand` with an external actor resolved by the authorization service. Assert the captured actor includes the correct workspace role and team role, not only flat IDs.

- [ ] **Step 2: Run the focused Go test and verify RED**

Run: `cd server && go test ./internal/service -run TestAgentRuntimeHostExecuteCommandPropagatesResolvedActor -count=1`

Expected: FAIL because `ExecuteCommand` currently passes the original context without `authorization.WithActor`.

- [ ] **Step 3: Implement actor propagation**

Change actor enrichment to return the resolved actor alongside any error:

```go
func (s *AgentRuntimeHostService) resolveCommandActor(
    ctx context.Context,
    meta *model.InternalCommandContext,
) (*authorization.Actor, error)
```

After populating `ActorRole` and `ActorTeamIDs`, wrap the command context with `authorization.WithActor(ctx, actor)` before calling `commandService.Execute`. Preserve current behavior for system-triggered runs without an actor.

- [ ] **Step 4: Verify the actor test is GREEN**

Run the command from Step 2. Expected: PASS.

- [ ] **Step 5: Write failing frontend target tests**

Assert the exported/visible custom-agent target options contain Sprints and Objectives and that both values are accepted by the target normalizer and run-now support set.

- [ ] **Step 6: Run frontend target tests and verify RED**

Run: `cd frontend && npm test -- --run src/pages/automation/__tests__/Agents.test.tsx`

Expected: FAIL because `AgentTargetType`, `CUSTOM_AGENT_TARGET_OPTIONS`, and `RUN_NOW_SUPPORTED_TARGETS` omit the new targets.

- [ ] **Step 7: Add target types and options**

Extend the union with `'sprint' | 'objective'`, add labeled options, and add both values to `RUN_NOW_SUPPORTED_TARGETS`. Do not change existing labels or target behavior.

- [ ] **Step 8: Re-run focused frontend tests**

Expected: PASS.

- [ ] **Step 9: Commit**

```bash
git add server/internal/service/agent_runtime_host.go server/internal/service/agent_runtime_host_test.go frontend/src/lib/pm-types/agents.ts frontend/src/pages/automation/Agents.tsx frontend/src/pages/automation/__tests__/Agents.test.tsx
git commit -m "feat: preserve actor context for PM agent runs"
```

---

### Task 2: Define PM tool metadata and enforce catalog/executor parity

**Files:**
- Modify: `server/internal/commandtools/metadata.go`
- Modify: `server/internal/agentcontract/tool_catalog.json`
- Modify: `server/internal/agentcontract/tool_catalog_test.go`
- Modify: `server/internal/service/internal_command_service_test.go`

- [ ] **Step 1: Write failing metadata tests**

Add table-driven assertions for these aliases and categories:

```text
Workspace: list_workspace_members
PM / Tasks: list_pm_labels, get_task, update_task,
  create_task_checklist_item, update_task_checklist_item, add_pm_comment
PM / Epics: list_epics, get_epic, create_epic, update_epic
PM / Sprints: list_sprints, get_sprint, list_sprint_tasks,
  create_sprint, update_sprint
PM / Objectives: list_objectives, get_objective, create_objective,
  update_objective, create_key_result, update_key_result
```

Also assert `create_task` exposes `sprint_id`, `severity`, `blocked`, `blocker`, and `checklist_items`; `list_tasks` exposes the filters in the spec; every list schema has a maximum bounded page/per-page or limit.

- [ ] **Step 2: Run metadata tests and verify RED**

Run: `cd server && go test ./internal/agentcontract ./internal/service -run 'TestPMToolCatalogContracts|TestPMToolCatalogExecutorParity' -count=1`

Expected: FAIL with missing aliases/schemas.

- [ ] **Step 3: Add canonical metadata and schemas**

Add one `RuntimeToolMetadata` entry per alias. Use `additionalProperties: false`, explicit required fields, `YYYY-MM-DD` descriptions for dates, enum constraints from model constants, `maxItems` for arrays, and `maximum: 100` for list sizes.

Task update excludes `team_id`, `workflow_id`, `state_id`, and `archived`; state changes remain in `update_task_state`. Association arrays document replacement semantics.

- [ ] **Step 4: Add catalog entries/categories**

Update the embedded JSON mechanically from the metadata definitions, preserving existing entries and order. Add ordered categories `PM / Epics`, `PM / Sprints`, and `PM / Objectives` after `PM / Tasks`. Keep `assign_task_agent` as a compatibility entry.

- [ ] **Step 5: Add parity test**

Build the internal command service with nil dependencies, collect exposed aliases by module/category, and compare them to the catalog. Assert unique aliases in both directions, including `list_workspace_members` and `list_workspace_teams`. Allow native runtime tools through an explicit non-command allowlist; do not blanket-ignore missing executors.

- [ ] **Step 6: Re-run focused tests**

Expected: metadata tests pass; parity may remain RED until executors are registered in Task 3, which is the intended next dependency.

- [ ] **Step 7: Commit metadata/catalog work**

```bash
git add server/internal/commandtools/metadata.go server/internal/agentcontract/tool_catalog.json server/internal/agentcontract/tool_catalog_test.go server/internal/service/internal_command_service_test.go
git commit -m "feat: define complete PM agent tool contracts"
```

---

### Task 3: Register PM dependencies, discovery, and task tools

**Files:**
- Create: `server/internal/service/internal_command_pm_tools.go`
- Create: `server/internal/service/internal_command_pm_tools_test.go`
- Modify: `server/internal/service/internal_command_service.go`
- Modify: `server/cmd/api/main.go`

- [ ] **Step 1: Write failing discovery execution tests**

Seed active/inactive workspace members, team memberships, labels, workflows, and tasks. Test:

- `workspace.list_members` returns active assignable members without email.
- `pm.list_labels` filters archived and team-incompatible labels.
- expanded `pm.list_tasks` forwards epic/sprint/workflow/state and pagination filters.
- team-scoped actors cannot see tasks outside their accessible team IDs.

- [ ] **Step 2: Verify discovery RED**

Run: `cd server && go test ./internal/service -run 'TestPMCommand(ListWorkspaceMembers|ListLabels|ListTasksFilters)' -count=1`

Expected: FAIL with unknown commands or missing filters.

- [ ] **Step 3: Add focused PM command registration**

Add fields for `workspaceRepo`, `epicService`, `sprintService`, `objectiveService`, and `checklistService`. Add setters instead of expanding the already-large constructor:

```go
func (s *InternalCommandService) SetPMOperationalServices(
    workspaceRepo *repository.WorkspaceRepository,
    epicService *PMEpicService,
    sprintService *PMSprintService,
    objectiveService *PMObjectiveService,
    checklistService *PMChecklistItemService,
)
```

Call `registerPMOperationalCommands()` from `registerDefaults`; executors may safely report an unavailable dependency only when invoked.

- [ ] **Step 4: Implement discovery commands and compact projections**

Use existing repositories/services, cap results at 100, copy only stable IDs/names/status fields, and use `accessibleTeamIDs(ctx)` through the propagated actor context.

- [ ] **Step 5: Verify discovery GREEN**

Run the command from Step 2. Expected: PASS.

- [ ] **Step 6: Write failing task mutation tests**

Cover:

- `create_task` persists explicit/defaulted `sprint_id` and rejects a sprint/team mismatch.
- sprint/epic target conflict rejection.
- `get_task` cross-workspace rejection.
- `update_task` edits allowed fields, clears sprint/epic on empty string, rejects no-op payloads, and never archives/moves teams.
- checklist list/create/update executors work through `PMChecklistItemService`.
- `add_pm_comment` defaults target entity and validates the enum.
- parent-target task/comment/checklist writes reject unrelated child tasks.

- [ ] **Step 7: Verify task RED**

Run: `cd server && go test ./internal/service -run 'TestPMCommand(CreateTask|GetTask|UpdateTask|Checklist|AddComment|ParentChild)' -count=1`

Expected: FAIL because the task extensions/executors are missing.

- [ ] **Step 8: Implement task tools**

Use `PMTaskService.Create/Update`, `PMChecklistItemService`, and `PMCommentService`. Add helpers:

```go
func resolveCommandEntityID(meta model.InternalCommandContext, explicit, entityType string) (string, error)
func (s *InternalCommandService) validateTaskWithinTarget(ctx context.Context, meta model.InternalCommandContext, task *model.PMTask) error
func compactCommandTask(detail *model.TaskDetail) map[string]any
```

Set `SprintID` on `model.CreateTaskRequest`; parse dates with the existing deadline parser; reuse workflow resolution; never accept archive/team-move fields.

- [ ] **Step 9: Wire services in `cmd/api/main.go` and verify GREEN**

Call `SetPMOperationalServices(workspaceRepo, pmEpicService, pmSprintService, pmObjectiveService, pmChecklistItemService)` after construction.

Run the command from Step 7. Expected: PASS.

- [ ] **Step 10: Run parity test**

Run: `cd server && go test ./internal/service -run TestPMToolCatalogExecutorParity -count=1`

Expected: still reports only epic/sprint/objective executors missing.

- [ ] **Step 11: Commit**

```bash
git add server/internal/service/internal_command_pm_tools.go server/internal/service/internal_command_pm_tools_test.go server/internal/service/internal_command_service.go server/cmd/api/main.go server/internal/commandtools/metadata.go
git commit -m "feat: add PM discovery and task agent tools"
```

---

### Task 4: Add epic operational tools

**Files:**
- Modify: `server/internal/service/internal_command_pm_tools.go`
- Modify: `server/internal/service/internal_command_pm_tools_test.go`

- [ ] **Step 1: Write failing epic tests**

Cover compact list filters/stats, get-by-ID workspace isolation, create/update service side effects, omitted association preservation, supplied-array replacement/clearing, date parsing, no-op update rejection, archive field rejection by schema, and member access limited to own teams.

- [ ] **Step 2: Verify RED**

Run: `cd server && go test ./internal/service -run 'TestPMCommand(ListEpics|GetEpic|CreateEpic|UpdateEpic)' -count=1`

- [ ] **Step 3: Implement epic commands**

Register `pm.list_epics`, `pm.get_epic`, `pm.create_epic`, and `pm.update_epic`. Parse dates as `YYYY-MM-DD`, pass `actorID` to the service, default get/update IDs from an epic target, and use existing `PMEpicService` access checks.

- [ ] **Step 4: Verify GREEN and parity progress**

Run focused tests plus `TestPMToolCatalogExecutorParity`. Expected: epic tests pass; parity reports only sprint/objective gaps.

- [ ] **Step 5: Commit**

```bash
git add server/internal/service/internal_command_pm_tools.go server/internal/service/internal_command_pm_tools_test.go
git commit -m "feat: add epic agent tools"
```

---

### Task 5: Add sprint tools and sprint-targeted runs

**Files:**
- Modify: `server/internal/service/internal_command_pm_tools.go`
- Modify: `server/internal/service/internal_command_pm_tools_test.go`
- Modify: `server/internal/service/agent.go`
- Modify: `server/internal/service/agent_runtime_host.go`
- Modify: `server/internal/service/agent_runtime_launch_context.go`
- Modify: `server/internal/service/agent_runtime_host_test.go`
- Modify: `server/internal/service/agent_policy_test.go`
- Modify: `server/cmd/api/main.go`

- [ ] **Step 1: Write failing sprint command tests**

Cover list/get/list-tasks, target ID defaulting, compact stats, workspace isolation, date parsing, create/update, omitted label preservation, empty label replacement, and team-manager enforcement using a real actor in context.

- [ ] **Step 2: Verify command RED**

Run: `cd server && go test ./internal/service -run 'TestPMCommand(ListSprints|GetSprint|ListSprintTasks|CreateSprint|UpdateSprint)' -count=1`

- [ ] **Step 3: Implement sprint commands**

Register the five sprint commands and call `PMSprintService`. Use its existing overlap/duration/team rules. Return compact sprint/stats/labels payloads.

- [ ] **Step 4: Verify sprint commands GREEN**

Run the Step 2 command. Expected: PASS.

- [ ] **Step 5: Write failing sprint run/target-context tests**

Assert `StartTargetRun(..., "sprint", ...)` checks workspace ownership and agent target allowance, creates a run with target type/ID, emits activity, and passes compact sprint context. Assert runtime host target context rejects a sprint from another workspace.

- [ ] **Step 6: Verify run RED**

Run: `cd server && go test ./internal/service -run 'Test(StartTargetRunSprint|AgentRuntimeHostResolveSprintTarget)' -count=1`

- [ ] **Step 7: Implement sprint run and context support**

Add sprint/objective service dependencies through setters on `AgentService` and `AgentRuntimeHostService`. Add a `case "sprint"` branch parallel to epic, without repo-specific planning behavior. Add `runtimeSprintContextData` and wire setters in `cmd/api/main.go`.

- [ ] **Step 8: Verify run GREEN and parity progress**

Run focused tests and parity. Expected: sprint tests pass; parity reports objective gaps only.

- [ ] **Step 9: Commit**

```bash
git add server/internal/service/internal_command_pm_tools.go server/internal/service/internal_command_pm_tools_test.go server/internal/service/agent.go server/internal/service/agent_runtime_host.go server/internal/service/agent_runtime_launch_context.go server/internal/service/agent_runtime_host_test.go server/internal/service/agent_policy_test.go server/cmd/api/main.go
git commit -m "feat: add sprint agent tools and targets"
```

---

### Task 6: Add objective and key-result tools and targets

**Files:**
- Modify: `server/internal/service/internal_command_pm_tools.go`
- Modify: `server/internal/service/internal_command_pm_tools_test.go`
- Modify: `server/internal/service/agent.go`
- Modify: `server/internal/service/agent_runtime_host.go`
- Modify: `server/internal/service/agent_runtime_launch_context.go`
- Modify: `server/internal/service/agent_runtime_host_test.go`
- Modify: `server/internal/service/agent_policy_test.go`

- [ ] **Step 1: Write failing objective command tests**

Cover workspace-wide reads for a team-restricted actor, create/update manager requirements, unteamed objective owner/admin requirements, association replacement semantics, key-result objective ownership, numeric field validation, no-op update rejection, and cross-workspace IDs.

- [ ] **Step 2: Verify command RED**

Run: `cd server && go test ./internal/service -run 'TestPMCommand(ListObjectives|GetObjective|CreateObjective|UpdateObjective|CreateKeyResult|UpdateKeyResult)' -count=1`

- [ ] **Step 3: Implement objective/key-result commands**

Register the six commands and use `PMObjectiveService`. Do not add accessible-team filtering to reads. Run writes with the propagated actor so `requireCanManageTeams` remains authoritative.

- [ ] **Step 4: Verify command GREEN**

Run the Step 2 command. Expected: PASS.

- [ ] **Step 5: Write failing objective run tests**

Test direct objective runs, compact target context, cross-workspace rejection, a team-restricted actor launching against a multi-team objective, and agent allowed-target validation.

- [ ] **Step 6: Verify run RED**

Run: `cd server && go test ./internal/service -run 'Test(StartTargetRunObjective|AgentRuntimeHostResolveObjectiveTarget)' -count=1`

- [ ] **Step 7: Implement objective target branch/context**

Add a `case "objective"` branch parallel to sprint. Objective reads are workspace-wide after workspace ownership validation; tool mutations still rely on team-manager service checks.

- [ ] **Step 8: Verify GREEN and full parity**

Run focused tests plus `TestPMToolCatalogExecutorParity`. Expected: all PASS.

- [ ] **Step 9: Commit**

```bash
git add server/internal/service/internal_command_pm_tools.go server/internal/service/internal_command_pm_tools_test.go server/internal/service/agent.go server/internal/service/agent_runtime_host.go server/internal/service/agent_runtime_launch_context.go server/internal/service/agent_runtime_host_test.go server/internal/service/agent_policy_test.go
git commit -m "feat: add objective agent tools and targets"
```

---

### Task 7: Apply exact built-in preset expansion

**Files:**
- Modify: `server/internal/service/agent_presets.go`
- Modify: `server/internal/service/agent_policy.go` only if preset policy sanitization rejects a required alias
- Modify: `server/internal/service/agent_presets_marketer_test.go`
- Modify: `server/internal/service/agent_policy_test.go`
- Modify: `server/internal/agentcontract/runtime_profiles_test.go`

- [ ] **Step 1: Write failing preset matrix test**

Encode the exact spec matrix. Assert:

- Ask Agent gets all new read aliases and no new writes.
- Command Agent gets all new read/write aliases.
- Epic Planner gets only member/label/task/epic reads.
- Task Planner gets only member/label/task reads.
- Marketer gets all new reads and no new mutation aliases.
- CRM, Support, Documentation, Code Builder, and Review receive no automatic expansion.

- [ ] **Step 2: Verify RED**

Run: `cd server && go test ./internal/service ./internal/agentcontract -run 'Test.*Preset.*PMTool' -count=1`

- [ ] **Step 3: Implement explicit alias slices**

Use named read/write slices and append/filter by preset key; normalize and de-duplicate. Do not infer grants from category or mutation status.

- [ ] **Step 4: Verify GREEN**

Run the Step 2 command. Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add server/internal/service/agent_presets.go server/internal/service/agent_policy.go server/internal/service/agent_presets_marketer_test.go server/internal/service/agent_policy_test.go server/internal/agentcontract/runtime_profiles_test.go
git commit -m "feat: expose PM tools to intended agent presets"
```

---

### Task 8: Full verification and contract cleanup

**Files:**
- Modify only files required by failures attributable to this feature.

- [ ] **Step 1: Format and static-check changed Go files**

Run: `cd server && gofmt -w internal/commandtools/metadata.go internal/service/internal_command_service.go internal/service/internal_command_pm_tools.go internal/service/internal_command_pm_tools_test.go internal/service/agent.go internal/service/agent_runtime_host.go internal/service/agent_runtime_launch_context.go cmd/api/main.go`

Run: `git diff --check`

Expected: no output.

- [ ] **Step 2: Run focused backend suites**

Run: `cd server && go test ./internal/commandtools ./internal/agentcontract ./internal/service ./internal/handler -count=1`

Expected: PASS.

- [ ] **Step 3: Run all backend tests**

Run: `cd server && go test ./... -count=1`

Expected: PASS.

- [ ] **Step 4: Run frontend tests**

Run: `cd frontend && npm test -- --run src/pages/automation/__tests__/Agents.test.tsx src/lib/__tests__/agentAccess.test.ts`

Expected: PASS.

- [ ] **Step 5: Run production build**

Run: `cd frontend && npm run build`

Expected: PASS with no TypeScript errors.

- [ ] **Step 6: Audit final capability parity**

Confirm from tests and catalog output that a sprint-targeted custom agent can select and execute member/team/workflow/label discovery, read its sprint/tasks, create tasks with `sprint_id`, update tasks, add comments/checklists/dependencies, and that no delete/archive/admin tool was added.

- [ ] **Step 7: Commit verification fixes if any**

```bash
git add <only feature-related files>
git commit -m "test: verify complete PM agent tool surface"
```
