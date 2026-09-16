# Community 0.1 implementation and verification

Updated September 16, 2026. Work is isolated on local `feat/community-v01`
worktrees for Helpin and Runtime. Nothing has been pushed or published. Existing
developer and SaaS services, databases and environment values were not changed.
The [reference plan](../plans/2026-09-15-community-release-plan.md) remains the
scope; its v0.2 work is deferred.

## Implemented

- Installation origins enforced on visitor HTTP and both WebSocket paths;
  origin-first settings; new Community report-only identity defaults; saved and
  EE defaults preserved. Unsigned identification cannot inherit verified history.
- Community support/docs/agents module defaults, navigation and API admission;
  staff-email feature gates removed from HEAD.
- Local signup without email verification, optional SMTP application mail,
  EE verification email policy (not a server-enforced login restriction), operator URLs and no default telemetry endpoints.
- Support-only SDK mode, local HTTP/WS addressing, signed private attachments,
  public help-center/snippet addressing, safe anonymous-to-known identification
  and isolation when switching between known users.
- Visible knowledge/embedding configuration separate from agent AI profiles.
- Runtime's exact-host HTTP callback opt-in, authentication and redirect
  rejection; strict Helpin run credentials. Its separate Community image omits
  browser and coding toolchains.
- Five-command installer, same-origin prebuilt frontend, explicit public URL
  settings, external HTTPS proxy example, and private infrastructure network.
- PostgreSQL 17/pgvector 0.8.6, Redis 7.2.16, NATS 2.14.7 and Temporal 1.32.0;
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
- Dependency security patches, current infrastructure images and small security
  rebuilds with upstream notices/corresponding source retained.

## Verification completed locally (Linux amd64)

| Check | Result |
| --- | --- |
| Helpin full backend suite and Community source-exclusion check | Passed |
| Runtime full Go suite after dependency updates | Passed |
| EE configuration/mail/edition/API/worker checks | Passed |
| Community frontend source-exclusion build | Passed |
| Frontend regression suite after dependency updates | 458 files; 2,748 passed, 3 existing skips |
| Help-center regression suite | 30 files; 116 passed |
| SDK regression suite and production build | 12 files; 206 passed |
| Support mobile typecheck/tests | Passed; 468 tests |
| Community and EE desktop typechecks | Passed |
| Fresh PostgreSQL migrations, populated reapplication, partial-schema rejection, checksum/transaction regressions | Passed against real PostgreSQL |
| Installer idempotency, independent generated keys and dotenv non-execution | Passed |
| Operator archive allowlist, checksums and image digest pinning | Passed |
| Workflow validation with actionlint; shell syntax; diff whitespace | Passed |
| Final service image vulnerability and secret scans | All 13 passed; no fixable HIGH/CRITICAL findings; SBOMs generated |
| Upstream infrastructure architecture manifests | amd64 and arm64 confirmed |

The complete `community/tests/run.sh` passed against the final infrastructure.
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
backup. It preserved the original encryption keys. The final frontend rebuild
and its strengthened browser test also passed.

Recorded schema state: Helpin `202609150002`, Temporal `1.19`, visibility `1.14`.
Runtime has no migration ledger head; record its image version/digest instead.
`setup.sh status` reports these diagnostics without environment values.

## Remaining release gates

These are not completed or claimed by local tests:

1. Owner chooses Helpin/Runtime/SDK licenses and the public EE-source boundary.
   No license grant, private-document deletion or history rewrite was invented.
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
5. Make the pinned Runtime commit available remotely, enable/verify private
   vulnerability reporting, configure the protected release environment, and
   publish reviewed artifacts only after the preceding gates pass.

The implementation is a locally tested candidate, not a published release.
