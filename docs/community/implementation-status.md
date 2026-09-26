# Community 0.1 implementation and verification

Historical implementation evidence recorded September 16, 2026. At that time,
Community work was consolidated on feature branches in both repositories. Test
totals, scan findings and migration heads below are a dated record, not current
checkout or release verification. This document is historical and is excluded
from operator bundles. Publication still requires
[the publication checklist](../../community/PUBLICATION.md). Current CI and
release commands are in [the development guide](development.md).

## Implemented

- Installation origins enforced on visitor HTTP and both WebSocket paths;
  origin-first settings; new Community report-only identity defaults; saved and
  Enterprise defaults preserved. Unsigned identification cannot inherit verified history.
- Community support/docs/agents module defaults, navigation and API admission;
  staff-email feature gates removed from HEAD.
- Local signup without email verification, optional SMTP application mail,
  Enterprise verification email policy (not a server-enforced login restriction), operator URLs and no default telemetry endpoints.
- Support-only SDK mode, local HTTP/WS addressing, signed private attachments,
  public help-center/snippet addressing, safe anonymous-to-known identification
  and isolation when switching between known users.
- Visible knowledge/embedding configuration separate from agent AI profiles.
- Runtime's exact-host HTTP callback opt-in, authentication and redirect
  rejection; strict Helpin run credentials. Its separate Community image omits
  browser and coding toolchains.
- Five-command installer, same-origin prebuilt frontend, explicit public URL
  settings, external HTTPS proxy example, and private infrastructure network.
- PostgreSQL 17/pgvector 0.8.6, Redis 7.2.16, NATS 2.14.7, Temporal 1.32.0,
  and upstream Garage 2.3.0;
  independent application, Runtime, Temporal and visibility databases/users.
  Temporal schema and namespace jobs run before workers.
- Separate API, worker, migrator, frontend and help-center images; pinned Runtime
  source pairing; native architecture CI; protected release workflow with image
  scans, SBOMs, tested digests, checksums and an allowlisted operator bundle.
- Guarded empty-database foundation before immutable historical migrations.
  Existing populated installations skip the foundation; partial/foreign schemas
  fail explicitly. No existing migration or checksum was changed.
- Documentation Markdown is indexed immediately, before opening the editor.
- Nginx refreshes Docker DNS after API container replacement and excludes query
  credentials from access logs. The help-center proxy exposes only public APIs.
- Unmodified upstream infrastructure images. PostgreSQL, NATS and Garage are
  digest-pinned; custom infrastructure/storage-init rebuilds were removed.
  Exact, expiring upstream scan exceptions replace infrastructure patch forks.
- Shared Docs attachment admission, hidden task association controls when PM is
  unavailable, and a PostgreSQL CI schema-parity guard using the API model list.
- Required verification without a mail sender fails startup; unauthenticated
  local SMTP relays are explicit. Origin settings accept a pasted trailing slash,
  and only session-bearing or write widget responses default to no-store.
- Private Garage imports stay authenticated. Help-center publication copies
  images, including HTML image blocks, into public asset paths. API startup
  provisions browser upload CORS. Existing ACL/CDN configurations remain intact.

## Verification completed locally (Linux amd64)

| Check | Result |
| --- | --- |
| Helpin full backend suite and Community source-exclusion check | Passed |
| Runtime full Go suite after dependency updates | Passed |
| Enterprise configuration/mail/edition/API/worker checks | Passed |
| Community frontend source-exclusion build | Passed |
| Prior full frontend regression baseline | 458 files; 2,748 passed, 3 existing skips |
| Help-center regression suite | 30 files; 116 passed |
| SDK regression suite and production build | 12 files; 206 passed |
| Support mobile typecheck/tests | Passed; 468 tests |
| Community and EE desktop typechecks | Passed |
| Fresh PostgreSQL migrations, populated reapplication, partial-schema rejection, checksum/transaction regressions | Passed against real PostgreSQL |
| Installer idempotency, independent generated keys and dotenv non-execution | Passed |
| Operator archive allowlist, checksums and image digest pinning | Passed |
| Workflow validation with actionlint; shell syntax; diff whitespace | Passed |
| Final Garage stack image scans | All 12 passed; application images have no fixable HIGH/CRITICAL findings; 26 exact upstream findings have expiring exceptions; SBOMs generated |
| Fresh ledger versus API model list | Passed on real upstream PostgreSQL; dedicated CI guard added |
| Frontend typecheck and origin settings tests after RC review | Passed |
| Upstream infrastructure architecture manifests | amd64 and arm64 confirmed |

The complete `community/tests/run.sh` passed against the upstream Garage stack.
The image exception rationale is recorded in [upstream-images.md](upstream-images.md).
It covers owner/workspace creation, empty/denied origins, private attachment
upload/read isolation, support messages/replies, published help-center SSR,
shared-profile agent execution through the durable Runtime worker, authenticated
compatible model calls, embedding calls, SMTP invitation/password-reset delivery,
and password-reset completion.

The final browser check loads the actual packaged SDK on another origin,
identifies a visitor after the anonymous session connects, sends a message,
receives a reply, reconnects after reload, rejects both WebSocket entry points
from an unapproved origin, signs into the real staff frontend, sends a staff UI
reply, and switches visitors without exposing prior conversation history.

The restore check copied all four persistent volumes to an independent project,
verified data counts and the Helpin ledger, reran migrations and the support
journey, and launched an agent using a profile/connection created before the
backup. The strengthened Garage check also read the exact bytes of a pre-backup
private attachment and confirmed its unsigned URL was still denied. It preserved the original encryption keys. The final frontend build and typecheck passed. The strengthened browser check
also covers hidden task-link controls with PM disabled.

Recorded schema state: Helpin `202609160001`, Temporal `1.19`, visibility `1.14`.
Runtime has no migration ledger head; record its image version/digest instead.
`setup.sh status` reports these diagnostics without environment values.

## Remaining release gates (open on September 16, 2026, historical)

These are not completed or claimed by local tests:

1. Licenses are now present; the owner must finish the public Enterprise-source and
   attribution decisions in the publication checklist.
2. Owner resolves publication hygiene: private docs, personal addresses in
   historical `featureFlags.ts`, and redacted history-scan matches. Scans found
   60 Helpin and two Runtime matches needing classification; they are findings,
   not confirmed exposed credentials. Private reports are outside the repos.
   Existing SaaS workflows still reference the legacy widget packages, so they
   were excluded from Community images rather than deleted as supposedly unused.
3. Run the native arm64 install/restore job. Manifest availability and Go's
   cross-compilation support do not substitute for execution on arm64 hardware.
4. Run the outside-origin journey on an approved public DNS/trusted-TLS host.
   Local browser origins do not prove public certificate/DNS configuration.
5. The pinned Runtime revision has been pushed. Enable/verify private
   vulnerability reporting, configure the protected release environment, and
   publish reviewed artifacts only after the preceding gates pass.

The implementation is a locally tested candidate, not a published release.

## Historical: local validation on September 16, 2026

Verified locally on Linux amd64:

- CI selection/required-status/cache policy: 5 tests passed.
- Runner, archive, image identity and promotion contracts: 14 tests passed;
  publication calls are mocked. Installer/readiness: 4 tests passed.
- Community backend build/tests with the `ee/` directories absent, and full Enterprise backend tests: passed.
- PostgreSQL 16 migration regressions and PostgreSQL 17 schema/privacy checks: passed.
- Community frontend build/artifact check: passed; 2,796 tests passed, 3 existing skips.
- Enterprise frontend build: passed; 2,829 tests passed, 1 existing skip.
- Full isolated browser/AI/mail/restore acceptance: passed with the existing local
  Community images. Restored credentials and private attachment bytes/policy passed.
- Real SIGTERM cancellation with mocked Docker, failure cleanup and ownership
  checks passed; no acceptance containers/volumes remained after live validation.
- Compose-to-Bake definitions, workflow lint, shell checks and bundle links passed.

Acceptance-only timings with existing images were approximately 58 seconds for
smoke and 239 seconds for the final full run. These are local observations, not
an end-to-end CI benchmark: builds, registry/cache transfer and runner scheduling
are excluded. The PR path now omits full browser/mail/restore phases, host workspace
installation for those phases, and duplicate Community frontend/API builds.

Native arm64, fresh candidate image builds/scans, GitHub cache behavior and real
registry/release promotion still run through their CI/release gates. They are not
claimed by these local checks. No release, tag, repository visibility or branch
protection setting was changed during this cleanup.
