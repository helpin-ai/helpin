# Dock work-plan placement

Approved scope: anchor new work plans in Ask Agent and docked agent runs; collapse plans when a later user turn exists; retain full-run side panel. Never invent positions for legacy plans.

1. Preserve plan origin (event, runtime sequence, turn, timestamp) and version history in durable stream snapshots; retain unchanged plans across follow-ups. Expose saved chat plan history across successor runs.
2. Mirror the same behavior in frontend event reduction. Render anchored plans in transcript order, without duplicate current-plan cards; put legacy/unplaceable current plans in a compact panel outside the message scroll.
3. Verify origin stability, changed-plan versions, reloads, repeated updates, historical loading, both dock surfaces and full-run compatibility. Commit on waqar-fixes.

Completed: durable origin/version snapshots, cross-run chat history, chronological dock placement, follow-up collapse, paginated-history filtering, and compact unanchored fallback. Full run side panel retained.

Validation: 126 frontend tests; targeted model/repository, runtime projection, coding-session, and dock-chat Go tests; targeted lint; production frontend build. Build reports the existing large-chunk warning.
