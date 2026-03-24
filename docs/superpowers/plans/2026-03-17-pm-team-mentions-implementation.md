# PM Team Mentions Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement real `@team-handle` mention semantics across PM so team mentions notify current team members with access, support all approved PM surfaces, and render sensibly in the UI.

**Architecture:** Add a shared backend PM mention resolver that expands raw handles into readable recipient user IDs for user and team mentions, then reuse it from story, comment, checklist, epic, sprint, and objective services. Keep the current editor model, add team support where checklist authoring is still user-only, and extend mention rendering to distinguish teams from people where the UI already renders mention-aware text.

**Tech Stack:** Go 1.24, GORM/SQLite test harness, React 19, TypeScript, Vitest

---

### Task 1: Backend mention resolution core

**Files:**
- Create: `server/internal/service/pm_mentions.go`
- Modify: `server/internal/repository/workspace.go`
- Modify: `server/internal/service/pm_comment.go`
- Test: `server/internal/service/pm_mentions_test.go`

- [ ] **Step 1: Write failing resolver tests**
- [ ] **Step 2: Run resolver tests to verify red**
- [ ] **Step 3: Implement shared handle extraction, team expansion, dedupe, actor exclusion, and readability filtering**
- [ ] **Step 4: Wire comment service to the shared resolver**
- [ ] **Step 5: Re-run resolver/comment tests to verify green**

### Task 2: Story/checklist backend integration

**Files:**
- Modify: `server/internal/service/pm_story.go`
- Modify: `server/internal/service/pm_checklist_item.go`
- Test: `server/internal/service/pm_story_mentions_test.go`

- [ ] **Step 1: Write failing story/checklist mention tests**
- [ ] **Step 2: Run the focused tests to verify red**
- [ ] **Step 3: Replace duplicated user-only resolution with shared mention resolution**
- [ ] **Step 4: Re-run focused tests to verify green**

### Task 3: Epic/sprint/objective backend mention fan-out

**Files:**
- Modify: `server/internal/service/pm_epic.go`
- Modify: `server/internal/service/pm_sprint.go`
- Modify: `server/internal/service/pm_objective.go`
- Modify: `server/internal/model/notification.go`
- Modify: `server/internal/model/notification_category_test.go`
- Test: `server/internal/service/pm_planning_mentions_test.go`

- [ ] **Step 1: Write failing epic/sprint/objective mention tests**
- [ ] **Step 2: Run the focused tests to verify red**
- [ ] **Step 3: Emit mention notifications on create/update and map `sprint.mention` into notification categories**
- [ ] **Step 4: Re-run focused tests to verify green**

### Task 4: Frontend mention rendering and checklist team autocomplete

**Files:**
- Modify: `frontend/src/components/pm/MentionText.tsx`
- Modify: `frontend/src/components/pm/CommentBody.tsx`
- Modify: `frontend/src/components/pm/ChecklistItems.tsx`
- Modify: `frontend/src/pages/pm/StoryDetail.tsx`
- Modify: `frontend/src/components/pm/StoryDetailPanel.tsx`
- Test: `frontend/src/components/pm/__tests__/MentionText.test.tsx`
- Test: `frontend/src/components/pm/__tests__/ChecklistItems.test.tsx`

- [ ] **Step 1: Write failing frontend tests for team rendering and checklist autocomplete**
- [ ] **Step 2: Run the focused frontend tests to verify red**
- [ ] **Step 3: Implement team-aware rendering and checklist team suggestions**
- [ ] **Step 4: Re-run focused frontend tests to verify green**

### Task 5: Verification

**Files:**
- Modify: `docs/superpowers/plans/2026-03-17-pm-team-mentions-implementation.md`

- [ ] **Step 1: Run focused Go tests for mention resolution and PM services**
- [ ] **Step 2: Run focused frontend Vitest suites**
- [ ] **Step 3: Run broader backend/frontend verification where feasible**
- [ ] **Step 4: Review diff for spec compliance and regressions**
