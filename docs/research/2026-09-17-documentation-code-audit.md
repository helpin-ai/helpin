# Documentation and code consistency audit

This audit compares repository documentation with the checked-out code to identify
obsolete instructions and distinguish current guides from proposals. The inventoried source-review pass is complete. Source inspection establishes what this checkout implements; it does
not establish what is deployed or prove that a documented procedure runs successfully.

## Coverage and method

The initial inventory screened 508 existing Markdown files, including component
READMEs, repository instructions, plans, specifications, and agent skills. It
extracted headings, status signals, and source-path references. That screening is
not a semantic review of every assertion. Missing paths can denote proposed files,
globs, or local configuration, so they require manual investigation.

The [per-file coverage record](2026-09-17-documentation-review-coverage.csv)
lists every inventoried Markdown file. “Source-compared findings” means the stated
claims were checked, not that every assertion on the page was proven. “Screened;
content review pending” is an explicit remaining item. Source-path candidates are
automatically extracted and can include examples or proposed files; unresolved
references are leads for investigation, not confirmed broken links. The ledger now includes the final parallel review results; no existing inventoried
document remains marked pending.

The table below records findings verified against source. Filename and link
validation is a separate check and does not establish factual accuracy.

## Verified findings

| Document | Evidence in this checkout | Action |
| --- | --- | --- |
| Redis scaling, smart crawler, and March editor/import plans | Redis relay/presence are wired at API startup; SmartCrawler is wired in the worker; DocsEditor registers expanded blocks; Help Scout import saves canonical JSON | Added dated source-linked notices to five plans whose opening current-state claims were obsolete; retained design history without asserting every planned requirement shipped |
| March–April Help Center routing, localization, preview, ordering, publishing, collection and performance plans/specifications | Current code includes SSR, PublicIDs, locale helpers, preview JWTs, publication snapshots, conditional sort-key ordering and caches; later migration removes collection/article slug uniqueness | Added source-linked status notices to 20 historical pages, including obsolete collection QA expectations; preserved historical design and migration-boundary tests |
| PM mentions, recurring work, sprint settings, route panels, rename and task-key plans | Current task models/routes, recurring scheduler, scoped mention resolver and sprint settings exist; later key migration adds organization scope | Annotated 11 historical plans/specifications, including superseded global key-uniqueness assumptions; retained rollout checkpoints as historical evidence |
| [MCP workflow package](../../integrations/helpin-mcp/README.md) and its 12 prompts/skills | Public catalog exposes `list_deals`, `list_contacts`, and `list_conversation_messages`; batch creation requires `epic_id` and 1–50 tasks | Corrected three stale aliases, documented the batch prerequisite, and qualified hosted endpoint availability |
| Eleven older runtime, support, CRM enrichment, proposal-review and logging plans/PRDs | Worker package removed; provider dispatch and product commands remain in service code; proposal review uses saved base Markdown and `buildMarkdownDiff` | Clarified historical extension points and logging counts, fixed a misleading “current” planner-reference link, and retained already-correct superseded notices |
| Follow-up, availability, unread/sidebar, and invited-assignment plans/PRDs | Current defaults and versioned sequence migration implement two reminders; handoff uses availability; V2 unread is personal; assignable-member queries include pending identities | Reviewed 11 documents; clarified outdated defaults, schema guidance and historical status while preserving rollout and delivery qualifications |
| [Widget security integration](../widget-messenger-security-integration-guide.md) | Server verifies newline-canonicalized HMAC proof in `identity_verification`, not the documented JWT | Replaced JWT examples with current signing/SDK flow, origin requirements, edition defaults and failure diagnosis |
| [Widget architecture](../widget-architecture.md) | Loader/alias/emoji paths match source; npm ESM build is separate | Linked npm build distinction and removed stale chunk-size/catalog-count claims presented as current measurements |
| Sidebar/PM-type contributor instructions and Rust capture instructions | Type barrel includes coding sessions, skills and task insights; repository uses pnpm; timestamp handling branches by credential kind | Completed the type map, aligned package commands and narrowed replay-window guidance to server credentials |
| [Role analysis](../rbac/pm-docs-role-analysis.md) | Epic service uses `requireCanEditTeamEpics`, with member access covered by tests; document listing filters non-admin drafts to the current user | Labeled the old matrix historical and documented the specific contradictions with source links |
| [Routing scenarios](../test-scenarios-triage-routing.md) | Current inbox wizard embeds conditions and AI routing; global Automated routing controls both manual rules and AI | Labeled the old manual checklist historical and described current UI differences; did not claim manual scenarios passed |
| [Lead guide](../../packages/sdk-js/docs/sending-lead-events.md) | `HelpinClient.lead` throws on non-object payloads, validates email, tracks analytics and separately syncs backend identity; loader drains `helpinQ` | Fixed callable queue stub, nullable client use, validation behavior, and Vue usage |
| [Form payload](../../packages/sdk-js/docs/form-tracking-payload-example.md) | `configured-capture.ts` observes configured submits, emits a field-name map, and has no field-change listener | Replaced obsolete DOM-array payload with current allowlist configuration and event attributes |
| [Magento workaround](../../packages/sdk-js/docs/MAGENTO_INTEGRATION_SOLUTION.md) | Vite now emits ES-format assets; retained tests use a simulated RequireJS page | Marked UMD workaround and its validation claims historical; replaced customer key in example with a placeholder |
| [Frontend browser tests](../../frontend/e2e/README.md) and [SDK browser tests](../../packages/sdk-js/test/e2e/README.md) | Frontend Playwright starts Vite directly and Vite resolves widget-core dist; SDK package script builds before browser tests | Added clean-checkout prerequisites and corrected SDK agent instructions to use the build-before-test script |
| [Pipeline test runbook](../../events-pipeline/e2e/README.md) | Compose publishes NATS client ports 14222–14224; `run-k6-remote.sh` does not forward `NO_VU_CONNECTION_REUSE` | Corrected port reservations and documented the wrapper limitation |
| [Frontend setup](../../frontend/README.md) | `frontend/vite.config.ts` enables React Compiler and resolves the compiled widget package; root package metadata pins pnpm | Replaced template boilerplate with repository-specific setup, build, and edition instructions |
| [Local infrastructure](../../docker/README.md) | Root Compose defines Temporal and NATS 2.14.5; API/frontend service examples are commented out | Added Temporal, corrected service descriptions, and passed Redis authentication into the container explicitly |
| [Runtime staging](../agent-runtime-staging-runbook.md) and [local setup](../agent-runtime-local-setup.md) | `server/cmd/api/main.go` starts the projection consumer under a PostgreSQL leader | Corrected projection ownership from worker to API |
| [CLI admission](../2026-09-15-cli-local-admission.md) | `CLIService.Discovery` adds gateway, event, and artifact capabilities when the gateway flag is enabled | Labeled the admission-only account as a historical Phase 3 snapshot and linked the successor guide |
| [Internal tools](../internal-tools-framework.md) | The worker tool registry files are absent; catalog/profile metadata lives in `agentcontract`, while product commands live in service code | Replaced obsolete registration and execution instructions with the provider/command-service workflow; corrected canonical-name compatibility boundaries |
| [Mattermost integration](../mattermost-integration.md) | Proposed Mattermost backend and frontend files are absent; no corresponding implementation found in the application sources | Explicitly labeled as an unimplemented proposal, not setup instructions |
| [Public MCP](../public-mcp-server.md) | Settings page is `frontend/src/pages/settings/MCPSettingsPage.tsx` | Corrected the implementation map |
| [Email architecture](../email-architecture.md) | Coverage digest sender is absent; retained model/repository records do not send mail; notification digest wiring is separate | Removed claims of an active weekly coverage digest sender |
| [AI connections](../ai-connections.md) | `server/go.mod` pins SDK v0.6.0 | Removed contradictory alpha pin and unsupported claim that both repositories share the same pin |
| Repository agent instructions | `.go-version`, `justfile`, provider dispatch, and Community migration configuration differ from older instructions | Corrected toolchain, pnpm setup, build shortcuts, tool naming, and AutoMigrate assumptions in root/backend guides |
| [Database migrations](../ops/database-migrations.md) | Docker targets and Kubernetes manifests use a separate migration image; Community disables AutoMigrate | Corrected image checks, additive migration requirements, and historical deployment claims |
| [Worker packaging](../../server/packaging/README.md) | `deploy-prod.yml` builds containers and has no worker Debian packaging/upload step | Documented manual packaging, edition selection, updater prerequisites, and fixed version expansion in the sample command |
| [Email fallback operations](../ops/support-email-fallback-runbook.md) | Service logs and recovery binary remain present | Replaced the undocumented `k` shell alias with `kubectl` |
| [JavaScript SDK](../../packages/sdk-js/README.md) | `HelpinClient.lead` takes `LeadProps`; `articleView` is a public method | Corrected the signature and added the missing method/type entries |
| [Capture API](../../events-pipeline/rust-capture-api-guide.md) | Authentication precedes body parsing; registry classifies credentials; enrichment bounds timestamps; current error mapping differs | Corrected authentication, recent-event replay limits, response shape, enrichment fields, and request limits |
| [Pipeline entry point](../../events-pipeline/README.md) | `scripts/stack.sh` uses `local/compose.yaml` | Corrected the directory map |
| [Mobile setup](../../apps/support-mobile/README.md), [icons](../../apps/support-mobile/ICONS.md), and [QA](../../apps/support-mobile/QA.md) | Icons and a real support UI exist; the TestFlight workflow initializes the absent native project | Removed obsolete placeholder/missing-icon claims and separated dated QA from pending device verification |
| [Provider matrix](../plans/provider-capability-matrix.md) and [planner reference](../plans/planner-tool-contract-reference.md) | Cited executor factories, tool registry, and worker tests were removed | Labeled historical and linked current tool/provider guidance |
| [Widget comparison](../widget-feature-parity.md), its [PRD](../prds/widget-sdk-feature-parity.md), and [lead plan](../../packages/sdk-js/docs/lead-function-implementation-plan.md) | Several listed widget gaps and the proposed lead method have implementations | Clarified historical scope and linked current package documentation |
| [AI usage metering](../ai-usage-metering.md) | Community usage recording and EE token pricing replaced the documented feature-floor formula | Rewrote the guide around current lifecycle, accepted pricing context, reservations, and settlement |
| [Native release runbook](../ops/native-release-runbook.md) and [subpath proxy guide](../ops/help-center-subpath-reverse-proxy.md) | Release hashes describe a preparation session; proxy context and settings now exist | Distinguished historical release/implementation claims from current source behavior |
| [CRM summaries](../crm-entity-summaries.md) | Company summary service, evidence assembly, UI, and read/refresh routes exist | Removed the contact/deal-only limitation and documented company support |
| [GitLab integration](../gitlab-integration.md) | Handler and UI use access tokens and permit self-hosted origins; no documented OAuth setup variables remain | Replaced OAuth setup with the implemented token connection workflow |
| [Customer.io contract](../customer-io/workspace-lifecycle-data-contract.md) | Identity mapper emits micro-USD usage fields, not `credits_remaining` or `modules` | Corrected the workspace attributes and separated campaign design from live campaign state |
| Frontend table performance and virtualization reports | Current CRM tables use `useTableSurface` and memoized row components; `StoryListView` is absent | Marked the old bug assertions as historical, requiring reproduction before action |

## Additional code comparisons

- Read all 35 marketing, CRM Playbook, and documentation-writing/maintenance skill
  files and references in the reviewed group. Compared embedding/catalog loading,
  named tools, and coverage-gap completion outcomes with current source. These are
  runtime instruction assets, not operator setup guides. Their conditional tool
  access and draft/execution boundaries were retained; no prompt behavior changed.
  External adaptation/license provenance was not reverified in this code audit.

- Community operator guidance was compared with Compose, the initializer,
  installer, ingress, callback configuration, signing implementation, and release
  workflows. Service/volume names, infrastructure pins, rate-limit defaults,
  embedding dimensions, HMAC format, and artifact retention match the inspected
  sources. The checked-in upstream exception inventory agrees with the documented
  counts and expiry. These checks do not rerun backup recovery, image scans,
  release publication, or public DNS/HTTPS acceptance. Per-file evidence and
  limits are recorded in the coverage CSV.

- External MCP rollout, host allowlist, OAuth client-auth defaults, and encryption
  configuration match the current configuration loader. Historical token/tenant
  path fragments found by the scanner are descriptive text, not missing files.

- Support preview routes enforce `support.admin` as documented; support AI-control
  routes enforce `support.edit`, and the service checks the expected control version.

- React, Next.js, and Vue package README exports and hook examples were compared
  with their package entry points, client factories, pageview hooks, and widget
  delegation. Widget-core exports and mount options were compared with its entry
  point. This does not verify the contents of packages currently published to npm.
- The demo login route and read-only middleware exist as described. The guide's
  seeded workspace and deployment status are operator setup claims, not something
  that source inspection can verify.
- The current Rust capture README's binary names, visitor sharding, NATS in-flight
  configuration, and spill settings match source. Capacity measurements and
  cluster readiness require their recorded benchmark/deployment evidence; they
  are not newly measured by this audit.


## Repository-wide link review

After the audit report was added, the complete inventory contained 509 Markdown
files. All 1,292 local links pass the repository link checker. This includes
component documentation, historical plans, instructions, and skills rather than
only the maintained-guide manifest.

The historical-link pass repaired 192 machine-specific links to source files
that still exist, replaced 51 links to absent source files with explicit
historical-path references, and corrected one moved notification research link.
It changed 20 documents. Removed files are not silently mapped to a similarly
named modern implementation; a historical design can describe a different
contract. Provider-matrix links removed earlier are counted separately.

This validates navigation, not the behavior described in every linked file.
External destinations and live installations were not validated in this pass.

The core runtime instruction pass compared approval/checkpoint schemas, preview
panel selection, epic/task persistence registrations, backend delivery, and support
reply outcomes against their current handlers. Seven additional instruction files
were reviewed without changes. Document editing’s adaptive reads and atomic edits match the newer command
metadata and handlers; the subsequent renderer/export comparison also confirmed
its static SVG and Word-export guidance. The static agent tool catalog
contains older document-read descriptions, so it is not sufficient evidence alone
for calling the skill outdated. No live agent session was exercised.

The remaining 18 runtime skill/reference files were read, including planning,
diagrams, dependency auditing, and security triage. Diagram behavior matches the
editor and export code, with SimpleDiag 0.2.4 pinned in the lockfile. Generic
dependency instructions were reviewed as policy; external registries were not
queried and no dependency upgrade assessment was performed.

Two corrections resulted: the task-decomposition example now uses explicit
placeholder paths instead of absent repository paths, and security triage no
longer guarantees that the worker image ships scanner binaries, rules, or
prewarmed databases. The current Helpin and sibling Agent Runtime Dockerfiles do
not support that guarantee. Agent Runtime's scanner handlers report missing
binaries, fallback warnings, and parse failures; the skill now treats availability
as deployment-dependent and warns against interpreting incomplete scans as clean.
Four focused existing agentcontract skill tests passed after updating the obsolete
packaging assertion in the security skill test. No scanners were run.

The framework-wrapper and widget-core pass compared five READMEs with package
manifests, exports, client factories, pageview hooks, middleware helpers, mount
options, and upload-test configuration. Four guides changed: React/Next.js
examples disable SDK route tracking when a hook owns it; Vue explains the same
requirement; Next.js documents the helper's NextRequest/NextResponse boundary;
and widget-core documents its private workspace status instead of public npm
installation. The mount-options table now links to the complete source contract.
The upload-suite README matched its mocked browser tests and CI configuration.
All five local links passed; no deployed CDN, npm publication, or framework app
was tested. SDK default pageview tracking and the hooks both observe navigation,
so duplication concerns route changes, not a verified duplicate initial event.

The mobile release/push pass found obsolete pre-implementation claims in the
native plugin README. Registration and foreground notifications now have app
consumers, and the Swift package uses Firebase iOS `from: "12.0.0"`; its comments
record a prior build issue, contradicting the old blanket “nothing compiled”
claim. The replacement guide documents current APIs, the single pending-tap
buffer, and remaining native acceptance checks. TestFlight runner, signing
inputs, staging defaults, build-number selection and artifact retention match
the workflow. Both guides now link their source/checklist. Ten local links pass;
no release was uploaded and native/APNs/FCM behavior was not tested.

The support AI preview and human-control guides match their current service,
repository, route, and frontend boundaries. Corrected the human-control guide's
“two optional escalation fields” to the actual three named fields, and added the
preview backend's 16,000-byte input / 100-history-turn limits. Source links now
cover both guides. Nine local links pass. Existing tests were inspected as
coverage evidence; no provider, inbox, or PostgreSQL acceptance run was performed
for this documentation pass.

The demo README previously described the intended production workspace as an
existing deployment. It now separates login/read-only implementation from
provisioning, seeding, and operating policy. Source inspection also shows that
login rejects platform admins and 2FA accounts but does not enforce viewer-only
membership in exactly one workspace; the guide now makes that manual setup
check explicit. The method guard applies to its authenticated route group, not
the public widget. Five local links pass; no production state was inspected.

Four notification research/index pages were read. The March queue comparison
recommends River, but the current module has no River dependency: Helpin manages
notification delivery records through its service/repository and runs a digest
sweep in the API at startup and every 15 minutes. The comparison now labels its
recommendation as historical, with source links. Linear and Shortcut research
also explicitly distinguish historical vendor observations from Helpin behavior;
external feature claims were not reverified. Twelve local links pass.

Six public contributor/policy documents were reviewed for internal consistency:
CONTRIBUTING, SUPPORT, SECURITY, CLA, CODE_OF_CONDUCT, and the PR template.
Their contribution terms, issue-form references, and validation instructions
match the corresponding repository artifacts; no text changes were needed.
This did not verify legal sufficiency, live GitHub reporting settings, or release
publication. The roadmap comparison was subsequently completed against deployment defaults,
API module classification, Compose and provider construction. Its default-surface
wording now distinguishes disabled navigation/modules from retained shared APIs,
including coding-session routes under Agents. Analytics suppression, embedding
dimensions/fallback, contact anonymization and mail configuration also match the
inspected implementation. Future release items remain explicitly planned.

The external MCP guide's first source pass removed an obsolete instruction to
build against an unreleased SDK through a local Go workspace: committed v0.6.0
already contains mcpauth and run-scoped MCP types. It also corrected the claim
that Kubernetes manifests explicitly disable MCP; the application defaults it
off, while deployment-supplied values must be inspected separately. Provider
choices now include the existing Linear/Sentry presets, and route permissions
and callback workspace resolution are explicit. Two local links pass. The subsequent lifecycle pass compared state consumption, refresh locks,
outbound address/redirect limits, run-bound encryption, credential-only rotation,
authentication pauses, and terminal cleanup with the pinned SDK and sibling
Runtime source. Cleanup errors are logged rather than guaranteed impossible;
notification emission can fail. The guide now states those limits. Existing
Helpin externalmcp tests passed against local fixtures. No external provider or
deployed Runtime revision was verified.

Two editor research notes now distinguish historical observations from current
source. Installed StarterKit includes ListKeymap and TrailingNode, so the old
missing-extension diagnosis is superseded. Callout exit handling remains custom;
the code-block extension changes rendering without adding shortcuts. Paragraph
spacing is explicit in current CSS (editor 1.05rem, help center 1.1rem, both line
height 1.7), rather than an unimplemented general recommendation. Browser gesture
and visual parity were not tested, and vendor comparisons were not refreshed.

Nine collection indexes were reviewed for purpose, status and navigation. All
local destinations exist; reviewing an index does not complete review of its
linked documents. The browser-testing index now identifies its hard-coded
legacy host/key fixture and directs contributors to the maintained SDK browser
suite. Mockup and operations labels now describe their subjects in plain language.
No legacy fixture was opened against its configured remote host.

The event-pipeline capacity page now separates reported August load results from
current source verification. Production replay memory has increased to a 768 MiB
request and 2 GiB limit; the table now matches the manifest. The historical
15k command omits the current harness p99 threshold, whose 25 ms default is
below the recorded 43.55 ms result. This mismatch is documented without changing
acceptance criteria or claiming a fresh benchmark. Deployment guidance now
accounts for per-workflow path filters and explicitly selected first-rollout
builds. The runtime and CRM deployment claims were subsequently reviewed below.

The Rust capture README now describes the actual readiness boundary: capture
remains ready until shutdown and does not probe spill writability or token
availability. Print mode still requires authentication and skips enrichment and
delivery. Added token prerequisites, the local capture/writer port collision,
and bootstrap’s process-environment requirement (it does not load `.env`). The
linked original implementation plan is labeled historical. These conclusions
come from source inspection; no ingestion service was launched for this review.

The deployment review now also covers retention migrations, certificates, API
evaluator wiring and activation boundaries. A ten-minute micro-batch sweep is
conditional on a ClickHouse client; a successful sweep can still report failed
rules. A single ingested event does not guarantee a signal. Verification steps
now require rule-specific qualifying evidence and separate ingestion from signal
creation. Workspace rollout defaults to live independently of seeded rule shadow
mode; existing activation must be inspected before assuming actions are disabled.
No live cluster or secrets were accessed during this comparison.

The March Kafka/session-windowing research is now explicitly superseded by the
NATS/Rust implementation, with current dependency, sessionizer and ClickHouse
recovery references. Vendor assessments remain dated research, not refreshed
recommendations. The retired reliability PRD now explains concrete differences
in probe behavior, token storage and retained fallback volumes and points to
current operating guides. The pipeline overview’s benchmark wording now agrees
with the capacity page’s historical evidence boundary.

Community publication policy, installation README and dated implementation
record were reviewed against installer/Compose, workflow gates and packaging
allowlists. The archive regression check caught source-only ROADMAP links added
during this audit: those now use repository URLs, keeping the operator bundle
self-contained. The existing packaging test passes, including digest pins,
checksums, exclusion checks and rejection of incomplete architecture inventories,
with registry commands mocked. Historical branch names, test totals and scan
findings are now explicitly dated rather than current verification claims.
Publication decisions and real architecture acceptance gates remain unresolved.

The contributor development guide matches current toolchain pins, task recipes,
Compose service names, dotenv initialization and frontend URL defaults. Added
the missing documentation naming/regression checks and explicit EE development
recipes. Its linked server environment example still contained the retired
`usermaven` ClickHouse database and unused Kafka migration variables; replaced
the DSN with the local stack’s `default` connection and removed those dead
settings. Current migrations and queries explicitly address `helpin.events`.
No application process was started for this source comparison.

The engineering index, writing convention, documentation-check guide and
publication triage were reviewed for consistency with the skill, actual checker
scope, maintained manifest and CI job. Removed a stale publication-triage claim
about projection ownership after confirming the staging guide already names the
API consumer correctly. Publication disposition concerns remain distinct from
that corrected technical issue. Index review does not certify linked product
contracts or historical plans.

The CRM email guide’s initial-sync account was obsolete: OAuth now preserves
an `initial_history_id` separately, allowing historical backfill before using
that cursor for incremental arrivals. Documented checkpoint precedence, forced
historical sync, manual workflow wakeups and owner-or-admin diagnostics access
(the repair endpoint remains admin-only). Participant association, purge and
downstream-effect claims were subsequently compared as described below.

The remaining CRM email comparison found a material purge evidence gap: the
service deletes only the account; legacy SQL and lifecycle fixtures provide
mailbox cascades that the current foundation schema lacks for threads, messages
and calendar events. The guide no longer promises complete purge on those
installations. This requires a separate schema/cleanup implementation fix; no
database was modified. Also corrected participant filtering, best-effort workflow
cancellation, repair scope, non-atomic message/association writes and logged
downstream failures. Multi-contact mail without a deal skips summary refresh.

Conversation-ingestion documentation still described the old phase1a detector.
Updated the implemented plain-text HTML fallback, 0.6 confidence floor, source
evidence verification, commercial qualification, English narrative validation
and commercial-v4-en metadata. Clarified that persistence-stage failures may
follow earlier writes, unlike an LLM response failure before storage. The guide’s
repository deduplication and consumer claims were subsequently reviewed below.

Completed the conversation-ingestion comparison with interpreted-signal
uniqueness, observation persistence, current-version/commercial-event thread
suppression and source reconciliation. The old source/type uniqueness description
now identifies the legacy compatibility path. Added the missing thread-reply
trigger and aggregate automation-health UI; detection queue admission is distinct
from qualified evidence, mapped interpretation and persisted signal visibility.

The agent-dock guide had missed runtime-owned approval and shared-chat
visibility. It now distinguishes risk_based/mutating_tools/always policies from
the legacy host proposal guard, permits direct child launch contracts where
configured, and separates visibility access from owner/run-credential checks.
Capability/repository/result details were subsequently compared below.

Completed the dock comparison: managed presets enforce risk-based approval,
capability rotation occurs on the next message to a paused run, and result
pagination defaults to 6,000 Unicode characters with a 12,000 maximum. Confirmed
plan-owned result access, 3,000-character delivery summaries, the 30-second
delivery backstop, immutable custom-agent drafts and repository-target recovery
from checkout metadata. Removed remaining unconditional proposal/approval wording.

Automation product-model navigation was outdated: Library now redirects to
Triggers, and Catalog groups Triggers, Tools and Skills. Corrected routes and
labels, removed stale branch attribution, and distinguished design expectations
from guaranteed detail-view behavior. Shared-worker checkpoint/Playbook handler
separation matches source; detailed Playbook publication contracts were subsequently compared below.

Completed the automation product-model comparison with publication compilation,
immutable snapshot storage, separate confirmed enable/adopt operations, live
member/module/entitlement checks, AI usage preflight and action authorization.
Added the actual compiler boundary: system Beacon, disabled supported CRM Flow,
native SDK and resolved specialization; executable publications require schema
version 2 and the work-due trigger. This does not certify deployment activation.

Coding-agent execution ownership matches the runtime launch and projection
boundary. Added an operationally important limitation: terminal state can persist
while product finalizers log failures, and summary-dependent delivery may be
deferred. PR/task delivery must be checked separately. Linked launch, repository
host and finalizer sources and distinguished adapter source support from the
capabilities of an actual deployed runtime image.

The CRM overview still described removed property/list routes and the old
`crm_sequence.go` implementation. Compared the current CRM router, role matrix,
frontend route inventory and hooks, outreach models and runner, and Temporal
worker dispatcher wiring. Corrected email paths, signal administration gates,
member-profile lookup, and the CRM overview redirect; replaced the obsolete file
inventory with linked entry points and documented current outreach delivery
without an exactly-once claim. Completed the remaining model/intelligence comparison against model constants,
suggestion execution, health scoring/read refresh and the API's six-hour sweep.
Removed retired task activities, separated suggestion decision and execution
status, qualified enrichment source labels, added Emails navigation, and corrected
the review redirect's preserved scope. The page is now source-compared; deployment
and provider availability were not tested.

Customer.io campaign guides described proposed schedules without exposing the
current instrumentation gap: milestone constants exist but no production caller
emits `module_first_value`. Compared the milestone catalogue/references, analytics
hostname guard and Community config, role matrix, identity/outbox paths and
Enterprise billing producers. Marked messaging as proposed, corrected legacy trial
trigger assumptions and unsupported manager audience labels, and documented that
hosted delays/frequency caps require configuration. Added a historical caveat to
the alignment design; its full plan/test comparison remains pending. No remote
campaign was inspected, changed, or activated. Three guides and 11 local links pass.

Reviewed all four August 10 Customer.io plans/specifications against the current
outbox schema, locking/fencing repository, worker retries, stable ULIDs, signup
callers, and Enterprise trial transactions. Added current source/method mappings
and historical status. Recorded actual Retry-After replacement/one-hour cap and
conditional-update expiry rather than the proposed maximum-delay/RETURNING
algorithms. Original RED/GREEN steps and hosted campaign state are not presented
as current verification. No provider calls or runtime changes were made.

Compared the Jev production guide against configuration/API wiring, provider
validation, triage/tagging, lifecycle guards, admission/usage and production
manifest/workflow sources. Current default modes and decision behavior match;
clarified invalid keyed configuration stops startup and timeout is rejected rather
than clamped. Branch names, test-pass claims and rollback image are now explicit
historical records; operators must use current deployment evidence. No Jev request,
production access, or deployment was performed.

Completed both historical frontend table reviews against current CRM tables,
TaskListView, shared surface/style code and deal edit hook. Added source-linked
status for each recommendation: shared scrolling and memoization are implemented,
overscan differs by table, and sizing-version invalidation is intentional.
Removed the unqualified high-confidence speedup conclusion; no browser benchmark
or fresh rendering defect is claimed by this source review.

Reviewed three forwarding backfill/setup documents against SQL, migration contract
tests, inbound verification and frontend progress/polling/cache code. Added
historical result labels and current sources, and clarified ordinary qualifying
inbound mail can verify a route as well as the token round trip. SQL-clause tests
do not establish applied database state; no mail or migration was executed.

Reviewed widget composer alignment and saved Mermaid design/plan pairs. Current
branding uses one shared attribution link and a 5px gap, while saved rich text
already delegates Mermaid, nwdiag and SVG code blocks to diagram components.
Added historical status and source/test links so old checkboxes, failure
expectations and branch merge instructions do not imply current unfinished work.
Mocked renderer tests are not claimed as visual rendering verification.

Reviewed the docs autosave reconciliation and support video attachment plans.
Current code implements snapshot acknowledgement with queued-save handling and
100MiB support uploads with staged confirmation, retry/cancellation, send guards,
and native video/download rendering. Added current source links and limitations:
snapshot matching is not conflict merging and accepted video types do not ensure
codec playback. Old test counts, screenshots and branch/agent instructions remain
historical records, not fresh verification claims.

Reviewed four agent sharing/timeline design and plan documents against public
share authorization/projection/menu, server interaction events, frontend event
reconciliation, streamed run state and completed-work grouping. Added implemented
status and current live timestamp ordering. Recorded that raw working-group duration
calculation can produce NaN for invalid authoritative timestamps despite the old
design's zero-duration guarantee; no runtime fix or fresh test result is claimed.
Public-token retrieval is distinguished from parent-chat checks at share creation.

Compared the billing usage-chart plan and August 11 metering design with current
EE chart, shared lifecycle, legacy wrappers, EE reservations/rates and provider
decoders. Chart paths moved to the EE tree and values are allowance percentages
from micro-USD. Marked fixed-unit/10%-cache/no-reservation financial assumptions
superseded while preserving relevant provider-normalization history. No billing
operations or deployed policy verification were performed.

Completed the paired August 11 metering implementation-plan review. Confirmed
runtime support delivery does not double-charge and named call sites use payload
or content-derived keys. Added current lifecycle ownership and clarified that
these keys are content-based rather than universally per-request. The legacy
fixed-unit architecture and old test steps remain explicitly historical.

Reviewed the My Work AI suggestions implementation record against route/service
permissions, personal repository scope and revision locks, review-only outcomes,
CRM exclusion, bounded classifier/Temporal schedule and frontend UI. Added actual
batch/deadline/input limits and historical-result labeling. No production schedule
health or old test/browser totals are claimed by this source comparison.

Reviewed OG metadata design/plan against marketing layout/helper/generator,
app shell and Netlify transformer. Clarified public-response fetching versus
title-only consumption, title truncation, path-only preview URL and host/platform
limits. All six committed PNG headers are 1200×630; no artwork inspection or live
crawler/cache validation was performed. Historical completion claims are labeled.

Reviewed the September 12 support reply-delivery record. Current draft state
clears manual channel choice after successful sends, and ReplyComposer no longer
renders or submits an editable email subject. Backend subject override validation
and metadata snapshots remain. Added these deviations, current presence/failure
behavior and source links, preserving old validation as historical only.

Reviewed CRM individual email foundation against signature ownership/escaping,
composer/reply drafts, deal entry and send validation. Documented the backend's
10,000-byte signature limit and provider-success versus local-persistence boundary;
missing local message/attachment/contact records must not imply resending mail.
Historical browser/test/vendor-inspiration claims remain explicitly historical.

Reviewed the Playbook publishing UI plan against setup/readiness, editor save and
confirmation revision handling, tooltip blockers, backend publication/enrollment
commands and automation Configure checks. Added implementation references and
explicitly separated form readiness, version publication, connection publication,
enrollment and activation. Historical browser/test/review results were not rerun.

The September 9 settings save feedback plan now distinguishes its historical
implementation record from current source behavior. Autosave serializes requests
within a scope but does not cancel dispatched requests; failed navigation can
explicitly discard edits. Server responses update the query cache without
necessarily normalizing the open draft. Original test counts and preview results
remain historical evidence rather than newly executed validation.

The August 11 forwarding round-trip plan and design now document differences
from the current implementation: test-subject messages are consumed even without
a matching token, verification uses an ordinary row save rather than a conditional
token update, and the UI polling window is not token expiry. The four-step setup
and later historical-route backfill supersede the original UI/rollout assumptions.
These are documentation findings; no email was sent or runtime code changed.

The lazy Dock chat history design now records the actual synthetic
`dock_work_summary` wire format, explicit-final precedence, and chat-scoped
multi-message work interval. Projection is page-local; the current frontend
requests 50 messages and eagerly imports ChatView. The planned smaller page and
separate bundle are not presented as shipped. Disclosure caching is in memory.

The April module-access progress tracker now describes the expanded managed
scope (CRM, Support, Automation) and deployment filtering of otherwise implicit
owner/admin access. Agents visibility follows configured deployment policy and
Automation access. Grant identifiers and query invalidation match current code;
old completion/test checklists remain historical rather than fresh validation.

The organization PRD now marks its pre-implementation state as historical.
Current code includes viewer members, owner/admin organization editing, separate
ownership transfer, default-organization onboarding, and Enterprise billing.
Organization roles do not bypass active workspace membership. The proposed
organization owner/admin restriction on workspace creation is not enforced in
the inspected handler/service/repository path, and organization ID remains
optional there; this is recorded as an implementation gap, not silently fixed.

The Ask media-attachment design is now explicitly historical: current code
supports documents, automatically analyzes source-context media, and handles
hosted images as well as private uploads. The notice distinguishes signature
check branches and heuristic decorative filtering from the proposal's stronger
guarantees. Fixed count/resolution/duration limits, scheduled analysis retention,
and durable per-file analysis activity are not established by the inspected path.

The September CLI delivery record now separates verified Helpin backend paths
from external CLI release/terminal claims. Compiled-CLI HTTP tests skip without
an explicit binary; runtime CLI source and referenced smoke scripts are absent
from this repository and the available adjacent runtime checkout. The document
qualifies replay/fencing and bounded settlement, gives the server test directory,
and replaces a machine-specific binary location with an explicit placeholder.

The existing-task epic-link design now distinguishes transactional membership
updates from best-effort post-commit delivery inheritance and activity logging.
Move review is a UI step, not a server review token; the handler caps the raw
request before deduplication. Whole-model task saves do not establish protection
against concurrent changes to unrelated fields. Historical tests remain labeled.

The May Support custom-views plan now identifies the implemented feature and
later built-in view overrides. Editing filters preserves the active custom view
with a dirty flag, superseding the original clear-on-edit requirement. Creation
requires Support edit access as well as service ownership/sharing checks; sharing
a preset does not bypass conversation access. Old workflow/test steps are historical.

The August Ask working-groups plan now records superseded UI claims: active
groups start collapsed, live grouping combines adjacent tool runs, and concise
tool rows do not expose all successful payloads. Segment preservation is distinct
from data transfer and visible detail. Later lazy history and pause-specific live
status behavior are linked rather than represented by the old completed checklist.

The deal-creation plan is partially reviewed: current form code lacks the
promised probability override and custom-property controls. Its annual revenue
default differs from the API's omitted-value one-time default. Atomic customer
associations and client-scoped record serialization were inspected. The subsequent stage/display/filter review confirmed source wiring, color
preservation and per-record rollback; visual/browser and database execution
claims remain historical results rather than freshly repeated checks.

The MCP registration release plan now scopes launch/data assumptions and
three-repository test claims to the original cutover. Catalog validation checks
schema presence/serialization rather than a full metaschema. Current Helpin and
the available adjacent runtime pin different SDK versions, so inspected runtime
retry/refresh code does not establish current released compatibility. Provider
refresh intervals remain configuration-specific.

The agent-run chronology implementation plan now matches its reviewed design:
projected sequences order persisted events, timestamps order mixed live/persisted
streams, and completed-only grouping is wired in both run views and Automation's
transcript pane. Raw invalid-date duration handling remains qualified. The old
unchecked task list no longer implies the implemented feature is missing.

The PM dropdown migration audit now separates historical inventory/browser
results from a fresh source-boundary check: 133 production TSX files in the two
PM folders contain no matches for the regression's direct legacy import/native
select pattern. Shared adapters remain wired; the scan does not establish every
indirect wrapper or visual behavior. Bulk Apply remains per-task partial-success
execution rather than an atomic batch.

The addressable-block plan now describes implemented storage, transactional
aggregate/block synchronization, revisioned mutations, block-aware embedding/search,
and deletion asset scanning. Supported top-level blocks are distinguished from
all nested nodes or a fully block-native renderer. Block-read errors fail sync;
aggregate fallback applies when block chunking yields no usable chunks. Migration
execution and universal compatibility completion are not inferred from source.

The June triage gap plan now records current sender-email/all-any conditions,
enabled defaults, optional later-message evaluation, and Jev/local/LLM selection.
Its fixed Haiku baseline is obsolete. Query-builder rules, general phrase
normalization, rule-change backfill, and the wider taxonomy/action/learning
roadmap remain proposals rather than inferred shipped behavior.

The archived team-workflow repair plan now points to task-era service/repository
code and legacy migration 045 rather than its proposed 043 story file. Current
remapping includes null-team tasks, performs separate per-state updates, and logs
failure without failing workflow creation. The legacy SQL has a narrower one-team
condition for unassigned stories and is not presented as a current repair command.

The setup required-flows plan now marks its checklist as historical and points
to implemented catalog/evidence/action/goal-editor code. Enabled template counts
are distinguished from successful execution, and the actionable-deal predicate
is explicit. Frozen Support scope and the Go 1.24 label are not current global
contracts; the module's toolchain requirement remains authoritative.

The Support runtime guide now includes optional pre-reply Jev handoff assessment
and its stale-message/control fences. Atomicity is scoped to Runtime reply storage
and processing-row settlement; websocket publication follows commit. Human control
revokes delivery independently of successful runtime cancellation. The main API
consumer/runtime execution split and retired heuristic ownership remain current.

The Support global-search plan now points to its actual page/route and migration.
Implemented search remains mailbox-scoped, caps reported totals at 1,000, and
limits query size/quoted phrases. PostgreSQL full-text and SQLite substring
paths are distinguished; safe highlights render text rather than injected HTML.
The original unchecked tasks and toolchain/build instructions are historical.

The sprint automation prompt dismissal design was source-compared. Disabled-row
dismissal and team-wide suppression are implemented, including closing the prompt.
The historical settings instructions are outdated: editing now lives in team Sprint
Settings. The comparison also records two code limitations: returned fetch errors
can still show the prompt, and the settings overview can label a disabled row as
“Auto-create ✓” because it checks existence rather than enabled state. No application
behavior was changed, and helper test source inspection is not a browser test run.

Three further historical documents were source-compared: demand attribution,
epic task linking, and Setup required-flow refinement. Attribution event IDs and
cache-hit recording are implemented, as is widget email-domain company matching.
The original safety claim is not established: company match provenance is stored
separately, while behavioral identity resolution carries person trust forward
without checking it. Email company fallback and production attribution targets
also remain unproven. Epic linking is transactional for membership writes, with
post-commit side effects; Setup completion checks enabled template rules rather
than successful executions. Each document now separates these findings from its
original proposal and test checklist.

The bulk-edit team-field plan and new-home hero design were source-compared.
Bulk editing now uses QuietDropdown, requires an explicit new-team status, retains
newly selected labels while clearing old labels, and can partially succeed across
per-task requests. The hero experiment's route/component and footer exception are
absent; its animation and accessibility requirements are labeled as historical
proposal criteria rather than current website behavior.

Task-page loading and featured-card subcollection documents were source-compared.
Task summary/board reads omit rich bodies while full List/detail reads retain them;
scoped JSON compression and workflow/reference-query reuse are implemented. The
featured picker supports nested collections, but selected-row breadcrumb props
are not passed and new cards follow collection order rather than append-only
ordering. Both records now distinguish present code from historical test claims.

The system-message event-type plan was source-compared. Typed persistence and
transport are implemented, but the admin keyword fallback remains. Widget visibility
now combines internal flags with an explicit event allowlist that includes delayed
team replies, and the shared event tuple omits several newer backend constants.
The plan now uses actual migration links and separates invariant test coverage
from an exhaustive emitter audit or proof of deployment.

The coverage-gap evidence grouping plan was source-compared. Sender-role
projection and conversation grouping are implemented, but cards initially show
the latest three fetched items and distinguish message bubbles from event rows.
The 50-row detail limit bounds grouping, and current displayed conversation/recent
counts cannot be described universally as per-message counts. These differences
and the limits of source-only test inspection are now explicit in the plan.

The sprint-planning redesign plan was source-compared with current components.
The current UI uses flattened sprint columns, virtualized task loading, closeout
progress, task-card terminology, and a small-screen backlog overlay. The historical
section layouts, story-card filename, mobile-hidden backlog, and filter ownership
are now explicitly distinguished from current behavior; browser parity is not claimed.

The early-access email sequence was reviewed against the current Customer.io
integration. Its `signed_up` exit event differs from implemented `user_signed_up`,
and no current application emitter was found for `early_access_signup`. The draft
now points to maintained lifecycle contracts and labels campaign state, sender ID,
launch copy, and broad import/access promises as unverified historical assumptions.
No external campaign configuration or message sending was performed.

The June coverage-gap strategy assessment now distinguishes implemented impact
scoring, recurrence reopening, embedding-based cluster rebuilding, and snapshot
persistence from its earlier missing-feature claims. Reopening after three post-close
evidence updates is not proof of citation-based deflection verification. Proposed
coverage automation triggers and a production digest sender were not established
by current call sites; competitive positioning remains historical.

The June product/engineering assessment now records source-backed changes to
billing/usage, Support/search, CRM boards, CI, rate limits, package publishing,
mobile, and onboarding. Its sweeping security, production-readiness, competitive,
and schedule claims are explicitly historical; source presence is distinguished
from branch protection, passing CI, deployment acceptance, and store availability.

The August inbox performance design was source-compared while preserving its
September supersession notice. Bounded cursor history, page-scoped hydration,
scroll/retry handling, and lazy surfaces are implemented. Cursor validation checks
unsigned JSON shape rather than tamper integrity, and conversation-keyed caches
are not proof of network cancellation. No latency or bundle benchmark is claimed.

The repository design-system skill was reviewed as contributor guidance. Its
reference links and named shared input/composer/dropdown/icon entry points resolve
to current source, and its scope remains product frontend work. No stale instruction
requiring a change was found. Its completion checklist is a requirement for UI work,
not evidence that every existing surface has passed a visual/accessibility audit.

The May Ask Agents orchestration plan now identifies the current Dock chat API,
storage, frontend, and Agent Runtime execution path. Proposed command-bar chat
routes are absent from current registration, and read-only chat answers are not
guaranteed to avoid backing runs. Historical proposal confirmation requirements
are separated from current permission-scoped tools and runtime interactions.

The agent-authorization design is now explicitly labeled a proposal. Current
models lack its explicit agent scope and run principal fields, and services use
legacy team fallbacks and command RBAC rather than the proposed agentauthz API.
The document records unrestricted-empty-scope, overlap, and role-less gate behavior
without treating helper inspection as proof of external reachability or a complete
security audit. Proposed rollout guarantees are not represented as shipped policy.

The April Codex workspace-write node-pool proposal is now explicitly historical.
Current Helpin worker manifests define the automation queue without the proposed
Codex pool/sandbox fields, while execution connects to Agent Runtime. Old platform
minimums and danger-full-access rollback steps are not current deployment advice;
no live infrastructure or secret configuration was inspected or changed.

The companion sprint-dismissal implementation plan now records implemented
helper/modal behavior and links its reviewed design. The helper returns response
data rather than the sample's full API result, modal close also persists dismissal,
and the original missing-file RED steps and build failures are historical. Current
manifest inspection is distinguished from a fresh test or build run.

The Ask Agent timeline ordering design now separates retained ordering logic
from changed disclosure behavior. Active groups start collapsed, completed headers
may show duration, and active tool labels can incorporate argument-derived context.
The old identity-only header guarantee and full verification matrix are not presented
as proven current behavior; no unauthorized-access conclusion is inferred.

The early Dock transcript-ordering plan now records the implemented envelope-time
projection fix and settled-history logic. Message-level RuntimeSequenceNo and the
proposed mirror data.CreatedAt preference are not present; SDK and rollout guidance
are historical. Artifact sequence metadata and new-message fixes do not prove
historical transcript repair or completion of optional hardening phases.

The shared Support email projection plan now records the implemented converter,
persistence, hydration, transport, and rendering paths. “Lossless” is qualified by
sanitization, output caps, and heuristic quote detection. Legacy hydration is an
in-memory projection using available stored text, not a persisted backfill or a
way to recover missing original content. No provider/browser acceptance is claimed.

The Ask efficient-execution design now links the versioned launch policy and
separates prompt guidance from measured model behavior. Transcript compaction now
uses working groups and includes DockRunView, contradicting the old root-Ask-only
boundary. Historical model and no-runtime-change constraints are not represented
as permanent current guarantees.

The widget messenger security implementation plan now distinguishes its JWT
proposal from current HMAC v1 proofs and off/report-only/enforced modes. Proposed
token callbacks, issuer auditing, encrypted rotation fields, and UI milestones are
not represented as supported integration APIs or completed guarantees. No live
identity configuration or secrets were changed.

The agent image-annotation plan now distinguishes the existing human editor from
the absent annotate_image tool and Go renderer. Coordinate contracts, bounds, cover
rejection, parity fixtures, and grounding scores remain proposals; the conflicting
Docs versus PM permission wording needs resolution before implementation. Existing
artifact/import primitives and UI source deletion do not prove the new tool's safety.

The companion shared-email projection design now clarifies unique-prefix text
splitting, bounded display output, in-memory legacy hydration, and versioned-schema
requirements. The inbox iframe derives quote presentation from HTML, whereas the
widget uses explicit projection fields. Original preservation and validation scope
statements are no longer presented as unlimited or permanent guarantees.

The PM editor attachment plan now distinguishes implemented upload/reassignment
paths from its original checklist, maps Story names to Task, and documents
non-atomic entity reassignment versus transactional comment attachment updates.
The ID-based repository helper alone is not an ownership validation guarantee.
Inline image deduplication preserves standalone image attachments.

The command-runs rail persistence plan now distinguishes implemented dismissal
APIs from the superseded frontend. Corrections cover actual routes and permissions,
limit reset behavior, missing dismissed-plan filtering in legacy recent runs, and
the capped plan-list fetch window. Current Dock selection persistence differs
from the proposed Zustand slice; isolation acceptance is not claimed verified.

The rail UX plan now labels the April baseline as historical. Current dependency
plans, scheduling limits, and staged delivery rendering contradict its old no-DAG
claim. Current Dock components and compact interaction presentation replace the
original component tree and always-visible approval assumptions.

The GitHub PR reconciliation plan now describes the implemented dry-run CLI and
its global oldest-open-link query, without claiming periodic convergence. Applying
repairs can change task workflow state when configured, contrary to the original
metadata-only mitigation. The update count is not a successful-write count; missing
worker/UI features and current conditional signature validation are explicit.

The reply-delivery card plan is now explicitly unimplemented and distinguished
from the existing delayed handoff notice. Its original timing contradiction,
non-atomic prior-message deduplication assumption, and unconditional email promise
are called out for future implementation. Proposed tests are not reported as
existing coverage.

The Docs AI translation design now identifies the structured pipeline as
implemented and the plain-text baseline as historical. It documents current node
support and protected-term enforcement, while qualifying empty-text validation,
editor-schema parity, absent render/review-metadata stages, and failure guarantees.
Draft generation is distinguished from publication and test source from test runs.

The marketing preset proposal now points to the consolidated Mira implementation,
manual trigger and approval defaults, current tool/skill names, and implemented
CRM writes. Browser capability references are updated without claiming universal
availability; campaign sending and proposed sequence tools are not inferred from
provider integrations.

The Help Scout import review now distinguishes its March findings from current
encrypted database jobs and Temporal execution. Permission, endpoint, slug, draft,
redirect and rate-handling changes are documented without claiming universal
recovery, atomicity, ownership verification, or current vendor-limit validation.

The team-scoped PM PRD now reflects workspace-member identity, existing handles
and nullable team labels, actual routes and separate permission gates. Proposed
private/access modes remain unimplemented in the team model; literal-handle
mentions do not satisfy the original stable-ID mention requirement.

The operational-tool design now distinguishes implemented adapters from universal
safety claims. Some payload decoders ignore unknown fields; central actor checks
require configured authorization and a populated role. Tool schemas, scope parity,
and transactional behavior are not treated as proof of every execution boundary.

The CRM email workflow record now distinguishes historical validation/delivery
from current source. Shared mailbox defaults reserve manual capacity, replacing
the earlier independent 100-sequence-send description. Scheduler wiring, uncertain
delivery reconciliation, and starter generation are documented without claiming
live execution, applied migrations, or exactly-once provider delivery.

The operational-tool implementation plan now points to existing adapters and
the reviewed design, labels unchecked steps as historical, and corrects toolchain
guidance. PM delivery and Docs metadata also use permissive JSON decoding, so
published strict schemas are not presented as universal execution validation.

The internal handoff plan now reflects retained ai_escalated naming, conditional
public replies, reason-dependent event order, and current control transitions.
Reasons are persisted in handoff analytics; the old arguments-only assertion and
unconditional widget reply are no longer presented as current behavior.

The September coverage diagnosis now separates historical production observations
from updated failure handling and lexical fallback. Remaining ten-minute workspace
activity and run-scoped materialization limits are explicit; no live recovery or
reanalysis outcome is claimed.

On September 18, the companion Help Scout plan review was updated for implemented
job fields, handler integration, permissions, Temporal execution, image abstraction
and shared conversion. Obsolete migration numbering is explicitly retired; interval
polling remains distinguished from the original query-hook suggestion.

The Shortcut tracker now labels its checkboxes as historical and documents current
client bounds, preview polling, Temporal execution, partial encrypted snapshots,
author attribution, and media handling. The proposed feature flag is not assumed
to gate rollout, and preview goroutines are distinguished from durable imports.

The v3.1 analyzer quality design now reflects v4 and the 0.015 reciprocal-rank
fusion threshold, replacing the incorrect interpretation of 0.016 as inherent
noise. Current recommendation behavior and semantic grouping are distinguished
from the old proposal, and the legacy migration claim is limited to its SQL predicate.

The timeline-ordering implementation plan now shares the reviewed design’s
qualifications: collapsed active groups, contextual tool labels, optional duration
headers, and current pending-message reconciliation differ from the original
acceptance assertions. Original test and push steps remain historical.

Support-link security documentation now qualifies customer-only scanning, shared
deadline limits and scanner observability. The frontend expiry predicate does not
explicitly reject Date.parse NaN, so invalid-expiry handling is not documented as
universally fail-closed. Proxy configuration logging is distinguished from scan logs.

The support-link implementation plan now shares the reviewed design’s limits
and points to public-widget allowlist projection, which supersedes its strip-only
metadata instructions. Toolchain and historical test/commit steps are clarified.

The March backlog is now an explicit historical snapshot with current roadmap
navigation. Automation, Shortcut import, realtime updates and estimate scales are
identified as implemented capabilities with limits; old Open labels and priorities
are no longer presented as the current backlog or proof of feature absence.

The agent badge navigation plan now distinguishes latest run from latest active
run, records run-agent-based visibility and current routing, and corrects pause
status and permission assumptions. Historical interaction scenarios are not
reported as fresh browser verification.

The mobile drawer design now documents workload totals versus unread dots, the
different count sources, sessionStorage and filter overrides. Shared web conversion
is implemented, but identical live results and permanent semantic parity are not
inferred from aliases or successful compilation.

The sprint board ordering plan now separates implemented ordering and realtime
refresh behavior from historical steps. It records state-wide position
normalization, the retained Done position payload, and the early return that
bypasses the truncation refresh for same-column reorders. Browser convergence
and test execution are not claimed by this source review.

The agent startup progress plan now distinguishes parent-level plan visibility
from the standalone panel empty states. It documents the running-only streaming
status row, absence of the proposed stage-derived startup item, and removed
Temporal activity entry point without claiming external runtime behavior.

The Superpowers evaluation now marks absent skill additions and visibility flags
as proposals, records current skill aliases and existing validation guidance,
and corrects concrete change counts. Upstream popularity, licensing, estimates,
and overlapping adoption totals are not represented as verified current facts.

The support inbox refactoring plan now records implemented presence separation,
draft persistence, connection checks and memoization. It qualifies widget
validation and message side-effect atomicity, records the unvirtualized list,
and withdraws the unvalidated display-ID allocation replacement.

The widget CRM identity requirements now document transactional backfill and
post-commit events, same-email name refresh, retained contact links, and Postgres
identity evidence. They distinguish actual source-specific lifecycle promotions
from the unimplemented settings-driven behavior and proposed default change.

The awaiting-input board plan now describes response enrichment instead of a
persisted pause-reason column, current badge labels and toast navigation limits.
It separates implemented notification consumers and read-state cleanup from
unverified production of the exact attention event after the old emitter removal.

The delegated-run finalizer record now corrects its terminal-replay retry claim,
explains marker/side-effect crash windows and the lack of a newer-active-run
bookkeeping guard, and records expanded finalizers plus API-entry wiring.
Individual error isolation is not described as a guarantee that projection
always succeeds.

The agent billing-tier design now separates current route configuration from
its original table, documents the native_sdk custom resolver and generated
labels, and qualifies heuristic legacy backfill and edition-dependent startup
validation. Universal UI and snapshot acceptance criteria are not reported as
verified merely because the tier fields exist.

The Kanban hardening audit now separates implemented normalization, member
reassignment and collapsed drop targets from remaining pointer-only interaction
and limited workflow/team mutation guards. It documents owner replacement and
the absence of a refetch inside member reassignment without claiming browser
or pagination convergence from source inspection.

The reply-delivery-card PRD now explicitly identifies an unimplemented proposal,
distinguishes the existing delayed-handoff notice, and flags internal-event
suppression and scheduling contradictions. Delivery capability and atomic
deduplication remain design requirements; competitor outcomes and metrics are
not reported as evidence. The mockup uses an example email address.

The PM image-attachment design now qualifies the stable application content URL,
private upload behavior, and non-atomic entity-create reassignment. It separates
transactional comment changes and pending-only cleanup from broader ownership
and deletion guarantees, matching the companion implementation review.

The epic links/attachments design now documents implemented generic routes and
entity fields, additive legacy-ID repair, current Related-section UI, and
non-blocking post-create link/file persistence. Its embedded migration and
rollback sketch are explicitly historical rather than current recovery advice.

The coverage email-classification design now records implemented v4 persistence
and normalization while qualifying its conservative-filter claim. Subject
substring matches and first-customer-only question-mark checks do not guarantee
that later genuine support requests survive deterministic skipping.

The Docs AI translation implementation plan now reflects the structured draft
pipeline and distinguishes registered preserve-only nodes from unsupported nodes.
It qualifies empty-body and HTML-render validation claims, points to current
test layout and versioned schema, and labels old execution steps as historical.

The meeting-intelligence record now documents calendar scheduling, hidden
deployment-owned provider settings, versioned schema and action permissions.
It qualifies task-acceptance idempotency because task creation precedes action
persistence, and labels provider rollout/test claims as historical observations.

The Ask Agent efficiency plan now distinguishes exact-policy deduplication from
arbitrary stale-policy removal and current grouped transcript presentation from
segment deletion. It corrects the root-only scope and documents final-response
precedence and separate retained-live versus runtime-active inputs.

The Docs module roadmap now identifies superseded remaining/deferred items,
including agent-backed section generation, CRM embeds, view previews, task
metadata conversion and Excalidraw. It distinguishes implemented entry points
from unverified universal permission, notification and export parity.

The AI settings UX plan now reflects tabbed table-based pages, current route
permissions, scoped fallback choices and row confirmations. It qualifies custom
model availability, minimum login polling intervals and conditional community
pricing copy, while retaining visual/build checks as historical requirements.

The AI profile corrections record now distinguishes backend shared-fallback
capability from the narrower editor, accepted-route preservation from ongoing
authorization, and source migration/CI artifacts from deployed or fresh passing
results. It records the current stable SDK dependency without rewriting history.

The opaque planning-session draft was renamed to
`docs/plans/planning-session-temporal-sandbox.md` and marked superseded. Its old
execution entry points are absent; cleanup and isolation claims are qualified,
and readers are directed to the current Agent Runtime architecture.

The callout plan now records semantic variants/defaults and current keyboard
behavior. It identifies legacy widget/help-center color selectors that do not
match the semantic classes emitted by the renderer and qualifies the original
filtered compiler command as insufficient build verification.

The setup-task design now links implemented copy, expansion and navigation
behavior while clarifying that recorded sessions, configured AI IDs, indexed
sources and routing settings prove setup evidence rather than production
installation or successful customer outcomes.

The team-scoped picker design now records conditional sprint filtering, nullable
team data and multi-owner behavior. It flags the absent inline team-change
confirmation/combined clearing and qualifies legacy selection preservation,
without inferring backend relationship integrity from frontend options.

The avatar upload plan now distinguishes the current picker/crop and generated
avatar UI from the original overlay. It corrects the implied server-side 2 MB
limit and records MIME-header validation and non-atomic object replacement,
without presenting historical build or manual checks as fresh results.

## Final review consolidation

Three reviewers completed the final 122 documents against the current source.
The per-file coverage record contains their evidence. This includes the root
README and architecture, contributor instructions, current technical guides,
and remaining historical plans, requirements and research. Historical acceptance
criteria remain requirements unless source evidence supports implementation.

The final integration also corrected the public MCP guide: checked-in staging
and production manifests enable MCP, and public MCP configuration flags default
to true. The disabled-first diagram is a procedure for new rollouts, not a claim
about current deployment state.

## Verification boundaries

This completes the inventoried documentation source review, not product QA or
publication. External links, vendor claims, hosted docs, production state and
historical test reports are not generally reverified. Individual documents
identify behavior gaps and operational assumptions; documentation changes do
not fix those application limitations. The ledger's screened vendored references
are explicitly distinguished from source-reviewed Helpin documentation.

The unrelated, pre-existing untracked CRM sales audit is left unchanged.
