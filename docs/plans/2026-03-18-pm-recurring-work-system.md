# PM Recurring Work System Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a first-class recurring work system for PM that lets users define recurring story templates, generate normal story instances from them, and manage lifecycle state from both story UI and a dedicated settings screen.

**Architecture:** Add a dedicated recurring-template subsystem in the backend, backed by database state plus a singleton Temporal cron workflow for due-template processing. Frontend work should add recurrence controls to story creation/detail surfaces and a `Settings -> Recurring Tasks` management screen, while generated stories remain normal stories linked back to the recurring template and run log.

**Tech Stack:** Go 1.24, GORM, Chi, Temporal, PostgreSQL, React 19, Vite 7, TypeScript 5.9, TanStack Router, TanStack Query, Zustand, shadcn/ui

---

## File Structure

### Backend

- Create: `server/internal/model/pm_recurring_template.go`
- Create: `server/internal/repository/pm_recurring_template.go`
- Create: `server/internal/service/pm_recurring_template.go`
- Create: `server/internal/handler/pm_recurring_template.go`
- Create: `server/internal/service/pm_recurring_template_test.go`
- Create: `server/internal/temporalapp/pm_recurring_workflow.go`
- Create: `server/internal/temporalapp/pm_recurring_workflow_test.go` if workflow-level tests are practical, otherwise keep runtime tests in service
- Modify: `server/internal/model/pm_story.go`
- Modify: `server/internal/repository/pm_story.go`
- Modify: `server/internal/service/pm_story.go`
- Modify: `server/internal/service/pm_story_test.go`
- Modify: `server/internal/handler/pm_story.go`
- Modify: `server/internal/router/router.go`
- Modify: `server/internal/temporalapp/engine.go`
- Modify: `server/cmd/api/main.go`

### Frontend

- Create: `frontend/src/components/pm/RecurringTemplateForm.tsx`
- Create: `frontend/src/components/pm/RecurringTemplateSummary.tsx`
- Create: `frontend/src/components/pm/RecurringTemplateList.tsx`
- Create: `frontend/src/components/pm/RecurringTemplateBadge.tsx`
- Create: `frontend/src/components/pm/__tests__/RecurringTemplateForm.test.tsx`
- Create: `frontend/src/components/pm/__tests__/RecurringTemplateSummary.test.tsx`
- Create: `frontend/src/components/pm/__tests__/RecurringTemplateList.test.tsx`
- Create: `frontend/src/lib/services/pmRecurringTemplateService.ts`
- Modify: `frontend/src/lib/pmTypes.ts`
- Modify: `frontend/src/lib/queryKeys.ts`
- Modify: `frontend/src/pages/Settings.tsx`
- Modify: `frontend/src/components/layout/Sidebar.tsx`
- Modify: `frontend/src/components/pm/CreateStoryModal.tsx`
- Modify: `frontend/src/pages/pm/StoryDetail.tsx`
- Modify: `frontend/src/components/pm/StoryDetailPanel.tsx`
- Modify: `frontend/src/components/pm/StoryCard.tsx`
- Modify: `frontend/src/components/pm/StoryListView.tsx`

## Task 1: Backend recurring models and rule validation

**Files:**
- Create: `server/internal/model/pm_recurring_template.go`
- Modify: `server/internal/model/pm_story.go`
- Test: `server/internal/service/pm_recurring_template_test.go`

- [ ] **Step 1: Write failing backend tests for recurring rule validation and story linkage fields**

Cover:
- valid time-based rules
- valid completion-based rules
- invalid mixed-rule combinations
- start/end constraints
- due-date offset constraints
- story linkage field serialization

- [ ] **Step 2: Run the focused tests and confirm they fail**

Run: `go test ./internal/service -run 'TestRecurringTemplateValidation|TestRecurringStoryLinkFields'`
Expected: FAIL because recurring models and validation do not exist yet.

- [ ] **Step 3: Add recurring template model types and story linkage fields**

Implement:
- recurring status constants
- recurrence mode enums
- recurrence rule payload structs
- template/run models
- request/response DTOs
- `PMStory` fields for recurring template/run/occurrence linkage

- [ ] **Step 4: Add validation helpers and minimal model tests**

Implement only the validation needed to satisfy the focused tests.

- [ ] **Step 5: Re-run the focused tests**

Run: `go test ./internal/service -run 'TestRecurringTemplateValidation|TestRecurringStoryLinkFields'`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add server/internal/model/pm_recurring_template.go server/internal/model/pm_story.go server/internal/service/pm_recurring_template_test.go
git commit -m "feat: add recurring template models"
```

## Task 2: Repository layer and run-log persistence

**Files:**
- Create: `server/internal/repository/pm_recurring_template.go`
- Test: `server/internal/service/pm_recurring_template_test.go`

- [ ] **Step 1: Write failing tests for recurring template CRUD and run-log persistence**

Cover:
- create/list/update template
- insert run history
- claim due templates
- dedupe-key conflict handling

- [ ] **Step 2: Run the focused tests and confirm they fail**

Run: `go test ./internal/service -run 'TestRecurringTemplateRepository'`
Expected: FAIL because repository methods are missing.

- [ ] **Step 3: Implement repository methods**

Add:
- template CRUD
- due-template query helpers
- transactional run creation helpers
- lifecycle updates for pause/resume/stop/skip

- [ ] **Step 4: Re-run the focused tests**

Run: `go test ./internal/service -run 'TestRecurringTemplateRepository'`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add server/internal/repository/pm_recurring_template.go server/internal/service/pm_recurring_template_test.go
git commit -m "feat: add recurring template persistence"
```

## Task 3: Recurring service, story generation, and lifecycle actions

**Files:**
- Create: `server/internal/service/pm_recurring_template.go`
- Modify: `server/internal/service/pm_story.go`
- Modify: `server/internal/repository/pm_story.go`
- Test: `server/internal/service/pm_recurring_template_test.go`
- Test: `server/internal/service/pm_story_test.go`

- [ ] **Step 1: Write failing tests for template creation, due processing, completion-triggered generation, and lifecycle actions**

Cover:
- create recurring template from story payload
- generate story on schedule
- generate next on completion
- pause/resume/stop behavior
- skip next occurrence
- generate now
- duplicate prevention on retry

- [ ] **Step 2: Run the focused tests and confirm they fail**

Run: `go test ./internal/service -run 'TestRecurringTemplateService|TestStoryCompletionAdvancesRecurringTemplate'`
Expected: FAIL because service logic does not exist yet.

- [ ] **Step 3: Implement recurring service**

Implement:
- create/update/list/detail service methods
- next-run calculation
- story-generation orchestration
- run-log recording
- lifecycle action handlers
- story-completion hook entrypoint

- [ ] **Step 4: Hook recurring advancement into story update/completion flows**

Call recurring service when:
- story becomes completed
- story moves to a done state

- [ ] **Step 5: Re-run the focused tests**

Run: `go test ./internal/service -run 'TestRecurringTemplateService|TestStoryCompletionAdvancesRecurringTemplate'`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add server/internal/service/pm_recurring_template.go server/internal/service/pm_story.go server/internal/repository/pm_story.go server/internal/service/pm_recurring_template_test.go server/internal/service/pm_story_test.go
git commit -m "feat: add recurring story generation service"
```

## Task 4: HTTP handlers and routing

**Files:**
- Create: `server/internal/handler/pm_recurring_template.go`
- Modify: `server/internal/handler/pm_story.go`
- Modify: `server/internal/router/router.go`
- Modify: `server/cmd/api/main.go`
- Test: `server/internal/service/pm_recurring_template_test.go`

- [ ] **Step 1: Write failing handler tests or service-level endpoint-shape tests**

Cover:
- recurring template CRUD endpoints
- pause/resume/stop/skip/generate-now actions
- story detail recurring summary response

- [ ] **Step 2: Run the focused tests and confirm they fail**

Run: `go test ./internal/service -run 'TestRecurringTemplateHTTPShape'`
Expected: FAIL because handlers/routes are missing.

- [ ] **Step 3: Implement handlers, DI wiring, and routes**

Add:
- `/api/pm/recurring-templates`
- action routes for lifecycle operations
- story recurring-summary route or detail enrichment

- [ ] **Step 4: Re-run the focused tests**

Run: `go test ./internal/service -run 'TestRecurringTemplateHTTPShape'`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add server/internal/handler/pm_recurring_template.go server/internal/handler/pm_story.go server/internal/router/router.go server/cmd/api/main.go
git commit -m "feat: expose recurring template APIs"
```

## Task 5: Temporal recurring runner

**Files:**
- Create: `server/internal/temporalapp/pm_recurring_workflow.go`
- Modify: `server/internal/temporalapp/engine.go`
- Modify: `server/cmd/api/main.go`
- Test: `server/internal/service/pm_recurring_template_test.go`

- [ ] **Step 1: Write failing tests around due-template processing idempotency**

Cover:
- singleton cron workflow startup does not duplicate
- processing due templates twice does not create duplicate stories

- [ ] **Step 2: Run the focused tests and confirm they fail**

Run: `go test ./internal/service -run 'TestRecurringTemplateProcessingIsIdempotent'`
Expected: FAIL because the scheduler hook is missing.

- [ ] **Step 3: Implement the singleton Temporal cron workflow and startup hook**

Follow the existing CRM summary pattern:
- fixed workflow ID
- ignore already-started errors
- short cron cadence
- activity that delegates to the recurring service

- [ ] **Step 4: Re-run the focused tests**

Run: `go test ./internal/service -run 'TestRecurringTemplateProcessingIsIdempotent'`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add server/internal/temporalapp/pm_recurring_workflow.go server/internal/temporalapp/engine.go server/cmd/api/main.go server/internal/service/pm_recurring_template_test.go
git commit -m "feat: add recurring template scheduler"
```

## Task 6: Frontend types, services, and shared recurrence UI

**Files:**
- Create: `frontend/src/lib/services/pmRecurringTemplateService.ts`
- Create: `frontend/src/components/pm/RecurringTemplateForm.tsx`
- Create: `frontend/src/components/pm/RecurringTemplateSummary.tsx`
- Modify: `frontend/src/lib/pmTypes.ts`
- Modify: `frontend/src/lib/queryKeys.ts`
- Test: `frontend/src/components/pm/__tests__/RecurringTemplateForm.test.tsx`
- Test: `frontend/src/components/pm/__tests__/RecurringTemplateSummary.test.tsx`

- [ ] **Step 1: Write failing UI tests for recurrence form state and summary rendering**

Cover:
- time-based options
- completion-based options
- start/end controls
- due-date behavior
- summary rendering for generated stories

- [ ] **Step 2: Run the focused tests and confirm they fail**

Run: `npx vitest run src/components/pm/__tests__/RecurringTemplateForm.test.tsx src/components/pm/__tests__/RecurringTemplateSummary.test.tsx`
Expected: FAIL because types/services/components do not exist yet.

- [ ] **Step 3: Implement frontend types, query keys, API service, and shared recurrence components**

- [ ] **Step 4: Re-run the focused tests**

Run: `npx vitest run src/components/pm/__tests__/RecurringTemplateForm.test.tsx src/components/pm/__tests__/RecurringTemplateSummary.test.tsx`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add frontend/src/lib/pmTypes.ts frontend/src/lib/queryKeys.ts frontend/src/lib/services/pmRecurringTemplateService.ts frontend/src/components/pm/RecurringTemplateForm.tsx frontend/src/components/pm/RecurringTemplateSummary.tsx frontend/src/components/pm/__tests__/RecurringTemplateForm.test.tsx frontend/src/components/pm/__tests__/RecurringTemplateSummary.test.tsx
git commit -m "feat: add recurring template frontend primitives"
```

## Task 7: Story create/detail integration

**Files:**
- Modify: `frontend/src/components/pm/CreateStoryModal.tsx`
- Modify: `frontend/src/pages/pm/StoryDetail.tsx`
- Modify: `frontend/src/components/pm/StoryDetailPanel.tsx`
- Create: `frontend/src/components/pm/RecurringTemplateBadge.tsx`
- Test: `frontend/src/components/pm/__tests__/RecurringTemplateSummary.test.tsx`

- [ ] **Step 1: Write failing tests for story create/edit recurrence integration**

Cover:
- enabling recurring from create flow
- editing recurring from detail
- `Make recurring` quick action
- generated-story badge / parent-link rendering

- [ ] **Step 2: Run the focused tests and confirm they fail**

Run: `npx vitest run src/components/pm/__tests__/RecurringTemplateSummary.test.tsx`
Expected: FAIL because story surfaces are not wired yet.

- [ ] **Step 3: Wire recurrence into create/detail/panel story surfaces**

Implement:
- create-story recurring section
- detail recurring summary
- quick action to convert story to recurring template
- generated story badge

- [ ] **Step 4: Re-run the focused tests**

Run: `npx vitest run src/components/pm/__tests__/RecurringTemplateSummary.test.tsx`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add frontend/src/components/pm/CreateStoryModal.tsx frontend/src/pages/pm/StoryDetail.tsx frontend/src/components/pm/StoryDetailPanel.tsx frontend/src/components/pm/RecurringTemplateBadge.tsx frontend/src/components/pm/__tests__/RecurringTemplateSummary.test.tsx
git commit -m "feat: add recurrence to story workflows"
```

## Task 8: Recurring management screen

**Files:**
- Create: `frontend/src/components/pm/RecurringTemplateList.tsx`
- Create: `frontend/src/components/pm/__tests__/RecurringTemplateList.test.tsx`
- Modify: `frontend/src/pages/Settings.tsx`
- Modify: `frontend/src/components/layout/Sidebar.tsx`

- [ ] **Step 1: Write failing tests for the recurring management view**

Cover:
- list rendering
- status filters
- search
- lifecycle actions
- last-run / next-run display

- [ ] **Step 2: Run the focused tests and confirm they fail**

Run: `npx vitest run src/components/pm/__tests__/RecurringTemplateList.test.tsx`
Expected: FAIL because the settings surface does not exist yet.

- [ ] **Step 3: Implement the settings page and sidebar wiring**

Add:
- `Settings -> Recurring Tasks` section
- list/search/filter UI
- row actions and mutation wiring

- [ ] **Step 4: Re-run the focused tests**

Run: `npx vitest run src/components/pm/__tests__/RecurringTemplateList.test.tsx`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add frontend/src/components/pm/RecurringTemplateList.tsx frontend/src/components/pm/__tests__/RecurringTemplateList.test.tsx frontend/src/pages/Settings.tsx frontend/src/components/layout/Sidebar.tsx
git commit -m "feat: add recurring task management screen"
```

## Task 9: Story list/board indicators and final verification

**Files:**
- Modify: `frontend/src/components/pm/StoryCard.tsx`
- Modify: `frontend/src/components/pm/StoryListView.tsx`
- Modify: `server/internal/service/pm_recurring_template_test.go`

- [ ] **Step 1: Add the final recurring-origin indicators**

Implement:
- recurring badge/icon in board card
- recurring badge/icon in list rows
- parent-template link where room allows

- [ ] **Step 2: Run backend verification**

Run: `go test ./internal/service/...`
Expected: PASS

- [ ] **Step 3: Run frontend verification**

Run: `./node_modules/.bin/tsc -p tsconfig.json --noEmit --incremental false`
Expected: PASS

Run: `NODE_OPTIONS=--max-old-space-size=4096 npx vite build`
Expected: PASS

- [ ] **Step 4: Run focused PM vitest coverage**

Run: `npx vitest run src/components/pm/__tests__/RecurringTemplateForm.test.tsx src/components/pm/__tests__/RecurringTemplateSummary.test.tsx src/components/pm/__tests__/RecurringTemplateList.test.tsx`
Expected: PASS

- [ ] **Step 5: Manual QA**

Validate:
- create recurring story
- edit recurring rule
- pause/resume/stop
- skip next
- generate now
- scheduled generation
- completion-based generation
- generated story badge and parent-link behavior

- [ ] **Step 6: Commit**

```bash
git add frontend/src/components/pm/StoryCard.tsx frontend/src/components/pm/StoryListView.tsx server/internal/service/pm_recurring_template_test.go
git commit -m "feat: finalize recurring story UX"
```
