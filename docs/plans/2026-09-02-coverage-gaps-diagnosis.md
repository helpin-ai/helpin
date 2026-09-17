# Coverage gaps diagnosis (ContentStudio, 2026-09-02)

> Historical production diagnosis, source-compared on 2026-09-17. Workspace
> counts, dates, billing observations, and incident conclusions below describe
> the September 2 sample. They are not current production measurements.

## Current source comparison

The [daily workflow](../../server/internal/temporalapp/coverage_analysis_workflow.go)
now logs child-workspace failures and continues scheduling the other workspaces.
Child workflow IDs include a timestamp suffix, so the fixed ID in the original
investigation instructions is not the current naming pattern. The daily input
window is three hours. The workspace activity still has a ten-minute timeout and
three retry attempts; a start heartbeat exists, but this is not the proposed
checkpointed batch-activity architecture.

The [analyzer](../../server/internal/service/support_coverage_daily_analyzer.go)
uses a successful cursor with overlap, pages candidates, collects item errors,
and materializes before marking a run failed for collected errors. The old
first-error `errgroup` description is no longer accurate. Materialization still
occurs after the page loop, not after each batch.

[Embedding materialization](../../server/internal/service/support_coverage_materializer.go)
now falls back to lexical grouping when the embedding provider is absent, fails,
or returns the wrong vector count. The original claim that these conditions
necessarily fail the whole run is obsolete; degraded grouping is not equivalent
to successful semantic matching.

The [analysis repository](../../server/internal/repository/support_coverage_analysis.go)
still selects unmaterialized findings by workspace **and run ID**. Its duplicate
check excludes failed conversation analyses but does not join the parent run's
status. The revised ordering helps collected item failures, but does not by itself
prove recovery of every finding from an interrupted run. The
[coverage repository](../../server/internal/repository/support_coverage.go) still
uses row count for `evidence_all` and distinct conversation-or-evidence identity
for the recent evidence count; those measures should not be described as identical.

The original reanalysis and cluster-rebuild steps are historical recommendations,
not actions performed by this audit. Expected reductions from 259 buckets are
hypotheses, not verified outcomes. The SQL and workspace identifiers below are
incident context and should pass publication review before public distribution.
No production API, database, or Temporal history was accessed for this update.

## Original September 2 diagnosis


Scope: the Support → Coverage page for the ContentStudio workspace (`fb464f68-…`), read-only via the API with a temporary token. No data was changed and no reanalysis was triggered.

Goal of the feature (from the plans in `docs/plans/2026-04-29-daily-coverage-gap-analysis.md` and `2026-06-17-coverage-gap-semantic-dedupe.md`): find issues the AI could not resolve, group them into a small number of topics, and propose the fix (article draft / update) so the AI handles the next occurrence.

## What the page showed on September 2

| Metric | Value |
|---|---|
| Summary "Open gaps" | 259 |
| Gaps in the list (status=open) | 12 |
| Gaps hidden by the raw-event filter | 247 (all `needs_review`) |
| Total evidence rows | 563 |
| Distinct conversations behind all evidence | 285 |
| Gaps backed by a single conversation | 244 of 259 |
| Gaps with LLM analysis output (category, confidence, embedding, issue key) | 0 |
| Gap metadata source | `event_detection` on all 259 |
| Draft suggestions attached to the 64 gaps sampled | 300, all `draft` |
| `last_analyzed_at` in summary | absent (never completed) |

Every one of the 12 visible gaps is a raw rule-based bucket, not a topic. Seven are titled "Human resolved after AI failure", and their "topic" is the first agent reply in the conversation. The evidence excerpts are the agent's own messages ("You're welcome.", "Have a good day!", "May I have your name please"). One gap ("Human resolved after AI failure", pricing_and_plans) has 28 evidence rows from a single conversation: 27 agent replies plus the resolve event.

## Root causes

### 1. The LLM daily analyzer has never completed a run for this workspace, and stopped completing for the other big workspaces in mid-June

Evidence:

- Summary `last_analyzed_at` is missing for ContentStudio. For ContentPen, Usermaven and Replug it is stuck at 2026-06-15 to 2026-06-18, the exact days the semantic-dedupe/materialization commits landed (`8c3d048fb`, `d22d8e21f`, `1d020d45d`, …). Helpin (1 gap) last completed 2026-08-24.
- Billing usage shows `coverage_gap_analysis` charges on several days (ContentStudio: Aug 26, 27, 31, Sep 2; Replug: Aug 22, 25, 28), but only 1–3 LLM actions per day per workspace. The run starts, analyzes a couple of conversations, then dies. It never reaches `CompleteRun`.
- Because `HasCompletedAnalysisRun` is false, the legacy v1 event path keeps creating "Human resolved after AI failure" gaps that the plan said would be retired once the analyzer is active (`service/support_coverage.go:174-180`).

Structural problems that make any failure fatal (independent of the specific trigger, which needs the `error_message` on `support_coverage_analysis_runs` or the Temporal history of `coverage-analysis-ws-<id>` to confirm):

- The whole workspace run is one Temporal activity with a 10-minute `StartToCloseTimeout` (`temporalapp/coverage_analysis_workflow.go:126`). Up to 1000 conversations × (analyze + knowledge match + refine) at concurrency 4 cannot finish in 10 minutes for a workspace with 300 conversations a fortnight, and there is no heartbeat, so the activity is killed and retried from scratch.
- Inside the run, `errgroup` aborts every in-flight conversation on the first non-LLM error (`support_coverage_daily_analyzer.go:317-335`): any DB error, `matchCurrentKnowledgeForAnalysis` error, or embedding failure fails the run. LLM errors are recorded per conversation and do not abort, but knowledge-match errors do.
- Materialization runs only at the end of a successful run and only for `run_id = this run` (`repository/support_coverage_analysis.go:189-205`). Meanwhile `AlreadyAnalyzedConversation` counts analyses from failed runs as done (`:249-265`). So a conversation analyzed in a failed run is never re-analyzed and its finding is never materialized. Findings are orphaned forever.
- `embedMaterializationFindings` returns an error when the embedding provider is nil or when the embedding API fails (`support_coverage_materializer.go:214-234`), which fails the whole run after all the LLM spend.
- The parent cron workflow stops scheduling remaining workspaces once any child fails while 10 are in flight (`coverage_analysis_workflow.go:68-76`). Workspaces are ordered by id; ContentStudio's id (`fb46…`) sorts last.

### 2. The v1 event-detection path produces the wrong unit of counting

- `human_reply_after_ai` fires for every agent message in an escalated widget conversation (`service/support_inbox.go:2108-2138`), and each message is an evidence row. `evidence_count` therefore counts agent replies, not customer problems. 563 evidence rows map to 285 conversations.
- The gap title is the rule title and the topic/cluster key is derived from the agent's reply text (`support_coverage_clusterer.go:117-123`), so the customer question never appears anywhere on the gap. Nothing to merge on, nothing to act on.
- `widget_search_performed` with no results creates a gap per query string, and queries arrive per keystroke: "YouTube Community pod", "YouTube Community podt", "tag post for a campaoiiign", "tag post for a campaoiii", "how often do I need to reconnect social media", "…social", "…reconnect". 92 such gaps, 12 with queries of 4 characters or fewer ("You", "Wot", "recc").
- Email conversations never involve the AI (259 of 300 sampled conversations have `ai_state = null`), so the only real "AI could not resolve" population is the 27 escalated widget conversations in the last two weeks. The page reports 259 open gaps.

### 3. Counting inconsistencies in the API itself

- `GetSummary.total_open_gaps` counts every `status='open'` row; `ListGaps` additionally hides `event_detection` + `needs_review` + `confidence < 0.7` rows (`repository/support_coverage.go:869-880`). Hence 259 vs 12. The list has no "show raw" toggle in the UI, so 247 gaps are invisible.
- `top_recurring_gaps` and `handoffs_after_fixes` are never populated; they are always 0.
- `evidence_all` is `COUNT(*)` while `evidence_30d` is `COUNT(DISTINCT conversation)`; the impact score mixes the two scales.
- `/coverage/conversations/{id}/state` never returns `gap_id` (the repo only looks at docs-issue feedback events), so the inbox cannot link a conversation to its gap even when it is in evidence.
- Spike enrichment / draft generation keeps adding drafts: sampled gaps carry up to 7 draft suggestions each (300 drafts on 64 gaps), all unapplied.

### 4. Merging cannot work on this data

The semantic dedupe (attach ≥ 0.90, suggest ≥ 0.78, rebuild suggest ≥ 0.72 / auto-merge ≥ 0.88) only runs on findings that come out of the analyzer, and needs embeddings. None of the 259 gaps has an embedding, an `issue_key`, or a `customer_need`, and the cluster rebuild has never run (`clusters/rebuild/latest` is null). Merge suggestions: 0.

## Recommended fixes, in order

1. **Make the analyzer run survive.** Split the per-workspace activity into a batching loop: one activity per N conversations (for example 25) with heartbeats, a cursor checkpoint after each batch, and a 10-minute timeout per batch rather than per workspace. Record per-conversation failures and continue; only abort on repeated infrastructure errors. Materialize after every batch, not only at the end.
2. **Stop orphaning findings.** Materialize `has_gap = true AND gap_id IS NULL` analyses for the workspace regardless of `run_id`, or exclude analyses from failed runs in `AlreadyAnalyzedConversation`.
3. **Don't let one workspace stop the others.** In `CoverageDailyAnalysisWorkflow`, collect child errors instead of breaking the scheduling loop.
4. **Expose run health.** Put the latest run status, error message, conversations analyzed and findings materialized into `GetSummary` and show it on the page. Right now a dead pipeline looks identical to a quiet one.
5. **Retire or demote the v1 event path once the analyzer is live.** At minimum: one evidence row per conversation (key on `conversation_id`, not `message_id`), use the customer's first message as the excerpt and topic instead of the agent reply, and debounce widget searches (only record the final query after 2 s of no typing, minimum 3 tokens).
6. **Make the summary and the list count the same thing.** Apply the same raw-event filter in `total_open_gaps`, or better, drop raw `needs_review` gaps from "open" entirely and show them under a separate "unreviewed signals" count.
7. **Re-run for ContentStudio once 1–3 ship.** Trigger `/support/coverage/reanalyze` for the workspace, then run the cluster rebuild so merge suggestions appear. Expect the 259 buckets to collapse into a few dozen customer-need topics with drafts.
8. Smaller cleanups: populate or remove `top_recurring_gaps` / `handoffs_after_fixes`; return `gap_id` from the conversation state endpoint; cap active drafts per gap at one; use `COUNT(DISTINCT conversation_id)` for `evidence_all`.

## How to confirm the specific run failure

```sql
select id, status, started_at, completed_at, conversation_cnt, gap_count, error_message
from support_coverage_analysis_runs
where workspace_id = 'fb464f68-0c05-4cbd-b571-5254bc88211f'
order by started_at desc limit 10;

select status, count(*) from support_coverage_conversation_analyses
where workspace_id = 'fb464f68-0c05-4cbd-b571-5254bc88211f' group by status;
```

And in Temporal: workflow `coverage-daily-analysis` (cron) and child `coverage-analysis-ws-fb464f68-0c05-4cbd-b571-5254bc88211f`; look for `StartToClose` timeouts on `CoverageAnalysisActivities.RunWorkspaceAnalysisActivity`.
