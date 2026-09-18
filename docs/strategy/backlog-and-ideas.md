# Historical backlog and ideas

**Purpose**: Capture features, improvements, bugs, and ideas that come up during development.
**Original snapshot**: March 3, 2026
**Source review**: September 18, 2026


This page preserves early product ideas for contributors researching decisions.
It is not the current roadmap or issue tracker. Priorities, phase labels, “Open”
statuses, and the empty bug section below describe the original planning snapshot.
Use the [current roadmap](../../ROADMAP.md) for Community scope and known limits.

## Implementation changes since this snapshot

- An [automation rule engine](../../server/internal/service/automation_rule_engine.go)
  now exists. This does not establish support for every example or every edition.
- Shortcut API import is implemented; the [reviewed progress record](../plans/2026-04-26-shortcut-api-import-progress.md)
  explains current execution, preview and media limits. It is no longer merely a
  future import idea.
- [Realtime synchronization](../../frontend/src/hooks/useRealtimeSync.ts) uses
  WebSocket events, with [fallback polling](../../frontend/src/hooks/useRealtimeFallbackPolling.ts).
  The old polling-only baseline is obsolete; these mechanisms do not guarantee
  instantaneous updates under all connection conditions.
- [Team estimate settings](../../server/internal/service/settings.go) support
  exponential, Fibonacci, linear, T-shirt and hours scales. The original
  workspace-only/custom-scale proposal is not the exact current contract.

Other entries remain historical ideas or assessment questions. Their presence here
is neither evidence of absence from today's code nor a commitment to deliver them.
In particular, generic API tokens, Slack replies, capacity/PTO planning, retention,
and universal pagination need separate scope and acceptance decisions rather than
being marked complete based on nearby features.

The decisions log records original rationale. “Zero risk” is not an engineering
guarantee, and GORM usage does not replace the current
[versioned migration process](../ops/database-migrations.md). External comparisons
and library claims were not reverified in this review. No runtime tests were run.

## Original planning snapshot

---

## How to Use This Document

The original instructions invited contributors to add items that:
- Are discovered during implementation but not in the current phase scope
- Come from team feedback during usage
- Are ideas for future improvement
- Are bugs found during development
- Are technical debt items to address later

### Item Format

```
#### [Category] Short title
- **Priority**: P0 (critical) / P1 (high) / P2 (medium) / P3 (low) / P4 (nice to have)
- **Source**: Who/what raised this (e.g., "During Phase 1 board implementation", "Team feedback")
- **Description**: What needs to be done
- **Status**: Open / In Progress / Done / Won't Do
- **Phase**: Which phase it should be addressed in (or "Future")
- **Notes**: Any additional context
```

---

## Feature Requests

#### Automation Rules Engine
- **Priority**: P2
- **Source**: PRD — Future Considerations
- **Description**: Rule-based automations without external tools. Examples: "When story state changes to Done → notify channel", "When all sub-tasks complete → move parent to Done" (this one is built-in), "When story created with label 'urgent' → set priority to high"
- **Status**: Open
- **Phase**: Future
- **Notes**: Similar to Shortcut's Event Handlers. Could start with simple if-then rules.

#### Import from Shortcut
- **Priority**: P3
- **Source**: PRD — Non-Goals (may be revisited)
- **Description**: One-time import tool to migrate existing Shortcut data (stories, epics, iterations, labels, comments) into Helpin PM. Map Shortcut IDs to Helpin display IDs.
- **Status**: Open
- **Phase**: Future
- **Notes**: Only needed if team wants historical data. Manual transition may be sufficient.

#### Real-time Updates (WebSocket)
- **Priority**: P2
- **Source**: PRD — Future Considerations
- **Description**: Replace polling with WebSocket connections for live updates on the board. When one user moves a story, other users see it move in real-time.
- **Status**: Open
- **Phase**: Future
- **Notes**: Polling every 30s is acceptable for V1. WebSockets add infrastructure complexity.

#### Slack Integration
- **Priority**: P2
- **Source**: PRD — Phase 3+ integration
- **Description**: Notify Slack channels when stories change state, @mentions happen. Allow replies from Slack threads to post as comments on stories.
- **Status**: Open
- **Phase**: Future
- **Notes**: Shortcut has deep Slack integration. Our team currently gets value from this.

#### Custom Estimate Scales
- **Priority**: P3
- **Source**: PRD — Future Considerations
- **Description**: Allow workspaces to configure their estimate scale (Fibonacci, Linear, Powers of 2, Custom). Currently we use integer points without a defined scale.
- **Status**: Open
- **Phase**: Future
- **Notes**: Add to PM workspace settings.

#### Sprint Retrospective Feature
- **Priority**: P3
- **Source**: PRD — Future Considerations
- **Description**: Built-in retrospective tool for iterations. Template with: What went well, What didn't go well, Action items. Action items can convert to stories for next iteration.
- **Status**: Open
- **Phase**: Future
- **Notes**: Could be a Doc template with special handling.

#### Capacity Planning
- **Priority**: P3
- **Source**: PRD — Future Considerations
- **Description**: Plan iteration capacity based on team size × velocity average. Show warning when iteration is over/under-planned. Factor in PTO/holidays.
- **Status**: Open
- **Phase**: Future

#### API Tokens for External Access
- **Priority**: P3
- **Source**: PRD — Future Considerations
- **Description**: Generate API tokens for workspace members to access PM data via REST API from external tools, scripts, or CI/CD pipelines.
- **Status**: Open
- **Phase**: Future

#### Figma Embed Previews
- **Priority**: P4
- **Source**: PRD — Future Considerations
- **Description**: When a Figma link is pasted into a story description or doc, render a live embed preview instead of a plain link.
- **Status**: Open
- **Phase**: Future

#### Story Type Custom Fields
- **Priority**: P3
- **Source**: Shortcut feature parity
- **Description**: Allow custom fields to be conditionally shown based on story type. E.g., "Severity" only shows on Bug stories (this is already handled as a built-in field, but custom fields may also need conditional visibility).
- **Status**: Open
- **Phase**: Future

#### Multiple Workflows per Team
- **Priority**: P3
- **Source**: Shortcut feature
- **Description**: Allow a team to have more than one workflow (e.g., "Engineering Flow" and "Design Flow") and assign stories to a specific workflow.
- **Status**: Open
- **Phase**: Future

---

## Integration Ideas

#### Bonus System ↔ PM Integration (Phase 2 Bridge)
- **Priority**: P1
- **Source**: Core business value
- **Description**: Create a mapping layer between PM iterations and bonus sprints. Allow PM completion metrics (stories completed, velocity) to feed into individual/team scoring. Design: `pm_bonus_mapping` table, computed views for scoring formulas, API bridge endpoints.
- **Status**: Open
- **Phase**: Post Phase 4 (dedicated integration phase)
- **Notes**: This is the ultimate value proposition of having PM in Helpin. Plan carefully.

#### GitHub Actions Integration
- **Priority**: P3
- **Source**: CI/CD workflow
- **Description**: Beyond basic webhook, integrate with GitHub Actions to show build/deploy status on stories. "Story TP-123 deployed to staging" activity entry.
- **Status**: Open
- **Phase**: Future

#### Email Integration
- **Priority**: P3
- **Source**: Notification enhancement
- **Description**: Send email notifications for critical events (assigned, mentioned, deadline passed). Configurable per user. Reply-by-email to add comments.
- **Status**: Open
- **Phase**: Future

---

## Technical Debt & Improvements

#### Per-Workspace Display ID Sequence
- **Priority**: P1
- **Source**: Phase 1 implementation consideration
- **Description**: The `pm_story_display_id_seq` is global. In a multi-workspace setup, display IDs would not be contiguous within a workspace. Consider: per-workspace counter in `workspace_settings` table, or use a sequence per workspace.
- **Status**: Open
- **Phase**: Phase 1 (during implementation)
- **Notes**: Decision needed: global sequence (simpler, IDs not contiguous per workspace) vs per-workspace counter (cleaner UX, more complex).

#### Database Connection Pooling
- **Priority**: P2
- **Source**: Performance consideration
- **Description**: As PM module adds many new tables and queries, ensure database connection pooling is properly configured. May need PgBouncer or similar.
- **Status**: Open
- **Phase**: Phase 2+

#### API Response Caching
- **Priority**: P2
- **Source**: Performance consideration
- **Description**: Frequently accessed data (workflow states, labels, team list) should be cached in-memory with short TTL. Reduces DB queries on every page load.
- **Status**: Open
- **Phase**: Phase 2+

#### Activity Log Cleanup/Archival
- **Priority**: P3
- **Source**: Data growth consideration
- **Description**: Activity log will grow indefinitely. Plan for: archival after N months, pagination optimization, or consider moving old logs to cold storage.
- **Status**: Open
- **Phase**: Future

#### Test Coverage
- **Priority**: P1
- **Source**: Engineering quality
- **Description**: Write tests as we build. Backend: unit tests for services, integration tests for handlers. Frontend: component tests for key interactions (board drag-drop, story CRUD).
- **Status**: Open
- **Phase**: Ongoing (each phase)
- **Notes**: Don't wait until the end. Test as part of each task.

#### Pagination Standardization
- **Priority**: P2
- **Source**: API consistency
- **Description**: Standardize pagination across all PM list endpoints. Use cursor-based pagination for large lists (stories, activity log) and offset-based for smaller lists (epics, iterations).
- **Status**: Open
- **Phase**: Phase 1 (define standard), apply across all phases

---

## Bugs

*No bugs yet — add them here as they're discovered during development.*

#### [Template] Bug title
- **Priority**: P?
- **Source**: Where it was found
- **Description**: What's happening vs what should happen
- **Steps to Reproduce**: 1. ... 2. ... 3. ...
- **Status**: Open
- **Phase**: Current phase
- **Notes**: Any workarounds

---

## UX / Design Feedback

*Add feedback from team usage here.*

#### [Template] UX feedback title
- **Priority**: P?
- **Source**: Who raised it
- **Description**: What the experience is and what it should be
- **Status**: Open
- **Phase**: ?
- **Notes**: Include screenshots if possible

---

## Decisions Log

Record important architectural and design decisions here for future reference.

#### Decision: Separate PM tables with `pm_` prefix
- **Date**: March 3, 2026
- **Decision**: All PM module tables are prefixed with `pm_` and completely independent from existing bonus system tables.
- **Rationale**: Zero risk to existing system, faster to ship, integrate later when patterns are clear.
- **Alternatives considered**: Shared tables (too risky), new schema (PostgreSQL schema isolation — overkill for this scale).

#### Decision: GORM for ORM
- **Date**: March 3, 2026
- **Decision**: Continue using GORM for PM tables, consistent with existing codebase.
- **Rationale**: Team familiarity, existing patterns established, auto-migration support.

#### Decision: TipTap for Rich Text Editor
- **Date**: March 3, 2026
- **Decision**: Use TipTap (headless) for the rich text editor in Docs and story descriptions.
- **Rationale**: Headless (full control over UI), extensible (custom extensions for @mentions), React-native, active maintenance, MIT license.
- **Alternatives considered**: Slate.js (lower-level, more work), Quill (opinionated styling), ProseMirror (TipTap is built on it, but TipTap adds React layer).

#### Decision: Recharts for Charts
- **Date**: March 3, 2026
- **Decision**: Use Recharts for all report charts (velocity, burndown, CFD, cycle time, lead time).
- **Rationale**: React-native, composable, popular, good documentation, supports all chart types needed.
- **Alternatives considered**: Chart.js (canvas-based, less React-native), D3 (too low-level), Nivo (good but less popular).

---

## Meeting Notes

*Capture relevant discussions and decisions from meetings here.*

---

## Reference Links

- [Shortcut.com Help Center](https://help.shortcut.com/)
- [Shortcut REST API v3](https://developer.shortcut.com/api/rest/v3)
- [TipTap Editor Documentation](https://tiptap.dev/)
- [Recharts Documentation](https://recharts.org/)
- [dnd-kit Documentation](https://dndkit.com/)
- [TanStack Table Documentation](https://tanstack.com/table)
- [React Flow Documentation](https://reactflow.dev/)
