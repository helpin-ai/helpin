# Helpin production monitoring

## Components and privacy

The API exposes a separate internal `:9090/metrics` listener only when
`METRICS_ADDR` is configured. The public ingress continues to target 8080.
VictoriaMetrics scrapes both API replicas every 30 seconds. HTTP metrics use
Chi route templates, never concrete IDs, request bodies or query strings.

The widget sends bounded, best-effort `POST /widget/telemetry` events behind
installation origin authorization and an independent Redis rate-limit budget.
Allowed fields are widget key, SDK release, stage, outcome and duration. Browser
family is derived server-side. Unknown fields and labels are rejected. No file
names, contents, raw errors, signed URLs or session tokens are reported. SDK
release appears in sanitized failure logs, not metric labels. A widget sends at
most 30 telemetry events/minute; failed telemetry never blocks an upload.

Client telemetry can be lost (offline browsers, blockers, closed tabs). Do not
interpret it as an exact denominator or a billing/audit record. Connection
counters include ordinary network disconnects; message receipts count inbound
frames, not a durable end-to-end delivery guarantee.

Jev metrics report evaluation outcomes, latency and routing acceptance/fallback.
Fallback includes confidence/policy skips as well as errors. Known AI-usage and
CRM-rule background errors are counted separately from arbitrary error text.
The existing Sentry integration remains responsible for captured exceptions.

## Live browser smoke

`helpin-widget-smoke` runs every five minutes in the `helpin` namespace using
Chromium and Firefox, pinned Playwright 1.58.2. It loads the public website and
CDN widget, waits for a real `session:joined` frame, uploads a synthetic PNG,
waits for Ready to send, deletes the owned unsent attachment, and revokes its
new session. It sends no conversation messages and holds no production secrets.
Cleanup failure is a failed smoke result. The DELETE endpoint refuses another
session's attachment and any attachment already linked to a message.

Metrics are written to the internal VictoriaMetrics import endpoint. A failed
check and missing/stale results have separate alerts. Job concurrency is
forbidden and execution is limited to four minutes. Dependencies use npm's
committed integrity lock. Regenerate the ConfigMap after source changes:

```sh
python3 scripts/monitoring/generate-smoke-manifest.py
```

The initial manifest is suspended until API/SDK rollout and a manual smoke pass
are verified. Enable by committing `suspend: false`, not by leaving cluster drift.

Local opt-in smoke (creates and cleans synthetic production uploads):

```sh
npm ci --prefix ops/widget-smoke
npx --prefix ops/widget-smoke playwright install --with-deps chromium firefox
node ops/widget-smoke/smoke.cjs
```

## Dashboard and alerts

Grafana dashboard UID: `helpin-product-health`, title **Helpin — Product health**.
VMRule: `helpin/helpin-product-health`. Alerts cover missing API metrics,
sustained 5xx errors, API p95 latency, browser upload/connection failures,
failed/missing browser smoke, Jev errors/fallbacks, and known background failures.
Initial thresholds are conservative operational defaults, not measured SLOs.
Review them against real traffic after one week. Existing infrastructure alerts
remain responsible for cluster, database and Kafka health.

Critical and warning severities match existing shared infrastructure Alertmanager
routes. A configured route is not proof of notification delivery; record the
controlled notification check separately. Do not print webhook URLs or secrets.

## Diagnosis

```sh
kubectl get pods,cronjob,jobs -n helpin
kubectl get vmservicescrape,vmrule -n helpin
kubectl logs -n helpin job/<latest-smoke-job> -c smoke
```

For an upload alert, compare initialization/confirmation HTTP metrics with browser
storage outcomes. Browser-to-R2 PUTs bypass API pods. Use sanitized smoke stage
logs and widget browser/release failures; never paste signed URLs or session
credentials. Investigate errors before raising alert thresholds.

Rollback: suspend the smoke CronJob in Git, remove the new monitoring resources
if needed, and revert application instrumentation. Keep upload transport fixes.

## Initial alert-delivery audit — 2026-09-17

Read-only production inspection found 73,159 Slack notification attempts and
73,159 failures with reason `clientError` over the current counter lifetime.
The previous 30 minutes of Alertmanager logs contained 37 HTTP 404 responses
with `no_active_hooks`. The configured Slack incoming webhook is inactive.
Existing alert rules therefore must not be treated as delivered notifications.
Restore the webhook or provision a replacement through the infrastructure secret
workflow, then verify a clearly labelled controlled notification. Never commit a
replacement webhook to source. The dashboard includes notification failures so
this dependency remains visible even while Slack itself is unavailable.

Application release: `7848f074f` (plus subsequent monitoring documentation fixes).
Deployment and live smoke verification remain pending explicit production
approval; Kubernetes server-side dry-run succeeded. Local checks passed: Go race
tests for metrics/telemetry/ownership/rate limits, Go vet/build, 29 SDK tests and
16 browser upload checks across Chromium and Firefox.
