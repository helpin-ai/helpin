# PRD: Backend Logging & Test Coverage

**Status:** Draft
**Created:** 2026-03-10
**Owner:** Engineering

---

## Problem Statement

The Go backend has critical observability and quality gaps:
- **173 of 183 files** lack structured logging (`log/slog`)
- **11 of 18 core packages** have zero test coverage (37 test files total)
- **178 silent `_ =` error discards** hide production failures
- **10 files** still use legacy `log.Printf` instead of structured `slog`

This creates blind spots in production debugging, audit compliance, and regression safety.

---

## Goals

1. Structured slog logging across all backend layers per CLAUDE.md standards
2. Test coverage for all 18 core packages (security-critical first)
3. Eliminate silent error discards (`_ =`) with logged warnings
4. Migrate all legacy `log.Printf` to `slog`

---

## Reference Implementations

| Reference | File | Purpose |
|-----------|------|---------|
| Logger pattern | `server/internal/service/notification.go:26-35` | `*slog.Logger` field + `slog.Default().With("service", name)` |
| Test pattern | `server/internal/service/notification_emit_test.go` | SQLite in-memory DB, stub interfaces, table-driven subtests |
| Middleware target | `server/internal/middleware/logging.go` | Upgrade from `log.Printf` to `slog` |
| Security test target | `server/internal/auth/jwt.go` | Self-contained JWT logic, trivial to test |

---

## Phase 1: Foundation — Logging Infrastructure + Security Tests

### 1A. Upgrade Logging Middleware

**File:** `server/internal/middleware/logging.go`

Replace `log.Printf` with structured slog. Log 4xx at Warn, 5xx at Error. This single change provides HTTP request logging for all 56 handlers.

```go
slog.InfoContext(r.Context(), "http request",
    "method", r.Method,
    "path", r.URL.Path,
    "status", rw.statusCode,
    "duration_ms", duration.Milliseconds(),
    "user_id", GetUserID(r.Context()),
)
```

| Task | Status |
|------|--------|
| Upgrade `middleware/logging.go` to slog with structured fields | [x] |
| Log 4xx at Warn, 5xx at Error | [x] |
| Include user_id from context when available | [x] |

### 1B. Security-Critical Unit Tests

| Task | File to Create | Status |
|------|----------------|--------|
| JWT tests (generate, validate, expired, tampered, wrong secret) | `auth/jwt_test.go` | [x] |
| Password tests (hash, check correct/wrong/empty) | `auth/password_test.go` | [x] |
| Encryption tests (roundtrip, wrong key, tampered ciphertext) | `crypto/encrypt_test.go` | [x] |
| Auth middleware tests (missing header, bad token, valid token) | `middleware/auth_test.go` | [x] |
| Logging middleware tests (correct slog attributes emitted) | `middleware/logging_test.go` | [x] |
| Context helper tests (WithUserID/GetUserID roundtrip) | `middleware/context_test.go` | [x] |

### 1C. Migrate Legacy Log in main.go

| Task | Status |
|------|--------|
| Replace `log.Fatalf`/`log.Println` with `slog.Error`+`os.Exit(1)`/`slog.Info` in `cmd/api/main.go` | [x] |

---

## Phase 2: Core Service Logging + Tests

### 2A. Add slog to Top 15 Services

**Pattern:** Add `logger *slog.Logger` field, init with `slog.Default().With("service", "<name>")` in constructor. Log Create/Update/Delete at INFO, errors at ERROR. Replace `_ =` discards with logged errors.

| # | Service File | Key Operations | `_ =` to Fix | Status |
|---|-------------|----------------|---------------|--------|
| 1 | `service/auth.go` | login, signup, refresh | 0 | [ ] |
| 2 | `service/workspace.go` | workspace CRUD | 0 | [ ] |
| 3 | `service/bonus.go` | reward calculations | 0 | [ ] |
| 4 | `service/settings.go` | workspace settings | 0 | [ ] |
| 5 | `service/invite.go` | invitation send/accept (replace `log.Printf`) | 0 | [ ] |
| 6 | `service/audit.go` | audit trail creation | 0 | [ ] |
| 7 | `service/pm_sprint.go` | sprint lifecycle | 0 | [ ] |
| 8 | `service/pm_epic.go` | epic CRUD | 11 | [ ] |
| 9 | `service/pm_objective.go` | objective CRUD | 2 | [ ] |
| 10 | `service/pm_automation.go` | auto-workflow (replace `log.Printf`) | 2 | [ ] |
| 11 | `service/pm_story.go` | story CRUD (extend existing) | 20+ | [ ] |
| 12 | `service/pm_workflow.go` | workflow state changes | 0 | [ ] |
| 13 | `service/pm_label.go` | label management | 0 | [ ] |
| 14 | `service/pm_view.go` | saved views | 0 | [ ] |
| 15 | `service/pm_comment.go` | comments (extend existing) | 4 | [ ] |

### 2B. Core Service Tests

| Task | File to Create | Status |
|------|----------------|--------|
| Auth service (signup, signin, refresh, duplicate email) | `service/auth_test.go` | [x] |
| Workspace service (CRUD, GetBySlug) | `service/workspace_test.go` | [x] |
| Bonus service (calculation logic, edge cases) | `service/bonus_test.go` | [x] |
| Settings service (get/update, defaults) | `service/settings_test.go` | [x] |
| Sprint service (create, start, complete, dates) | `service/pm_sprint_test.go` | [x] |
| Epic service (CRUD, archive) | `service/pm_epic_test.go` | [x] |
| Story service (CRUD, state transitions) | `service/pm_story_extended_test.go` | [ ] |
| Invite service (create, accept, duplicate) | `service/invite_test.go` | [x] |

---

## Phase 3: External Integration Logging + Tests

### 3A. Add slog to External Clients

| File | Log at INFO | Never Log | Status |
|------|-------------|-----------|--------|
| `storage/s3.go` | PutObject, DeleteObject, presigned URL gen | Presigned URLs, file contents | [ ] |
| `email/postmark.go` | SendEmail (to, subject), SendInviteEmail | HTML body | [ ] |
| `oauth/gmail.go` | ExchangeCode, RefreshToken success/fail | Tokens, secrets | [ ] |
| `llm/claude.go` | Request (provider, model), response time | Prompts, responses | [ ] |
| `llm/openai.go` | Same as claude.go | Prompts, responses | [ ] |
| `websocket/handler.go` | Replace `log.Printf` with `slog.Error` | — | [ ] |

### 3B. External Client Tests

| Task | File to Create | Approach | Status |
|------|----------------|----------|--------|
| Postmark email client | `email/postmark_test.go` | `httptest.Server` mock | [ ] |
| Gmail OAuth flow | `oauth/gmail_test.go` | `httptest.Server` for token endpoint | [ ] |
| S3 URL logic | `storage/s3_test.go` | Unit tests for URL generation | [ ] |
| LLM provider interface | `llm/provider_test.go` | Interface contract tests | [ ] |
| Gmail sync parsing | `sync/gmail_test.go` | `httptest.Server` for Gmail API | [ ] |
| WebSocket auth | `websocket/handler_test.go` | `httptest.Server` + mock AuthzService | [ ] |

### 3C. Legacy Log Migration

| File | Change | Status |
|------|--------|--------|
| `websocket/hub.go` | `log.Printf` to `slog` | [ ] |
| `worker/executor.go` | `log.Printf` to `slog.Warn` | [ ] |
| `repository/org_migration.go` | Startup logging to `slog` | [ ] |
| `cmd/temporal-worker/main.go` | Structured logging | [ ] |

---

## Phase 4: CRM + Temporal Logging + Tests

### 4A. CRM Service Logging (15 files)

| Service File | Key Operations | Status |
|-------------|----------------|--------|
| `service/crm_contact.go` | Contact CRUD, lifecycle stages | [ ] |
| `service/crm_company.go` | Company CRUD | [ ] |
| `service/crm_deal.go` | Deal CRUD, stage progression | [ ] |
| `service/crm_activity.go` | Activity logging | [ ] |
| `service/crm_association.go` | Entity associations | [ ] |
| `service/crm_calendar.go` | Calendar sync | [ ] |
| `service/crm_enrichment.go` | Data enrichment | [ ] |
| `service/crm_import.go` | Data import | [ ] |
| `service/crm_list.go` | List management | [ ] |
| `service/crm_property.go` | Custom properties | [ ] |
| `service/crm_search.go` | CRM search | [ ] |
| `service/crm_sequence.go` | Email sequences | [ ] |
| `service/crm_writing_profile.go` | Writing profiles | [ ] |
| `service/crm_signal.go` | Signal storage | [ ] |
| `service/crm_pipeline.go` | Pipeline management | [ ] |

### 4B. Temporal + Worker Logging

| File | Changes | `_ =` to Fix | Status |
|------|---------|---------------|--------|
| `temporalapp/activities.go` (2,149 lines) | Add slog at activity start/complete | 16 | [ ] |
| `worker/executor.go` | Replace `log.Printf`, add structured logging | 8 | [ ] |
| `worker/claude.go` | Add slog for LLM interactions | 5 | [ ] |
| Audit `deal_management_workflow.go` | Verify completeness | — | [ ] |
| Audit `email_sync_activities.go` | Verify completeness | — | [ ] |
| Audit `signal_detection_workflow.go` | Verify completeness | — | [ ] |

### 4C. CRM + Temporal Tests

| Task | File to Create | Status |
|------|----------------|--------|
| Contact CRUD + lifecycle | `service/crm_contact_test.go` | [ ] |
| Deal CRUD + stage progression | `service/crm_deal_test.go` | [ ] |
| Deal automation thresholds | `service/crm_deal_automation_test.go` | [ ] |
| Signal detection + confidence | `service/crm_signal_detection_test.go` | [ ] |
| Suggestion approval/rejection | `service/crm_suggestion_test.go` | [ ] |
| Email thread linking | `service/crm_email_test.go` | [ ] |
| CSV import + duplicates | `service/crm_import_test.go` | [ ] |
| Temporal activities | `temporalapp/activities_extended_test.go` | [ ] |
| Worker executor | `worker/executor_test.go` | [ ] |
| Worker Claude client | `worker/claude_test.go` | [ ] |

---

## Phase 5: Handler Tests + Remaining Coverage

### 5A. Handler Test Infrastructure

| Task | File to Create | Status |
|------|----------------|--------|
| Test helpers (newTestRouter, injectUserCtx, assertJSONResponse) | `handler/testutil_test.go` | [ ] |

### 5B. Handler Tests (12 files, priority order)

| # | Handler | File to Create | Status |
|---|---------|----------------|--------|
| 1 | Auth (signup/signin/refresh) | `handler/auth_test.go` | [ ] |
| 2 | Workspace (CRUD, logo upload) | `handler/workspace_test.go` | [ ] |
| 3 | Stories (CRUD, filters) | `handler/pm_story_test.go` | [ ] |
| 4 | Sprints (lifecycle endpoints) | `handler/pm_sprint_test.go` | [ ] |
| 5 | Support (ticket creation) | `handler/support_test.go` | [ ] |
| 6 | CRM Contacts | `handler/crm_contact_test.go` | [ ] |
| 7 | CRM Deals | `handler/crm_deal_test.go` | [ ] |
| 8 | Settings | `handler/settings_test.go` | [ ] |
| 9 | Notifications | `handler/notification_test.go` | [ ] |
| 10 | Agent | `handler/agent_test.go` | [ ] |
| 11 | Docs | `handler/docs_test.go` | [ ] |
| 12 | Search | `handler/search_test.go` | [ ] |

### 5C. Remaining Service Logging (~20 files)

| Service File | Status |
|-------------|--------|
| `service/docs_collection.go` | [ ] |
| `service/docs_article.go` | [ ] |
| `service/docs_category.go` | [ ] |
| `service/docs_space.go` | [ ] |
| `service/docs_search.go` | [ ] |
| `service/docs_content.go` | [ ] |
| `service/docs_version.go` | [ ] |
| `service/docs_link.go` | [ ] |
| `service/pm_attachment.go` | [ ] |
| `service/pm_external_link.go` | [ ] |
| `service/pm_story_template.go` | [ ] |
| `service/agent.go` | [ ] |
| `service/agent_planning.go` | [ ] |
| `service/agent_policy.go` | [ ] |
| `service/search.go` | [ ] |
| `service/git.go` | [ ] |
| `service/github.go` | [ ] |
| `service/organization.go` | [ ] |
| `service/user.go` | [ ] |
| `service/quarter.go` | [ ] |

---

## Verification

After each phase:
1. `cd server && go vet ./...` — no compilation errors
2. `cd server && go test ./...` — all tests pass
3. Start server locally, trigger endpoints, verify structured JSON logs in stdout
4. `grep -rn "_ =" server/internal/service/` — track `_ =` reduction
5. `grep -rn '"log"' server/internal/` — track legacy log migration

## Target Metrics

| Metric | Before | After |
|--------|--------|-------|
| Files with slog | 14 | ~120 |
| Legacy `log.Printf` files | 10 | 0 |
| Silent `_ =` discards | 178 | ~10 (intentional) |
| Test files | 37 | ~90 |
| Packages with zero tests | 11/18 | 0/18 |
