# Long-running agent continuity: plan and harness comparison

Status: implementation authorized; first safety/foundation slice implemented, full continuity integration and production enablement pending.
Date: 2026-09-07.
Scope: Helpin Ask Agent/Dock conversations and long-running native SDK work; define reusable boundaries for other Helpin agent surfaces.
Reviewed Helpin baseline: `56fa5caf5`; Agent Runtime baseline: `121f189`.

## Decision

Keep the current portable compactor. Add durable conversation continuity and bounded evidence retrieval around it. Do not equate a smaller prompt with reliable memory, or replace the implementation with another harness wholesale.

Three distinct stores are needed:

| Layer | Purpose | Owner |
| --- | --- | --- |
| Active execution checkpoint | Exact provider-visible history, compaction generation, resume/recovery state | Agent Runtime, scoped by app and run |
| Conversation handoff | Bounded task state across backing runs, with source coverage and provenance | Helpin, scoped by workspace and conversation |
| Evidence archive | Recoverable messages, safe tool outcomes, and artifact references | Original owning service; exposed through authorized, bounded recall |

These stores do not replace the authoritative approval, cancellation, tool-execution, product-state, or billing records. Summaries cannot grant permission or establish that an action succeeded.

## Starting plan, before harness comparison

1. Preserve objectives, constraints, decisions, pending work, and evidence references across Dock backing runs.
2. Let the agent retrieve details that leave its active prompt.
3. Evaluate real-model behavior across repeated compactions and successor runs.
4. Enable only through per-model/preset rollout gates.

The research below refines this plan: retain a separate bounded slice of real user messages, harvest exact identifiers deterministically, make recovery pointers usable before aggressive trimming, and keep general-purpose learned memory out of the first release.

## Current behavior and gaps

- `agent-runtime/internal/runtime/native_context.go` already provides configurable thresholds, recent complete tool groups, preservation of the latest real user request and loaded skills, bounded tool-free summarization, failure handling, and context events. It remains opt-in.
- `native_checkpoint.go` restores the compacted history for the same app/run and stores usage and checkpoint generation. This is execution continuity, not a conversation-level memory contract.
- `helpin/server/internal/service/dock_chat.go:buildCarryForward` carries at most 20 recent messages, 500 characters per message, and 6,000 characters total to a successor. Earlier requirements and decisions can be absent even when native compaction itself works correctly.
- `native_context_test.go:TestNativeContextLongRunBoundedAndDurable` checks 120 tool rounds using a canned summarizer. It verifies mechanics, not real-model retention or task quality.
- Raw cumulative input/output tokens measure repeated work as well as new work. A bounded prompt still permits a very expensive long loop. Report replay, cached/uncached input, summary usage, retrieval usage, and task outcome separately.
- Separate existing blocker: the paused-chat `AIUsageRepository.Checkpoint` duplicate path still replaces a whole output summary. Fix and regression-test this before deployment; conversation continuity must not be layered onto regressing billing watermarks. The explicitly deferred P2 rounding issue is not part of this plan.

## Harness findings

Source inspection is pinned to snapshots, not a claim about every installed release. No harness benchmarks or paid model evaluations were run for this research.

### Codex

Inspected `/tmp/harness-context-study.QklvdS/codex` at `7769bccbb2b4e9469a36b12510e73594fa03c5d5`. The older `/tmp/codex` directory is a source copy without Git metadata; use the pinned checkout for reproducibility.

- The local compaction path preserves a bounded selection of actual user messages, newest-first under a 20,000-token budget, independently of the generated summary. Initial-context reinjection differs between mid-turn and pre-turn compaction. This is stronger than trusting a summary alone to retain all requirements. [Source: compact.rs](https://github.com/openai/codex/blob/7769bccbb2b4e9469a36b12510e73594fa03c5d5/codex-rs/core/src/compact.rs).
- Resume/fork reconstruction treats a surviving replacement-history checkpoint as a complete base and rebuilds the relevant suffix and context metadata. The active prompt is not reconstructed by blindly replaying all retired history. [Source: rollout reconstruction](https://github.com/openai/codex/blob/7769bccbb2b4e9469a36b12510e73594fa03c5d5/codex-rs/core/src/session/rollout_reconstruction.rs).
- Cross-session memories are a separate, feature-gated pipeline: per-rollout extraction followed by serialized consolidation, with leases, bounded selection, retry backoff, provenance artifacts, and secret redaction. This is not the compaction summary doing every memory job. [Source: memory pipeline](https://github.com/openai/codex/blob/7769bccbb2b4e9469a36b12510e73594fa03c5d5/codex-rs/memories/README.md).
- Provider-side compaction is a distinct option, not evidence that portable textual summaries have identical behavior. Do not couple Helpin conversation continuity to opaque provider state. [Official OpenAI conversation-state documentation](https://developers.openai.com/api/docs/guides/conversation-state).

Adopt: bounded real-user anchors, explicit resume bases, separate memory lifetimes, and revisioned background work. Defer Codex-style global consolidation; Helpin is multi-tenant and its first requirement is task continuity, not autonomous learning across unrelated conversations.

### Hermes

Downloaded the public repository to `/tmp/helpin-hermes-continuity-study`; inspected `03f3b09222b8f03becb203a6ebb9bac1f927b8b6`.

- Hermes documents separate in-loop compression and pre-agent session hygiene. Its stable-session mode archives retired messages instead of treating the compressed active list as the only record. [Context compression documentation](https://hermes-agent.nousresearch.com/docs/developer-guide/context-compression-and-caching/).
- The inspected lean compressor combines a smaller recent tail with bounded verbatim user excerpts, a deterministic identifier index, and a recovery pointer. Large tool results can become stubs referring back to stored history. Budgets mean these excerpts/indexes are not exhaustive. [Source: context_compressor.py](https://github.com/NousResearch/hermes-agent/blob/03f3b09222b8f03becb203a6ebb9bac1f927b8b6/agent/context_compressor.py).
- `session_search` separates discovery from reading around a message and returns stored evidence rather than requiring another LLM to paraphrase it. [Source: session_search_tool.py](https://github.com/NousResearch/hermes-agent/blob/03f3b09222b8f03becb203a6ebb9bac1f927b8b6/tools/session_search_tool.py).
- Token estimation can use persisted provider usage plus the appended-message delta, guarded by a transcript fingerprint. Resetting or changing the transcript invalidates the anchor. [Source: usage_anchor.py](https://github.com/NousResearch/hermes-agent/blob/03f3b09222b8f03becb203a6ebb9bac1f927b8b6/agent/usage_anchor.py).
- Persistent curated memory is bounded and distinct from historical search; session-start snapshots keep prompt prefixes stable. Its per-profile/personal-agent assumptions are not sufficient tenant isolation for Helpin. [Memory documentation](https://hermes-agent.nousresearch.com/docs/user-guide/features/memory/).
- Its compaction evaluation asks recall questions about the retired region and compares policies with an uncompacted control. Borrow the method, not published performance claims; Helpin must also evaluate actions and permissions. [Source: evaluation README](https://github.com/NousResearch/hermes-agent/blob/03f3b09222b8f03becb203a6ebb9bac1f927b8b6/evals/compaction/README.md).

Adopt: recovery-backed summaries, exact identifiers, real-user excerpts, and recall evaluations. Do not copy numeric thresholds, personal memory scope, raw-history exposure, or its full plugin architecture without Helpin-specific evidence.

## Proposed implementation

### Phase 0 — correctness baseline and evaluation fixtures

Owner: both repositories.

- Close the paused-checkpoint duplicate-write blocker. Inventory every writer of billing/checkpoint-bearing summaries, not only terminal projection. Tests must cover paused → completed/cancelled → stale paused duplicate and concurrent finalizer/replay writes.
- Keep native compaction off by default. Freeze the current behavior as an evaluation baseline.
- Build deterministic fixtures with known source IDs, approvals, actions, user corrections, and expected outcomes. Do not use private production conversations in external harnesses.
- Add real-model evaluation entry points, but do not execute paid evaluations until their provider/data-use permissions and spending ceiling are approved.

Exit: existing safety regressions pass; fixtures reproduce the known long-run continuity loss; test accounting separates execution, compaction, and retrieval costs.

### Phase 1 — conversation handoff, outside output_summary

Owner: Helpin (`internal/model`, `internal/repository`, `internal/service/dock_chat.go` and successor-run launch paths).

Add a dedicated revisioned conversation-state record. Start with Dock/Ask Agent, leaving the structure reusable for other explicitly identified conversation surfaces. Avoid a new generic global memory service.

Proposed fields:

- workspace/conversation identity, format version, revision, previous revision;
- source coverage cursor, source run IDs, generation/model/prompt version;
- current objective and pending work, decisions with reasons, constraints and user corrections;
- completed-action references to authoritative records, unresolved interactions and child-run references;
- evidence references and a bounded chronological progress digest;
- status/freshness and generation failure information.

Separate deterministic fields from model-authored fields. IDs, source cursors, actual action outcomes, user-message provenance, and interaction references come from records; the model may propose narrative task state. Each material claim needs a source reference or an explicit assumption label.

Maintain explicit message provenance (`human`, `tool`, `system_notification`, `summary`) instead of relying on role alone. A child-result notification is not a new user instruction. Corrections supersede earlier requirements; an explicit fresh-chat/reset does not silently inherit the previous task.

Generate/update a bounded handoff after meaningful completed work or a paused conversational turn, and when a backing run ends. Use leases and compare-and-swap against the source cursor/revision; stale generation must not overwrite a newer user correction or child result. Do not add an unconditional LLM call after every tool or short message.

Successor initialization uses the latest valid handoff plus the unsummarized tail after its coverage cursor and the new user request. The task state is historical context, never an instruction to resume obsolete work. Rebuild current permissions and tool availability independently.

If handoff generation fails, keep the prior valid revision and covered tail. Never advance coverage on failure. If this cannot fit, use a bounded reconstruction/retrieval path or return an explicit recoverable pause; do not silently chop requirements or make an oversized model call. Do not prolong a terminal run solely to generate memory.

Deletion/retention and permission revocation apply to derived handoffs too. Suppress or regenerate affected content, not just its source links. Do not claim that rechecking a link makes an already-injected restricted excerpt safe.

Exit: ending, expiring, restarting, or rotating a backing run preserves task continuity without resurrecting old permissions or losing the latest user turn. Short-chat behavior remains unchanged under the disabled flag.

### Phase 2 — bounded evidence recall

Owner: Helpin for conversation authorization and indexing; Agent Runtime for its private run evidence and app-scoped access.

First audit existing history tools and storage coverage. Do not assume the compact Dock UI transcript contains complete tool evidence or that the runtime journal is already searchable through an authorized API. Index redacted, model-visible evidence with stable IDs; do not expose credentials, hidden reasoning, or raw private audit payloads.

Initial retrieval implementation: exact IDs plus PostgreSQL full-text search and bounded message windows. No vector database is required for v1; measure missed queries before adding semantic retrieval.

Proposed model-facing read-only contracts, subject to catalog audit:

- `search_conversation_history`: `query`, optional opaque cursor, bounded `limit`; default scope is the trusted current conversation and explicitly associated runs.
- `read_conversation_history`: stable `message_id`, bounded before/after window; use returned evidence IDs rather than arbitrary tenant/run paths.
- Responses include source IDs, role/provenance, run identity, timestamps, bounded excerpts, truncation flags, and pagination/recovery pointers. Reading archived evidence does not rerun the original tool.

Use strict snake_case JSON schemas with no extra properties. Workspace/app/actor come from authenticated runtime context. Recheck conversation access and referenced product access at query time. Index/filter at the source boundary; no cross-workspace or unrelated-chat recall by default. Handle deleted/revoked sources explicitly. Child workers receive only the parent's authorized task evidence, not an automatic full-chat grant.

Per the internal-tool-contracts framework: update runtime registration and app/run allowlists, host callback/internal-command contracts, Helpin catalogs and Ask Agent managed-preset exposure, and old pinned-preset tests together. Host authorization must land before enabling the tools. Do not introduce a model-facing memory-write tool in v1.

Exit: the agent can retrieve exact earlier requirements and tool facts that are absent from active context; every retrieval is bounded, scoped, auditable, and tested against revoked access.

### Phase 3 — strengthen native compaction

Owner: Agent Runtime (`native_context.go`, `native_checkpoint.go`, provider adapters and tests).

- Retain a separately budgeted set of real-user excerpts, prioritizing recent corrections and unresolved requirements. Pin the latest request; preserve older originals in the archive. Do not promise all user messages remain verbatim in the prompt.
- Add deterministic entity/document/artifact/run/tool-call identifiers and evidence pointers from typed tool envelopes. Regex extraction is supplemental, not the source of truth for product IDs.
- Keep a bounded working-state summary and recent complete tool groups. Reinject current host instructions and required skills through their authoritative paths; never promote retrieved instructions into system authority.
- Add tool-result stubs only after recoverable evidence storage is verified. No truncation of unresolved approval or in-flight tool groups.
- Carry measurement anchors only when transcript fingerprint, provider/model, tool schemas, and system-context identity still match. Invalidate on model changes, edits, compaction and relevant prompt changes. Retain conservative hard-limit checks.
- Check context before the first request on resume/successor and after large attached inputs, not only during the tool loop. Use one authoritative compaction policy; do not add competing host/runtime summarizers that fire on every turn.
- Persist compaction generation, replacement history and archive references atomically. Include crash/timeout tests around archive commit and active-checkpoint replacement.

Keep portable summaries as the default engine. Provider-native compaction can be a later adapter-specific optimization; never transfer opaque state across incompatible providers or use it as Helpin's only conversation handoff.

Exit: repeated compaction and process restart preserve invariants and recoverability under the same evaluated context budget.

### Phase 4 — progress and spending controls

Owner: runtime telemetry plus Helpin orchestration/product policy.

- Track repeated identical reads, retrieval loops, repeated summaries without progress, and tool rounds between durable outcomes. Repeated reads after files change or compaction may be legitimate; do not turn a heuristic into an automatic failure on its own.
- Add bounded recovery/replan steps before a user-visible pause for sustained no-progress behavior.
- Enforce per-turn/run budgets and a separately configured parent/child aggregate budget. A long-lived chat must not permanently exhaust its ability to accept a new user turn because one old task consumed the lifetime budget.
- Count handoff generation and history-retrieval token impact; no free hidden maintenance loops.
- Report compaction generation, trigger reason, before/after tokens, retained-source coverage, retrieval count, repeated-read ratio, latency, task outcome, and cached/uncached costs. Keep sensitive transcript text out of telemetry.

### Phase 5 — real-model gates and staged rollout

Proposed initial evaluation matrix: at least 20 curated scenarios, repeated runs, and the intended Ask Agent model plus one alternate supported model. Record exact model/provider/configuration and evaluator versions; size the actual experiment to an approved budget.

Scenarios must include:

- 100+ tool rounds and at least 5 compactions;
- dozens of user turns with requirements introduced early, corrected later, and referenced again;
- exact IDs, multilingual requirements, large tool output, images/attachments and deleted artifacts;
- mid-task pause/resume, process restart, idle expiry, successor run and permission-driven run rotation;
- delayed/out-of-order child results and the distinction between notifications and human instructions;
- pending/rejected approvals, cancellation, ambiguous mutation outcomes and replay;
- summary failures, provider overflow, model switches, stale handoff generations and retrieval outages;
- competing writers and adversarial instructions in retrieved content;
- no-progress loops and aggregate child spending.

Compare current compaction, proposed compaction without retrieval, proposed compaction with retrieval, and an uncompacted control where it fits. Recall questions must target the retired region, not facts still present in the tail. Grade task completion, latest-request alignment and actions as well as factual recall; do not rely on an LLM judge for permission/safety assertions.

Proposed acceptance gates (targets, not measured results):

- zero unauthorized actions, cross-tenant disclosures, duplicated mutations, or billing/checkpoint regressions in the safety suite;
- all deterministic critical IDs and pending interaction references preserved or exactly recoverable;
- at least 95% scored requirement/decision recall with retrieval; report misses individually;
- task success no more than 5 percentage points below a fitting uncompacted control, and no regression from the current implementation on short tasks;
- at least 30% lower median replay-input tokens on the long-history fixtures versus current behavior, without a task-success tradeoff; also report actual total cost including summaries and retrieval;
- all outbound requests fit the validated model limits; no repeated compaction-failure loop; bounded recovery latency;
- repeated-compaction and successor-run results meet the same gates, not just a one-time compression benchmark.

Roll out in order: offline fixtures → internal Ask Agent canary → selected workspaces/presets → broader enablement. Keep separate flags for handoff use, history recall, and new compaction retention. Disabling a flag must not restore retired history into the prompt. Support read compatibility for existing checkpoints throughout deployment.

## Deferred deliberately

- Autonomous general-purpose user/workspace memory across unrelated chats.
- Skill self-modification or global memory consolidation agents.
- A vector store before full-text retrieval is evaluated.
- Wholesale adoption of Hermes/Codex implementations or their numerical defaults.
- Enabling aggressive truncation before archived evidence is reachable.
- Any claim of indefinite, lossless semantic memory or guaranteed cost reduction.

## Implementation order and review boundaries

1. Independent correctness PR for the existing paused-checkpoint blocker.
2. Add fixtures/telemetry and the revisioned Helpin handoff schema behind flags.
3. Add authorized archive recall and cross-repository contract tests.
4. Wire successor runs, then enhance retention/anchors in native compaction.
5. Run approved evaluations, tune per-model budgets, and canary.

Each implementation PR needs its own regression review. Production enablement still requires the evaluation and authorization gates above.

## Implementation progress — first slice

Implemented:

- Paused billing checkpoint retries validate the durable watermark and never restore an old whole summary. Projection aborts on a stale watermark. Two remaining support-command whole-summary writers now use atomic marker updates.
- Dedicated `dock_chat_handoffs` storage, separate from run summaries: bounded claim payloads, source IDs, revisions and coverage, expiring leases, CAS publication, fixed failure codes, and explicit invalidation that erases derived text and revokes in-flight writers. Source citations are checked for conversation/workspace membership and delivery. This is storage validation, **not product authorization or factual validation**.
- AutoMigrate registers the new table with chat-deletion cascade. No production service writes or reads handoffs yet; merely migrating does not enable memory, add LLM calls, or expose tools.
- Optional `native_context.user_anchor_tokens` in both repositories, default zero. Older confirmed-human messages are retained newest-first within this extra budget, in chronological order, while the latest eligible request remains independently pinned. Summaries/explicit host events cannot become human anchors.
- Native checkpoint provenance metadata is backward compatible. Actor-bearing replies/approval responses are distinguished from authentication-completion events; actorless legacy responses remain unknown and preserve prior latest-request behavior. **Child-result notifications still use actorless `reply` requests and require an explicit end-to-end provenance contract before they can be safely distinguished from old human clients.**
- Failed compaction persistence restores the in-memory checkpoint state and its token adjustment, while retaining already-accounted summary usage.
- Offline regressions cover stale/expired leases, competing generations, invalidation, tenant/source isolation, oversize payloads, deletion cascade, and six compaction/restart generations retaining exact human constraints/corrections over 120 history rounds. Existing tool-loop and billing regressions remain in place. These are deterministic mechanics tests, not model-quality evidence.

Still required before describing the full plan as implemented:

1. Source-level authorization/retention invalidation wiring and complete explicit message provenance across Helpin, SDK, and Runtime.
2. Metered handoff generation with deterministic action/interaction/evidence envelopes, complete source snapshots, and bounded successor assembly with uncovered-tail recovery.
3. Authorized evidence archive/search/read contracts and their registration/preset/callback tests; no history tools are exposed by this slice.
4. Typed identifier recovery, request-fingerprint anchors, and progress/parent-child spending controls.
5. Real-model evaluation entry points and the approved evaluation matrix, then internal canary. No paid model evaluations have run and no flags/presets were enabled.

Do not enable older-human anchors merely because storage/mechanical tests pass. The existing carry-forward path is unchanged; this slice alone does not solve long-lived Ask Agent continuity or establish savings for the reported expensive run.
