# Superpowers skills evaluation and integration plan

**Historical upstream reference:** https://github.com/obra/superpowers
(upstream contents, popularity, and license were not reverified in this review).
**Date:** 2026-05-14
**Status:** Historical proposal; source-compared on 2026-09-18.

This document records an earlier proposal for adapting coding-methodology skills.
Use it to understand the suggested improvements, not as a list of installed
skills or an approved implementation schedule.

## Current implementation and limits

- The six proposed new directories in the change list below are absent from
  `server/skills/system/`. Standalone brainstorming is also a proposal, not an
  installed skill established by this document.
- The [skill catalog](../../server/internal/agentcontract/skill_catalog.go) aliases
  older keys: `code_builder` maps to `code_implementation` and `review_agent` to
  `code_review`. Directory names and public skill keys are not interchangeable.
- [Code implementation](../../server/skills/system/code_builder/SKILL.md) already
  requires relevant validation and reporting only checks actually run. The
  [review skill](../../server/skills/system/review_agent/SKILL.md) also requires
  targeted validation when possible and explicit outcomes after agreed fixes.
  The original claim of no explicit verification guidance is outdated.
- Current catalog composition and [runtime activation](../../server/internal/agentskills/activation.go)
  distinguish configured skills from active instructions. Adding a Markdown
  directory alone does not prove every coding agent receives its content.
- No `customer_visible`/`CustomerVisible` implementation was found in server or
  frontend Go/TypeScript sources. The flag, picker filter, automatic customer
  benefit, and one-day estimate below remain proposed work. The current
  [skill catalog page](../../frontend/src/pages/automation/SkillCatalog.tsx) is a
  concrete UI entry point to inspect when designing that work.
- Historical `docs/superpowers/plans/` references refer to material now organized
  under `docs/plans/`. They do not authorize automatic subagent dispatch, mandatory
  approval gates, or importing external skills for routine documentation work.

The original recommendations overlap: dispatching parallel agents appears in
both the adapt and skip groups, and requesting/receiving review are combined.
Treat the headline adoption totals as historical estimates, not a reconciled
inventory. The concrete list contains six new directories and four existing
files to modify. Licensing and nested-agent execution still require their own
review before any import or implementation; Markdown format does not establish
permission to reuse content. No upstream import or runtime test was performed.

## Original evaluation


## TL;DR

Superpowers is **not** a marketing-skill collection like `coreyhaines31/marketingskills` was. It's a **coding-agent methodology**: a tight, opinionated, process-shaped skill set covering brainstorm → plan → worktree → TDD → execute → verify → review → finish. 14 skills total.

Helpin already overlaps in 6 places (`code_builder`, `task_planner_context`, `task_decomposition`, `approval_protocol`, `review_agent`, `general_agent_behavior`) and the team is already partially using a superpowers-style plan-doc workflow under `docs/superpowers/plans/`. So this is mostly about **filling gaps in our existing engineering skills** rather than greenfield additions — plus exposing the generic ones to customers who build their own agents in Helpin.

## Recommendation summary

- **Adopt 9 skills** — 3 fill clear gaps in our current catalog, 6 strengthen existing skills
- **Skip 2 skills** — replaced by Helpin-specific equivalents
- **Make 7 of them customer-visible** in the agent builder; keep 2 internal-only (they're for Helpin's own `code_builder` runtime)
- **Acknowledge existing usage** — `docs/superpowers/plans/` already exists; codify what the team is already doing

---

## Superpowers' 14 skills mapped to Helpin

### Skills that fill a real gap (3 — adopt clean)

| Superpowers skill | Why Helpin needs it | What changes |
|---|---|---|
| **test-driven-development** | `code_builder` says "make changes, run validation." Zero TDD guidance. Real gap. | New system skill `test_driven_development`. `code_builder` references it. |
| **systematic-debugging** | No debugging skill anywhere in the catalog. Bugs are a daily customer use case. | New system skill `systematic_debugging` (4-phase root-cause process). |
| **verification-before-completion** | `code_builder` ends at "local commit" — no explicit verify step. `approval_protocol` is about *human* approval, not self-verification. | New system skill `verification_before_completion`, composed into `code_builder` and `review_agent`. |

### Skills that strengthen existing Helpin skills (6 — adapt/merge)

| Superpowers skill | Existing Helpin skill(s) | What to do |
|---|---|---|
| **brainstorming** | `task_planner_context`, `prd_authorship` | Borrow the HARD-GATE pattern ("do not write code until design is approved") and the Socratic "one question at a time" structure. Today our task planner asks scope-gating questions, but the framing is weaker. Inject into `task_planner_context`. |
| **writing-plans** | `task_decomposition` | Our skill already covers vertical slicing + tool contracts (`publish_task_plan`). Upstream adds explicit anti-pattern callouts and forces "junior engineer with no context could follow this" framing. Merge those into our existing skill. |
| **executing-plans** | `code_builder` | Upstream is methodology over our minimal 7-bullet skill. Adopt the *checkpoint per batch* pattern; this matches our automation-rule `agent_run.approved` triggers naturally. |
| **requesting-code-review** + **receiving-code-review** | `review_agent` | Our `review_agent` is heavy on the *reviewer* side (with `review_checkpoint`). Almost nothing for the *author* side — pre-review checklist, how to respond to feedback. Add an `authoring_for_code_review` skill (combines both upstream skills) that `code_builder` references before handing off to `review_agent`. |
| **subagent-driven-development** | `code_builder` | This is genuine new methodology — author dispatches a subagent for each batch, then runs spec-compliance review, then code-quality review. Maps cleanly to our worker model. Adopt as a separate skill that `code_builder` activates for batch sizes above N tasks. |
| **using-git-worktrees** | (none — we *use* worktrees but no skill describes when) | Helpin already has `.worktrees/` populated by background work. Codify the conventions: when to branch, when to reuse, when to discard. New skill `using_git_worktrees`. |

### Skills to fold into our orchestration, not import as-is (3 — adapt)

| Superpowers skill | Action |
|---|---|
| **dispatching-parallel-agents** | We already have `automation_rule.start_agent_run` and the generic agent runtime per `agents-and-automation.md`. Don't import a skill — instead, write a **Helpin-flavored variant** that maps the upstream pattern to our automation rules + `agent_run` primitive. |
| **finishing-a-development-branch** | Upstream presents merge/PR/keep/discard options. In Helpin, "remote delivery is backend-managed after the run succeeds" (from `code_builder`). Adopt only the *verify-tests-clean* prefix, drop the branch-disposition prompts. Fold into `verification_before_completion`. |
| **writing-skills** | Meta-skill for the *team* building Helpin's skills catalog, not for customer runtime. Keep internal — put under `docs/skills/authoring-guide.md` rather than `server/skills/system/`. |

### Skills to skip (2)

- **using-superpowers** — onboarding skill. Replace with a Helpin-specific "using_agent_runs" if we feel a gap.
- **dispatching-parallel-agents** as a skill (covered above — folded into orchestration instead).

---

## Concrete change list to `server/skills/system/`

**Add (6 proposed new directories):**
1. `test_driven_development/` — RED-GREEN-REFACTOR cycle, testing anti-patterns reference
2. `systematic_debugging/` — 4-phase root-cause process
3. `verification_before_completion/` — explicit verify gate before declaring done
4. `using_git_worktrees/` — when to branch/reuse/discard worktrees
5. `authoring_for_code_review/` — pre-review checklist + responding to feedback
6. `subagent_driven_development/` — batched subagent dispatch + two-stage review

**Modify (4 existing files):**
- `code_builder/SKILL.md` — reference TDD, verification, subagent-driven-development; expand from 7 bullets to actual methodology
- `review_agent/SKILL.md` — reference `authoring_for_code_review` for the symmetric author-side guidance
- `task_planner_context/SKILL.md` — borrow brainstorming's HARD-GATE language and Socratic framing
- `task_decomposition/SKILL.md` — borrow upstream's "junior engineer could follow this" framing + explicit anti-pattern callouts

**Leave alone:** the existing tooling-contract bits (`publish_task_plan`, `request_approval`, `review_checkpoint` shapes). Those are Helpin-specific and shouldn't be replaced.

---

## How to make these available to customers

Helpin's `agents-and-automation.md` already states custom agents are `native_sdk`, generic, and built by composing skills. Two distinct surfaces:

### Transparent benefit (no customer action needed)

`code_builder` and `review_agent` are Helpin's own system agents. They get the new skills composed in by default. Every customer running these benefits immediately, with no UI change. **This is most of the value.**

### Customer-visible skills in the agent builder (7)

Customers who build custom agents inside Helpin should see these in the skills picker:

| Skill | Customer use case |
|---|---|
| `test_driven_development` | A customer building a "test-writer" agent over their codebase |
| `systematic_debugging` | "Bug triage agent" pulling from issue tracker |
| `verification_before_completion` | Any custom agent — generic quality gate |
| `using_git_worktrees` | Customers wiring agents to GitHub repos |
| `authoring_for_code_review` | Custom code-author agents pairing with their CI |
| `subagent_driven_development` | Advanced customers building team-of-agents flows |
| `brainstorming` (merged into `task_planner_context` content but **also exposed as standalone**) | Non-code use cases — product managers brainstorming features |

### Keep internal (2)

- `executing-plans` — overlaps too closely with `code_builder`; expose via that agent
- `finishing-a-development-branch` (the parts we keep) — folded into verification, not separately exposed

### Surfacing mechanism

We already have a skills catalog in `server/skills/system/` exposed through `internal/agentskills/`. The frontend custom-agent builder reads from there (per the agent preset system). Two small wires needed:
- Add a `customer_visible: true` flag in skill frontmatter
- Filter the agent-builder skills picker by that flag

This is a ~1-day backend change plus a tiny frontend filter. No new product surface required.

---

## Comparison: what each Helpin skill already does well vs. what to borrow

| Helpin skill | Strength to keep | Gap superpowers fills |
|---|---|---|
| `code_builder` | Tight scope; Helpin tool contracts; backend delivery model | TDD, verification, subagent dispatch, plan-execution methodology |
| `task_planner_context` | Tool contracts (`publish_task_plan_doc`); chat-loop framing | HARD-GATE language; Socratic one-question-at-a-time pattern |
| `task_decomposition` | Vertical slicing; structured `proposed_tasks` schema | "Junior engineer could follow this" framing; anti-pattern callouts |
| `approval_protocol` | Inline approval contract; preview binding | Nothing material — this is genuinely Helpin-specific orchestration |
| `review_agent` | Reviewer-side checkpoint protocol; structured findings | Author-side pre-review checklist; how to respond to feedback |
| `general_agent_behavior` | Repository safety; chat-loop framing | Nothing material — keep as the baseline |

---

## Acknowledgment of existing usage

`docs/superpowers/plans/` already contains plan documents. There's a feedback memory: "Skip superpowers workflow for small/focused tasks." So the team has informally adopted some superpowers conventions already. This proposal **codifies what's already happening** plus closes the obvious gaps (TDD, debugging, verification).

## Implementation order

1. **Week 1 — gap fillers (3 skills):** `test_driven_development`, `systematic_debugging`, `verification_before_completion`. Drop-in, mostly markdown. Wire into `code_builder` and `review_agent`.
2. **Week 1 — strengthen existing (3 skills):** Edit `task_planner_context`, `task_decomposition`, and add `authoring_for_code_review`. All text changes.
3. **Week 2 — methodology additions (2 skills):** `using_git_worktrees`, `subagent_driven_development`. The second has runtime implications — confirm worker can dispatch subagents per the current model.
4. **Week 2 — customer surface:** `customer_visible` flag + frontend filter. ~1 day.

## Open questions for you

- **Subagent dispatch in worker**: does the current worker actually launch nested agent runs, or is this still "planned" per `agents-and-automation.md`? `subagent_driven_development` lands hollow without the primitive.
- **Customer-visible skills picker**: confirm the custom-agent UI is the right surface, or should these go through a marketplace-style flow?
- **Licensing**: Superpowers is on GitHub but I haven't confirmed the license. Confirm the applicable license and attribution obligations before any bulk import.
- **Branding**: keep using "superpowers" as the directory name (consistent with existing `docs/superpowers/`) or rebrand to Helpin terminology?
