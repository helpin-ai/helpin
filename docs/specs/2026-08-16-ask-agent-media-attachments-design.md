# Ask Agent media attachments

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
