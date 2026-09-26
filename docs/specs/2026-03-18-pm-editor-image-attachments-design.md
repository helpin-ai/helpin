# PM editor image attachments design


This historical design explains attachment-backed inline images for PM editors.
Core metadata and upload paths exist; use the current notes before relying on
its original save, delete, and ownership guarantees.

## Current implementation and limits

Source-compared on 2026-09-18. See the
[implementation review](../plans/2026-03-18-pm-editor-image-attachments.md) for
current create-flow entry points. Historical “story” filenames and entity names
below now correspond to tasks in these paths.

- [The editor upload helper](../../frontend/src/hooks/useEditorImageUpload.ts)
  returns `attachmentId` and `publicUrl`. Despite the latter name, the returned
  value is a stable application content URL, whose backend resolves a download
  URL; do not assume it is a permanently public object-store URL. Private uploads
  omit the public-read ACL. Image uploads check image MIME type and a 50 MB limit.
- [The image extension](../../frontend/src/components/ui/resizable-image-extension.ts)
  preserves `attachmentId` as `data-attachment-id`, and
  [HTML helpers](../../frontend/src/components/pm/editorImageAttachments.ts)
  extract/remove references. URL-only legacy images remain distinguishable.
- [Task creation](../../server/internal/service/pm_task.go),
  [epic creation](../../server/internal/service/pm_epic.go),
  [objective creation](../../server/internal/service/pm_objective.go), and
  [sprint creation](../../server/internal/service/pm_sprint.go) log attachment
  reassignment errors after entity creation. They do not implement the proposed
  “fail create and keep the draft open” guarantee. In contrast,
  [comment creation/update](../../server/internal/service/pm_comment.go) performs
  reassignment within its database transaction.
- [Generic reassignment](../../server/internal/repository/pm_attachment.go)
  filters by attachment IDs and validates affected counts; it does not itself
  constrain workspace ownership. Its existence alone does not verify permission
  enforcement across all callers.
- [CommentThread](../../frontend/src/components/pm/CommentThread.tsx) hides an
  image from the compact strip only when its attachment ID appears inline.
  Standalone image attachments remain visible. Pending draft cleanup uses
  `pendingOnly`; it is distinct from removal of an already-owned attachment.

The original all-editor acceptance matrix and delete-failure guarantees are
requirements, not proof of current end-to-end behavior. This review did not run
application tests, upload files, or perform browser QA. Competitor descriptions
below are historical design context, not newly verified product comparisons.

## Original design

## Summary

Add paste-from-clipboard and drag-and-drop image upload to all current PM rich-text editors, and make those uploaded images real PM attachments instead of editor-only blobs.

The image should still render inline in the edited content, but the system should treat it as a single attachment-backed object with clear ownership and delete behavior. This applies to:

- story descriptions
- epic descriptions
- objective descriptions
- sprint descriptions
- story comments
- story comment replies

## Goals

- Let users paste or drop images directly into PM editors.
- Store those images through the PM attachment system, not a separate editor-only store.
- Render images inline in descriptions and comments after upload.
- Keep attachment ownership and deletion rules predictable.
- Support temporary uploads for unsaved create flows and comment drafts.
- Extend attachment-backed image support to objective and sprint entities.

## Non-Goals

- Adding paste/drop support for non-image files in rich-text editors.
- Migrating historical inline image URLs into attachment-backed records.
- Adding new PM comment surfaces that do not exist today.
- Building collaborative draft persistence for unfinished uploads.
- Reworking the full attachment gallery UX beyond what is needed for correct ownership.

## Current Behavior

- `TiptapEditor` already intercepts pasted and dropped images and uploads them through `uploadEditorImage`.
- `CommentEditor` does the same for story comments and replies.
- Those uploads currently create image URLs for inline rendering, but they are not modeled as a first-class inline attachment object with shared delete semantics.
- Comments already support attachment reassignment through `attachment_ids` on comment creation.
- PM attachments currently allow `story`, `epic`, `comment`, and `editor_upload` entity types.
- Objective and sprint descriptions do not currently have attachment-backed upload semantics.
- Story and story-detail side panels expose an attachments section. Epic, objective, and sprint detail pages do not.

## Product Pattern

The strongest public precedent is:

- Shortcut: screenshots/images in comments are shown inline while story-level files also exist as attachments.
- Jira: images can appear in comments, and deletion follows the source surface rather than a detached global attachment model.
- GitHub: pasted and dropped images in comment boxes render inline while still being backed by uploaded files.

The recommended product rule for Helpin PM is:

- images in comments and descriptions render inline
- non-image files remain attachment-only
- the edited content is the source of truth for inline images

## Product Decisions

### Inline vs attachment-only

Pasted and dropped images should be both:

- inline in the editor/body
- stored as PM attachments

We should not make comment screenshots attachment-only, because that is worse for conversational context and weaker than the product pattern above.

### Ownership

There is one underlying uploaded file. The inline image and the attachment record are two views of that same object, not two separate objects.

### Deletion model

Deletion should be source-owned:

- remove an inline image from a description or comment -> remove its attachment when no inline references remain
- remove an image attachment from a description-level attachments panel -> remove the matching inline image node(s) from that description after confirmation
- comment-owned inline images should not also appear as duplicate image cards in the compact comment attachment strip

This avoids making a single image independently editable from multiple unrelated surfaces.

## Approaches Considered

### 1. Recommended: Attachment-backed inline images with node metadata

Upload images through the PM attachment API, store the returned `attachment_id` on the editor image node, and keep the content plus attachment record in sync.

Why this is preferred:

- One uploaded file, one DB record, one canonical ownership model.
- Reuses the existing PM attachment infrastructure.
- Supports predictable delete behavior.
- Works for both persisted entities and temporary draft uploads.

Trade-offs:

- Requires editor-node metadata and synchronization logic.
- Requires generic attachment reassignment instead of comment-only reassignment.

### 2. Dual-write inline URL plus separate attachment

Keep current inline upload behavior and also create a second attachment record.

Advantages:

- Smaller short-term implementation.

Disadvantages:

- Doubles storage.
- Delete semantics become confusing.
- Inline image and attachment list can drift.

### 3. Save-time extraction

Let the editor upload inline images first, then parse the saved HTML and convert those images into attachments later.

Advantages:

- Less editor integration up front.

Disadvantages:

- Brittle save pipeline.
- Worse error handling.
- Poor fit for comment drafts and attachment deletion.

## Approved Design

### Supported Surfaces

The first implementation covers all current PM rich-text authoring surfaces in this codebase:

- story description editors
- epic description editors
- objective description editors
- sprint description editors
- story comment editors
- story reply editors
- story create modal description editor
- global PM create modal description editors that use the shared rich-text editor

If additional PM comment surfaces are added later, they should use the same attachment-backed image flow.

### Entity Model

Extend PM attachments to support:

- `objective`
- `sprint`

Keep existing support for:

- `story`
- `epic`
- `comment`
- `editor_upload`

`editor_upload` remains the temporary staging entity type for unsaved drafts and create flows.

### Upload Result Shape

The upload path used by editors should expose:

- `attachment_id`
- `public_url`

The editor should not treat uploaded images as URL-only objects anymore.

### Editor Node Metadata

Inline image nodes should store:

- `src`
- `attachmentId`

The existing resizable image node is the right place for this metadata.

This lets the frontend distinguish:

- attachment-backed images created by the PM editor flow
- historical or external images that only have a URL

### Persisted Entity Flow

When editing an existing story, epic, objective, sprint, or comment:

- paste/drop image
- upload directly to that entity through PM attachments
- insert inline image node with `src` + `attachmentId`
- refresh attachment-backed UI as needed

No reassignment is needed for persisted entities because the upload can point at the final entity immediately.

### Draft and Create Flow

When editing an entity that does not exist yet:

- upload images as `editor_upload`
- associate them with a stable temporary draft key for that editor session
- store returned `attachment_id` in inline nodes
- on successful create, reassign those attachments to the created entity

This applies to:

- create story modal
- global PM create flows that use the shared editor
- new comment and reply drafts before the comment exists

### Generic Reassignment

Replace the current comment-only reassignment behavior with a generic repository/service operation:

- reassign attachment IDs to any target entity type and entity ID

This generic operation should be used by:

- comment create
- story create
- epic create
- objective create
- sprint create

This keeps temporary upload lifecycle logic in one place.

### Description Attachment Panels

Descriptions for attachment-owning entities should expose an attachments panel:

- story: existing behavior remains
- epic: add attachments panel
- objective: add attachments panel
- sprint: add attachments panel

Those panels should show files attached to the entity itself.

For image attachments that are also referenced inline in the description:

- the panel may still list them
- delete action must clearly indicate it will remove matching inline images too

### Comment Rendering

Comment images should render inline inside the comment body.

The compact comment attachment strip should:

- continue to show non-image files
- stop duplicating image attachments that already render inline in the comment body

This keeps screenshots readable in conversation while preserving a place for PDFs, docs, and other files.

### Delete Synchronization

The content body is the source of truth for inline image existence.

Rules:

- If an inline image with `attachmentId` is removed from the editor, delete that attachment when no other inline node in the same body still references it.
- If an attachment-panel delete removes an image attachment that is referenced inline, prompt for confirmation and then remove the matching inline image node(s) from the description body.
- For temporary `editor_upload` images in unsaved drafts, deleting the inline image should immediately delete the temporary attachment.
- Historical URL-only images are not auto-deleted because they do not have `attachmentId`.

### Save and Submit Rules

- Save/submit actions must wait for in-flight image uploads to finish.
- Users should not be able to submit a comment or save a description while a pasted image is still uploading.
- If upload fails, remove the placeholder image node and show an error toast.

### Draft Cleanup

If a create modal or unsaved draft editor is abandoned, temporary `editor_upload` attachments created in that session should be deleted.

This cleanup should happen on:

- explicit cancel/close when possible
- inline image removal before submit

Best-effort cleanup is acceptable for abrupt browser closes in the first version.

## Implementation Shape

### Backend

- Extend allowed PM attachment entity types to include `objective` and `sprint`.
- Add generic attachment reassignment in the attachment repository/service.
- Update create flows for story, epic, objective, and sprint so temporary uploaded images can be reassigned on successful entity creation.
- Keep comment creation using the same generic reassignment path instead of a comment-specific helper.
- Preserve current attachment size and MIME validation rules.

### Frontend

- Replace the current editor upload helper return type so it returns attachment metadata, not just a URL.
- Extend the resizable image node to carry `attachmentId`.
- Add editor synchronization logic that can detect removed inline attachment-backed images.
- Add attachment-aware save guards for rich-text editors and comment editors.
- Add attachments panels to epic, objective, and sprint detail pages.
- Filter comment attachment rendering so inline image attachments are not duplicated in the attachment strip.

## Error Handling

- Upload failure: remove placeholder node and show a toast.
- Reassignment failure on create: fail the save/create request and keep the draft open if possible.
- Attachment delete failure: leave inline content unchanged and show an error.
- Missing `attachmentId` on older content: render normally and do not attempt synchronized deletion.

## Testing

### Backend

- Unit tests for allowed entity types including `objective` and `sprint`.
- Unit tests for generic attachment reassignment to comment, story, epic, objective, and sprint.
- Regression tests for create flows that reassign temporary uploaded images on successful create.

### Frontend

- Tests for paste image upload in shared PM rich-text editor.
- Tests for paste image upload in comment editor.
- Tests that uploaded inline image nodes store `attachmentId`.
- Tests that removing an inline image deletes the backing attachment when appropriate.
- Tests that comment rendering shows inline images in body and non-image attachments in the attachment strip.
- Tests that draft cleanup removes temporary uploads on cancel/remove.

### Manual QA

- Paste image into story, epic, objective, and sprint descriptions.
- Drop image into story, epic, objective, and sprint descriptions.
- Paste image into story comment and reply editors.
- Remove inline image before save and verify cleanup.
- Delete image from description attachments panel and verify inline removal.
- Verify non-image attachments still behave as attachment cards/chips.
- Verify failed uploads do not leave broken inline nodes.

## Risks

- Delete synchronization adds more editor-state bookkeeping than the current URL-only model.
- Temporary upload cleanup is inherently best-effort for abruptly abandoned browser sessions.
- Reassignment touches multiple create flows, so regression coverage matters.

## Open Questions

- None for the first implementation.

The approved behavior is:

- all current PM rich-text editors accept pasted and dropped images
- those images are real PM attachments
- images render inline in descriptions and comments
- non-image files remain attachment-only
- inline image deletion owns attachment cleanup
