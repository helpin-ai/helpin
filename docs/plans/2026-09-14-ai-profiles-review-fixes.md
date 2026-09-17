# AI profiles and edition isolation: review corrections

Status: implemented and verified locally. This correction plan supersedes the earlier code-complete
assessment in `2026-09-14-ai-profiles-and-ee-billing-plan.md`. Live acceptance is
still pending; SaaS BYOK remains disabled until its release gates pass.

## Decisions and preserved behavior

- Same-run retries, approval resumes, and recovery keep the accepted route and
  tariff. A new continuation or trusted child inherits its parent's route and
  ownership, but must pass current edition policy and freeze a new tariff.
  CRM retains its reviewed route and its accepted-run retry snapshot.
- Unattended execution permits shared credentials; it does not waive active
  membership checks when a user is attributed to the launch.
- Shared profile deletion must not silently change an agent's funding route.
  Reject deletion while current agents reference it, with an actionable error.
  Historical versions and accepted runs retain their snapshots.
- Personal profiles use an owner's personal primary connection and may have an
  authorized shared fallback, as originally specified. Shared profiles remain
  shared-only. Existing broader profiles must be corrected before new launches.
- SDK-supported controls remain authoritative. UI choices may use friendly
  canonical names, with aliases handled consistently rather than competing lists.
- Helpin and Runtime commits stay local. Do not restart live services, apply live
  migrations, enable workspace flags, or change live tariffs. Use isolated fixtures.

## Delivery and acceptance

1. **Admission and authorization.** Preserve the resolved endpoint in flat BYOK
   metering; distinguish new-run policy admission from restoration; check attributed
   membership for automated and legacy launches; enforce personal primary scope.
   Test compatible admission, tariff rotation/disable, unchanged accepted runs,
   legacy overrides/conflicts, and disconnect before launch.
2. **Edition and control boundaries.** Tag every EE Go source; verify Community
   cannot import EE packages. Select EE explicitly for SaaS desktop release and
   type-check both editions. Remove the independent service-tier list. Move
   commercial copy and billing route registration behind build-selected sources;
   verify Community output, not only compilation.
3. **Persistence and operator correctness.** Guard profile deletion, add indexes
   through a new migration, add a real PostgreSQL migration-ledger regression,
   make bootstrap dotenv loading explicit, and add model-catalog export drift
   coverage. Do not modify historical migration checksums.
4. **Launch acceptance.** Exercise profiles through ordinary continuation,
   CRM launch and accepted retry, and Dock chat launch; cover disconnection and
   authorization failures without runtime submission. Review each major change,
   fix discovered issues, and commit verified steps separately.
5. **Final verification and handoff.** Run affected backend tests, vet, Community
   and EE builds/checks, desktop type checks, frontend launch regressions, and
   isolated PostgreSQL integration. Update this plan and the current AI connections guide with
   exact results and remaining live gates; leave working trees clean.

## Progress

- `2f829d4bc`: plan recorded before implementation.
- `b10159511`: compatible endpoint propagation, new-run tariff admission,
  attributed membership, personal primary scope, and targeted regressions.
- `bd77197a3`: all EE Go sources tagged, explicit SaaS desktop edition and matching
  TypeScript configuration, Community route/copy exclusion, SDK tier choices.
- `5d02b5e5a`: referenced-profile deletion guard, additive indexes and active-profile
  assignment trigger, explicit bootstrap dotenv path, model export drift test,
  and real PostgreSQL transaction/ledger regression plus CI job.
- SDK `v0.6.0-alpha.3` publishes canonical service-tier choices and alias handling.
  Helpin consumes the release without a module replacement. Runtime needs no
  behavior change; its existing accepted values remain compatible.
- `45862fdb3`: launch regression coverage now exercises the ordinary continuation entry point,
  CRM reviewed launch and accepted retry, Dock-to-runtime launch, and disconnection
  during registration or submission. UI coverage submits the chosen Dock profile.
- Real PostgreSQL checks passed: optional EE rows, core checksum enforcement,
  unique-key conflict rollback, retry, idempotent indexes and assignment guard.
  The disposable test container was removed. No development database was touched.
- SDK tests, backend EE service suites, backend vet, EE binary builds, desktop
  Community/EE type checks, the model export drift test, and race-enabled launch
  regressions passed. The Community backend source-removal check passed.
- Focused UI suite: 102 passed, one existing skip. Community artifact checks prove
  the identified commercial copy and billing routes are absent.
- Full Community frontend source-removal check passed: production build, artifact
  assertions, 437 test files, 2,640 passing tests and three existing skips.
- Final EE frontend production build and type check passed after the SDK-derived
  tier export update. Both desktop editions type-check. EE binary builds passed.
- Final Go module tidy/verification passed. Helpin commits remain local; only SDK
  tag `v0.6.0-alpha.3` was published. No deployment action performed.

## Deployment handoff

- Apply new core migrations `202609140008_ai_profile_reference_indexes.sql` and
  `202609140009_agent_ai_profile_liveness.sql` through the existing migrator. No
  historical SQL changed, and no live migration was applied during this work.
- Rebuild/restart Helpin API and worker with `-tags ee` for SaaS, and web/desktop
  with the EE edition. Community remains the default for community distributions.
- Runtime has no new source change or required restart for these corrections.
- Keep SaaS BYOK activation behind its existing live acceptance and explicit tariff
  gates. Expired ChatGPT refresh/reconnect/revocation canaries remain a deployment
  gate; local fixtures do not claim to replace them.
