# Runtime quality skills implementation plan

This historical proposal considers test-driven development, debugging, and
verification skills inspired by Superpowers. It is not an installed skill pack or
a current runtime rollout checklist. The proposed additions remain absent in the
reviewed checkout, while the catalog organization has changed.

## Source review — 2026-09-18

- The six proposed skill directories (`test_driven_development`, `systematic_debugging`, `verification_before_completion`, `using_git_worktrees`, `authoring_for_code_review`, and `brainstorming_standalone`) do not exist under `server/skills/system`.
- [Built-in embedding](../../server/skills/builtin.go) still embeds the system directory, but [catalog loading and preset bundles](../../server/internal/agentcontract/skill_catalog.go) now live in `agentcontract`, not the removed worker catalog path. Adding a directory must satisfy catalog parsing and runtime/preset contracts; embedding alone is not proof of usable activation.
- Existing disk directories remain `code_builder` and `review_agent`, with canonical keys `code_implementation` and `code_review`. [Code Implementation](../../server/skills/system/code_builder/SKILL.md) already requires relevant validation and honest reporting, and includes a preview-run no-commit exception. [Code Review](../../server/skills/system/review_agent/SKILL.md) now includes an interactive checkpoint protocol, so the original sparse-bundle description is not a complete behavior inventory.
- [Stage activation](../../server/internal/agentskills/activation.go) covers Epic Planner, Task Planner, and Documentation Agent. There are no proposed thorough/quick Code Builder modes or corresponding workspace/task settings in the inspected source. Configured/core/available skill sets should not be equated with all skills being injected on every turn.
- The proposed `customer_visible`, `CustomerVisible`, and `visible_to=customer` contract is absent from the inspected catalog/UI path. [The frontend hook](../../frontend/src/hooks/queries/useSkills.ts) requests the workspace skill catalog without that filter.
- Nested execution should be assessed against the current Agent Runtime/host orchestration contracts, not the removed `eino_executor.go` path. This review did not verify the separate runtime repository, benchmark token costs, install upstream content, or recheck its license. The plan's numeric cost multiplier and timeline are estimates, not measured outcomes.

## Original proposal

**Date:** 2026-05-15
**Base branch:** `origin/develop`
**Scope:** Bring 3 gap-filler skills (TDD, systematic debugging, verification) into Helpin's skill catalog, wire them into `code_builder` and `review_agent`, expose selected ones to customers in the agent builder.

This plan replaces the abstract proposal in `2026-05-14-superpowers-skills-eval.md` with concrete file edits grounded in current `develop` state.

## State of `develop` (verified, not assumed)

- `server/skills/builtin.go` = trivial `//go:embed system`. **New skill directories auto-register.** No catalog code changes needed for additions.
- `server/internal/worker/skill_catalog.go` holds `builtInPresetSkillBundles[]` — `AgentPresetCodeBuilder` currently has **one** skill (`code_builder`); `AgentPresetReviewAgent` has **one** skill (`review_agent`). Both are sparse.
- `server/internal/agentskills/activation.go` only wires `EpicPlanner` and `TaskPlanner` to per-stage activation. `CodeBuilder` / `ReviewAgent` have no per-stage gating — skills in the bundle are always active for those presets.
- `skill_catalog_test.go` has `TestListBuiltInSkillsContainsExpectedKeys` listing required built-in keys — must extend when adding skills.
- Frontend skill catalog UI: `frontend/src/pages/automation/SkillCatalog.tsx` + `frontend/src/hooks/queries/useSkills.ts` + `frontend/src/lib/pm-types/skills.ts`.
- `code_builder/SKILL.md` on develop is **7 bullets total** — meaningful expansion is room, not regression risk.

## Out of scope for this plan

- `subagent_driven_development` skill — gated on worker actually supporting nested agent runs. Separate spike needed.
- `dispatching-parallel-agents` content — already covered by Helpin's automation rules. Not adding a skill for it.
- `writing-skills` (meta) — internal authoring guide, not a runtime skill.
- Marketing skills (separate plan, `2026-05-12-marketing-skills-catalog-eval.md`).

---

## Phase 1 — Gap-filler skills (must-haves)

### 1.1 New skill: `test_driven_development`

**Create:**
- `server/skills/system/test_driven_development/SKILL.md` (target ~1.5KB — compressed Helpin-style, not upstream verbatim)
- `server/skills/system/test_driven_development/references/anti_patterns.md` (loaded on demand only)

**Frontmatter:**
```yaml
---
name: test_driven_development
description: RED-GREEN-REFACTOR cycle for implementation work. Activate on feature and bug tasks where tests are practical.
metadata:
  title: Test-Driven Development
  supported_runtimes:
    - native_sdk
    - codex
    - opencode
  customer_visible: true
---
```

**Body:** Compressed RED-GREEN-REFACTOR explanation. Reference to `run_command` for executing tests. Hard rule: do not write implementation before a failing test exists for the unit of behavior. Reference `verification_before_completion`. Pointer to `references/anti_patterns.md` for the long list.

### 1.2 New skill: `systematic_debugging`

**Create:**
- `server/skills/system/systematic_debugging/SKILL.md` (~1.5KB)
- `server/skills/system/systematic_debugging/references/root_cause_tracing.md`

**Frontmatter:** Same shape as above. `customer_visible: true`.

**Body:** 4-phase loop — reproduce → hypothesize → instrument/test → fix or refine. Hard rule: state hypothesis before changing code. Reference `verification_before_completion` for the verify step. Pointer to `references/root_cause_tracing.md`.

### 1.3 New skill: `verification_before_completion`

**Create:**
- `server/skills/system/verification_before_completion/SKILL.md` (~1KB — smallest of the three, single clear protocol)

**Frontmatter:** `customer_visible: true`.

**Body:** Mandatory checks before declaring done — run relevant tests, run build/type-check, scan diff for unintended changes, re-read acceptance criteria. Hard rule: do not return a completion summary without naming the verification commands actually run.

### 1.4 Wire into preset bundles

**Edit `server/internal/worker/skill_catalog.go`:**

```go
model.AgentPresetCodeBuilder: {
    Preamble:  "You are Code Builder. ...",
    SkillKeys: []string{
        "code_builder",
        "test_driven_development",
        "systematic_debugging",
        "verification_before_completion",
        "general_agent_behavior",
    },
},
model.AgentPresetReviewAgent: {
    Preamble:  "You are Review Agent. ...",
    SkillKeys: []string{
        "review_agent",
        "verification_before_completion",
        "general_agent_behavior",
    },
},
```

### 1.5 Tests

**Edit `server/internal/worker/skill_catalog_test.go`:**
- Add `"test_driven_development"`, `"systematic_debugging"`, `"verification_before_completion"` to the required-keys list in `TestListBuiltInSkillsContainsExpectedKeys`.
- Add `TestCodeBuilderBundleIncludesQualitySkills` — asserts each expected key is in the preset bundle.
- Add `TestVerificationSkillReferencesValidationCommands` — pattern-matches `run_command` etc. in the skill body (same pattern as existing `TestSecurityTriageSkillReferencesScannerWorkflow`).

**Verify locally:** `cd server && go test ./internal/worker/... ./internal/agentskills/...`

### 1.6 Expand `code_builder/SKILL.md`

7 bullets → ~12 bullets, with explicit references to the new skills:
- Before implementing a behavior change, the failing test for it must exist (see `test_driven_development`).
- For any bug-typed task, follow the hypothesis-first loop (`systematic_debugging`).
- Before declaring the task complete, follow `verification_before_completion`.

No removal of existing rules. Keep the "local commit only, backend manages remote delivery" lines.

**Estimated effort:** 1 day, mostly markdown + 1 small Go edit + tests.

---

## Phase 2 — Cost control: per-stage activation for CodeBuilder

The original planning estimate was that TDD might make `code_builder` runs 2–3× more expensive; no benchmark is recorded here. We want it opt-in per task or per workspace, not blanket.

### 2.1 Add `planning_stage` values for code builder

**Edit `server/internal/model/agent.go` (or wherever `PlanningStage*` constants live):**

```go
const (
    CodeBuilderStageThorough = "code_builder_thorough"
    CodeBuilderStageQuick    = "code_builder_quick"
)
```

### 2.2 Extend `activation.go`

**Edit `server/internal/agentskills/activation.go` — add a case to `activeBuiltInSkillSet`:**

```go
case model.AgentPresetCodeBuilder:
    switch planningStage {
    case model.CodeBuilderStageQuick:
        return map[string]bool{
            "code_builder":                    true,
            "verification_before_completion":  true,
            "general_agent_behavior":          true,
        }, true
    case model.CodeBuilderStageThorough, "":
        return map[string]bool{
            "code_builder":                     true,
            "test_driven_development":          true,
            "systematic_debugging":             true,
            "verification_before_completion":   true,
            "general_agent_behavior":           true,
        }, true
    }
```

Update `isPhaseSelectableBuiltInSkill` to include the three new keys so they participate in stage selection.

### 2.3 Workspace setting

Add `code_builder_default_mode: 'thorough' | 'quick'` to workspace settings (default `'thorough'`). Surfaced in workspace settings page.

### 2.4 Per-task override

PM task already has metadata fields. Add optional `code_builder_mode` per task; if set, overrides workspace default when the agent run is launched.

**Estimated effort:** ~2 days backend + ~1 day frontend (settings + task field).

---

## Phase 3 — Customer-visible flag

### 3.1 Skill definition

`SkillDefinition` already exists in `worker/skill_catalog.go`. Add:

```go
type SkillDefinition struct {
    // ... existing fields
    CustomerVisible bool
}
```

Read from frontmatter `customer_visible: true` in `LoadBuiltInSkills`.

### 3.2 Filter API

In the handler/service that backs the skills catalog endpoint, accept a `?visible_to=customer` query param and filter to skills where `CustomerVisible == true`.

### 3.3 Frontend

`frontend/src/hooks/queries/useSkills.ts` — pass the filter. The agent-builder skills picker on `SkillCatalog.tsx` calls with `visible_to=customer`. The Helpin internal admin view (if separate) calls without the filter.

### 3.4 Initial visible set (matches frontmatter declarations above)

- `test_driven_development`
- `systematic_debugging`
- `verification_before_completion`
- `using_git_worktrees` (Phase 4)
- `authoring_for_code_review` (Phase 4)
- `brainstorming_standalone` (Phase 5 — TBD whether to extract from `task_planner_context`)

Hidden by default (internal-only): `code_builder`, `review_agent`, `crm_operator`, `support_agent`, `epic_state_routing`, `prd_authorship`, `task_decomposition`, `task_planner_context`, `approval_protocol`, `general_agent_behavior` — these are tightly coupled to Helpin's own preset agents.

**Estimated effort:** 1 day backend + 1 day frontend.

---

## Phase 4 — Methodology additions (lower priority)

### 4.1 `using_git_worktrees`

- `server/skills/system/using_git_worktrees/SKILL.md` (~1KB)
- When to branch / when to reuse / when to discard
- Helpin already has `.worktrees/` populated; this codifies conventions

### 4.2 `authoring_for_code_review`

- `server/skills/system/authoring_for_code_review/SKILL.md` (~1.5KB)
- Pre-review self-checklist + responding-to-feedback pattern
- Wire into `code_builder` bundle so it activates before any handoff to `review_agent`
- `review_agent/SKILL.md` gets a one-line reference to it (symmetric guidance)

**Estimated effort:** ~1 day total.

---

## Phase 5 — Strengthen existing skills (text-only edits)

No new files, no Go changes. Markdown edits to incorporate upstream patterns:

| File | Change |
|---|---|
| `server/skills/system/task_planner_context/SKILL.md` | Add HARD-GATE language ("do not initiate any implementation skill before plan approval") + Socratic "one question at a time" framing |
| `server/skills/system/task_decomposition/SKILL.md` | Add anti-pattern callouts (over-decomposition, batch sizing, "junior engineer could follow this" framing) |
| `server/skills/system/review_agent/SKILL.md` | One-line reference to `authoring_for_code_review` for the author-side counterpart |

**Estimated effort:** half a day.

---

## Phase 6 — Subagent-driven development (spike, not commit)

`subagent_driven_development` only pays off if the worker can launch nested agent runs. Before scoping the skill:

1. Read `server/internal/worker/eino_executor.go` + `server/internal/temporalapp/` to confirm whether a running agent can `start_agent_run` for a child task and await it.
2. If supported: write the skill, wire into `AgentPresetCodeBuilder` thorough mode.
3. If not supported: park the skill until the primitive exists. The other Phase 1–5 skills don't depend on it.

**Estimated effort:** 1-day spike, then either ~2 days or 0.

---

## Sequencing & timeline

| Week | Phase | Outcome |
|---|---|---|
| 1 | Phase 1 | TDD + debugging + verification live, wired into `code_builder` and `review_agent`. All runs use them. |
| 1 | Phase 5 (parallel) | Existing skills strengthened. Pure markdown edits, ship same week. |
| 2 | Phase 2 | Per-stage thorough/quick mode lands. Workspace setting + per-task override. **This is the cost-control valve.** |
| 2 | Phase 3 | Customer-visible flag + frontend filter. Skills appear in agent builder. |
| 3 | Phase 4 | Worktrees + code-review-authoring skills. |
| 3 | Phase 6 | Subagent spike. Decide go/no-go. |

Phase 1 alone is shippable on its own — everything after is incremental.

---

## Risks & mitigations

| Risk | Mitigation |
|---|---|
| Token cost balloons on `code_builder` runs | Phase 2 ships the thorough/quick switch. Default to thorough only after Phase 2 lands. Until then, monitor token usage on staging. |
| Skill bodies drift toward verbose like upstream | Hard cap at ~1.5KB SKILL.md; push depth to `references/*.md`. `dependency_auditor` is the in-repo precedent (uses `ecosystems/` subdirectory). |
| Tests in `skill_catalog_test.go` become flaky on minor edits | Use substring assertions, not byte matches. Pattern follows existing `TestSecurityTriageSkillReferencesScannerWorkflow`. |
| Customer-visible flag exposes skill we shouldn't | Default to **false**; opt-in per skill via frontmatter. Code review gates additions. |
| TDD doesn't apply on doc/config tasks | `code_builder` body should say "skip TDD when the task is not behavior change (docs, config, rename)". Activation stays on; the agent decides whether to author a test. |

---

## Files touched (concrete list)

**New:**
- `server/skills/system/test_driven_development/SKILL.md`
- `server/skills/system/test_driven_development/references/anti_patterns.md`
- `server/skills/system/systematic_debugging/SKILL.md`
- `server/skills/system/systematic_debugging/references/root_cause_tracing.md`
- `server/skills/system/verification_before_completion/SKILL.md`
- (Phase 4) `server/skills/system/using_git_worktrees/SKILL.md`
- (Phase 4) `server/skills/system/authoring_for_code_review/SKILL.md`

**Modified:**
- `server/internal/worker/skill_catalog.go` (Phase 1 bundle wiring + Phase 3 `CustomerVisible` field)
- `server/internal/worker/skill_catalog_test.go` (test coverage)
- `server/internal/agentskills/activation.go` (Phase 2 per-stage gating)
- `server/internal/model/agent.go` (Phase 2 stage constants)
- `server/skills/system/code_builder/SKILL.md` (Phase 1 expansion)
- `server/skills/system/review_agent/SKILL.md` (Phase 5 cross-reference)
- `server/skills/system/task_planner_context/SKILL.md` (Phase 5 HARD-GATE)
- `server/skills/system/task_decomposition/SKILL.md` (Phase 5 anti-patterns)
- `frontend/src/hooks/queries/useSkills.ts` (Phase 3 visibility filter)
- `frontend/src/pages/automation/SkillCatalog.tsx` (Phase 3 surfacing)
- `frontend/src/lib/pm-types/skills.ts` (Phase 3 type addition)
- Workspace settings handler/service/migration (Phase 2 default mode)

---

## What to decide before starting

1. **Default mode for Phase 2** — ship Phase 1 as "always thorough" (more tokens, better output) and add the toggle in Phase 2? Or hold Phase 1 until Phase 2 is ready so we never have a thorough-only window?
2. **Skill naming** — keep underscored snake_case (Helpin convention) or use kebab-case to match upstream? Existing catalog uses snake_case, this plan follows it.
3. **License confirmation** — superpowers repo license check before importing content verbatim, even compressed.
4. **`subagent_driven_development`** — should the Phase 6 spike happen earlier so we know the full scope, or is parking it fine?
