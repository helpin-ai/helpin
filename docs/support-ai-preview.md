# Support AI preview

Support administrators can test the **saved** assistant in Settings → AI Assistant → Test AI response. The admin playground also accepts explicit text history or an existing conversation visible to the caller. Unsaved settings do not affect a test. Preview does not enable AI replies or change workspace settings.

Answer previews use the normal AgentService admission, shared-profile resolution, encrypted run credentials, Runtime provider, saved agent prompt/model controls, usage metering, knowledge retrieval and server reply gate. Personal manual-run profiles are deliberately ineligible, matching unattended visitor runs. Explicit requests for a human use the same deterministic handoff check before invoking a model.

Each test has an isolated `support_preview` target and an immutable text snapshot. It creates no support conversation, processing attempt, customer message, handoff, assignment, coverage trace or email. `search_knowledge` uses real workspace knowledge and stores the same evidence used by the live reply validator. `send_support_reply` and `escalate_to_human` capture an outcome in a run artifact instead of delivering it. The first outcome wins under retries. Cited source details remain in that artifact after normal temporary-evidence cleanup.

Only knowledge search, the two captured outcome tools, and reads of the preview's own conversation snapshot are available. The host checks the persisted run and rejects forged targets and other commands. External MCP attachments, child runs, browser actions and other tools are excluded and listed in the result. This tests grounded support answers; it does not certify external integrations, channel delivery, takeover races, or live conversation eligibility. Embeddings retain the workspace's existing server-side configuration; choosing a local agent model does not change that.

The existing admin retrieval-only option calls the shared search function without launching a model. It reports `retrieval_only`, not a simulated agent answer. The old in-process preview answer generator and fabricated planner display are removed.

## API and lifecycle

All routes require `support.admin` and a workspace actor. The message and each
explicit history turn are limited to 16,000 bytes, with at most 100 history
turns. These are backend byte limits, so non-ASCII input can reach them before
the text field’s character limit.

Available routes:

- `POST /api/pm/agents/{id}/support-preview?workspace_id=…` starts a test and returns `run_id` and `final_decision: pending`, or an immediate retrieval/hard-handoff result.
- `GET /api/pm/agents/{id}/support-preview/{runId}?workspace_id=…` polls the captured outcome. Only its initiating user can read it. Generic run-detail, transcript and artifact endpoints apply the same owner check; list payloads omit preview text and preview transcript events are not broadcast to the workspace.
- `DELETE` on that result URL stops an active test through the normal Runtime cancellation path.

A captured outcome triggers cancellation if Runtime is still active, preserving normal usage settlement and credential cleanup. A paused run is reported as blocked; completion without an outcome is reported as failure. The existing reconciliation sweep cancels previews older than three minutes, including abandoned browser sessions; tool callbacks also reject expired/terminal previews. Sweep scheduling can delay cancellation beyond the three-minute eligibility deadline.

Model and retrieval calls are billed/metered under the normal edition policy. A test is isolated from customer effects, not free of model usage. No new migration, environment variable, SDK version, or Runtime deployment is required; restart Helpin API and rebuild the frontend/admin apps to expose this feature.

## Verification

Tests cover resolved profile credentials, isolated target context, restricted tool projection, cross-target and cross-user rejection, shared reply validation, duplicate outcome capture, evidence retention, model failure, expiry and cancellation. Existing service/handler/repository/router suites cover unchanged live support paths. UI verification covers asynchronous result polling and clearly displaying rejected proposals. A real provider smoke after restarting Helpin remains a deployment check; deterministic tests use a fake Runtime transport and never send customer messages.

## Source references

The [preview service](../server/internal/service/support_preview.go) owns
admission, snapshots, result access, and timeout reporting. The
[preview command handler](../server/internal/service/internal_command_support_preview.go)
restricts tools and captures outcomes. The
[settings component](../frontend/src/components/settings/SupportAIPreview.tsx)
polls results, and [service tests](../server/internal/service/support_preview_test.go)
exercise isolation with a fake Runtime transport.
