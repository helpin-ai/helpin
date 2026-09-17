# Agent Authorization Architecture

## Summary

Build a centralized authorization layer for agents, runs, and tools. Every runnable agent has an explicit scope, every run has an explicit or resolved principal, and every tool call is authorized at execution time.

Core rules:
- Agents are either `team` scoped or `workspace` scoped.
- `team` scope uses `agent_team_access` as the canonical source.
- `workspace` scope is intentional, not a missing team.
- Manual and command-bar runs execute as the current user and re-check current access on each tool call.
- Automation runs execute as fixed product-defined service principals, not as stale rule-author snapshots.

## Key Changes

- Add a central `agentauthz` policy surface:
  - `CanViewAgent(actor, agent)`
  - `CanManageAgent(actor, agent)`
  - `CanStartRun(actor, agent, target)`
  - `CanViewRun(actor, run, agent, target)`
  - `CanInteractWithRun(actor, run, agent, target, action)`
  - `AuthorizeTool(ctx, principal, tool, target, input)`
- Add explicit agent scope:
  - `scope_type`: `team` or `workspace`
  - `team` scope reads only through a helper backed by `agent_team_access`
  - `workspace` scope must have no team rows
  - legacy `agents.team_id` is compatibility-only during rollout
- Add run principal fields:
  - `principal_type`: `user`, `team_service`, `workspace_service`, `system_legacy`
  - `principal_user_id`
  - `principal_team_id`
  - `principal_policy` JSON for service principals only
- Add one resolver for old/new runs:
  - explicit `principal_type` wins
  - legacy `triggered_by_user_id` becomes `user`
  - legacy single-team agent without user becomes `team_service`
  - unresolved legacy/system runs become `system_legacy` with restricted visibility/interaction

## Access Rules

- Agent CRUD:
  - Custom agents default to `team` scope and require at least one team.
  - Workspace-scoped agent creation/editing is admin/owner only.
  - Team managers can manage agents only for teams they own.
  - Admin/owner can manage all agents.
- Run visibility:
  - User can view a run if they can view the agent and can view the target, or they triggered the run, or they are admin/owner.
  - Team-scoped runs inherit target/team visibility.
  - Workspace-scoped runs are not admin-only when the target is narrower; target access still grants visibility.
- Run launch:
  - Manual/API starts require agent access, target access, module permission, and scope compatibility.
  - Team-scoped agents can run only against matching team targets.
  - Workspace-scoped agents can run against workspace/global targets, and against team targets only when the user has target access.
- Command bar:
  - Candidate agents are filtered by `CanViewAgent`.
  - Dispatch validates every step with `CanStartRun`.
  - One-shot runs use `principal_type=user`.
  - Tool calls re-check the triggering user's current RBAC, module access, and team access.
- Automation:
  - Team automations use `principal_type=team_service` plus explicit `team_id`.
  - `team_service` has a fixed policy: allowed modules, tools, and explicit mutations declared by the rule and accepted by product policy.
  - `workspace_service` is allowed only for reviewed system flows in a single code registry, not DB-configurable policy.

## Tool Authorization

- Add metadata for every internal command tool:
  - required module
  - required permission
  - read vs mutation
  - supported target types
  - scope resolution rule
- Scope resolution order:
  - target entity team, when present
  - explicit input `team_id` for create/move operations
  - run principal team for team-service runs
  - workspace scope only for tools marked workspace-safe
- Cross-scope rules:
  - move tools must authorize both source and destination scopes
  - bulk tools require every target scope to pass
  - unresolved scope denies by default unless the tool is workspace-safe read-only
- Add structured deny logging with:
  - `workspace_id`, `agent_id`, `run_id`, `principal_type`, `principal_user_id`, `principal_team_id`, `tool`, `target_type`, `target_id`, `reason`
  - no prompt, body, token, or customer-sensitive content

## Phased Rollout

- Phase 1: Policy foundation and read safety
  - Add scope fields/helpers, run principal fields, resolver, deny logging, and central policy package.
  - Apply policy to agent list/get/usage and run read paths: runs, messages, artifacts, snapshots, interactions.
  - Backfill:
    - existing `agent_team_access` rows -> `scope_type=team`
    - legacy `agents.team_id` -> `scope_type=team` and create access row
    - known global system agents -> `scope_type=workspace`
    - unscoped custom agents -> migration report and disabled/unrunnable until explicitly scoped, unless a deliberate default-team fallback is approved
- Phase 2: Launch enforcement
  - Enforce `CanStartRun` across PM, automation API, support run-agent, docs regeneration, command-bar dispatch, and direct target runs.
  - Add frontend scope display/editing.
  - Workspace scope option appears only for admin/owner.
- Phase 3: Tool enforcement
  - Add tool metadata registry.
  - Gate `InternalCommandService.Execute` through `AuthorizeTool`.
  - Re-check current user access for `principal_type=user`.
  - Enforce fixed product policies for `team_service` and reviewed registry policies for `workspace_service`.
- Phase 4: Automation cutover
  - Require automation rules/schedules to declare service-principal scope.
  - Validate rule-declared tools/mutations against the service-principal policy at create/update time.
  - Pause or fail invalid existing rules with clear audit logs.
- Phase 5: Cleanup
  - Remove direct reads of `agents.team_id`.
  - Keep all scope reads through the helper.
  - Later migration can drop legacy `agents.team_id`.
  - Add authz deny metrics/dashboarding.

## Test Plan

- Agent policy tests:
  - team member can view/run team agent
  - non-team member cannot view team agent or output
  - team manager can manage owned-team agents only
  - admin/owner can manage workspace-scoped agents
  - non-admin cannot create workspace-scoped agents
- Run tests:
  - target access grants workspace-scoped run visibility
  - triggering user can see their own command-bar/manual run
  - legacy principal resolver handles user, single-team, and unresolved system cases
  - launch rejects team mismatch and missing scope
- Tool tests:
  - user principal loses access after role/team change
  - team service cannot access another team
  - workspace service can use only registry-approved tools
  - move/bulk tools require all involved scopes to pass
- Migration tests:
  - backfill is idempotent
  - `agent_team_access` is canonical for team scope
  - unscoped custom agents are reported and not silently widened to workspace scope

## Assumptions

- Workspace scope is first-class and intentionally global.
- Custom agents are team-scoped by default.
- Service principals use fixed product policies, not user permission snapshots.
- `authorization.Actor` remains the user identity model; `agentauthz` composes with it inside service methods and selected middleware.
