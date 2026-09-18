# Support Conversation Company Context Implementation Plan

This historical implementation plan explains how support conversations retain an account association while the visitor switches active companies. The core feature exists; the unchecked tasks below preserve the original work breakdown.

## Source review: September 18, 2026

- The [company-context migration](../../server/internal/dbmigrate/sql/202608050001_support_conversation_company_context.sql) adds nullable references with delete-to-null foreign keys. Its historical backfill requires exactly one **distinct** company per workspace/contact, including associations in either direction, and never overwrites an existing conversation company.
- [Widget identity application](../../server/internal/service/support_inbox_widget.go) updates active session company and uses `SetCRMCompanyIfUnset` for current conversations. The [repository helpers](../../server/internal/repository/support_inbox.go) restrict anonymous-ID fallback to non-revoked, unexpired sessions and scope company writes by workspace. Stable conversation association does not mean the underlying company attributes are snapshotted.
- [Company matching](../../server/internal/service/support_inbox.go) keeps a declared external ID authoritative and locks that lookup. Later behavior also attempts email-domain matching when no company is declared, recording a separate match method. That inference is weaker than a declared or verified account claim and is not described by the original plan's matching rules.
- The manual `PUT /api/support/inbox/conversations/{id}/crm-company` route is registered under `PermSupportEdit`. The service validates the selected company's workspace, and [visitor context](../../server/internal/service/support_inbox_visitor.go) loads the current company by the conversation's direct ID with partial-failure status.
- [The SDK client](../../packages/sdk-js/src/core/client.ts) persists active company context and merges inline/persisted company data. [The sidebar](../../frontend/src/components/support/ConversationDetailSidebar.tsx) renders Company Details between contact details and other conversations; [company editing](../../frontend/src/components/support/SidebarCompanyDetails.tsx) is gated by `support.edit`.

The Go 1.24 reference and expected test results below belong to the original plan; [the module](../../server/go.mod) now declares Go 1.25.0. This documentation review did not rerun application tests or the manual account-switching scenario.

## Original implementation plan

**Goal:** Capture the visitor's active CRM company on each support conversation, keep that company identity stable, and show its live subscription/account details in the inbox sidebar after the consolidated Contact Details section.

**Architecture:** Add direct nullable CRM-company references to widget sessions and conversations. Widget identity updates mutate the session's active company and fill only an unlinked current conversation; conversation reads resolve current CRM company data through the existing visitor-context endpoint. The SDK synchronizes its persisted active group through the identity path, while the inbox exposes a permission-aware manual correction control without changing CRM primary-company membership.

**Tech Stack:** Go 1.24, Chi, GORM/PostgreSQL, React 19, TypeScript 5.9, TanStack Query, Vitest/Testing Library, Tailwind/shadcn, Helpin JavaScript SDK.

---

## File structure

### Backend

- `server/internal/model/support_inbox.go` — direct company fields, company update DTO, and typed visitor-company response contracts.
- `server/internal/dbmigrate/sql/202608050001_support_conversation_company_context.sql` — nullable columns, indexes, foreign keys, and unambiguous historical backfill.
- `server/internal/repository/support_inbox.go` — session/conversation company mutation helpers with set-if-null semantics.
- `server/internal/repository/crm_company.go` — workspace-safe direct company lookup and concurrency-safe external-ID resolution support.
- `server/internal/service/support_inbox_widget.go` — active-company resolution, membership semantics, and session/conversation capture.
- `server/internal/service/support_inbox.go` — manual company editing, direct-company association consumers, and company-repository wiring.
- `server/internal/service/support_inbox_visitor.go` — current company/options/status in the existing visitor-context response.
- `server/internal/handler/support_inbox.go` and `server/internal/router/router.go` — support-edit company endpoint.
- `server/cmd/api/main.go` — inject the CRM company repository into the support service.

### SDK

- `packages/sdk-js/src/core/client.ts` — merge and synchronize persisted active company for `id` and `group`.
- `packages/sdk-js/src/core/widget.ts` — include persisted company during automatic widget session upgrades.
- `packages/sdk-js/test/unit/core/client.test.ts` and `packages/sdk-js/test/unit/core/widget.test.ts` — identity ordering, account switches, and boot persistence coverage.

### Frontend

- `frontend/src/lib/pm-types/visitor.ts` and `frontend/src/lib/pm-types/support.ts` — typed company context and update payloads.
- `frontend/src/lib/services/supportService.ts` and `frontend/src/hooks/queries/useSupport.ts` — update mutation and visitor-context refresh.
- `frontend/src/hooks/useRealtimeSync.ts` — refresh visitor/company sidebar data after support-conversation and CRM-company events.
- `frontend/src/components/support/SidebarVisitorContext.tsx` — consolidated Contact Details and separately renderable Other Conversations.
- `frontend/src/components/support/SidebarCompanyDetails.tsx` — live company/subscription presentation and selector.
- `frontend/src/components/support/ConversationDetailSidebar.tsx` — required section ordering and direct-company de-duplication.
- `frontend/src/components/support/SidebarAssociations.tsx` — omit the canonical conversation company from generic CRM associations.
- `frontend/src/components/support/__tests__/SidebarVisitorContext.test.tsx`, `SidebarCompanyDetails.test.tsx`, and relevant sidebar tests — rendering, state, permission, and ordering coverage.

## Task 1: Add the direct company schema and model contract

**Files:**
- Modify: `server/internal/model/support_inbox.go`
- Create: `server/internal/dbmigrate/sql/202608050001_support_conversation_company_context.sql`
- Test: `server/internal/dbmigrate/migrator_test.go` or the nearest existing migration integration test

- [ ] **Step 1: Write the failing schema/model tests**

Add assertions that both support models expose nullable `CRMCompanyID` fields and that the migration is registered, idempotent, and includes the safe historical backfill rule. If the migration test harness supports PostgreSQL integration, seed contacts with zero, one, and two company associations and assert only the one-company conversation is linked.

- [ ] **Step 2: Run the focused tests and confirm the expected failure**

Run: `cd server && go test ./internal/dbmigrate ./internal/model`

Expected: FAIL because the model fields and migration do not exist.

- [ ] **Step 3: Add nullable model fields and the idempotent migration**

Add `CRMCompanyID *string` with UUID/index metadata to `SupportConversation` and `SupportWidgetSession`. The SQL migration must:

```sql
ALTER TABLE support_conversations
  ADD COLUMN IF NOT EXISTS crm_company_id uuid;
ALTER TABLE support_widget_sessions
  ADD COLUMN IF NOT EXISTS crm_company_id uuid;

CREATE INDEX IF NOT EXISTS idx_support_conversations_crm_company_id
  ON support_conversations (crm_company_id);
CREATE INDEX IF NOT EXISTS idx_support_widget_sessions_crm_company_id
  ON support_widget_sessions (crm_company_id);
```

Add idempotent foreign keys to `crm_companies(id)` with `ON DELETE SET NULL`. Backfill only rows where `support_conversations.crm_company_id IS NULL`, and only from contacts whose company-association count is exactly one. This predicate is mandatory so rerunning the migration never overwrites an explicit conversation company. Do not infer a company for existing widget sessions.

- [ ] **Step 4: Validate migrations and focused tests**

Run: `cd server && go run ./cmd/migrate validate && go test ./internal/dbmigrate ./internal/model`

Expected: PASS.

- [ ] **Step 5: Commit the schema slice**

```bash
git add server/internal/model/support_inbox.go server/internal/dbmigrate/sql/202608050001_support_conversation_company_context.sql server/internal/dbmigrate
git commit -m "feat(support): add conversation company context schema"
```

## Task 2: Make widget company resolution authoritative and concurrency-safe

**Files:**
- Modify: `server/internal/service/support_inbox.go`
- Modify: `server/internal/service/support_inbox_widget.go`
- Modify: `server/internal/repository/crm_company.go`
- Test: `server/internal/service/support_inbox_company_identity_test.go`
- Test: `server/internal/service/support_inbox_widget_test.go`

- [ ] **Step 1: Add failing identity-resolution tests**

Cover these cases:

- an external `company.id` matches only workspace-scoped `external_id` and never falls through to domain/name;
- a missing external ID creates a distinct company even when domain/name matches another externally identified company;
- domain fallback is used only without external ID, and name fallback only without ID/domain;
- unknown and nested custom fields preserve JSON scalar types;
- omitted custom fields remain unchanged while an explicit `null` removes the field;
- two concurrent resolves for the same workspace/external ID return one company identity.

- [ ] **Step 2: Run the focused tests and confirm the expected failures**

Run: `cd server && go test ./internal/service -run 'Test.*(CompanyIdentity|WidgetCompany)' -count=1`

Expected: FAIL on fallback, null-removal, and concurrency behavior.

- [ ] **Step 3: Refactor matching and custom-property merging**

Change `matchOrCreateCRMCompanyIdentityTx` so a supplied external ID is authoritative. Use a transaction-scoped PostgreSQL advisory lock derived from workspace ID plus normalized external ID before the lookup/create path, with a deterministic non-Postgres test fallback. Re-check after acquiring the lock. Do not add a global external-ID unique constraint; identity uniqueness is workspace-scoped.

Change `mergeCRMCustomProperties` so omitted fields are untouched and incoming `nil` deletes the corresponding stored key.

- [ ] **Step 4: Replace forced-primary behavior with membership semantics**

Split `ensurePrimaryContactCompanyAssociationTx` into an idempotent membership helper that marks the first company membership primary but never demotes an existing primary when the active widget company changes. Add tests for first and subsequent memberships.

- [ ] **Step 5: Re-run the focused tests**

Run: `cd server && go test ./internal/service -run 'Test.*(CompanyIdentity|WidgetCompany|ContactCompanyAssociation)' -count=1`

Expected: PASS.

- [ ] **Step 6: Commit the identity slice**

```bash
git add server/internal/service/support_inbox.go server/internal/service/support_inbox_widget.go server/internal/repository/crm_company.go server/internal/service/*company*test.go server/internal/service/support_inbox_widget_test.go
git commit -m "fix(support): preserve active company membership semantics"
```

## Task 3: Capture mutable session company and stable conversation company

**Files:**
- Modify: `server/internal/repository/support_inbox.go`
- Modify: `server/internal/service/support_inbox_widget.go`
- Test: `server/internal/repository/support_inbox_test.go`
- Test: `server/internal/service/support_inbox_widget_test.go`

- [ ] **Step 1: Write failing repository and widget-flow tests**

Test that:

- an identity/group update sets the latest active session company;
- a new conversation copies the session company;
- a late identity update fills the current session conversation only when its company is null;
- switching the session to a second company does not overwrite the existing conversation;
- the next conversation uses the second company;
- anonymous-ID HTTP fallback treats only non-revoked, unexpired sessions as active, updates those sessions, and updates only conversations referenced by those returned active sessions—never unrelated historical conversations.

- [ ] **Step 2: Run the focused tests and confirm failure**

Run: `cd server && go test ./internal/repository ./internal/service -run 'Test.*(SessionCompany|ConversationCompany|WidgetCreateConversation|UpgradeWidgetSession)' -count=1`

Expected: FAIL because company persistence helpers are absent.

- [ ] **Step 3: Add targeted repository mutations**

Add helpers with explicit semantics, for example:

```go
UpdateSessionCompany(ctx context.Context, sessionID string, companyID *string) error
SetConversationCompanyIfUnset(ctx context.Context, conversationID, companyID string) (bool, error)
UpdateActiveSessionsCompanyByAnonymousID(ctx context.Context, workspaceID, anonymousID string, companyID *string) ([]SupportWidgetSession, error)
```

`UpdateActiveSessionsCompanyByAnonymousID` must filter `revoked_at IS NULL` and `expires_at > now`, and return only the rows it actually considers active. Use those returned session conversation IDs as the complete update scope. Keep the existing broad contact identity updates unchanged; do not add company updates to all anonymous-ID conversations.

- [ ] **Step 4: Wire capture into identity and conversation creation**

On successful company resolution, update the current session and conditionally fill its current conversation. When `WidgetCreateConversation` or the implicit create path in `WidgetCreateMessage` creates a conversation, copy `session.CRMCompanyID` into it.

- [ ] **Step 5: Re-run focused tests**

Run: `cd server && go test ./internal/repository ./internal/service -run 'Test.*(SessionCompany|ConversationCompany|WidgetCreateConversation|UpgradeWidgetSession)' -count=1`

Expected: PASS.

- [ ] **Step 6: Commit the capture slice**

```bash
git add server/internal/repository/support_inbox.go server/internal/repository/support_inbox_test.go server/internal/service/support_inbox_widget.go server/internal/service/support_inbox_widget_test.go
git commit -m "feat(support): capture widget company on conversations"
```

## Task 4: Add manual conversation-company editing and association parity

**Files:**
- Modify: `server/internal/model/support_inbox.go`
- Modify: `server/internal/repository/support_inbox.go`
- Modify: `server/internal/service/support_inbox.go`
- Modify: `server/internal/handler/support_inbox.go`
- Modify: `server/internal/router/router.go`
- Modify: `server/internal/service/support_inbox_association_test.go`
- Test: `server/internal/handler/support_inbox_test.go`

- [ ] **Step 1: Add failing service/handler tests**

Cover workspace validation, set, clear, support-edit authorization, activity creation, realtime event publication, contact-company membership creation, primary membership preservation, and rejection of a company from another workspace.

Also assert that direct conversation company is included when copying conversation associations to tasks and is not duplicated if the same company already appears in generic associations.

- [ ] **Step 2: Run the focused tests and confirm failure**

Run: `cd server && go test ./internal/service ./internal/handler -run 'Test.*(UpdateConversationCRMCompany|CopyConversationAssociations)' -count=1`

Expected: FAIL because the endpoint and direct-company consumer do not exist.

- [ ] **Step 3: Implement the service operation and repository wiring**

Add a CRM company repository dependency without widening every existing constructor call unnecessarily; prefer a narrow setter wired from `server/cmd/api/main.go` if that matches current support-service conventions. Validate workspace ownership explicitly before updating. If the conversation has a CRM contact, ensure ordinary membership exists, but do not change the primary association.

- [ ] **Step 4: Add the endpoint**

Implement:

```text
PUT /api/support/inbox/conversations/{id}/crm-company
{ "crm_company_id": "uuid-or-null" }
```

Register it beside `crm-contact` under `PermSupportEdit`. Record the actor activity and emit the same support-conversation update event used by contact edits.

- [ ] **Step 5: Add direct-company association parity**

Update task-copying and support-context consumers to include the direct company exactly once. Preserve generic association behavior for all other CRM records.

- [ ] **Step 6: Re-run focused tests and commit**

Run: `cd server && go test ./internal/service ./internal/handler -run 'Test.*(UpdateConversationCRMCompany|CopyConversationAssociations)' -count=1`

Expected: PASS.

```bash
git add server/cmd/api/main.go server/internal/model/support_inbox.go server/internal/repository/support_inbox.go server/internal/service/support_inbox.go server/internal/handler/support_inbox.go server/internal/router/router.go server/internal/service/support_inbox_association_test.go server/internal/handler/support_inbox_test.go
git commit -m "feat(support): allow editing conversation company"
```

## Task 5: Extend visitor context with live company data

**Files:**
- Modify: `server/internal/model/support_inbox.go`
- Modify: `server/internal/service/support_inbox.go`
- Modify: `server/internal/service/support_inbox_visitor.go`
- Modify: `server/cmd/api/main.go`
- Test: `server/internal/service/support_inbox_visitor_test.go`

- [ ] **Step 1: Write failing visitor-context tests**

Assert:

- company loads strictly from `conversation.crm_company_id`;
- subscription/custom fields are returned with number/boolean/string/null scalar types intact;
- contact custom properties also retain scalar types;
- company options come from contact-company memberships and include the selected company;
- no direct company returns `company_context_status: "unlinked"`, without guessing among memberships;
- a company repository failure returns the rest of visitor context with `company_context_status: "error"`;
- error logs include workspace/conversation/entity IDs but not attribute values.

- [ ] **Step 2: Run the focused tests and confirm failure**

Run: `cd server && go test ./internal/service -run 'Test.*VisitorContext' -count=1`

Expected: FAIL because the response lacks company context and typed custom properties.

- [ ] **Step 3: Add typed visitor DTOs and partial-failure behavior**

Add `VisitorCompanyData`, `VisitorCompanyOption`, and `CompanyContextStatus` contracts. Change visitor contact custom properties from `map[string]string` to `map[string]any`. Load the selected company by direct ID with workspace validation, collect membership options, and keep optional failures non-fatal.

- [ ] **Step 4: Re-run tests and commit**

Run: `cd server && go test ./internal/service -run 'Test.*VisitorContext' -count=1`

Expected: PASS.

```bash
git add server/internal/model/support_inbox.go server/internal/service/support_inbox.go server/internal/service/support_inbox_visitor.go server/cmd/api/main.go server/internal/service/support_inbox_visitor_test.go
git commit -m "feat(support): expose live company visitor context"
```

## Task 6: Synchronize persisted SDK group identity

**Files:**
- Modify: `packages/sdk-js/src/core/client.ts`
- Modify: `packages/sdk-js/src/core/widget.ts`
- Modify: `packages/sdk-js/test/unit/core/client.test.ts`
- Modify: `packages/sdk-js/test/unit/core/widget.test.ts`

- [ ] **Step 1: Add failing client tests**

Cover:

- `group(company)` persists company and synchronizes with an identified email;
- `group` before `id` defers backend sync, then `id` sends the persisted company;
- inline `id.company` wins and becomes the persisted active group;
- `lead` does not inherit an unrelated persisted group;
- `doNotSendEvent` suppresses analytics only, not identity sync;
- `reset` clears both identity and company.

- [ ] **Step 2: Add the failing widget boot test**

Assert automatic session upgrade includes the persisted active company after page refresh/rejoin.

- [ ] **Step 3: Run the focused tests and confirm failure**

Run: `pnpm --dir packages/sdk-js test -- test/unit/core/client.test.ts test/unit/core/widget.test.ts`

Expected: FAIL on group synchronization and boot company propagation.

- [ ] **Step 4: Implement identity merging and boot propagation**

Centralize the effective identity payload rule:

```ts
const activeCompany = inlineCompany ?? persistedCompany;
```

Use it for identified `id` synchronization, persist inline companies, and let `group` call the existing identity backend path only when a stored user email is available. Keep lead behavior explicit. Extend `WidgetUser`/boot data just enough to carry the persisted company into `sendSessionUpgrade`.

- [ ] **Step 5: Re-run SDK tests/build and commit**

Run: `pnpm --dir packages/sdk-js test -- test/unit/core/client.test.ts test/unit/core/widget.test.ts`

Run: `pnpm --dir packages/sdk-js build`

Expected: PASS.

```bash
git add packages/sdk-js/src/core/client.ts packages/sdk-js/src/core/widget.ts packages/sdk-js/test/unit/core/client.test.ts packages/sdk-js/test/unit/core/widget.test.ts
git commit -m "feat(sdk): sync active company with support identity"
```

## Task 7: Add frontend company contracts and mutation

**Files:**
- Modify: `frontend/src/lib/pm-types/visitor.ts`
- Modify: `frontend/src/lib/pm-types/support.ts`
- Modify: `frontend/src/lib/services/supportService.ts`
- Modify: `frontend/src/hooks/queries/useSupport.ts`
- Modify: `frontend/src/hooks/useRealtimeSync.ts`
- Create: `frontend/src/lib/services/__tests__/supportService.test.ts`
- Create: `frontend/src/hooks/queries/__tests__/useSupportCompany.test.tsx`
- Modify: `frontend/src/hooks/__tests__/useRealtimeSync.test.tsx`

- [ ] **Step 1: Add failing type/service/hook tests**

Assert the service sends `{ crm_company_id }` to the new route, and that a successful mutation invalidates the conversation visitor context plus relevant conversation/association queries. Add realtime assertions that `support_conversation` events invalidate that conversation's visitor-context and association queries, while `crm_company` events invalidate workspace visitor-context queries so live subscription changes appear without reopening the inbox.

- [ ] **Step 2: Run focused frontend tests and confirm failure**

Run: `pnpm --dir frontend test -- src/lib/services/__tests__/supportService.test.ts src/hooks/queries/__tests__/useSupportCompany.test.tsx src/hooks/__tests__/useRealtimeSync.test.tsx`

Expected: FAIL because the request contract, mutation, and targeted realtime invalidations do not exist.

- [ ] **Step 3: Implement typed contracts and mutation**

Model company custom properties as `Record<string, unknown>`, add the three-state status union, add the service adapter, and expose a TanStack mutation that refreshes the current sidebar data after set/clear. Extend realtime invalidation narrowly: use the event conversation ID for `support_conversation`, and invalidate workspace-scoped visitor-context queries for `crm_company` because a live company update may affect any open conversation linked to it.

- [ ] **Step 4: Re-run focused tests and commit**

Run: `pnpm --dir frontend test -- src/lib/services/__tests__/supportService.test.ts src/hooks/queries/__tests__/useSupportCompany.test.tsx src/hooks/__tests__/useRealtimeSync.test.tsx`

Expected: PASS.

```bash
git add frontend/src/lib/pm-types/visitor.ts frontend/src/lib/pm-types/support.ts frontend/src/lib/services/supportService.ts frontend/src/hooks/queries/useSupport.ts frontend/src/hooks/useRealtimeSync.ts frontend/src/lib/services/__tests__/supportService.test.ts frontend/src/hooks/queries/__tests__/useSupportCompany.test.tsx frontend/src/hooks/__tests__/useRealtimeSync.test.tsx
git commit -m "feat(support): add company context client contract"
```

## Task 8: Consolidate Contact Details and preserve sidebar order

**Files:**
- Modify: `frontend/src/components/support/SidebarVisitorContext.tsx`
- Modify: `frontend/src/components/support/ConversationDetailSidebar.tsx`
- Modify: `frontend/src/components/support/__tests__/SidebarVisitorContext.test.tsx`
- Modify/Test: the nearest `ConversationDetailSidebar` component test

- [ ] **Step 1: Write failing rendering and order tests**

Assert one open-by-default `Contact Details` section replaces `User details` plus the old contact section. CRM contact fields appear first, followed by a subtle `Current visit` label and the visit/device rows. Assert the sidebar order remains routing, tags, email recipients, contact details, company placeholder position, other conversations, tasks, CRM, docs.

- [ ] **Step 2: Run focused tests and confirm failure**

Run: `pnpm --dir frontend test -- src/components/support/__tests__/SidebarVisitorContext.test.tsx`

Expected: FAIL on the old two-section structure.

- [ ] **Step 3: Split the visitor component at section boundaries**

Export separately renderable contact-details and other-conversations components (or equivalent focused components) so `ConversationDetailSidebar` controls the exact product order. Retain current compact rows and omit duplicate name/email already present in the customer header.

- [ ] **Step 4: Re-run focused tests and commit**

Run: `pnpm --dir frontend test -- src/components/support/__tests__/SidebarVisitorContext.test.tsx`

Expected: PASS.

```bash
git add frontend/src/components/support/SidebarVisitorContext.tsx frontend/src/components/support/ConversationDetailSidebar.tsx frontend/src/components/support/__tests__/SidebarVisitorContext.test.tsx
git commit -m "refactor(support): consolidate contact sidebar details"
```

## Task 9: Build Company Details and correction flow

**Files:**
- Create: `frontend/src/components/support/SidebarCompanyDetails.tsx`
- Create: `frontend/src/components/support/__tests__/SidebarCompanyDetails.test.tsx`
- Modify: `frontend/src/components/support/ConversationDetailSidebar.tsx`
- Modify: `frontend/src/components/support/SidebarAssociations.tsx`
- Modify/Test: relevant sidebar association tests

- [ ] **Step 1: Write failing company-section tests**

Cover:

- linked company name/profile link and domain;
- ordered plan, subscription badge, support tier, seats, trial/renewal, MRR/ARR rows;
- localized currency/date, Yes/No booleans, humanized custom keys, and `Show N more`;
- omission of empty values plus `sdk_company_id` and `sdk_created_at`;
- relative updated time;
- multi-membership selector, unlinked `Select company`, no-membership `Link company`, clear action, loading skeleton, and explicit retry state;
- edit controls hidden without support-edit permission;
- selected direct company omitted from generic CRM associations.

- [ ] **Step 2: Run focused tests and confirm failure**

Run: `pnpm --dir frontend test -- src/components/support/__tests__/SidebarCompanyDetails.test.tsx`

Expected: FAIL because the component does not exist.

- [ ] **Step 3: Implement the compact company presentation**

Reuse `CollapsibleSection`, existing compact label/value rows, badges, popovers, and CRM search service. Keep the section open by default and place it immediately after Contact Details. Show membership options first; `Link another company` searches the workspace CRM and calls the new mutation. The component must use live visitor-context data and never store a sidebar snapshot.

- [ ] **Step 4: De-duplicate the canonical company**

Pass the direct company ID into `SidebarAssociations` and filter only the matching company association. Keep other companies and other CRM objects available in the generic section.

- [ ] **Step 5: Re-run focused tests/build and commit**

Run: `pnpm --dir frontend test -- src/components/support/__tests__/SidebarCompanyDetails.test.tsx src/components/support/__tests__/SidebarVisitorContext.test.tsx`

Run: `pnpm --dir frontend build`

Expected: PASS.

```bash
git add frontend/src/components/support/SidebarCompanyDetails.tsx frontend/src/components/support/__tests__/SidebarCompanyDetails.test.tsx frontend/src/components/support/ConversationDetailSidebar.tsx frontend/src/components/support/SidebarAssociations.tsx frontend/src/components/support/__tests__
git commit -m "feat(support): show live company details in sidebar"
```

## Task 10: End-to-end regression verification

**Files:**
- Modify only files required by failures attributable to this feature

- [ ] **Step 1: Run backend verification**

```bash
cd server
go run ./cmd/migrate validate
go test ./internal/repository ./internal/service ./internal/handler
go test ./...
```

Expected: PASS.

- [ ] **Step 2: Run SDK verification**

```bash
pnpm --dir packages/sdk-js test
pnpm --dir packages/sdk-js build
```

Expected: PASS.

- [ ] **Step 3: Run frontend verification**

```bash
pnpm --dir frontend test
pnpm --dir frontend build
```

Expected: PASS.

- [ ] **Step 4: Check the final diff**

Run: `git diff --check && git status --short`

Expected: no whitespace errors; only scoped feature files and pre-existing user files appear.

- [ ] **Step 5: Perform the behavior audit**

Manually verify with a test visitor/contact:

1. identify in company A and create a conversation;
2. switch the active widget group to company B;
3. confirm the existing conversation remains linked to A;
4. create a new conversation and confirm it links to B;
5. update company A's subscription in CRM and confirm the old conversation shows the new live value;
6. manually change/clear the conversation company with an editor account and confirm a read-only account cannot edit;
7. confirm the sidebar order and direct-company de-duplication.

- [ ] **Step 6: Commit any verification-only fixes**

```bash
git add <only-files-fixed-during-verification>
git commit -m "test(support): cover conversation company context"
```
