# PM Editor Image Attachments Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add paste/drop image upload to all current PM rich-text editors, store those images as PM attachments, render them inline, and keep create-flow/comment-flow attachment ownership correct.

**Architecture:** Reuse the PM attachment backend as the only storage path, extend it to objective and sprint entities, and generalize create-time reassignment beyond comments. On the frontend, make editor uploads attachment-aware by storing `attachmentId` on inline image nodes, staging unsaved uploads as `editor_upload`, and integrating those attachment IDs into create flows, description rendering, and comment rendering without duplicate image cards.

**Tech Stack:** Go services/repositories with GORM, React 19 + TypeScript, TipTap, existing PM attachment REST endpoints, Vitest, Go test.

---

### Task 1: Backend Attachment Entity Support And Reassignment

**Files:**
- Modify: `server/internal/model/pm_attachment.go`
- Modify: `server/internal/model/pm_story.go`
- Modify: `server/internal/model/pm_epic.go`
- Modify: `server/internal/model/pm_objective.go`
- Modify: `server/internal/model/pm_sprint.go`
- Modify: `server/internal/repository/pm_attachment.go`
- Modify: `server/internal/service/pm_attachment.go`
- Modify: `server/internal/service/pm_story.go`
- Modify: `server/internal/service/pm_epic.go`
- Modify: `server/internal/service/pm_objective.go`
- Modify: `server/internal/service/pm_sprint.go`
- Modify: `server/internal/service/pm_comment.go`
- Test: `server/internal/service/pm_attachment_test.go` or existing PM attachment/service tests
- Test: `server/internal/service/pm_story_test.go`
- Test: `server/internal/service/pm_epic_test.go`
- Test: `server/internal/service/pm_objective_test.go`
- Test: `server/internal/service/pm_sprint_test.go`

- [ ] **Step 1: Write failing backend tests**

Add tests for:
- allowing `objective` and `sprint` attachment entity types
- generic attachment reassignment to `story`, `epic`, `objective`, `sprint`, and `comment`
- create-flow reassignment from temporary uploads for story/epic/objective/sprint

- [ ] **Step 2: Run backend tests to verify they fail**

Run:
```bash
go test ./internal/service -run 'TestPMAttachment|TestPMStory|TestPMEpic|TestPMObjective|TestPMSprint'
```

- [ ] **Step 3: Implement minimal backend changes**

Implement:
- expanded allowed attachment entity types
- generic `ReassignToEntity` repository/service support
- create DTO `attachment_ids` for story/epic/objective/sprint
- create-service reassignment using the generic path
- comment service switched from comment-only reassignment helper to the generic path

- [ ] **Step 4: Run backend tests to verify they pass**

Run:
```bash
go test ./internal/service -run 'TestPMAttachment|TestPMStory|TestPMEpic|TestPMObjective|TestPMSprint'
```

### Task 2: Shared Frontend Attachment Metadata Plumbing

**Files:**
- Modify: `frontend/src/hooks/useEditorImageUpload.ts`
- Modify: `frontend/src/components/ui/resizable-image-extension.ts`
- Modify: `frontend/src/components/ui/resizable-image-component.tsx`
- Modify: `frontend/src/components/ui/tiptap-editor.tsx`
- Modify: `frontend/src/components/pm/CommentEditor.tsx`
- Create: `frontend/src/components/pm/editorImageAttachments.ts`
- Test: `frontend/src/components/pm/__tests__/editorImageAttachments.test.ts`
- Test: `frontend/src/components/pm/__tests__/TiptapEditor.test.tsx` or existing editor tests
- Test: `frontend/src/components/pm/__tests__/CommentEditor.test.tsx` or existing comment tests

- [ ] **Step 1: Write failing frontend unit tests for attachment metadata**

Cover:
- extracting inline attachment IDs from HTML
- removing inline images by `attachmentId`
- upload helper returning attachment metadata instead of URL only
- image node parse/render preserving `attachmentId`

- [ ] **Step 2: Run frontend tests to verify they fail**

Run:
```bash
npx vitest run src/components/pm/__tests__/editorImageAttachments.test.ts
```

- [ ] **Step 3: Implement minimal shared editor plumbing**

Implement:
- `attachmentId` support in rich-text image nodes
- shared HTML helpers for attachment ID extraction/removal
- upload helper returning `{ attachmentId, publicUrl }`
- upload state callbacks so parents can block save/submit while images are still uploading
- comment editor switched to temporary `editor_upload` image staging instead of story-owned inline uploads

- [ ] **Step 4: Run shared frontend tests to verify they pass**

Run:
```bash
npx vitest run src/components/pm/__tests__/editorImageAttachments.test.ts
```

### Task 3: Description Editors, Panels, And Create Flows

**Files:**
- Modify: `frontend/src/components/pm/CreateStoryModal.tsx`
- Modify: `frontend/src/components/pm/GlobalCreateModals.tsx`
- Modify: `frontend/src/components/pm/Attachments.tsx`
- Modify: `frontend/src/pages/pm/StoryDetail.tsx`
- Modify: `frontend/src/components/pm/StoryDetailPanel.tsx`
- Modify: `frontend/src/pages/pm/EpicDetail.tsx`
- Modify: `frontend/src/pages/pm/ObjectiveDetail.tsx`
- Modify: `frontend/src/pages/pm/SprintDetail.tsx`
- Modify: `frontend/src/lib/pmTypes.ts`
- Test: `frontend/src/components/pm/__tests__/Attachments.test.tsx`
- Test: `frontend/src/components/pm/__tests__/CreateStoryModal.test.tsx`
- Test: `frontend/src/components/pm/__tests__/GlobalCreateModals.test.tsx`

- [ ] **Step 1: Write failing integration-style tests for create and description flows**

Cover:
- create story sends inline image `attachment_ids` with manual file attachments still working
- epic/objective/sprint create flows stage temporary uploads and submit attachment IDs
- attachment panels can exist for epic/objective/sprint entity attachments
- description-side delete removes inline images from saved HTML before attachment delete

- [ ] **Step 2: Run frontend tests to verify they fail**

Run:
```bash
npx vitest run src/components/pm/__tests__/Attachments.test.tsx src/components/pm/__tests__/CreateStoryModal.test.tsx src/components/pm/__tests__/GlobalCreateModals.test.tsx
```

- [ ] **Step 3: Implement minimal description/create-flow integration**

Implement:
- create request types including `attachment_ids`
- temporary-upload cleanup in create modals
- upload-aware submit disabling
- epic/objective/sprint attachment panels
- description-save/delete sync using attachment ID diffs against saved HTML

- [ ] **Step 4: Run frontend tests to verify they pass**

Run:
```bash
npx vitest run src/components/pm/__tests__/Attachments.test.tsx src/components/pm/__tests__/CreateStoryModal.test.tsx src/components/pm/__tests__/GlobalCreateModals.test.tsx
```

### Task 4: Comment Flow Rendering And Dedupe

**Files:**
- Modify: `frontend/src/components/pm/CommentThread.tsx`
- Modify: `frontend/src/components/pm/CommentBody.tsx`
- Modify: `frontend/src/lib/services/pmCommentService.ts` (if request shape changes)
- Test: `frontend/src/components/pm/__tests__/CommentThread.test.tsx`
- Test: `frontend/src/components/pm/__tests__/CommentBody.test.tsx`

- [ ] **Step 1: Write failing comment-flow tests**

Cover:
- pasted/dropped image uploads remain tied to pending comment attachments until submit
- inline comment images render in body
- non-image attachments still render in the attachment strip
- image attachments are not duplicated below the comment

- [ ] **Step 2: Run comment-flow tests to verify they fail**

Run:
```bash
npx vitest run src/components/pm/__tests__/CommentThread.test.tsx src/components/pm/__tests__/CommentBody.test.tsx
```

- [ ] **Step 3: Implement minimal comment-flow changes**

Implement:
- comment draft inline image staging via temporary uploads
- submit-time attachment ID merge for inline images and manual files
- image dedupe in `CommentAttachments`

- [ ] **Step 4: Run comment-flow tests to verify they pass**

Run:
```bash
npx vitest run src/components/pm/__tests__/CommentThread.test.tsx src/components/pm/__tests__/CommentBody.test.tsx
```

### Task 5: Final Verification

**Files:**
- Review all modified PM attachment/editor files

- [ ] **Step 1: Run targeted backend verification**

Run:
```bash
go test ./internal/service -run 'TestPMAttachment|TestPMStory|TestPMEpic|TestPMObjective|TestPMSprint'
```

- [ ] **Step 2: Run targeted frontend verification**

Run:
```bash
npx vitest run src/components/pm/__tests__/editorImageAttachments.test.ts src/components/pm/__tests__/Attachments.test.tsx src/components/pm/__tests__/CreateStoryModal.test.tsx src/components/pm/__tests__/GlobalCreateModals.test.tsx src/components/pm/__tests__/CommentThread.test.tsx src/components/pm/__tests__/CommentBody.test.tsx
```

- [ ] **Step 3: Run the frontend build**

Run:
```bash
npm run build
```

- [ ] **Step 4: Run focused manual smoke checks if the app can be launched locally**

Check:
- paste and drop in each description editor
- paste and drop in story comment and reply
- remove inline image before save/submit
- delete image from description attachments panel

