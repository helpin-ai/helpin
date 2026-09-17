# Ask Agent media attachments

> Historical design, reviewed against the checkout on 2026-09-17. This page
> records the original attachment proposal for contributors. Several selection,
> format, transport, and security requirements below differ from current code;
> they must not be treated as implemented guarantees.

## Current implementation and gaps

[Dock chat handling](../../server/internal/service/dock_chat.go) accepts media
plus PDF, DOCX, JSON, plain text, Markdown, and CSV. Explicit attachments must be
uploaded editor-upload objects belonging to the workspace and sending user, with
recorded size at most 20 MiB. The [composer](../../frontend/src/components/agents/dock/ChatView.tsx)
uploads privately, shows upload/failure state, and allows removal. Its image
preview and temporary “Analyzing” status are not evidence of a durable per-file
analysis activity with an expandable completion/error record.

Source-context media is not metadata-only: the service gathers attachments from
supported source entities and passes them alongside explicit attachments to the
reader before starting or continuing the run. It also discovers hosted images
from Support message links/previews. Decorative filtering is a filename-marker
heuristic for files no larger than 15 KiB, not an absolute guarantee that every
signature, logo, or tracking asset is excluded.

The [reader](../../server/internal/service/dock_chat_media.go) extracts document
text locally (truncating at 80,000 bytes per document) and requests concise media
observations with a four-minute timeout and 700-token output limit. Observations
are text, not a validated structured result. Explicit stored media is read for
signature checks, then passed to the provider using object URLs; resolved source
media with an existing URL bypasses that same local signature-check branch.
The [private read helper](../../server/internal/service/pm_attachment.go) checks
workspace, uploader, upload completion, and editor-upload origin.

[Hosted-image fetching](../../server/internal/service/dock_chat_remote_media.go)
requires allowed HTTPS URLs, checks resolved addresses and redirects, caps reads
at 10 MiB, validates supported image signatures, and supplies transient data URLs
to the provider. This supersedes the proposal's blanket exclusion of public URLs.
The inspected Ask path does not establish the proposed fixed attachment count,
image-resolution limit, video-duration limit, or scheduled deletion of provider
files and derived analysis. Those remain unproven requirements, not runtime
promises. Signature checks validate file headers, not full decoding safety.

Analysis failure returns an error before the run starts/continues. The composed
turn retains attachment metadata and an `attachment_analysis` block; the reader
prompt and document sections label embedded instructions as untrusted. The
primary run follows its configured profile/connection/model, so this design's
DeepSeek-specific wording is not a universal current model contract.

## Original proposal

## Goal

Let people attach images and short videos directly to an Ask Agent message, while keeping DeepSeek Flash as the primary agent model and tool executor.

## Chosen approach

Ask uses a dedicated multimodal reader only when media must be understood. The reader produces a compact, structured observation; DeepSeek receives that observation as untrusted context and completes the normal agent turn.

This avoids routing an entire conversation to a more expensive model.

## User experience

- The Ask composer accepts images and short videos through an attachment control.
- Selected media has a local preview, upload state, and remove action.
- Sent media remains attached to the user message after reload.
- Before the Ask run starts, the timeline shows a normal activity row such as `Analyzing screenshot.png`.
- Successful analysis remains a compact completed timeline item. Failures expose their error on expansion and do not prevent the user from retrying or sending a text-only turn.

## Media selection

- Explicit Ask composer attachments are always analyzed.
- Support, task, document, and other source attachments are initially represented only by metadata.
- Ask can invoke a specific `analyze_attachment` capability when the user asks about a source attachment or the agent determines it is relevant.
- Inline email signatures, logos, and tracking images are never auto-analyzed. Inbound attachment metadata will retain inline/disposition, content ID, dimensions, and size so decorative assets can be filtered conservatively.

## Data flow

1. The composer uploads an allowlisted image/video into private workspace storage and sends attachment IDs with the Ask message.
2. The server validates that the current workspace and user can use each attachment, persists the references on the Ask message, and creates the run.
3. A media reader fetches the stored bytes server-side and submits them to the multimodal provider. Images produce observations; videos produce concise timestamped observations.
4. The analysis result is stored with the message/run and appended to the DeepSeek prompt in an explicit untrusted-data envelope.
5. DeepSeek runs normally, including any existing internal tools and approval rules.

## Security and limits

- Allowlisted MIME types with server-side signature validation.
- Private storage and server-authorized reads; no public or arbitrary URLs are handed to the reader.
- No executing, unzipping, rendering HTML, or following instructions embedded in attachments.
- Attachment output is untrusted data and cannot change system policy, grant permissions, or authorize a tool call.
- Fixed attachment count, image resolution, video size, and video duration limits.
- Temporary provider-side files and derived analysis are deleted on a defined retention schedule.

## Deliberate exclusions

- No whole-run model switch.
- No automatic analysis of every attachment in a support thread.
- No custom OCR/frame extraction/transcription pipeline in the first release.
- No broad document/archive support in the first release.

## Verification

- Composer tests cover allowed media selection, removal, and upload failure.
- API tests verify authorization, allowlist/signature checks, and persisted message attachment IDs.
- Runtime tests verify DeepSeek receives structured untrusted observations and remains the execution model.
- Transcript tests cover in-order `Analyzing …` activity and completed/error display.
