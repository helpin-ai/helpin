# Documentation agent skills implementation plan

This historical plan explains the first eight built-in documentation skills. Contributors should read the actual packages when changing product-agent behavior; this plan predates the documentation preset, target-based activation, and later coverage completion rules. Repository writing conventions are maintained separately in the [documentation guide](../documentation-guide.md).

## Source review — 2026-09-18

- All eight skill directories in the original file list exist under [built-in skills](../../server/skills/system). Their frontmatter and current instructions were reviewed. The loader/catalog now live in [agentcontract](../../server/internal/agentcontract/skill_catalog.go), not `internal/worker`; historical worker test commands do not validate the relocated catalog.
- Files retain some legacy directory names while their canonical keys changed: `external_help_doc_writing` → `public_help_doc_writing`, `api_doc_writing` → `api_reference_doc_writing`, `docs_information_architecture` → `docs_architecture_review`, `release_to_docs_update` → `post_release_docs_update`, and `support_gap_to_docs` → `support_gap_docs_update`. The catalog preserves aliases; use canonical keys in new configuration.
- The documentation preset and Quill instructions now exist in the catalog. [Skill activation](../../server/internal/agentskills/activation.go) selects documentation skills by target: document maintenance, support-gap work, or repository/task/epic release work, with broader sets for workspace/other targets. These integrations are no longer merely the follow-up scope described below.
- The current [support-gap skill](../../server/skills/system/support_gap_to_docs/SKILL.md) requires documentation search, relevant source verification, and `complete_support_coverage_gap` with a durable disposition. It distinguishes verified resolution from `review_ready`, `routed`, and `blocked`; completing a run does not automatically resolve its gap. The earlier completion wording below is superseded.
- Current skills use runtime workspace context for product naming and preserve explicit publication policy. No product skills were edited or agent runs executed by this review; the original expected test results are not a current validation record.

## Original implementation plan


**Goal:** Add the first built-in documentation skill set that a future Documentation system agent can use for internal docs, public help docs, API docs, support gaps, release-driven updates, and docs organization.

**Architecture:** Built-in skills are embedded from `server/skills/system/**/SKILL.md` and loaded by `server/internal/worker/skill_loader.go`. This change creates standalone skill packages only; it does not add the Documentation system agent or selective activation yet. Each skill has precise frontmatter and focused instructions so the later `documentation_agent` preset can attach and route them cleanly.

**Tech Stack:** Go 1.24, embedded `fs`, existing built-in skill loader, `go test`.

---

## File Structure

- Create: `server/skills/system/external_help_doc_writing/SKILL.md`
  - Rules for writing new customer-facing public help center articles.
- Create: `server/skills/system/api_doc_writing/SKILL.md`
  - Rules for writing new API reference or API guide docs.
- Create: `server/skills/system/internal_docs_maintenance/SKILL.md`
  - Rules for keeping internal workspace docs accurate and operationally useful.
- Create: `server/skills/system/public_help_docs_maintenance/SKILL.md`
  - Rules for updating existing public help center docs.
- Create: `server/skills/system/api_docs_maintenance/SKILL.md`
  - Rules for updating existing API docs as endpoints, fields, auth, examples, or versions change.
- Create: `server/skills/system/docs_information_architecture/SKILL.md`
  - Rules for spaces, collections, subcollections, titles, slugs, links, duplication, and harmony across docs.
- Create: `server/skills/system/release_to_docs_update/SKILL.md`
  - Rules for translating releases, epics, and shipped features into doc updates.
- Create: `server/skills/system/support_gap_to_docs/SKILL.md`
  - Rules for converting support coverage gaps into missing, weak, or stale doc work.
- Modify: `server/internal/worker/skill_catalog_test.go`
  - Add catalog coverage for the new skill keys and instruction snippets.

Out of scope for this plan:

- Adding `documentation_agent` preset key.
- Seeding a system Documentation Agent.
- Adding docs-specific selective skill activation.
- Creating new docs tools.

Prompt/content rule for every documentation skill in this plan:

- Do not hardcode the product, company, or workspace name in skill instructions.
- When a product, company, or workspace name is needed in generated docs, use the workspace name from runtime context.
- Refer to the current workspace, product, customers, users, and docs surfaces generically when the runtime context does not provide a specific name.
- Rely on runtime workspace context, target context, and source evidence for names and branding.

---

### Task 1: Add Failing Catalog Coverage For Documentation Skills

**Files:**
- Modify: `server/internal/worker/skill_catalog_test.go`

- [ ] **Step 1: Add new expected built-in skill keys**

In `TestListBuiltInSkillsContainsExpectedKeys`, extend the expected key list:

```go
for _, key := range []string{
    "approval_protocol",
    "prd_authorship",
    "task_decomposition",
    "epic_state_routing",
    "general_agent_behavior",
    "task_planner_context",
    "code_builder",
    "review_agent",
    "crm_operator",
    "support_agent",
    "dependency_auditor",
    "security_triage",
    "external_help_doc_writing",
    "api_doc_writing",
    "internal_docs_maintenance",
    "public_help_docs_maintenance",
    "api_docs_maintenance",
    "docs_information_architecture",
    "release_to_docs_update",
    "support_gap_to_docs",
} {
    if !containsString(keys, key) {
        t.Fatalf("expected built-in skill %q in registry, got %v", key, keys)
    }
}
```

- [ ] **Step 2: Add documentation skill content test**

Add a focused test near the other skill content tests:

```go
func TestDocumentationSkillsDeclareExpectedGuidance(t *testing.T) {
    cases := map[string][]string{
        "external_help_doc_writing": {
            "Write for customers and end users",
            "Do not publish directly",
            "place the article in the most specific existing collection",
        },
        "api_doc_writing": {
            "Document authentication, permissions, request shape, response shape, errors, and examples",
            "Do not invent endpoints, fields, limits, or SDK behavior",
            "include at least one realistic request example and one realistic response example",
        },
        "internal_docs_maintenance": {
            "Internal docs may include implementation details",
            "Prefer updating the existing source of truth",
            "preserve operational details",
        },
        "public_help_docs_maintenance": {
            "Preserve stable public URLs and slugs unless a redirect plan exists",
            "Avoid exposing internal implementation details",
            "Do not silently publish customer-facing changes",
        },
        "api_docs_maintenance": {
            "Check for changed endpoints, parameters, response fields, errors, auth, rate limits, pagination, and version notes",
            "Mark deprecations and breaking changes explicitly",
            "Keep examples synchronized with the documented schema",
        },
        "docs_information_architecture": {
            "Organize docs into spaces, collections, and subcollections",
            "Avoid duplicate articles unless the audience or workflow is genuinely different",
            "Maintain naming, ordering, and related-link consistency",
        },
        "release_to_docs_update": {
            "Map shipped changes to internal docs, public help docs, and API docs",
            "Separate user-visible behavior from internal operational changes",
            "Call out uncertainty instead of filling gaps with guesses",
        },
        "support_gap_to_docs": {
            "Read the gap evidence before deciding what to write",
            "Decide whether the gap needs a new article, an update to an existing article, or an information architecture change",
            "Do not close or mark a gap resolved until the doc work is actually created, updated, or explicitly handed off",
        },
    }

    for key, snippets := range cases {
        skill, ok := GetBuiltInSkill(key)
        if !ok {
            t.Fatalf("expected %s built-in skill", key)
        }
        if !containsString(skill.SupportedRuntimes, "native_sdk") {
            t.Fatalf("expected %s to support native_sdk, got %v", key, skill.SupportedRuntimes)
        }
        for _, snippet := range snippets {
            if !strings.Contains(skill.Instructions, snippet) {
                t.Fatalf("expected %s instructions to contain %q\n%s", key, snippet, skill.Instructions)
            }
        }
    }
}
```

- [ ] **Step 3: Run test to verify it fails**

Run:

```bash
cd server
go test ./internal/worker -run 'TestListBuiltInSkillsContainsExpectedKeys|TestDocumentationSkillsDeclareExpectedGuidance' -count=1
```

Expected: FAIL because the new skill packages do not exist yet.

---

### Task 2: Create New Public Help Doc Writing Skill

**Files:**
- Create: `server/skills/system/external_help_doc_writing/SKILL.md`

- [ ] **Step 1: Create the skill package**

Add:

```markdown
---
name: external_help_doc_writing
description: Writing new customer-facing public help center articles.
metadata:
  title: External Help Doc Writing
  supported_runtimes:
    - native_sdk
---

Use this skill when creating a new public help center article.

## Audience

- Write for customers and end users.
- Prefer plain, task-oriented language over internal terminology.
- Avoid exposing internal implementation details, roadmap assumptions, private customer data, or support-only notes.

## Structure

- Start with the user problem or outcome.
- Include prerequisites when setup, permissions, plans, or integrations matter.
- Use ordered steps for procedures and short sections for concepts.
- Include expected results, edge cases, and troubleshooting only when they help the user complete the task.
- Suggest screenshots, diagrams, or media when the article depends on visual UI state, but do not claim an image exists unless it does.

## Placement

- Place the article in the most specific existing collection that matches the user workflow.
- If no collection fits, propose the collection or subcollection name instead of forcing the article into an unrelated area.
- Use a concise, searchable title and a stable slug.
- Add related links when they prevent duplicate explanations.

## Safety

- Do not publish directly unless the run explicitly asks for publishing and the available approval policy allows it.
- Prefer a draft, proposal, or review request for customer-facing changes.
- Call out missing product facts instead of guessing.
```

- [ ] **Step 2: Keep the skill ASCII-only**

Run:

```bash
cd server
LC_ALL=C grep -n '[^ -~]' skills/system/external_help_doc_writing/SKILL.md
```

Expected: no output.

---

### Task 3: Create New API Doc Writing Skill

**Files:**
- Create: `server/skills/system/api_doc_writing/SKILL.md`

- [ ] **Step 1: Create the skill package**

Add:

```markdown
---
name: api_doc_writing
description: Writing new API reference docs and API guides.
metadata:
  title: API Doc Writing
  supported_runtimes:
    - native_sdk
---

Use this skill when creating new API documentation.

## Accuracy

- Do not invent endpoints, fields, limits, or SDK behavior.
- Ground API docs in source code, schemas, route definitions, generated specs, or explicitly provided product facts.
- Call out uncertainty when the implementation or contract is not available.

## Required Coverage

- Document authentication, permissions, request shape, response shape, errors, and examples.
- Include path parameters, query parameters, headers, body fields, enum values, pagination, rate limits, idempotency, and webhooks when applicable.
- Include at least one realistic request example and one realistic response example when the endpoint has a request or response body.
- Show error examples for common failure cases when known.

## Style

- Use precise field names and stable casing.
- Keep examples valid JSON or valid code for the stated language.
- Separate conceptual API guides from endpoint reference pages.
- Mention versioning and compatibility when behavior differs by version or release.
```

- [ ] **Step 2: Keep the skill ASCII-only**

Run:

```bash
cd server
LC_ALL=C grep -n '[^ -~]' skills/system/api_doc_writing/SKILL.md
```

Expected: no output.

---

### Task 4: Create Internal Docs Maintenance Skill

**Files:**
- Create: `server/skills/system/internal_docs_maintenance/SKILL.md`

- [ ] **Step 1: Create the skill package**

Add:

```markdown
---
name: internal_docs_maintenance
description: Keeping internal workspace documentation accurate and useful.
metadata:
  title: Internal Docs Maintenance
  supported_runtimes:
    - native_sdk
---

Use this skill when updating internal workspace docs.

## Purpose

- Internal docs may include implementation details, operating procedures, team ownership, decision history, and private context.
- Preserve operational details that help teammates run, debug, support, sell, or maintain the product.
- Prefer updating the existing source of truth over creating a parallel doc.

## Update Rules

- Compare the current doc with the latest source evidence before editing.
- Keep links, owners, dates, status notes, and related artifacts current.
- Remove or mark stale information instead of leaving contradictions in place.
- Keep internal caveats visible when public docs would omit them.

## Organization

- Keep docs near the team, product area, or workflow that owns them.
- If a doc becomes too broad, propose a split with clear child documents.
- Link internal docs to public docs only when the relationship helps humans maintain both.
```

- [ ] **Step 2: Keep the skill ASCII-only**

Run:

```bash
cd server
LC_ALL=C grep -n '[^ -~]' skills/system/internal_docs_maintenance/SKILL.md
```

Expected: no output.

---

### Task 5: Create Public Help Docs Maintenance Skill

**Files:**
- Create: `server/skills/system/public_help_docs_maintenance/SKILL.md`

- [ ] **Step 1: Create the skill package**

Add:

```markdown
---
name: public_help_docs_maintenance
description: Keeping existing public help center docs accurate and safe.
metadata:
  title: Public Help Docs Maintenance
  supported_runtimes:
    - native_sdk
---

Use this skill when updating existing public help center articles.

## Public Contract

- Avoid exposing internal implementation details, private customer data, support-only notes, or unannounced roadmap items.
- Preserve stable public URLs and slugs unless a redirect plan exists.
- Do not silently publish customer-facing changes.

## Update Workflow

- Identify what changed and whether the existing article still answers the customer problem.
- Keep the article focused on the user workflow rather than the internal feature name.
- Update titles, headings, related links, prerequisites, limitations, and troubleshooting when the product behavior changes.
- Remove outdated promises, screenshots references, or instructions that no longer match the product.

## Review

- Prefer a draft, proposal, or approval request before publishing.
- Call out public-facing risks, unresolved product facts, and redirect needs.
```

- [ ] **Step 2: Keep the skill ASCII-only**

Run:

```bash
cd server
LC_ALL=C grep -n '[^ -~]' skills/system/public_help_docs_maintenance/SKILL.md
```

Expected: no output.

---

### Task 6: Create API Docs Maintenance Skill

**Files:**
- Create: `server/skills/system/api_docs_maintenance/SKILL.md`

- [ ] **Step 1: Create the skill package**

Add:

```markdown
---
name: api_docs_maintenance
description: Keeping existing API docs synchronized with shipped API behavior.
metadata:
  title: API Docs Maintenance
  supported_runtimes:
    - native_sdk
---

Use this skill when updating existing API documentation.

## Change Detection

- Check for changed endpoints, parameters, response fields, errors, auth, rate limits, pagination, and version notes.
- Confirm whether the change is additive, deprecated, breaking, or internal-only.
- Do not document behavior that is not shipped or explicitly approved for docs.

## Updates

- Keep examples synchronized with the documented schema.
- Mark deprecations and breaking changes explicitly.
- Update SDK snippets, curl examples, field tables, enum values, and error examples together.
- Preserve compatibility notes when older clients may observe different behavior.

## Evidence

- Ground claims in route definitions, handlers, schemas, generated specs, tests, changelogs, or release artifacts.
- Call out missing implementation evidence before writing speculative API docs.
```

- [ ] **Step 2: Keep the skill ASCII-only**

Run:

```bash
cd server
LC_ALL=C grep -n '[^ -~]' skills/system/api_docs_maintenance/SKILL.md
```

Expected: no output.

---

### Task 7: Create Docs Information Architecture Skill

**Files:**
- Create: `server/skills/system/docs_information_architecture/SKILL.md`

- [ ] **Step 1: Create the skill package**

Add:

```markdown
---
name: docs_information_architecture
description: Organizing docs into coherent spaces, collections, subcollections, names, links, and navigation.
metadata:
  title: Docs Information Architecture
  supported_runtimes:
    - native_sdk
---

Use this skill when organizing or reorganizing documentation.

## Organization

- Organize docs into spaces, collections, and subcollections based on audience, workflow, and product area.
- Maintain naming, ordering, and related-link consistency.
- Prefer the most specific useful location over broad catch-all collections.
- Avoid duplicate articles unless the audience or workflow is genuinely different.

## Harmony Rules

- Keep titles parallel within a collection.
- Keep article depth consistent across similar workflows.
- Use redirects or link updates when moving public docs.
- Preserve internal source-of-truth links when splitting or merging docs.

## Reorganization Workflow

- Inventory the affected docs before proposing moves.
- Identify duplicates, stale pages, overloaded collections, missing landing pages, and orphaned articles.
- Propose the new structure before making broad moves.
- Keep content changes separate from pure organization changes when practical.
```

- [ ] **Step 2: Keep the skill ASCII-only**

Run:

```bash
cd server
LC_ALL=C grep -n '[^ -~]' skills/system/docs_information_architecture/SKILL.md
```

Expected: no output.

---

### Task 8: Create Release To Docs Update Skill

**Files:**
- Create: `server/skills/system/release_to_docs_update/SKILL.md`

- [ ] **Step 1: Create the skill package**

Add:

```markdown
---
name: release_to_docs_update
description: Updating internal docs, public help docs, and API docs from released product changes.
metadata:
  title: Release To Docs Update
  supported_runtimes:
    - native_sdk
---

Use this skill when a release, feature, task, epic, or changelog requires documentation updates.

## Triage

- Map shipped changes to internal docs, public help docs, and API docs.
- Separate user-visible behavior from internal operational changes.
- Identify whether the change creates a new workflow, changes an existing workflow, deprecates behavior, or fixes stale docs.

## Update Plan

- Update public docs for customer-visible behavior.
- Update internal docs for operating procedures, implementation context, ownership, rollout notes, and support context.
- Update API docs for changed contracts, examples, errors, auth, limits, versioning, and deprecations.
- Include information architecture changes when new docs need a home.

## Evidence

- Ground updates in release notes, merged tasks, PRs, specs, product screenshots, support context, or explicit human instructions.
- Call out uncertainty instead of filling gaps with guesses.
- Prefer a concise checklist of affected docs when the release touches multiple surfaces.
```

- [ ] **Step 2: Keep the skill ASCII-only**

Run:

```bash
cd server
LC_ALL=C grep -n '[^ -~]' skills/system/release_to_docs_update/SKILL.md
```

Expected: no output.

---

### Task 9: Create Support Gap To Docs Skill

**Files:**
- Create: `server/skills/system/support_gap_to_docs/SKILL.md`

- [ ] **Step 1: Create the skill package**

Add:

```markdown
---
name: support_gap_to_docs
description: Converting support coverage gaps into missing, weak, or stale documentation work.
metadata:
  title: Support Gap To Docs
  supported_runtimes:
    - native_sdk
---

Use this skill when working from a support coverage gap, repeated customer question, weak article signal, missing article signal, or stale article signal.

## Evidence First

- Read the gap evidence before deciding what to write.
- Identify the customer question, failed search, weak answer, stale article, or conflicting article.
- Distinguish one-off confusion from a durable documentation gap.

## Decide The Doc Action

- Decide whether the gap needs a new article, an update to an existing article, or an information architecture change.
- Prefer updating an existing relevant article when the customer intent already has a home.
- Create a new article only when the topic is missing or the existing article would become unfocused.
- Link related articles when the answer spans multiple workflows.

## Completion Rules

- Do not close or mark a gap resolved until the doc work is actually created, updated, or explicitly handed off.
- Preserve the evidence trail so reviewers understand why the doc changed.
- For public help docs, draft or request approval before publishing.
```

- [ ] **Step 2: Keep the skill ASCII-only**

Run:

```bash
cd server
LC_ALL=C grep -n '[^ -~]' skills/system/support_gap_to_docs/SKILL.md
```

Expected: no output.

---

### Task 10: Verify Skill Loader And Catalog

**Files:**
- Test: `server/internal/worker/skill_catalog_test.go`
- Test: `server/internal/worker/skill_loader_test.go`

- [ ] **Step 1: Run targeted worker tests**

Run:

```bash
cd server
go test ./internal/worker -count=1
```

Expected: PASS.

- [ ] **Step 2: Run skill-related service tests**

Run:

```bash
cd server
go test ./internal/agentskills ./internal/worker -count=1
```

Expected: PASS.

- [ ] **Step 3: Run broader backend compile check**

Run:

```bash
cd server
go test ./internal/worker ./internal/agentskills ./internal/service -count=1
```

Expected: PASS.

---

### Task 11: Prepare The Follow-Up Documentation Agent Plan

**Files:**
- No code changes in this task.

- [ ] **Step 1: Record the next integration points**

After the skills pass, the next plan should add:

- `model.AgentPresetDocumentationAgent = "documentation_agent"` in `server/internal/model/agent.go`.
- A Documentation Agent preset bundle in `server/internal/worker/skill_catalog.go` using:
  - `docs_information_architecture`
  - `external_help_doc_writing`
  - `api_doc_writing`
  - `internal_docs_maintenance`
  - `public_help_docs_maintenance`
  - `api_docs_maintenance`
  - `release_to_docs_update`
  - `support_gap_to_docs`
  - likely `approval_protocol`
  - likely `general_agent_behavior`
- Runtime/default policy in `server/internal/service/agent_policy.go`.
- Seeding/reconciliation coverage in `server/internal/service/agent_create.go`.
- Frontend preset type/fallback display updates in:
  - `frontend/src/lib/pm-types/agents.ts`
  - `frontend/src/pages/automation/Agents.tsx`
  - `frontend/src/lib/presetStyles.ts`
- Docs-specific selective activation in `server/internal/agentskills/activation.go` and `server/internal/temporalapp/native_skill_activation.go`.

- [ ] **Step 2: Commit the skills change**

Run:

```bash
git add server/skills/system server/internal/worker/skill_catalog_test.go
git commit -m "feat: add documentation agent skills"
```

Expected: commit succeeds.
