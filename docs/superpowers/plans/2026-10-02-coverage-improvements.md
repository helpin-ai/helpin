# Coverage improvements implementation plan

**Goal:** Make coverage counts consistent, expose earlier detections for manual
review, explain analysis state accurately, and connect paginated insights to fixes.

**Architecture:** Keep the existing gap detail and resolution actions. Share the
visibility predicate between list and summary queries. Add additive pagination
metadata to insight APIs and use TanStack infinite queries for those lists.
Preserve the duplicate-check control above the table on the right. Work in
`waqar-fixes`, without sub-agents or additional planning documents.

**Tech stack:** Go/GORM, React/TypeScript, TanStack Query, Vitest.

- [x] Add repository regression tests for visible summary metrics, hidden review
  counts, review-only lists, workspace isolation, and conversation filters. Run
  them red, then implement shared predicates in `support_coverage.go`, the DTO
  fields in `model/support_coverage.go`, and handler parsing.
- [x] Add pagination totals with regression tests in
  `repository/support_coverage_v2_test.go`; expose them through the existing
  v2 handlers/services while preserving list API compatibility.
- [x] Add status presentation regression tests covering disabled, waiting,
  queued/running, completed, failed, and quality-gated processing. Implement a
  small presenter and use it in `CoverageInsights.tsx` with readable reasons.
- [x] Add infinite-list hook regressions for loading beyond the first page,
  retry, workspace changes, and refresh after review. Wire the hook and adapter
  pagination into `SupportCoveragePage.tsx` and `CoverageInsights.tsx`.
- [x] Add review access and classification through existing authorized actions;
  link topic findings to open gaps for the source conversation. Keep supplementary
  explanations concise and preserve current resolution/duplicate behavior.
- [x] Run relevant Go and frontend tests, frontend lint/type checks, and inspect
  the final diff. Update this document with results and remaining limitations.

The task changes application code only. The earlier global pipeline configuration
is already published. Existing records will not be automatically rejected,
merged, or classified as part of this UI work.

## Verification

- Coverage tests passed across repository, service, handler, and model packages.
- 42 relevant frontend tests passed; pagination retry, workspace isolation,
  refresh after removal, and nine analysis status cases are covered.
- Frontend lint and TypeScript checks passed. Backend vet and build passed.
- A local browser using synthetic data verified desktop duplicate-control
  placement, complete topic/signal pagination, dismissal refresh, topic-to-gap
  navigation, classification, and the waiting status. At 390px, the page had no
  horizontal overflow. No browser exceptions were recorded.
- Temporary preview files were removed. The final diff preserves existing
  resolution and duplicate-check behavior.

Application deployment is separate from this branch update. The existing backlog remains
available for manual classification or rejection; no bulk record changes or
historical reanalysis were performed.
