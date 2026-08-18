# CRM Meeting Intelligence

Status: code-complete on `feat/meeting-intelligence-providers`; Recall is the initial runtime provider, Vexa is an available configuration switch, and production provider canaries remain an operational rollout step.

## Product boundary

Meeting Intelligence is a built-in Helpin automation, not an Agent Runtime agent. The capture provider supplies the bot, provider lifecycle, transcript, and optional recording. Helpin owns the durable meeting record, provider-neutral transcript, AI processing, CRM projections, review flow, and project tasks.

The first release intentionally keeps calendar auto-join out of the critical path. A member pastes a Meet, Zoom, Teams, or Webex URL and either starts the notetaker immediately or saves the meeting for a later manual start.

## Provider-neutral architecture

```text
Meetings UI / API
       |
       v
CRMMeetingService ---- env: CRM_MEETING_CAPTURE_PROVIDER=recall|vexa
       |
       +---- meetingcapture.RecallProvider ---- Recall REST + Svix webhooks
       |
       `---- meetingcapture.VexaProvider ------ Vexa REST + webhook.v1 HMAC
                     |
                     v
          immutable CRMMeetingCapture
                     |
          transcript-ready provider event
                     |
                     v
       CRMMeetingProcessingWorkflow (Temporal)
                     |
       provider-neutral canonical transcript
                     |
        Helpin AI usage preflight + metering
                     |
        summary / decisions / risks / actions
                     |
        CRM activity + buyer signals + follow-up
                     |
        reviewed action -> canonical PM task
```

The provider interface is deliberately narrow:

- identity/readiness: `Name`, `Configured`, `Supports`
- capture lifecycle: `StartCapture`, `StopCapture`, `GetStatus`
- artifacts: `GetTranscript`, `GetRecording`, `DeleteArtifacts`
- callbacks: `VerifyWebhook`, `NormalizeWebhook`

No provider response escapes this adapter boundary. Each capture attempt stores its provider and provider capture ID permanently. Changing the workspace default affects only new attempts; an in-flight or historical meeting continues to use the provider recorded on its capture. Adding a third provider requires one adapter plus contract tests, not changes to processing, CRM projection, or UI detail data.

## Data model

```text
crm_meetings
  1---* crm_meeting_captures       immutable provider attempts
  1---1 crm_meeting_transcripts    Helpin canonical transcript
  1---1 crm_meeting_intelligence   generated structured output
  1---* crm_meeting_action_items   pending / accepted / dismissed
  1---* crm_meeting_provider_events idempotent webhook inbox

crm_meeting_settings               workspace provider and policy
crm_associations                   meeting <-> contact/company/deal/task/etc.
crm_activities                     projected completed meeting activity
crm_suggestions                    projected follow-up draft
pm_tasks                           accepted action items only
```

GORM AutoMigrate creates the new tables and columns. Provider capture IDs and request idempotency keys have database uniqueness constraints. A PostgreSQL advisory lock serializes each workspace/idempotency-key launch across API replicas, and provider bots are stopped when canonical persistence fails. Webhook events are deduplicated, but an event left unprocessed by a transient workflow failure is retried on delivery replay.

## Processing and billing

The same Temporal workflow runs for every provider. It fetches and normalizes the final transcript, optionally copies audio into private Helpin S3 storage, and runs a fixed-schema Helpin intelligence prompt. The LLM call uses the shared metered provider with the `meeting_intelligence` feature key. An exhausted allowance keeps the transcript, marks the summary `blocked_usage`, and allows retry after capacity is available. This flow does not create or execute an `agent_run`.

Generated output includes:

- concise Markdown summary
- key points, decisions, objections, risks, and next steps
- follow-up subject/body draft
- reviewable action items with evidence and optional assignee/due date
- buyer signal detection using the existing CRM signal pipeline
- a completed CRM meeting activity

An action item becomes a PM task only after a user chooses a destination team and accepts it. Dismiss and accept operations are idempotent; the resulting task is associated back to the meeting.

## UI

```text
CRM
  Contacts
  Companies
  Deals
  Meetings  <--- new
  Review
  Insights

+--------------------------------------------------------------------------------+
| Meetings                                  [Settings] [Add meeting]              |
+--------------------------------------------------------------------------------+
| Search meetings                                                               |
+--------------------------------------------------------------------------------+
| Discovery call with Acme             Ready       Google Meet      Aug 14, 1 PM |
| Product review                        Recording   Zoom             Aug 15, 9 AM |
+--------------------------------------------------------------------------------+

+--------------------------------------------------------------------------------+
| <- Discovery call with Acme  [Ready]              [Recording] [Retry/Stop]      |
+-------------------------------------------------------+------------------------+
| Meeting intelligence                                  | Capture details        |
| Summary                                                | Provider: Recall       |
| Key points                 Decisions                   | Visibility: Workspace |
| Risks                      Next steps                  +------------------------+
| Follow-up draft [Copy]                                 | Associations           |
+-------------------------------------------------------+ Contacts        +      |
| Action items                                          | Companies       +      |
| [Destination team v] [Create task] [Dismiss]          | Deals           +      |
+-------------------------------------------------------+ Tasks           +      |
| Transcript [Copy]                                     +------------------------+
| Speaker       02:14  ...                              |
+-------------------------------------------------------+------------------------+
```

Workspace admins configure the provider under Settings → CRM → Meeting Intelligence. CRM readers can read effective settings so the Meetings page can accurately show whether capture is enabled; only CRM admins can update them.
Meeting artifacts are workspace-visible through CRM RBAC in the MVP. Participant/private visibility values are reserved in the data contract but rejected by validation until identity-level enforcement is implemented.

## API surface

- `GET/POST /api/crm/meetings`
- `GET/PUT/DELETE /api/crm/meetings/{id}`
- `POST /api/crm/meetings/{id}/capture`
- `POST /api/crm/meetings/{id}/capture/stop`
- `POST /api/crm/meetings/{id}/process`
- `GET/DELETE /api/crm/meetings/{id}/recording`
- `POST /api/crm/meetings/{id}/action-items/{itemID}/accept`
- `POST /api/crm/meetings/{id}/action-items/{itemID}/dismiss`
- `GET/PUT /api/crm/meeting-settings`
- `POST /api/webhooks/meeting-capture/{provider}`

Capture starts require `Idempotency-Key`. Authenticated CRM routes retain workspace RBAC. Provider webhooks are public only at the HTTP layer and reject unverified payloads before parsing or persistence.

## Provider rollout

Both the API server and Temporal worker receive secrets through the existing `helpin-secrets` Kubernetes Secret, so no Kubernetes manifest shape change is required.

Initial Recall configuration:

```text
RECALL_BASE_URL=https://us-east-1.recall.ai
RECALL_API_KEY=...
RECALL_WEBHOOK_SECRET=whsec_...
```

In Recall's dashboard, register:

```text
https://<api-host>/api/webhooks/meeting-capture/recall
```

Subscribe at minimum to bot status, `transcript.processing`, `transcript.done`, and `transcript.failed`. Recall dashboard deliveries use Svix verification headers and the raw payload.

Optional Vexa configuration:

```text
VEXA_BASE_URL=https://api.cloud.vexa.ai
VEXA_API_KEY=...
VEXA_WEBHOOK_SECRET=<dedicated-random-secret>
```

Configure Vexa with `PUT /user/webhook` using the Helpin Vexa callback URL and the same dedicated secret. Current `webhook.v1` deliveries are verified with `X-Webhook-Timestamp` and `X-Webhook-Signature: sha256=<HMAC-SHA256(secret, timestamp.raw-body)>`; the legacy `Authorization: Bearer <secret>` contract remains accepted during migration. Subscribe to `meeting.completed` and `bot.failed` at minimum. The adapter also understands legacy `meeting.status_change` and `transcription.ready` events. Legacy Teams `/l/meetup-join/...` URLs carry their meeting identity in the URL and do not need `p`; short Teams links require `p`. Zoom `pwd` values are forwarded when present.

The adapter is contract-tested against Vexa v0.12.x structures, including nested `data.meeting` envelopes and current transcript `start`/`end` timestamps. A live canary against the selected hosted or self-hosted deployment is still required before changing production away from Recall.

To switch later:

1. Deploy and validate the Vexa service and its STT/storage dependencies.
2. Add the Vexa API and webhook secrets to both API and Temporal worker environments.
3. Configure the Vexa callback and run one canary meeting through capture, transcript, recording, and deletion.
4. Set `CRM_MEETING_CAPTURE_PROVIDER=vexa` for the API deployment and restart it.
5. Keep Recall credentials until every Recall-owned in-flight capture has completed and its provider artifacts have been copied/deleted.

Rollback is the same environment change back to `recall`. No data migration is needed. Provider selection is deployment-owned and is not returned by the workspace settings API or shown in the customer UI.

## Verification checklist

Automated coverage completed:

- provider contract tests cover request auth, launch identity, both Vexa verification contracts, `webhook.v1`, current transcript timestamps, legacy Teams links, and numeric recording IDs
- meeting URL parsing tests cover supported platforms, real legacy/short Teams shapes, and invalid input
- lifecycle tests cover failed webhook replay, stale capture isolation, provider switching, canonical transcript/intelligence/action persistence, participant derivation, and provider artifact cleanup
- API and Temporal binaries compile with all DI wiring
- production frontend build includes list, detail, and settings routes

Operational checks required with deployed credentials:

- manual canary: start, lobby/admission, record, stop, transcript, summary, CRM projections, task acceptance, recording playback, delete
- replay the same capture idempotency key under concurrent requests; verify one provider bot and one canonical meeting
- replay the same webhook and capture idempotency key; verify no duplicate workflow, event, or task
- exhaust AI allowance; verify transcript retention, `blocked_usage`, upgrade UI, and successful retry
- change `CRM_MEETING_CAPTURE_PROVIDER` during a live Recall meeting and restart the API; verify that meeting finishes on Recall and only the next attempt uses Vexa

## Follow-on work

- Google/Microsoft calendar connection and policy-driven auto-join
- live transcript streaming; the first version polls durable meeting state
- retention cleanup workflow for transcript/audio policy deadlines
- richer participant identity resolution and participant/private visibility enforcement before those policies are enabled broadly
- provider health/reconciliation job for missed webhooks
- production Vexa Helm deployment when moving from hosted Vexa to self-hosted capture

## Primary provider references

- Recall Create Bot: https://docs.recall.ai/reference/bot_create
- Recall recording/transcript webhooks: https://docs.recall.ai/docs/recording-webhooks
- Vexa meeting API: https://docs.vexa.ai/api/meetings
- Vexa webhooks: https://docs.vexa.ai/webhooks
- Vexa transcripts: https://docs.vexa.ai/api/transcripts
- Vexa recording storage: https://docs.vexa.ai/recording-storage
