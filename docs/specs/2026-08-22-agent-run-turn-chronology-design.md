# Agent Run Turn Chronology Design

## Goal

Make standalone agent-run threads preserve the same human/agent chronology and completed-work disclosure behavior as Ask Agent chats.

## Chronology

Persisted REST projections already assign one top-level `sequence_no` after sorting messages, interactions, artifacts, and run state by timestamp. That sequence is the canonical cross-type timeline. The frontend will therefore use the top-level sequence for events whose source is a persisted projection (`agent_run_message`, `agent_run_interaction`, `agent_run_artifact`, or `agent_run`). Legacy/runtime events that do not carry a persisted source may continue to use a message payload sequence when available.

This preserves the legacy live-status ordering case while preventing an approval resolution from being sorted after the assistant messages that followed it. A regression test will reproduce HEL-102's exact shape: interaction resolution at projected sequence 23, approval resume message at projected sequence 24 with message sequence 13, and assistant messages at projected sequences 27+ with message sequences 14+.

## Completed work disclosure

Standalone runs already load their complete event transcript, so they do not need the Dock chat's lazy work-detail endpoint. The shared transcript renderer will receive a completed-run interval and derive one collapsed `Worked for …` group from the progress prose and tool activity before each final assistant response. The final response remains visible outside the disclosure.

Chat-backed history remains unchanged: its server-generated `dock_work_summary` and lazy detail endpoint continue to render through `DockWorkDisclosure`. Active runs remain expanded and keep their current live working presentation.

Duration is calculated from the conversation boundary timestamp to the final assistant response timestamp, capped at zero for invalid or reversed timestamps. This avoids the existing passive completion timer's use of the current clock after a run has finished.

## Safety and tests

- Preserve chronological source segments; grouping changes presentation only.
- Do not alter live-run reconciliation or Dock chat pagination.
- Cover canonical persisted ordering, legacy payload ordering, standalone completed grouping, direct-answer behavior, multiple turns, and the final-response boundary.
- Run focused Vitest suites, frontend `tsc -b`, and the production build.
