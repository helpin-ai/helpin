# API routing recheck

Approved scope: provide an authenticated recheck action that bypasses the Temporal worker, reuses transcript classification, routes existing internal follow-ups to their appropriate personal queue, and reports why other items stay in CRM.

1. Add a workspace-scoped, visibility-checked repository scan of pending unclassified meeting follow-ups (three per request, cursor for continuation). Revalidate access, eligibility, and exact revision before writing routing metadata.
2. Add a bounded API-side classifier operation and PM endpoint protected by PM edit and CRM read/edit. Use the existing metered classifier; return explicit per-item outcomes and billing failures.
3. Add a quiet Recheck routing action to My Work AI suggestions, with pending state, concise results, continuation, query refresh, and standard billing dialog.
4. Verify repository authorization/concurrency guards, service outcomes, handler permission checks, and browser UX. Review and push only this change to waqar-fixes.

This is an alternate execution path for the existing classifier, not a diagnosis or repair of the Temporal worker. Deployment is required before the live action is available.
