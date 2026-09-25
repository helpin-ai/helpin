# Support video attachments implementation plan

> Historical implementation record, source-compared on 2026-09-17. The
> [support attachment service](../../server/internal/service/support_attachment.go)
> enforces 100 MiB (104,857,600 bytes), displayed as 100 MB, and retains private
> storage and uploader checks. The [SDK transport](../../packages/sdk-js/src/transport/attachment-upload.ts)
> initializes the upload, sends storage bytes with XHR, then confirms it; progress
> reaches 100 only after confirmation. It accepts cancellation and progress options.
>
> The [widget upload hook](../../packages/widget-core/src/hooks/useAttachmentUploads.ts)
> retains files for retry and aborts removed/inactive uploads; ComposeBar blocks
> sending until every pending attachment is uploaded. Current
> [inbox composer](../../frontend/src/components/support/ReplyComposer.tsx) also
> batches pending entries and gates Send. Widget MessageBubble and the
> [inbox gallery](../../frontend/src/components/support/SupportAttachmentGallery.tsx)
> use native video controls and download fallbacks; supported uploads do not
> guarantee browser codec playback.
>
> Test totals, screenshots, build results, branch names and agent assignments below
> are records of the original implementation session. They were not reproduced and
> are not instructions to delegate or merge during this documentation review.

Goal: Allow support files up to 100 MB with video playback and reliable uploads.

1. Backend and inbox: isolate the support upload limit at 100 MiB (displayed as 100 MB), retain other modules' limits; align validation/errors, inbox picker and video playback. Test size boundaries and existing upload ownership behavior.
2. SDK transport: add optional progress and AbortSignal options to attachment upload, use XHR for storage progress, expose useful failures, retain staged upload and confirmation. Test cancellation, errors and progress.
3. Widget: retain files for retry, cancel removed/inactive uploads, report validation errors, register all selected files before uploading, block Send while uploading or failed; add native video controls with download fallback. Preserve text drafts.
4. Verify package tests/typechecks, backend tests/build and production widget build. Review changes, commit to waqar-fixes. No push until requested.

Execution: independent agent work on backend/inbox and SDK transport; root implements widget state and shared upload contract. Shared contract: optional third callback argument { signal?: AbortSignal; onProgress?: (percent: number) => void }; callback returns existing attachment result or null and may throw descriptive errors. PendingAttachment adds error?: string.

Completed: 100 MB support validation; widget progress, cancellation, retry, size errors and send guards; video controls/download fallbacks; inbox batch/send safeguards. Review fixes preserve initial conversation promotion and private-storage ACL behavior. Verified 239 widget tests,198 SDK tests,36 inbox tests, backend attachment tests, frontend/widget TypeScript checks, SDK production build and API/worker build. Desktop/mobile upload layout checked with browser screenshots. No production writes.
