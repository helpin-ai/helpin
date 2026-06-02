# Flow Templates (Unified Starters + Templates)

**Date:** 2026-05-10
**Owner:** Automation simplification
**Status:** Plan, not yet implemented
**Related work:**
- `9d43ef69` — Custom agent simplification (split custom-agent path from template path)
- Today's automation surface lives in `frontend/src/pages/automation/AutomationFlows.tsx` (starters via `FlowTemplate[]`) and `server/internal/service/agent_templates.go` (templates via type-cased Go).

---

## 1. Why this matters

Helpin's automation surface today carries two parallel "ready-to-run" concepts that look like the same thing to users but are wired completely differently:

- **Starters** (frontend, `FlowTemplate[]` in `AutomationFlows.tsx`) — six hard-coded form pre-fillers ("Review merged PRs," "Hourly digest," etc.). Each is an `apply: (base) => FlowDraft` function that mutates a default flow draft. They never create agents; the user still has to wire one. They are pure form sugar.
- **Templates** (backend, `AgentTemplate` from `agent_templates.go`) — four opinionated bundles (Release Notes Writer, Competitive Intel, Dependency Auditor, Security Triage). Each ships type-cased Go (`createReleaseNotesStarterFlow`, `releaseNotesInputFromTemplateFlow`, etc.) and a `create_flow: bool` checkbox in the agent creation drawer. They install both an agent and a rule.

Two separate concepts, two parallel code paths, two surfaces in the UI. Both are called "templates" in places. Each has gaps:

- The four templates are gated behind a checkbox on a drawer that lives on the **Agents** page — wrong home, since the user thinking about automation lands on Flows.
- The six starters are pure form sugar — they don't help the user who needs an agent provisioned, only the user who already has one.
- Adding new patterns requires editing JS (starters) or writing a new Go file with a bespoke parser (templates).

**The deep insight:** these aren't two concepts. They're one concept differentiated only by *how the agent is sourced*. Some patterns ship with an opinionated agent; some reuse a system agent; some let the user pick their own; some don't need an agent at all. All of them are "ready-made automations."

This plan unifies them into a single manifest-driven primitive on the **Flows** page. One mental model, one backend, one drawer. Adding a template that uses existing triggers, actions, agent modes, input types, and tools becomes "drop a YAML file"; new product capabilities still require normal backend/frontend work.

---

## 2. Goals

- Users find ready-made automations where they're already thinking about automation — the Flows page — through one entry point.
- **Ship all 10 of today's patterns plus the 3 module-coverage additions as functional templates in v1.** No deferrals. If supporting agent tooling is missing for any pattern, that tooling lands as part of this work.
- Adding a template that fits existing platform primitives is **adding a manifest file**, not editing TS or writing Go.
- Templates handle the four real agent-sourcing modes: create new, reuse system, pick existing, none.
- The custom-flow path stays as the escape hatch for power users who want to build from scratch.
- Removing the duality eliminates ~150 LOC of starter pre-filler logic and ~600 LOC of bespoke template Go.

## 3. Non-goals (out of v1 scope)

- Workspace-authored templates (custom recipes a workspace can save). Manifest format is workspace-agnostic by design; v2 adds DB-backed manifests alongside the embedded ones.
- Template marketplace beyond what ships in the binary.
- "Update available" / version-drift UI when manifests change. Frozen-at-install behavior is the v1 contract; v2 layers a merge engine on top.
- First-class "Reinstall" preserving user prompt edits.
- Search and featured surfaces in the picker. (Categories *are* in scope — see §9.1.)
- Direct PR-comment-posting from agents (the Review merged PRs template ships with `create_task` / `add_task_comment` outputs in v1; native PR comment posting is a v2 polish).

---

## 4. Today's surface vs. after

| | Today | After |
|---|---|---|
| Discovery surface for ready-made automations | Mixed: 4 templates on Agents drawer, 6 starters on Flows drawer | Single template gallery on Flows drawer |
| Naming | "Templates" (Agents) and "FlowTemplates" (Flows) — overloaded | Templates only |
| Agent creation | Opt-in via `create_flow: bool` checkbox (always-on for the 4 real templates) | Manifest declares the agent mode; no checkbox |
| Form pre-fillers (the 6 starters) | Frontend `FlowTemplate[]` array, JS-only | Removed; all 6 reborn as YAML manifest templates with the appropriate `agent` mode |
| Adding a template that fits existing primitives | New Go file (~150 LOC) and bespoke parsing | New YAML manifest |
| Per-template parsers in Go | 4 (`releaseNotesInputFromTemplateFlow`, etc.) | 0 |
| `CreateFlow bool` and `Flow *CreateAgentFromTemplateFlow` payload shape | Present | Removed |

---

## 5. UX

### 5.1 Where templates live

On the **Flows page**, accessed via the existing primary CTA.

```
/w/{slug}/automation/flows

┌── Flows ────────────────────  [+ New flow]──┐
│                                              │
│  (existing flows list)                       │
│                                              │
└──────────────────────────────────────────────┘
```

Click `+ New flow` → drawer opens with the template gallery first, "Build custom flow" as a link below. **No "Or start with a starter" section** — starters are absorbed.

### 5.2 The new-flow drawer

```
┌── New flow ────────────────────────────────────────┐
│                                                    │
│  Templates                                         │
│                                                    │
│  [All]  [Engineering]  [Sales]  [Support]          │
│  [Marketing]  [Docs]  [Workflow]                   │
│                                                    │
│  ┌──────────────┐  ┌──────────────┐                │
│  │ 📜 Release   │  │ 📊 Competitor│                │
│  │ Notes Writer │  │ watch        │                │
│  └──────────────┘  └──────────────┘                │
│  ┌──────────────┐  ┌──────────────┐                │
│  │ 🔒 Security  │  │ 📦 Dep.      │                │
│  │ Triage       │  │ Auditor      │                │
│  └──────────────┘  └──────────────┘                │
│  ┌──────────────┐  ┌──────────────┐                │
│  │ 👀 Review    │  │ 🛠 Triage    │                │
│  │ merged PRs   │  │ failing chks │                │
│  └──────────────┘  └──────────────┘                │
│  ┌──────────────┐  ┌──────────────┐                │
│  │ ✓ Advance    │  │ 🔀 Merge     │                │
│  │ on approval  │  │ when done    │                │
│  └──────────────┘  └──────────────┘                │
│  ┌──────────────┐  ┌──────────────┐                │
│  │ 🚀 Run on    │  │ ⏰ Run on    │                │
│  │ release      │  │ a schedule   │                │
│  └──────────────┘  └──────────────┘                │
│  ┌──────────────┐  ┌──────────────┐                │
│  │ 💰 Buyer     │  │ ⏳ Stale     │                │
│  │ signal →     │  │ task escal.  │                │
│  │ task         │  │              │                │
│  └──────────────┘  └──────────────┘                │
│  ┌──────────────┐                                  │
│  │ 📝 Docs      │                                  │
│  │ freshness    │                                  │
│  └──────────────┘                                  │
│                                                    │
│  ─────────────────────────────────────────────     │
│                                                    │
│  Or build a custom flow →                          │
│                                                    │
└────────────────────────────────────────────────────┘
```

Filter chips above the cards. Default `All` shows the full gallery. Selecting a chip filters via the manifest's `categories: [...]` tags. Templates may carry multiple categories so a single template can appear under multiple filters (e.g., Release Notes Writer surfaces under both Engineering and Docs). The `All` chip stays the most-common default; chips are progressive disclosure for users who think departmentally and become essential as the catalogue grows beyond ~20 templates.
```

Each card: icon, name, short description. No category chips, no "installed" indicator (templates always create fresh).

### 5.3 Install form

Click a card → drawer flips to the install form, generated from the manifest's `inputs:` block. Required fields first, optional below. A name field at the bottom (optional, defaults to template name) so two installs of the same template don't collide.

```
┌── Install: Release Notes Writer ─── ←Back ──┐
│                                             │
│  Drafts release notes from a GitHub release │
│  and publishes them as a doc proposal.      │
│                                             │
│  Required                                   │
│                                             │
│  Repository                                 │
│  [ Pick a repository           ▾ ]          │
│                                             │
│  Where to publish                           │
│  [ Pick a docs collection      ▾ ]          │
│                                             │
│  Optional                                   │
│                                             │
│  Include prereleases                        │
│  [ ] Off                                    │
│                                             │
│  Name (optional)                            │
│  [ Release Notes Writer                ]    │
│                                             │
│                  [ Cancel ]  [ Install ]    │
└─────────────────────────────────────────────┘
```

Submit → backend installs (create agent if needed, create rule, link them, transactional). User lands on the new flow's detail page with a one-time toast "Release Notes Writer installed."

### 5.4 Custom flow path

Click "Build a custom flow" → existing flow builder. Unchanged.

### 5.5 Custom agent path (Agents page)

The **Agents page** stops showing template options. `+ New agent` opens only the custom-agent drawer (the one shipped in `9d43ef69`). Mental models stay clean: templates → Flows page; custom agents → Agents page.

### 5.6 Agents created by templates

A custom agent created by a template install is fully visible on the Agents page with a label:

```
Quill                  System
Release Notes Bot      from Release Notes Writer template
my-cron-agent          Custom
```

The user can open it, edit prompt/tools, see runs. The label is informational, not restrictive.

### 5.7 Uninstall

A flow installed from a template gets an "Uninstall template" action in its overflow menu (separate from the regular delete-flow action which removes only the rule).

```
Uninstall Release Notes Writer?

The flow will be removed.

The agent "Release Notes Bot" was created by this template
and isn't used by any other flow.

  ( • ) Keep the agent (you can run it manually or wire it
        to another flow later)
  ( ○ ) Delete the agent too

[ Cancel ]                              [ Uninstall ]
```

Default: **Keep the agent**. The user must explicitly opt into deletion.

If the agent is used by another flow, the radio buttons disappear:

```
The agent "Release Notes Bot" will keep running for:
  · Marketing Updates flow
```

If the user chooses "Keep the agent," strip the `template_instance_id` from the agent so it surfaces as a regular custom agent thereafter (label changes from `from Release Notes Writer template` to `Custom`).

For templates with `agent: none` or `agent.reuse_system` or `agent.pick_existing`, the dialog skips the agent question entirely (nothing to delete or strip).

**User edits to the installed flow are lost on uninstall.** The rule is deleted regardless of whether the user tweaked its conditions, parameters, or actions. The dialog includes a one-liner — "Any edits you've made to this flow will be lost" — when the rule's stored content differs from the manifest defaults. (Edits to the agent are preserved when the user picks "Keep the agent.")

### 5.8 Reinstall

v1 path: uninstall + install. Two clicks. v2 problem.

---

## 6. Backend: manifest-driven templates

### 6.1 Where manifests live

Embedded into the binary via `//go:embed`. Embed paths are relative to the consuming Go package, so manifests live under `server/internal/templates/manifests/<key>/template.yaml`. The registry (`server/internal/templates/registry.go`) embeds `manifests/*` directly.

```
server/internal/templates/
  registry.go
  schema.go
  installer.go
  uninstaller.go
  manifests/
    release_notes_writer/
      template.yaml
      description.md         (optional long-form, referenced from yaml)
      prompt.md              (optional system prompt, referenced from yaml)
    competitive_intelligence_digest/
      template.yaml
    dependency_auditor/
      template.yaml
    security_triage/
      template.yaml
    review_merged_prs/
      template.yaml
      prompt.md
    triage_failing_checks/
      template.yaml
      prompt.md
    advance_on_approval/
      template.yaml
    merge_when_done/
      template.yaml
    run_on_release/
      template.yaml
    run_on_a_schedule/
      template.yaml
    buying_signal_to_task/
      template.yaml
      prompt.md
    stale_task_escalation/
      template.yaml
      prompt.md
    docs_freshness_sweep/
      template.yaml
```

Loaded into an in-memory `TemplateRegistry` at boot. No DB row per system template — the manifest is the source of truth.

### 6.2 Manifest shape

```yaml
key: release_notes_writer
version: 1
name: Release Notes Writer
icon: scroll
short_description: Drafts release notes from a GitHub release and publishes them as a doc proposal.
description_ref: ./description.md
categories: [engineering, docs]   # closed vocabulary, multi-tag

agent:
  # Exactly one of: create | reuse_system | pick_existing | none
  create:
    preset: documentation_agent
    runtime_kind: native_sdk
    name_template: "{{template_name}} agent"
    system_prompt_ref: ./prompt.md
    allowed_tools: [get_release_context, find_tasks_for_git_changes, ...]
    allowed_targets: [repository, document]
    approval_mode: always

trigger:
  type: event              # or "cron"
  event: github.release_published

inputs:
  - { key: repository_id,             type: repository,  required: true,  label: "Repository" }
  - { key: destination_collection_id, type: collection,  required: true,  label: "Where to publish" }
  - { key: include_prerelease,        type: bool,        required: false, default: false, label: "Include prereleases" }

flow:
  action: start_agent_run
  target:
    from_input: repository_id
  conditions:
    - { field: repository_id, op: eq, from_input: repository_id }
  parameters:
    include_prerelease:        { from_input: include_prerelease }
    destination_collection_id: { from_input: destination_collection_id }
```

### 6.3 The four `agent` modes

```yaml
# Mode 1: create a new custom agent for this install (most opinionated templates)
agent:
  create:
    preset: ...
    system_prompt_ref: ./prompt.md
    allowed_tools: [...]

# Mode 2: reuse an existing system agent (Quill, Beacon, etc.)
agent:
  reuse_system: documentation_agent

# Mode 3: user picks one of their existing agents at install time
agent:
  pick_existing:
    required: true
    constraints:               # optional; scopes the picker
      presets: [code_builder]
      targets: [repository]

# Mode 4: no agent at all (for non-agent flow actions)
agent:
  none: true
```

Validator (boot-time, fail-fast):
- Exactly one mode must be set per manifest.
- `flow.action` is validated against the existing `automation_rules` action constants in `server/internal/model/automation_rule.go`: `start_agent_run`, `move_to_state`, `merge_branch`, `run_command`. (`run_agent` and `start_flow` are legacy/unsupported and rejected.)
- Mode 4 (`none`) requires `flow.action` to be a non-agent action (`move_to_state`, `merge_branch`, `run_command`).
- Modes 1–3 require `flow.action: start_agent_run`.
- `reuse_system` must reference a known system agent preset key.
- `categories` values must come from the closed vocabulary (`engineering`, `sales`, `support`, `marketing`, `docs`, `workflow`); empty is allowed (renders as uncategorized — surfaces only under `All`).

### 6.4 Input types

Existing flow-builder pickers cover what we need. Manifest input `type:` maps to:

| Type | Renders as |
|---|---|
| `repository` | Repository picker |
| `team` | Team picker |
| `collection` | Docs collection picker |
| `workflow` | Workflow picker |
| `workflow_state` | Workflow state picker (filters by `depends_on: workflow`) |
| `cron` | Cron expression picker (with friendly schedule presets) |
| `agent` | Agent picker (filters by `pick_existing.constraints` if applicable) |
| `bool`, `string`, `int`, `enum<...>` | Native form inputs |
| `branch` | Free text with default `main` |

Inputs may declare `depends_on:` to indicate they re-render when another input changes:

```yaml
- { key: workflow_id,    type: workflow,       required: true, label: "Workflow" }
- { key: from_state_id,  type: workflow_state, required: true, label: "When task is in",
    depends_on: workflow_id }
- { key: to_state_id,    type: workflow_state, required: true, label: "Move it to",
    depends_on: workflow_id }
```

### 6.5 The thirteen v1 manifests at a glance

| Key | Name (UI) | Agent mode | Trigger | Categories |
|---|---|---|---|---|
| `release_notes_writer` | Release Notes Writer | `create` | `github.release_published` | engineering, docs |
| `competitive_intelligence_digest` | Competitive Intelligence Digest | `create` | `cron` | marketing |
| `dependency_auditor` | Dependency Auditor | `create` | `cron` | engineering |
| `security_triage` | Security Triage | `create` | `cron` | engineering |
| `review_merged_prs` | Review merged PRs | `create` | `github.pull_request_merged` | engineering |
| `triage_failing_checks` | Triage failing checks | `create` | `github.check_suite_completed` (failure) | engineering |
| `advance_on_approval` | Advance on approval | `none` | `agent_run.approved` | workflow |
| `merge_when_done` | Merge when done | `none` | `task.state_entered` | engineering, workflow |
| `run_on_release` | Run on release | `pick_existing` | `github.release_published` | engineering |
| `run_on_a_schedule` | Run on a schedule | `pick_existing` | `cron` | (uncategorized) |
| `buying_signal_to_task` | High-intent buyer signal → task | `create` | `cron` (agent uses `list_buyer_signals` + `create_task`) | sales |
| `stale_task_escalation` | Stale task escalation | `create` | `cron` (agent uses `list_tasks` + `add_task_comment` / `create_task`) | workflow |
| `docs_freshness_sweep` | Docs freshness sweep | `reuse_system: documentation_agent` | `cron` | docs |

Two renames vs. earlier draft: `fix_failing_checks` → `triage_failing_checks` (honest — v1 diagnoses, doesn't fix), and `schedule_an_agent` → `run_on_a_schedule` (drops "agent" jargon for first-time users).

Three additions vs. earlier draft to close module gaps: `buying_signal_to_task` (CRM), `stale_task_escalation` (PM ops), `docs_freshness_sweep` (Docs, reuses Quill).

`run_on_a_schedule` is intentionally uncategorized — it's a universal scheduling primitive that doesn't belong to a department; it surfaces under `All` only.

### 6.6 Tooling readiness audit

Each agent-bearing template needs the right tools for the agent to deliver value. Audit:

| Template | Tools / triggers needed | Status |
|---|---|---|
| `release_notes_writer` | `get_release_context`, `find_tasks_for_git_changes`, doc tools | ✅ all present |
| `competitive_intelligence_digest` | web search, `create_task` | ✅ all present |
| `dependency_auditor` | filesystem, `run_command`, `create_task`, scanners | ✅ all present |
| `security_triage` | `scan_semgrep`, `scan_trivy`, `scan_gitleaks`, `create_task` | ✅ all present |
| `review_merged_prs` | **`get_pull_request_diff`** (NEW), `find_tasks_for_git_changes`, `create_task`, `add_task_comment` | ⚠️ 1 new tool |
| `triage_failing_checks` | **`get_check_run_logs`** (NEW), filesystem, `run_command`, `create_task` | ⚠️ 1 new tool |
| `run_on_release` | `get_release_context` + whatever the user-picked agent has | ✅ all present |
| `advance_on_approval` | (agent: none — pure rule, existing event) | ✅ no tools needed |
| `merge_when_done` | (agent: none — git rule, existing event) | ✅ no tools needed |
| `run_on_a_schedule` | (whatever the user-picked agent has) | ✅ no fixed requirement |
| `buying_signal_to_task` | `list_buyer_signals` + `create_task` (agent reads via tools, not engine SQL) | ✅ all present |
| `stale_task_escalation` | `list_tasks` + `add_task_comment` / `create_task` (agent reads via tools) | ✅ all present |
| `docs_freshness_sweep` | reuses Quill (`reuse_system: documentation_agent`); doc tools | ✅ all present |

**New tools to ship as part of v1:**

1. **`get_pull_request_diff(owner, repo, pull_number)`** — returns file list + per-file additions/deletions/patch. Wraps GitHub `GET /repos/{owner}/{repo}/pulls/{pull_number}/files`. The GitHub client (`server/internal/githubapp/client.go`) already has `GetPullRequest`; add a sibling `GetPullRequestFiles`. Tool registration in `worker/tools_git.go` (or a new `tools_pr.go`). Estimate: ~1 day including tests.

2. **`get_check_run_logs(owner, repo, check_run_id)`** — returns conclusion, output title/summary/text, and structured annotations. Wraps GitHub `GET /repos/{owner}/{repo}/check-runs/{check_run_id}`. The GitHub client has no check-run helpers; add `GetCheckRun` and a small struct. Tool registration in `worker/tools_git.go`. Estimate: ~1.5 days including tests (more because the client needs new types).

**v1 outputs vs. v2 polish:**
- `review_merged_prs` v1: agent reads diff + creates a PM task with review notes (or comments on the linked task). v2: directly post a comment on the PR (requires a `comment_on_pull_request` tool — non-goal in v1).
- `triage_failing_checks` v1: agent reads failure logs + creates a PM task with diagnosis. v2: agent opens a fix PR (requires the existing code-builder tools to be wired through, plus deeper testing).

### 6.7 New backend services

```
internal/templates/
  registry.go              # loads manifests at boot; lookup by key
  schema.go                # Manifest type, validation
  installer.go             # Install(workspaceID, key, inputs) -> (agent?, rule)
  uninstaller.go           # Uninstall(instanceID, alsoDeleteAgent bool)
  registry_test.go
  installer_test.go
  manifests/               # //go:embed root, sibling of registry.go (per §6.1)
```

`Installer.Install` runs in a single transaction:
1. Validate inputs against the manifest schema.
2. Resolve the agent according to the manifest's `agent` mode:
   - `create`: build a custom agent row.
   - `reuse_system`: look up the system agent by preset key.
   - `pick_existing`: validate the user-supplied `agent_id` is in the workspace and matches `constraints`.
   - `none`: skip.
3. Create the automation rule, parameter-substituting from `inputs`. The picked/created agent's id is written into `action_config.agent_id` (JSONB) per the existing rule shape.
4. **Stamping rule (consistent with §6.9):** stamp the rule always; stamp the agent **only if the install created it (`agent.create`)**. For `reuse_system` and `pick_existing`, the agent existed before and shouldn't carry a per-install identity. The `template_instance_id` link is therefore agent → rule for `create` installs, and rule-only for the other modes.

`Uninstaller.Uninstall` runs in a single transaction:
1. Delete the rule.
2. If the install was `agent.create` AND the original agent has `template_instance_id` matching this install AND **no other automation rules reference this agent**, then handle per the user's choice:
   - "Delete the agent too": delete it.
   - "Keep the agent": clear all template fields (`template_key`, `template_instance_id`, `template_version`) so it surfaces as a regular custom agent thereafter.
3. The "no other rules reference this agent" check is a JSONB query against `automation_rules.action_config -> agent_id` (the agent reference is not a normal column). Test coverage must include the case where another rule references the same agent.

For `reuse_system` and `pick_existing` installs, no agent decision is needed (the agent wasn't created by the install).

Both operations log at `INFO` via `slog` with `template_key`, `template_instance_id`, `workspace_id`, `actor_id`. Failures log at `ERROR` with the same context plus the wrapped error.

### 6.8 New API endpoints

Following the existing automation API convention (`/automation/...` with `workspace_id` as a query param, parity with `/automation/flows`):

```
GET    /automation/templates?workspace_id=...                                   — pm.read
POST   /automation/templates/{key}/install?workspace_id=...                     — pm.admin.automations
POST   /automation/template-instances/{instanceID}/uninstall?workspace_id=...   — pm.admin.automations
```

Replaces the existing `POST /agent-templates/{templateID}/create-agent` flow.

The list endpoint returns, per template, what the picker needs to render: `key`, `name`, `short_description`, `icon`, `categories[]`, plus the `inputs` schema (so the install form can be rendered without a second round-trip). Long `description.md` content is fetched lazily by the install drawer when a card is clicked, to keep the list payload small.

### 6.9 Schema additions

Both `agents` and `automation_rules` tables get:

```sql
ALTER TABLE agents
  ADD COLUMN template_key          TEXT,
  ADD COLUMN template_instance_id  UUID,
  ADD COLUMN template_version      INT;

ALTER TABLE automation_rules
  ADD COLUMN template_key          TEXT,
  ADD COLUMN template_instance_id  UUID,
  ADD COLUMN template_version      INT;

CREATE INDEX idx_agents_template_instance         ON agents(template_instance_id);
CREATE INDEX idx_automation_rules_template_inst   ON automation_rules(template_instance_id);
```

`template_instance_id` is the join key between an installed agent and an installed rule (when the install creates an agent). For `agent: none` and `agent: reuse_system` and `agent: pick_existing` installs, only the rule carries the stamp.

`template_version` is captured at install time so future v2 "update available" mechanics can detect drift against the shipped manifest.

Migration: the columns must be defined as struct fields on `model.Agent` and `model.AutomationRule` so AutoMigrate adds them on startup, **before** the dbmigrate file that creates the indexes runs. dbmigrate ordering matters in fresh CI/deploy environments — verify the migration runs after AutoMigrate or guards itself with `IF EXISTS` on the column references.

---

## 7. Migration

### 7.1 Existing template-created rows

Rows produced by today's `CreateAgentFromTemplate` path do not carry the new columns. v1 does not backfill them; they continue to function as plain custom agents and rules. Documented behavior: legacy installs miss the new lifecycle features (uninstall + label) until the user manually reinstalls via the new path.

### 7.2 Replaced starters

The 6 entries in `FlowTemplate[]` are deleted. **All 6 are reborn as YAML manifests** — none are lost:

- "Review merged PRs" → `review_merged_prs` (`agent.create`, ships with the new `get_pull_request_diff` tool)
- "Fix failing checks" → `triage_failing_checks` (renamed; `agent.create`, ships with the new `get_check_run_logs` tool)
- "Advance on approval" → `advance_on_approval` (`agent: none`)
- "Merge when done" → `merge_when_done` (`agent: none`)
- "Run on release" → `run_on_release` (`agent.pick_existing`)
- "Hourly digest" → `run_on_a_schedule` (renamed; `agent.pick_existing`, generalized to any cron schedule)

Frontend `AutomationFlowsSearch` query params (`template`, `template_title`, `template_description`) are kept for backwards-compat; their handler logic redirects to the new install drawer with the matching template key.

### 7.3 The `agent_templates` DB table

Stops being seeded from the new manifest registry. Existing rows are harmless and stay. A separate cleanup PR can decide drop vs. repurpose for workspace-authored templates in v2.

---

## 8. Implementation phases

### Phase 1 — Backend foundation (~2 days)
- `internal/templates/registry.go` + manifest schema + boot-time validation
- `//go:embed` wiring of `manifests/*` from inside `server/internal/templates/`
- All four `agent` modes: `create`, `reuse_system`, `pick_existing`, `none`
- `depends_on` semantics for inputs
- Tests: load all manifests; fail-build on invalid manifest; validate each `agent` mode

### Phase 2 — Migrate existing 4 manifests (~half day)
- Move Release Notes Writer, Competitive Intel, Dependency Auditor, Security Triage configs into manifests
- Schema-validation tests: each manifest loads, validates, and round-trips through the registry
- Behavior parity test is deferred to Phase 4 (when the installer exists and can actually produce rows to compare)

### Phase 3 — Author 9 new manifests (~1.5 days)
- `advance_on_approval` — `agent: none`
- `merge_when_done` — `agent: none`
- `run_on_release` — `agent.pick_existing`
- `run_on_a_schedule` — `agent.pick_existing`
- `review_merged_prs` — `agent.create` (depends on Phase 3.5)
- `triage_failing_checks` — `agent.create` (depends on Phase 3.5)
- `buying_signal_to_task` — `agent.create` (cron-triggered; agent reads `list_buyer_signals`, filters by buying_intent + confidence threshold, calls `create_task` per match. Engine has no generic SQL-scan action, so the work happens inside the agent via existing tools.)
- `stale_task_escalation` — `agent.create` (cron-triggered; agent reads `list_tasks`, filters by state-age, calls `add_task_comment` / `create_task`. Same rationale.)
- `docs_freshness_sweep` — `agent.reuse_system: documentation_agent`

### Phase 3.5 — New GitHub tools (~2.5 days)
- `server/internal/githubapp/client.go`: add `GetPullRequestFiles(...)` returning per-file patch + line stats
- `server/internal/githubapp/client.go`: add `GetCheckRun(...)` returning conclusion + output + annotations
- `server/internal/worker/tools_git.go` (or a new file): register `get_pull_request_diff` and `get_check_run_logs` with the tool registry; add to `tool_catalog.go` Git category
- Tests: stubbed GitHub client + happy-path + error-path assertions for both tools

No new trigger events are introduced in v1. `buying_signal_to_task` and `stale_task_escalation` are cron-based with conditions, using the existing rule-engine event vocabulary. Event-driven variants of either are v2 work.

### Phase 4 — Installer / uninstaller (~2 days)
- `internal/templates/installer.go` + `uninstaller.go`
- Schema columns + dbmigrate file for indexes
- Service tests with in-memory SQLite covering: install (each agent mode), uninstall (keep), uninstall (delete), uninstall when agent has other refs, atomic rollback on failure mid-install
- **Behavior parity test for the existing 4 templates** — deferred from Phase 2 — confirms an install through the new path produces an agent + rule equivalent to what today's `CreateAgentFromTemplate` would have produced

### Phase 5 — API endpoints (~1 day)
- New routes: list templates, install, uninstall
- Permission: `pm.admin.automations`
- Activity log entries: `template.installed`, `template.uninstalled`
- Handler tests

### Phase 6 — Frontend Flows page integration (~2.5 days)
- `+ New flow` drawer redesign: single template gallery, "Build custom flow" link below
- **Filter chip row** above the gallery (`All`, `Engineering`, `Sales`, `Support`, `Marketing`, `Docs`, `Workflow`); default `All`; multi-tag aware (a card with `categories: [engineering, docs]` shows under both `All`, `Engineering`, and `Docs`).
- Template picker cards (generated from registry list endpoint)
- Install form generated from manifest `inputs` schema (reuse existing pickers; wire `depends_on` for state pickers)
- Optional Name field at bottom of install form
- Land on flow detail page after install

### Phase 7 — Uninstall flow (~1 day)
- Overflow action on flows installed from templates
- Modal with keep-agent / delete-agent radios (default: keep)
- Handle the "agent in use by another flow" case (no radios, info-only)
- Frontend label updates on Agents page

### Phase 8 — Remove old paths (~1 day)
- Delete `createReleaseNotesStarterFlow`, `releaseNotesInputFromTemplateFlow`, and the three sibling pairs (~600 LOC)
- Delete `CreateFlow bool` + `Flow *CreateAgentFromTemplateFlow` from `CreateAgentFromTemplateRequest`
- Remove the `create_flow` checkbox and template-picker code from the Agents-page drawer
- Delete `FlowTemplate[]` and `FlowTemplateGallery` from `AutomationFlows.tsx` (~150 LOC)
- Update tests; remove the agent-template DB seeding from boot

**Total estimate: ~13.5 dev days for one engineer.** Breakdown vs. earlier draft: +1 day (3 more manifests in Phase 3), +0.5 day (filter chips in Phase 6). Phase 3.5 holds at 2.5 days now that no new trigger events are needed.

---

## 9. Decisions

### 9.1 Resolved (v1 scope)

- **Unification.** Starters and templates are the same concept differentiated by `agent` mode. v1 removes the starter primitive entirely.
- **Manifest update behavior — frozen.** Existing installs are not auto-updated when the manifest changes. Each install records `template_version` at install time. Cosmetic fields (name, icon, descriptions) are read live from the registry. To pick up template changes, users uninstall and reinstall.
- **Permission gate — `pm.admin.automations`** for install and uninstall (parity with `/automation/flows` create/update/delete; verified in `router.go:494-498`). List is `pm.read`. This is intentionally not a permission expansion to `pm.edit`.
- **Route shape — parity with existing automation routes.** `/automation/templates...` with `workspace_id` as a query param, not `/workspaces/{id}/automation/...`. Matches `/automation/flows` convention.
- **Manifest location — `server/internal/templates/manifests/<key>/`.** Embedded by `server/internal/templates/registry.go` via `//go:embed manifests/*`. Cannot be `server/templates/system/` because `//go:embed` paths are relative to the consuming Go package.
- **Stamping rule.** Stamp the rule always; stamp the agent **only when the install created it (`agent.create`)**. For `reuse_system` and `pick_existing` the agent existed before — no per-install identity. The "keep the agent" path on uninstall clears all template fields (`template_key`, `template_instance_id`, `template_version`), not just `template_instance_id`.
- **Activity log entries.** Emit `template.installed` and `template.uninstalled` workspace activity events with actor and template key.
- **Atomicity.** Install and uninstall each run inside a single `db.Transaction`. Either both rows persist/disappear, or neither does.
- **Naming on install.** Optional Name field on the install form; defaults to template name.
- **Default-enabled on install.** The created flow is enabled immediately, matching today's custom-flow create behavior. Users can disable from the detail page.
- **`agent_templates` DB table.** Stop seeding from new manifests; leave existing rows alone. Cleanup is a separate PR.
- **All 13 patterns ship as v1 templates.** No deferrals. Two new GitHub tools (`get_pull_request_diff`, `get_check_run_logs`) are built as part of this work to make `review_merged_prs` and `triage_failing_checks` deliver real value. v1 outputs from those two land as PM tasks (review notes / failure diagnosis); native PR comments and auto-fix PRs are v2 polish.
- **Two renames vs. the original starter set.** "Fix failing checks" → "Triage failing checks" (honest about v1 deliverable); "Schedule an agent" → "Run on a schedule" (drops Helpin jargon).
- **Three additions for module coverage.** `buying_signal_to_task` (Sales / CRM), `stale_task_escalation` (Workflow), `docs_freshness_sweep` (Docs / reuses Quill). Added because the original 10 had zero CRM coverage and only one Docs entry.
- **Categories field on manifests + filter chips in the picker.** Closed-vocabulary multi-tag (`engineering`, `sales`, `support`, `marketing`, `docs`, `workflow`). Default view is `All`; chips are progressive disclosure today and become essential as the catalogue grows. Adding the field now is cheap and avoids a schema migration later.

### 9.2 Still open

- **Input vocabulary extensions.** Future templates may need `tasklist`, `support_view`, etc. v1 ships with the listed types; extend per template demand.
- **`pick_existing.constraints` shape — locked for v1.** Two fields only: `presets: []string` and `targets: []string`. No arbitrary field bag in v1; additional dimensions (`is_system`, `runtime_kind`, etc.) are v2 work and will require a versioned schema bump.
- **Default monthly token budget.** Custom agents have a budget field that's currently "coming soon." Defer manifest support until the budget feature lands.
- **Reinstall as a first-class action.** Two-click v1 path is acceptable; v2 work.

### 9.3 Future work (out of v1, recorded for context)

- **v2 manifest-update mechanics.** "Update available" badge on the flow detail page when manifest version > install version. Merge engine: tool list = union; user-edited prompts kept; new required inputs prompted. Defer until customer behavior shows what users actually edit.
- **Workspace-authored templates.** Custom recipes a workspace can save. Manifest schema is workspace-agnostic by design; the registry just gets a second backend (DB) alongside the embedded one.
- **Direct PR comment posting.** Ship a `comment_on_pull_request` tool so `review_merged_prs` can deliver findings on the PR itself rather than as a PM task.
- **Auto-fix PRs for failing checks.** Wire the existing code-builder tools into `triage_failing_checks` so the agent can open a fix PR rather than just diagnose.

---

## 10. Risks and mitigations

| Risk | Mitigation |
|---|---|
| Manifest validation is loose; bad templates ship broken | Hard fail on boot if any system manifest fails schema validation; CI runs the same load + validate |
| New tools (`get_pull_request_diff`, `get_check_run_logs`) drag the timeline | Bounded scope: GitHub client extensions plus tool registration. Both have well-defined GitHub API surfaces. Tested with stubbed clients. ~2.5 days estimated. |
| Existing template-created rows confuse users post-rollout | Acknowledge the gap; ship a one-time banner if a workspace has legacy template rows: "Want to upgrade these to the new template lifecycle? Reinstall →" |
| Two installs of the same template collide on agent name | Force user to name on install form (already in design) |
| `depends_on` form rendering is fiddly | Reuse the existing flow builder's workflow + state picker components; they already implement this |
| `pick_existing` shows agents the user shouldn't see | Use the recently-shipped `agentAccess.ts` filter (visibility, target match, team scope) on top of `constraints` |
| Atomicity edge case: rule creation fails after agent creation | `db.Transaction` rolls back both. Test: simulated rule-creation failure leaves no orphan agent. |
| `pick_existing` fails when workspace has no qualifying agent | Empty state in picker: "No qualifying agents. Build a custom agent first → /agents" |
| `reuse_system` references an agent the workspace has disabled | Install-time validation: surface clear error "System agent X isn't available in this workspace." |
| Removing 600+150 LOC could break unrelated tests | Phase 8 runs after Phase 6+7 are green; full test pass before removal lands. |
| Concurrent uninstall of the same template instance | DB row-level locking on the rule via the `db.Transaction`: the second caller hits a not-found and gets a clear error. Test: simulated concurrent uninstall asserts only one succeeds. |

---

## 11. Definition of done

- Visiting `+ New flow` on the Flows page shows a single template gallery with **13 templates** and a "Build custom flow" link. A filter chip row (`All` / `Engineering` / `Sales` / `Support` / `Marketing` / `Docs` / `Workflow`) sits above the gallery; default is `All`.
- Selecting a chip filters via the manifest's `categories: [...]` tags (multi-tag aware — a template can appear under multiple chips).
- Clicking any of the 13 templates shows an install form generated from the manifest's `inputs` block, including a `depends_on`-aware workflow/state picker for templates that need it.
- Submitting the install form creates the agent (if applicable) and the rule inside a single transaction. Failure mid-install leaves no rows.
- New flow lands on its detail page; the agent (when one was created) appears on the Agents page with a `from <template> template` label.
- Install and uninstall each emit a workspace activity event AND a structured `slog` log line (INFO on success, ERROR on failure) including `template_key`, `template_instance_id`, `workspace_id`, `actor_id`.
- Uninstalling a template-installed flow shows the radio dialog (keep agent default) when the install created an agent, or a plain confirm otherwise.
- Choosing "Keep the agent" clears all three template fields on the agent (`template_key`, `template_instance_id`, `template_version`) so it shows as a regular custom agent.
- The "agent referenced by another flow" check on uninstall is implemented as a JSONB query against `automation_rules.action_config -> 'agent_id'`, with explicit test coverage for the case where another rule references the same agent.
- Existing custom-flow path is untouched.
- Existing template installs (legacy) keep running; their rows pass-through without the new columns set.
- All 13 v1 manifests load and install end-to-end:
  - Release Notes Writer (`create`) — `engineering, docs`
  - Competitive Intelligence Digest (`create`) — `marketing`
  - Dependency Auditor (`create`) — `engineering`
  - Security Triage (`create`) — `engineering`
  - Review merged PRs (`create`, depends on `get_pull_request_diff`) — `engineering`
  - Triage failing checks (`create`, depends on `get_check_run_logs`) — `engineering`
  - Advance on approval (`none`) — `workflow`
  - Merge when done (`none`) — `engineering, workflow`
  - Run on release (`pick_existing`) — `engineering`
  - Run on a schedule (`pick_existing`) — uncategorized
  - High-intent buyer signal → task (`create`, agent uses `list_buyer_signals` + `create_task`) — `sales`
  - Stale task escalation (`create`, agent uses `list_tasks` + `add_task_comment`) — `workflow`
  - Docs freshness sweep (`reuse_system: documentation_agent`) — `docs`
- Two new GitHub tools are registered, tested, and added to `tool_catalog.go`:
  - `get_pull_request_diff(owner, repo, pull_number)`
  - `get_check_run_logs(owner, repo, check_run_id)`
- The GitHub client (`server/internal/githubapp/client.go`) gains `GetPullRequestFiles` and `GetCheckRun` methods with unit-test coverage.
- Manifest schema includes a `categories: [string]` field validated against the closed vocabulary (`engineering`, `sales`, `support`, `marketing`, `docs`, `workflow`); boot fails on an unknown category.
- Frontend filter chips above the gallery filter the visible cards via the `categories` field; multi-tag templates appear under all matching chips.
- Manifest validator fails the build on: zero or multiple `agent` modes, action/mode mismatch, unknown `reuse_system` preset, missing required field.
- Permission gate on install is `pm.admin.automations`; on uninstall is `pm.admin.automations`.
- The `create_flow: bool` checkbox no longer exists.
- The four bespoke `createXxxStarterFlow` + `xxxInputFromTemplateFlow` functions are deleted (~600 LOC).
- `FlowTemplate[]`, `FlowTemplateGallery`, and `TEMPLATE_TONE` are deleted from `AutomationFlows.tsx` (~150 LOC).
- The agent-template DB seeding from boot is removed.
- Adding a new template that fits existing platform primitives (the 14th) requires only a new `template.yaml` (and optionally `description.md` / `prompt.md`) — no Go, no TS.
