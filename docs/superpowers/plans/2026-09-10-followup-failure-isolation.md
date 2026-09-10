# Follow-up failure isolation

1. Trace and test the real runtime tool catalog so scheduled assessments can call finish_support_follow_up.
2. Separate technical failure from an intentional assessment handoff: bounded retries, then failed episode with private explanation; preserve conversation ownership and escalation state and prevent automatic closure.
3. Make genuine handoffs visible with a normal escalation event; show readable failure explanations in support inbox.
4. Add regression tests for retries, unchanged customer-facing state and normal explicit handoff. Run targeted backend/frontend checks and review, then commit to waqar-fixes. No production mutation or push.

Completed: reproduced missing finish tool through saved-agent/run-tool intersection; added system-run-scoped completion permission and fail-fast required-tool checks. Technical failures now stop with a private note and no escalation; genuine handoffs persist an escalation event. Verified focused/broader runtime tests, command catalog tests, follow-up/delayed-reply regressions, real PostgreSQL transaction tests,14 status UI tests, TypeScript and API/worker builds. Review found no remaining blockers.
