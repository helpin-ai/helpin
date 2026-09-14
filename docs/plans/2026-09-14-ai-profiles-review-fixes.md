# AI profiles and edition isolation: review corrections

Status: implementing. This correction plan supersedes the earlier code-complete
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
   isolated PostgreSQL integration. Update this plan and the main checkpoint with
   exact results and remaining live gates; leave working trees clean.

## Progress

- Plan recorded before implementation. No deployment action performed.
