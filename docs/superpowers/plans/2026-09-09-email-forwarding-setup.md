# Four-step forwarding setup

Approved design: retain four steps, expand the current step, allow revisiting completed instructions, provider-specific setup and Gmail refresh guidance, explicit acknowledgements for external actions, and delivery evidence before verification. Preserve confirmation inbox links and verification collapse animation.

1. Extract a focused EmailForwardingSetup component and pure progress/test-state helpers. Save guide progress/provider/source draft per route in browser storage, with graceful storage failure; server verification remains authoritative.
2. Add regression tests for confirmation arrival, saved progress, external acknowledgement, test request failure, timeout, and inbox isolation. Implement neutral accordion-style steps, clear primary actions, provider selector, copy address, source validation, and retry.
3. Poll unverified routes while the forwarding setup page is open, refresh on returning to the page, and offer manual status refresh. Seed successful test response in query cache. Correct premature enabled/automatic test wording.
4. Run targeted settings tests, TypeScript, responsive browser checks and review diff. Commit only task files; leave unrelated work untouched.

Completed: all four tasks. The shared overview checklist was replaced with a short introduction to avoid duplicating per-inbox instructions. Manual progress is stored only in this browser; server delivery evidence remains authoritative.

Validation: 22 tests passed across five forwarding/routing files; frontend TypeScript build passed; diff whitespace check passed. Local browser preview of the actual setup component exercised all four steps and the waiting state at 1440×1000 and 390×844, with no horizontal overflow. Independent review findings (source draft restoration, preserving exit-animation contents, visible refresh errors) were fixed and covered by regression tests; follow-up review found no remaining blockers. No production emails were sent and no production provider configuration was changed.
