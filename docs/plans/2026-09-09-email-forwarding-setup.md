# Four-step forwarding setup

> Historical implementation record (2026-09-09), source-compared on 2026-09-17.
> The four-step component and progress helpers exist. Test counts, browser sizes,
> and review outcomes below describe that implementation session and were not
> reproduced during this documentation audit.

Current sources are [EmailForwardingSetup](../../frontend/src/components/settings/EmailForwardingSetup.tsx),
[progress helpers](../../frontend/src/components/settings/emailForwardingProgress.ts),
and [support query hooks](../../frontend/src/hooks/queries/useSupport.ts).
Progress is stored under a workspace-and-route key, with storage errors tolerated.
The delayed-test message appears after two minutes. While setup is open, unverified
routes poll every three seconds; outside setup, recent pending tests poll for up
to ten minutes. Manual refresh exposes errors, and successful test responses update
the route cache. UI progress never substitutes for `forwarding_verified_at`.


Approved design: retain four steps, expand the current step, allow revisiting completed instructions, provider-specific setup and Gmail refresh guidance, explicit acknowledgements for external actions, and delivery evidence before verification. Preserve confirmation inbox links and verification collapse animation.

1. Extract a focused EmailForwardingSetup component and pure progress/test-state helpers. Save guide progress/provider/source draft per route in browser storage, with graceful storage failure; server verification remains authoritative.
2. Add regression tests for confirmation arrival, saved progress, external acknowledgement, test request failure, timeout, and inbox isolation. Implement neutral accordion-style steps, clear primary actions, provider selector, copy address, source validation, and retry.
3. Poll unverified routes while the forwarding setup page is open, refresh on returning to the page, and offer manual status refresh. Seed successful test response in query cache. Correct premature enabled/automatic test wording.
4. Run targeted settings tests, TypeScript, responsive browser checks and review diff. Commit only task files; leave unrelated work untouched.

Completed: all four tasks. The shared overview checklist was replaced with a short introduction to avoid duplicating per-inbox instructions. Manual progress is stored only in this browser; server delivery evidence remains authoritative.

Validation: 22 tests passed across five forwarding/routing files; frontend TypeScript build passed; diff whitespace check passed. Local browser preview of the actual setup component exercised all four steps and the waiting state at 1440×1000 and 390×844, with no horizontal overflow. Independent review findings (source draft restoration, preserving exit-animation contents, visible refresh errors) were fixed and covered by regression tests; follow-up review found no remaining blockers. No production emails were sent and no production provider configuration was changed.
