# Support Inbox Module — Refactoring Plan

**Date**: 2026-03-16
**Scope**: Frontend + Backend support inbox, real-time presence, widget WebSocket, drafts

---

## 1. Critical Fixes (Do First)

### 1.1 WebSocket Connection State Validation
**Problem**: Components call `wsSend` without checking if the connection is open. On first render, `wsSend` is `null` for ~100ms until `useRealtimeSync` sets it. Sends silently fail.

**Fix**:
- Expose `isConnected` boolean from `useWebSocket`
- Store it in `supportInboxStore` alongside `wsSend`
- Components that depend on WS should guard on `isConnected` or queue messages

**Files**: `useWebSocket.ts`, `supportInboxStore.ts`, `MessageThread.tsx`, `ReplyComposer.tsx`

### 1.2 Widget Handler Type Safety
**Problem**: All widget WS message data uses `map[string]any` with unchecked type assertions (`email, _ := msg.Data["email"].(string)`). Failed casts silently produce zero values.

**Fix**:
- Define typed structs for each widget message type (session:create, message:send, typing:start, etc.)
- Validate with `json.Unmarshal` into typed structs instead of map access
- Return explicit errors to the widget client on malformed messages

**Files**: `widget_handler.go`, `model/support_inbox.go`

### 1.3 Race Condition: Event Published Before DB Persist
**Problem**: `CreateConversationMessage` in the service publishes a WebSocket event before the message is confirmed persisted to DB. If the DB write fails after the event is sent, other clients see a phantom message.

**Fix**:
- Move `wsPublisher.Publish()` calls to after successful DB operations
- Or wrap in a transaction callback

**Files**: `service/support_inbox.go`

### 1.4 Conversation Display ID Atomicity
**Problem**: Advisory lock on workspace for display_id, but concurrent transactions can still race.

**Fix**:
- Use `SELECT COALESCE(MAX(display_id), 0) + 1 FROM support_conversations WHERE workspace_id = ? FOR UPDATE` inside a transaction
- Or use a PostgreSQL sequence per workspace

**Files**: `repository/support_inbox.go`

---

## 2. State Management Cleanup (High Priority)

### 2.1 Consolidate Zustand vs TanStack Query
**Problem**: Typing indicators, agent typing, viewing presence, and drafts live in Zustand. Conversation data lives in TanStack Query cache. No clear contract on which source owns what.

**Fix**:
- Keep in Zustand (correct): `typingIndicators`, `agentTyping`, `viewingAgents`, `drafts`, `wsSend`, UI state (navFilter, replyMode, etc.)
- Keep in TanStack Query (correct): conversations, messages, conversation detail
- Document the boundary clearly

**Files**: `supportInboxStore.ts`

### 2.2 Extract Presence State into Dedicated Store/Hook
**Problem**: `supportInboxStore.ts` has grown to manage UI state, presence, typing, drafts, and WS connection — too many concerns.

**Fix**:
- Extract `useSupportPresenceStore` — owns `typingIndicators`, `agentTyping`, `viewingAgents`, `wsSend`
- Keep `useSupportInboxStore` for UI-only state — `navFilter`, `replyMode`, `selectedConversationId`, `drafts`
- Or create a `useSupportPresence` hook that encapsulates the store

**Files**: `supportInboxStore.ts` → split into `supportInboxStore.ts` + `supportPresenceStore.ts`

### 2.3 Draft Persistence to localStorage
**Problem**: Drafts only live in Zustand memory — lost on page refresh.

**Fix**:
- Debounce-persist drafts to localStorage (reuse the existing `savePersisted` pattern)
- Load on init
- Clear on send

**Files**: `supportInboxStore.ts`

---

## 3. Error Handling & Resilience (High Priority)

### 3.1 Surface Mutation Errors to UI
**Problem**: Multiple mutation hooks (assign agent, update status, send message) swallow errors or only log to console.

**Fix**:
- Add `onError` callbacks to all `useMutation` hooks that show toast notifications via `sonner`
- Ensure `unwrap()` is called consistently on all API responses

**Files**: `useSupport.ts`, `MessageThread.tsx`, `ConversationDetailSidebar.tsx`

### 3.2 WebSocket Reconnection Robustness
**Problem**: Reconnection timer can fire after component unmount. No connection state feedback to user.

**Fix**:
- Add `isConnecting` / `isConnected` / `isDisconnected` state
- Show a subtle banner in the inbox when disconnected (like the widget's connection banner)
- Ensure cleanup cancels all pending timers

**Files**: `useWebSocket.ts`, `SupportInboxLayout.tsx`

### 3.3 Silent Error Swallowing
**Problem**: `catch(() => {})` used extensively for typing/viewing/presence calls.

**Fix**:
- At minimum, add `console.warn` in dev mode
- For production, consider a lightweight error counter/metric

**Files**: `ReplyComposer.tsx`, `MessageThread.tsx`, `useRealtimeSync.ts`

---

## 4. Performance Improvements (Medium Priority)

### 4.1 Virtualize Conversation List
**Problem**: All conversations rendered — no virtualization. Sluggish with 100+ conversations.

**Fix**:
- Use `@tanstack/react-virtual` (already a project dependency)
- Virtualize `ConversationList` scroll area

**Files**: `ConversationList.tsx`

### 4.2 Memoize MessageBubble
**Problem**: `MessageBubble` re-renders on any parent re-render (no `React.memo`).

**Fix**:
- Wrap `MessageBubble` in `React.memo`
- Memoize `groupedMessages` (already done with `useMemo` ✓)

**Files**: `MessageBubble.tsx`

### 4.3 Reduce ConversationRow Re-renders
**Problem**: Each `ConversationRow` subscribes to 4+ Zustand selectors. Any typing/viewing change re-renders ALL rows.

**Fix**:
- Wrap `ConversationRow` in `React.memo`
- Use Zustand `useShallow` for array/object selectors
- Consider a single selector that returns a composite state object per conversation

**Files**: `ConversationRow.tsx`

### 4.4 Batch Query Invalidations
**Problem**: `useRealtimeSync` fires multiple `invalidateQueries` calls for a single event (messages + conversations + conversation detail).

**Fix**:
- Use `queryClient.invalidateQueries` with broader key prefix to batch
- Or debounce invalidations for rapid-fire events

**Files**: `useRealtimeSync.ts`

---

## 5. Code Quality & Cleanup (Medium Priority)

### 5.1 Remove Dead/Legacy HTTP Presence Endpoints
**Problem**: HTTP POST endpoints for `/typing` and `/viewing` are still registered but no longer used (replaced by WebSocket).

**Fix**:
- Keep as fallback but add deprecation comments
- Or remove entirely if WS is the only path

**Files**: `router.go`, `handler/support_inbox.go`, `service/support_inbox.go`, `supportService.ts`

### 5.2 Extract Avatar Color Logic
**Problem**: `getAvatarColor` and `AVATAR_COLORS` defined inside `ConversationRow.tsx`. Used there and could be needed in `MessageBubble`, `MessageThread`, etc.

**Fix**:
- Move to `support/helpers.ts` or `lib/utils.ts`
- Reuse across all support components

**Files**: `ConversationRow.tsx` → `support/helpers.ts`

### 5.3 Consistent Presence Event Naming
**Problem**: Widget uses `typing:start`/`typing:stop`, agents use `support:typing:start`/`support:typing:stop`, hub broadcasts `typing_started`/`typing_stopped`. Three naming conventions.

**Fix**:
- Document the convention clearly:
  - Client → Server: `support:typing:start` (agent WS), `typing:start` (widget WS)
  - Server → Client: `typing_started` / `typing_stopped` (broadcast events)
- Add a constants file for event type strings

### 5.4 Remove Debug Logging Before Production
**Problem**: Multiple `console.debug` and `console.warn` calls added during development.

**Fix**:
- Wrap all in `import.meta.env.DEV` guards (some already are, some aren't)
- Or use a debug utility that auto-strips in production

**Files**: `useRealtimeSync.ts`, `ReplyComposer.tsx`, `MessageThread.tsx`, `widget.ts`

---

## 6. Test Coverage Plan

### 6.1 Server Tests (Go)

| Test | File | Priority |
|------|------|----------|
| Hub `shouldReceive` with viewing events | `hub_test.go` | High |
| Presence registry: SetViewing/ClearViewing/ClearAllForUser | `presence_test.go` (new) | High |
| Agent handler: parsing support:viewing:start/stop messages | `handler_test.go` (new) | High |
| Widget handler: typing:start with/without conversation ID | `widget_handler_test.go` (new) | Medium |
| Repository: List with last_message subquery | `support_inbox_test.go` (new) | Medium |
| Repository: message create bumps conversation updated_at | `support_inbox_test.go` (new) | Medium |
| Service: PublishTypingIndicator with content | `support_inbox_service_test.go` (new) | Low |

### 6.2 Frontend Tests (Vitest)

| Test | File | Priority |
|------|------|----------|
| `supportInboxStore` — setDraft, clearDraft, setTyping, setAgentTyping | `supportInboxStore.test.ts` (new) | High |
| `ConversationRow` — renders last_message, draft, note, typing states | `ConversationRow.test.tsx` (new) | High |
| `ReplyComposer` — draft save/load/clear on send | `ReplyComposer.test.tsx` (new) | High |
| `useRealtimeSync` — typing event routing (widget vs agent) | `useRealtimeSync.test.ts` (new) | Medium |
| `useWebSocket` — reconnection, send buffering | `useWebSocket.test.ts` (new) | Medium |
| `TypingIndicatorBar` — shows/hides based on store state | `MessageThread.test.tsx` (new) | Low |

### 6.3 Integration Tests

| Test | Priority |
|------|----------|
| Agent A types → Agent B sees typing bubble + preview | High |
| Agent A views conversation → Agent B sees avatar | High |
| Customer types in widget → Agent sees indicator in list + thread | High |
| Agent sends message → typing/draft cleared for both agents | Medium |
| WS disconnect → presence cleaned up server-side | Medium |
| Draft persists across conversation switches | Medium |

---

## 7. Implementation Order

```
Phase 1 (Stabilize — 1-2 days):
  ├── 1.1 WS connection state validation
  ├── 1.3 Fix event-before-persist race
  ├── 3.1 Surface mutation errors
  └── 6.1 Server tests for presence + hub

Phase 2 (Clean Up — 1-2 days):
  ├── 2.1 Document state ownership
  ├── 2.3 Draft localStorage persistence
  ├── 5.2 Extract avatar color logic
  ├── 5.4 Guard debug logging
  └── 6.2 Frontend store + component tests

Phase 3 (Performance — 1 day):
  ├── 4.1 Virtualize conversation list
  ├── 4.2 Memoize MessageBubble
  ├── 4.3 Reduce ConversationRow re-renders
  └── 4.4 Batch query invalidations

Phase 4 (Architecture — 2-3 days):
  ├── 1.2 Widget handler type safety
  ├── 2.2 Split presence into dedicated store
  ├── 5.1 Clean up legacy HTTP endpoints
  └── 6.3 Integration tests
```

---

## Files Changed in This Feature Branch

| Category | Files |
|----------|-------|
| **Server WebSocket** | `hub.go`, `handler.go`, `presence.go` (new), `widget_handler.go`, `publisher.go` |
| **Server HTTP** | `handler/support_inbox.go`, `handler/support_inbox_widget.go`, `router.go` |
| **Server Data** | `model/support_inbox.go`, `repository/support_inbox.go`, `service/support_inbox.go` |
| **Frontend Hooks** | `useWebSocket.ts`, `useRealtimeSync.ts`, `useSupport.ts` |
| **Frontend Components** | `ConversationRow.tsx`, `ConversationList.tsx`, `MessageThread.tsx`, `ReplyComposer.tsx`, `MessageBubble.tsx`, `SupportInboxLayout.tsx` |
| **Frontend State** | `supportInboxStore.ts`, `supportService.ts`, `pmTypes.ts` |
| **Widget** | `packages/sdk-js/src/core/widget.ts`, `packages/widget-core/src/components/ComposeBar.tsx`, `ConversationView.tsx`, `ChatWindow.tsx`, `index.ts` |
