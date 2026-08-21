# Agent Run Timeline Parity Design

## Goal

Make Ask Agent Chats, Agent Runs, and the full coding-session surface agree on event identity, causal ordering, interaction state, and run status while preserving the product-specific controls around each surface.

## Design

Interaction lifecycle events have a canonical semantic identity: interaction ID plus lifecycle status. The server publishes realtime events with the same deterministic ID used by the REST projection and timestamps resolutions at `resolved_at`. The frontend also reconciles by semantic identity so mixed server versions, reconnects, or a realtime-to-REST handoff cannot render the same decision twice.

Agent Runs will use the same authoritative runtime-timeline presentation contract as Ask Agent Chats: compact assistant progress, current plan, actor attribution, and retained runtime chronology. It will derive controls and live state from the freshest streamed session, falling back to the list summary only before stream data is available. Chat-only suggestions and run-only repository/actions remain intentionally separate.

The full-session surface retains its richer layout, but shares the canonical event reconciler so it has identical ordering and interaction semantics.

## Interaction UX

After a successful approval or review action, the pending interaction disappears immediately and one resolved decision is shown while the agent resumes. The realtime event is replaced by its persisted representation rather than appended. The related approval tool must no longer appear as an unresolved duplicate once the resolution lifecycle event is present.

## Safety and tests

Regression coverage will prove that realtime and persisted copies collapse into one event, resolution chronology uses `resolved_at`, unrelated events are retained, Agent Runs use compact progress and plans, and streamed session status controls the composer and live state. Existing chat and full-session tests must continue to pass.
