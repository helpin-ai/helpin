# Enable coverage analysis for every workspace

Set these environment values on the production API and every Temporal worker
deployment that consumes `automation-default`:

```yaml
- name: SUPPORT_COVERAGE_V2_MODE
  value: "v2_write"
- name: SUPPORT_COVERAGE_V2_WORKSPACE_MODES
  value: "{}"
```

The global mode applies to existing and newly created workspaces. The empty
override map removes workspace exceptions. Explicit deployment `env` entries
override values imported through `envFrom`.

Apply these entries through the authoritative `helpin-ai/gitops` production
manifests, preserving existing environment entries and container images. The
known deployment names are `helpin-server` and `helpin-temporal-automation` in
namespace `helpin`; confirm the current worker topology before applying. Updating
the pod template triggers a rollout. Alternatively, update the production secret
source with both values, synchronize it, and restart the API and automation
workers. Editing local `server/.env` does not change production.

Verify deployment readiness and `/api/health`, then query
`GET /api/support/coverage/v2/health?workspace_id=<workspace-id>` with an authorized
session. Expected rollout values are `requested_mode: v2_write` and
`capture_enabled: true`; assignment, read, and write should also be enabled unless
the existing quality gates report pause reasons.

Confirm the `coverage-analysis-v2` Temporal cron workflow exists and consumes
the `automation-default` queue. Analysis is scheduled every 12 hours. Verify an
eligible workspace produces a batch; an enabled flag alone does not prove that
analysis ran. Newly created workspaces are picked up automatically once they
have eligible conversations.

Existing quality gates remain in effect. Activation does not repair the separate
summary/list counting mismatch or promise a full historical backfill.

Published on 2026-10-02 in GitOps commit
`1475eb7f391d07cf7049e5f7dd1931caaf7a6448`. Both production manifests explicitly
set `v2_write` and the empty workspace override map. The GitOps Validate workflow
passed and the public production API health endpoint returned HTTP 200.

Using a refreshed session, ContentStudio's live coverage health endpoint confirmed
`requested_mode: v2_write` and capture, assignment, read, and write enabled. There
were no batches or failures yet, and topics and review signals were empty. Worker
readiness and successful analysis execution remain unconfirmed; verify a
subsequent analysis batch using the checks above.

## Automatic cadence: 12 hours

Automatic analysis uses `0 */12 * * *` (00:00 and 12:00 UTC). The workspace
analysis window spans 12 hours; existing cursors and unchanged-transcript
deduplication still apply. Manual Re-analyze retains its 30-day window.

Temporal stores cron schedules when the workflow is created. Updating the
application constant does not change an existing `coverage-analysis-v2` cron.
During rollout, wait for any active workspace analyses to finish, terminate
the idle cron execution, then restart the API so `EnsureDailyAnalysis` creates
it with the new schedule. Verify the registered cron is `0 */12 * * *`.
