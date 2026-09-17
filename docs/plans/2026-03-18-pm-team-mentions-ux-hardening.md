# PM Same-Team Mentions Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Restrict PM team mentions to the entity's own team scope while making the allowed team handle easy to find in autocomplete across PM authoring surfaces.

**Architecture:** Enforce same-team-only semantics in both layers. On the backend, resolve `@team-handle` only when the mentioned team belongs to the entity's allowed team IDs. On the frontend, pass only the entity-scoped teams into mention-aware editors and use a shared ranking helper so the allowed team handle remains visible even when many members match the same query.

**Tech Stack:** Go 1.24, React 19, TypeScript 5.9, TipTap 3, Vitest

---

### Task 1: Enforce same-team-only team mentions in backend resolution

**Files:**
- Modify: `server/internal/service/pm_mentions.go`
- Modify: `server/internal/service/pm_comment.go`
- Modify: `server/internal/service/pm_checklist_item.go`
- Modify: `server/internal/service/pm_story.go`
- Modify: `server/internal/service/pm_epic.go`
- Modify: `server/internal/service/pm_sprint.go`
- Modify: `server/internal/service/pm_objective.go`
- Modify: `server/internal/service/pm_mentions_test.go`

- [x] **Step 1: Write failing backend tests for out-of-scope team handles**

Add focused cases covering:
- a story for Engineering mentioning `@design` does not notify Design members
- a story/comment/checklist still notifies the story's own team handle
- an objective only allows mentions for teams already attached to that objective

- [x] **Step 2: Run the focused backend tests to verify red**

Run:
`go test ./internal/service -run 'TestResolveMentionRecipients_TeamMentionsUseReadableTeamMembers|TestResolveMentionRecipients_IgnoresOutOfScopeTeamHandles|TestPMStoryService_CreateAndUpdate_TeamMentions|TestPMCommentService_CreateAndUpdate_TeamMentions|TestPMChecklistItemService_CreateAndUpdate_TeamMentions|TestPMPlanningEntityServices_TeamMentions'`

Workdir: `server`

Expected: FAIL because out-of-scope team handles are still expanded today.

- [x] **Step 3: Implement allowed-team filtering in the shared resolver**

Update `pm_mentions.go` to:
- use the existing entity team-scope input as the allowed mention team scope
- skip any matched team handle whose team ID is not in the allowed scope
- preserve existing person-handle resolution behavior
- keep actor exclusion and readable-recipient filtering intact

- [x] **Step 4: Thread allowed team IDs through each PM entity flow**

Use:
- story/comment/checklist: current story team only
- epic/sprint: current entity team only
- objective: the objective's attached team IDs

- [x] **Step 5: Re-run the focused backend tests to verify green**

Run:
`go test ./internal/service -run 'TestResolveMentionRecipients_TeamMentionsUseReadableTeamMembers|TestResolveMentionRecipients_IgnoresOutOfScopeTeamHandles|TestPMStoryService_CreateAndUpdate_TeamMentions|TestPMCommentService_CreateAndUpdate_TeamMentions|TestPMChecklistItemService_CreateAndUpdate_TeamMentions|TestPMPlanningEntityServices_TeamMentions'`

Expected: PASS.

### Task 2: Share mention ranking so the allowed team stays visible

**Files:**
- Create: `frontend/src/components/pm/mentionSuggestions.ts`
- Create: `frontend/src/components/pm/__tests__/mentionSuggestions.test.ts`
- Modify: `frontend/src/components/pm/ChecklistItems.tsx`
- Modify: `frontend/src/components/pm/CommentEditor.tsx`
- Modify: `frontend/src/components/ui/tiptap-editor.tsx`

- [x] **Step 1: Write failing frontend ranking tests**

Cover:
- exact team-handle query stays in the result slice
- team and member matches can coexist in the visible results
- inactive members stay excluded
- the result limit does not hide an exact team-handle match behind many member matches

- [x] **Step 2: Run the focused ranking tests to verify red**

Run:
`npx vitest run src/components/pm/__tests__/mentionSuggestions.test.ts`

Expected: FAIL because the shared ranking helper does not exist yet.

- [x] **Step 3: Implement a shared suggestion helper**

In `mentionSuggestions.ts`, add:
- shared `buildMemberHandle`
- shared mention option type
- `getMentionSuggestions(query, members, teams, limit)` with exact/prefix-handle ranking

- [x] **Step 4: Replace duplicated member-first list building in all three editors**

Update:
- `ChecklistItems.tsx`
- `CommentEditor.tsx`
- `tiptap-editor.tsx`

All three should call the same helper and keep current keyboard behavior.

- [x] **Step 5: Re-run the focused frontend tests to verify green**

Run:
`npx vitest run src/components/pm/__tests__/mentionSuggestions.test.ts src/components/pm/__tests__/ChecklistItems.test.tsx`

Expected: PASS.

### Task 3: Pass only entity-scoped teams into PM mention surfaces

**Files:**
- Modify: `frontend/src/pages/pm/StoryDetail.tsx`
- Modify: `frontend/src/components/pm/StoryDetailPanel.tsx`
- Modify: `frontend/src/pages/pm/EpicDetail.tsx`
- Modify: `frontend/src/pages/pm/SprintDetail.tsx`
- Modify: `frontend/src/pages/pm/ObjectiveDetail.tsx`
- Modify: `frontend/src/components/pm/CreateStoryModal.tsx`
- Modify: `frontend/src/components/pm/GlobalCreateModals.tsx`

- [x] **Step 1: Write or extend failing tests for scoped team inputs where practical**

At minimum cover pure helper behavior if the page-level components are too expensive to mount. Prefer extracting a tiny `filterMentionTeams(...)` helper if that makes the scope testable.

- [x] **Step 2: Run the focused scope tests to verify red**

Run the smallest relevant Vitest target for the extracted helper or page/component test.

- [x] **Step 3: Filter teams before passing them to mention-aware editors**

Apply these rules:
- story surfaces: only the current story team
- epic/sprint surfaces: only the current entity team
- objective surfaces: only teams already attached to the objective
- create flows: only the currently selected team(s); if none selected yet, provide no team suggestions

- [x] **Step 4: Re-run the focused scope tests to verify green**

Run the same Vitest target from Step 2.

Expected: PASS.

### Task 4: Verification

**Files:**
- Modify: `docs/plans/2026-03-18-pm-team-mentions-ux-hardening.md`

- [x] **Step 1: Run the mention-focused frontend suite**

Run:
`npx vitest run src/components/pm/__tests__/mentionSuggestions.test.ts src/components/pm/__tests__/ChecklistItems.test.tsx src/components/pm/__tests__/MentionText.test.tsx src/components/pm/__tests__/RichTextMentionContent.test.tsx`

Expected: PASS.

- [x] **Step 2: Run the mention-focused backend suite**

Run:
`go test ./internal/service -run 'TestResolveMentionRecipients_TeamMentionsUseReadableTeamMembers|TestResolveMentionRecipients_IgnoresOutOfScopeTeamHandles|TestPMStoryService_CreateAndUpdate_TeamMentions|TestPMCommentService_CreateAndUpdate_TeamMentions|TestPMChecklistItemService_CreateAndUpdate_TeamMentions|TestPMPlanningEntityServices_TeamMentions|TestEmit_SkipFollowersLimitsDeliveryToExplicitRecipients'`

Workdir: `server`

Expected: PASS.

- [x] **Step 3: Run a frontend build and record unrelated blockers**

Run: `npm run build`

Workdir: `frontend`

Expected: either PASS, or fail only on the already-known unrelated TypeScript issues in `src/components/settings/MembersTab.tsx`, `src/components/settings/TeamsTab.tsx`, or `src/pages/Workspaces.tsx`.

Observed result: FAIL only on the already-known unrelated TypeScript issues in `src/components/settings/MembersTab.tsx`, `src/components/settings/TeamsTab.tsx`, and `src/pages/Workspaces.tsx`.

- [ ] **Step 4: Manual QA on highest-risk flows**

Verify:
- story description only suggests the story's own team handle
- comment editor only suggests the story's own team handle
- checklist input only suggests the story's own team handle
- another workspace team handle typed manually does not produce notifications
- the allowed team handle still stays visible under crowded member matches

- [x] **Step 5: Review the diff against the product rule**

Confirm:
- same-team-only is enforced in backend and frontend
- autocomplete does not over-suggest other teams
- the remaining allowed team mention is discoverable
