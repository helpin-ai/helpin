# Support live chat, inbox, and AI messenger

**Status:** In Progress — Phase 2 Complete
**Version:** Draft v15
**Date:** 2026-03-11
**Module:** Support
**Product context:** Helpin is a full operating suite spanning PM, CRM, Docs, Notifications, Agents, and Support. This PRD defines support as a first-class suite surface, not a standalone chat product.

---

## Phase Progress Tracker

| Phase | Name | Status | Deliverables |
|-------|------|--------|-------------|
| 0 | Product & Naming Alignment | ✅ Complete | Conversation-first model approved, Chi/GORM canonical |
| 1 | Package Setup, SDK Migration & Widget-Core | ✅ Complete | 6 packages, 11 components, 131 tests |
| 2 | Data Modeling & Go APIs | ✅ Complete | Models, repos, services, handlers, migration, 23 tests |
| 3 | Inbox & Widget MVP | ⬜ Not Started | Dashboard inbox, widget integration, settings, CSAT, typing |
| 4 | Docs-Aware AI Support | ⬜ Not Started | AI response flow, citations, handoff, email transcript |
| 5 | Advanced Automation & Operations | ⬜ Not Started | SLA rules, routing, analytics, embeddings |

---

## Phase 0: Product & Naming Alignment ✅

- [x] Approve conversation-first support model
- [x] Approve story escalation as the primary workflow
- [x] Approve docs/external docs as AI knowledge sources
- [x] Approve Chi/GORM architecture as canonical

---

## Phase 1: Package Setup, SDK Migration & Widget-Core ✅

### 1a. Monorepo & Package Setup
- [x] Create `packages/` directory with pnpm workspace + Turborepo at repo root
- [x] Add `pnpm-workspace.yaml` and `turbo.json`
- [x] Create `tsconfig.base.json` for shared TypeScript config

### 1b. SDK Migration (from `/root/helpin-convex-main`)
- [x] `@helpin-ai/shared` — Shared types (Conversation, Message, WidgetConfig)
- [x] `@helpin-ai/sdk-js` — JavaScript SDK with analytics + widget support
- [x] `@helpin-ai/react` — React hooks and provider
- [x] `@helpin-ai/nextjs` — Next.js SSR-safe hooks
- [x] `@helpin-ai/widget-core` — Chat widget components (Preact)
- [x] `@helpin-ai/widget-embed` — IIFE bundle for embeddable widget

### 1c. Widget-Core Components (11)
- [x] `ChatWindow` — Main container with open/close animation
- [x] `WidgetLauncher` — Floating button with unread badge
- [x] `WidgetHeader` — Header with title, logo, close button
- [x] `MessageList` — Scrollable message thread with date separators, smart auto-scroll
- [x] `MessageBubble` — Individual message with role styling, XSS-safe rendering
- [x] `ComposeBar` — Text input with send button, Enter-to-send
- [x] `PreChatForm` — Email/name capture before chat
- [x] `QuickReplies` — Quick reply buttons
- [x] `TypingIndicator` — Animated typing dots
- [x] `CsatRating` — 5-point emoji rating with feedback textarea
- [x] `StreamingText` — Animated AI response text with cursor

### 1d. SDK Widget API
- [x] `boot()` — Initialize widget with config
- [x] `show()` / `hide()` / `toggle()` — Widget visibility
- [x] `showMessages()` / `showConversation()` / `showArticle()` — Navigation
- [x] `onShow` / `onHide` / `onUnreadCountChange` / `onUserEmailSupplied` — Callbacks
- [x] `getVisitorId()` — Visitor identification
- [x] WebSocket connection with exponential backoff + jitter + max retries
- [x] Event listener cleanup on `shutdown()` (memory leak prevention)

### 1e. CSS & Accessibility
- [x] Tailwind-aligned design tokens (colors, spacing, radius, shadows)
- [x] Responsive mobile layout
- [x] Shadow DOM isolation ready
- [x] ARIA labels on all interactive elements (buttons, inputs, lists, radio groups)

### 1f. Testing
- [x] 82 unit tests for sdk-js (Vitest)
- [x] 49 unit tests for widget-core (Vitest + @testing-library/preact)
- [x] E2E test infrastructure with mock API server
- [x] All 131 tests passing

### Bundle Sizes (gzipped)
| Package | Size |
|---------|------|
| `@helpin-ai/widget-core` | 4.6 KB |
| `@helpin-ai/sdk-js` | 10.4 KB |
| `@helpin-ai/widget-embed` | 0.25 KB |

---

## Phase 2: Data Modeling & Go APIs ✅

### 2a. Data Model
- [x] `SupportConversation` model — renamed from SupportTicket (with backward compat alias)
- [x] `SupportMessage` — added `ConversationID`, `MessageType`, `Metadata` fields
- [x] `SupportCannedResponse` — new table for saved reply templates
- [x] Migration `038_support_conversations.sql` — new tables/columns, backfill `conversation_id`

### 2b. Repository Layer
- [x] `SupportConversationRepository` — Create (auto display_id), GetByID, List (pagination, filters), Update
- [x] `SupportMessageRepository` — Create, ListByTicket (supports both conversation_id and legacy ticket_id)
- [x] `SupportCannedResponseRepository` — Create, List, GetByID, Search (ranked), Update, Delete

### 2c. Service Layer
- [x] Canned response CRUD methods
- [x] Typing indicator broadcast functionality
- [x] `truncate()` utility with UTF-8/emoji support
- [x] `validConversationStatuses` validation map
- [x] `generateSecureToken()` for secure token generation

### 2d. Handler Layer
- [x] Canned response CRUD endpoints (list, create, update, delete)
- [x] Typing indicator endpoint for real-time collaboration

### 2e. API Wiring
- [x] `server/cmd/api/main.go` — Wired cannedResponseRepo
- [x] `server/internal/router/router.go` — Registered new routes

### 2f. Testing
- [x] `server/internal/service/support_test.go` — 23 test cases passing
  - `TestSupportConversationRepository` (6 subtests)
  - `TestSupportMessageRepository` (4 subtests)
  - `TestWidgetInstallationRepository` (5 subtests)
  - `TestWidgetSessionRepository` (3 subtests)
  - `TestSupportCannedResponseRepository` (5 subtests)
  - `TestTruncate` (6 subtests)
  - `TestValidConversationStatuses`, `TestGenerateSecureToken` (2 subtests)
- [x] SQLite test compatibility (ILIKE → LIKE)
- [x] Test data isolation with unique workspace IDs

---

## Phase 3: Inbox & Widget MVP ⬜

> **Goal:** End-to-end live chat — visitor starts conversation in widget, agent responds in inbox dashboard, real-time sync between both.

### 3a. Dashboard Inbox Page
- [ ] Two-pane layout (conversation list + thread) — see Section 5.1
- [ ] Slide-out context drawer (CRM contact, associations, story link, metadata) — Section 14.11
- [ ] Assignment/status flows with auto-set `resolved_at`/`closed_at`
- [ ] Internal notes in conversation thread
- [ ] Frontend terminology migration (ticket → conversation throughout) — Section 14.9

### 3b. Widget Integration
- [ ] Wire pre-chat form to workspace settings (email/name enforcement)
- [ ] Widget creates `support_conversation` on first message (not ticket)
- [ ] "Talk to a Human" button triggers handoff
- [ ] Branding config from `SupportInboxInstallation.Settings`
- [ ] WebSocket connection in widget with session token auth

### 3c. Canned Responses (Frontend)
- [ ] `/shortcode` composer trigger with ranked search dropdown — Section 6.7.1
- [ ] Canned response management page at `/w/:slug/settings/chat-responses`
- [ ] Variable interpolation on send: `{{contact.name}}`, `{{contact.email}}`, `{{agent.name}}`

### 3d. Typing Indicators
- [ ] Agent ↔ customer bidirectional via WebSocket — Section 6.7.3
- [ ] Debounced typing detection (300ms) in reply composer
- [ ] `typing_on`/`typing_off` in `useRealtimeSync.ts`
- [ ] Widget shows `TypingIndicator` on agent typing, auto-dismiss after 30s

### 3e. CSAT Survey
- [ ] Auto-send `csat_survey` message on conversation resolution — Section 6.7.2
- [ ] `PUT /api/widget/support/messages/{id}` for CSAT submission
- [ ] `csat_enabled` field in `SupportInboxSettings` (default: true)
- [ ] CSAT toggle in `ChatAITab.tsx` settings

### 3f. Unread Message Overlay
- [ ] Floating preview card near launcher when widget is closed — Section 6.7.5
- [ ] Sender name + truncated message (120 chars), dismiss button
- [ ] Auto-hide after 15 seconds
- [ ] Track unread count in `sessionStorage`, fire `onUnreadCountChange` callback

### 3g. Chat Settings (Backend)
- [ ] Define `SupportInboxSettings` struct with 20+ fields — Section 6.5
- [ ] Settings validation: threshold range, handoff team exists, hex color, business hours
- [ ] `GET /api/support/inbox/installations` — read full settings
- [ ] `PATCH /api/support/inbox/installations` — partial update
- [ ] `POST /api/support/inbox/installations/regenerate-key` — regenerate widget key
- [ ] `GET /api/widget/support/config` — public-facing settings subset
- [ ] `is_online` computation from business hours + timezone

### 3h. Chat Settings (Frontend — 3 Pages)
- [ ] `ChatGeneralTab.tsx` — Widget Installation + Identity Capture + CRM Integration
- [ ] `ChatAITab.tsx` — AI Auto-Reply + Handoff Routing + Business Hours + CSAT toggle
- [ ] `ChatAppearanceTab.tsx` — Branding + Launcher + Live Widget Preview
- [ ] Register `chat-general`, `chat-ai`, `chat-appearance` in `SETTINGS_SECTIONS`
- [ ] Widget key display with copy-to-clipboard + embed code snippet
- [ ] TanStack Query hooks: `useChatSettings`, `useUpdateChatSettings`, `useRegenerateWidgetKey`

### 3i. Realtime Sync
- [ ] Add `support_conversation` + `support_conversation_message` to `useRealtimeSync.ts`
- [ ] Invalidate query keys on support events
- [ ] Widget WebSocket auth path (session token alongside JWT)
- [ ] Scope widget WS clients to their active conversation only

### 3j. RBAC Permissions
- [ ] Add `PermSupportRead`, `PermSupportEdit`, `PermSupportAdmin` — Section 14.12
- [ ] Update role-permission matrix
- [ ] Update router to use support permissions instead of PM permissions

### 3k. Remaining Backend Cleanup
- [ ] Rename repository/service/handler constructors per Section 14.13
- [ ] Update `CRMObjectSupportTicket` → `CRMObjectSupportConversation`
- [ ] Update agent run integration (ticket → conversation) — Section 14.14
- [ ] Update docs link object type — Section 14.15
- [ ] Update WebSocket event entity names (ticket → conversation) — Section 14.8

### Phase 3 Exit Criteria
- A website visitor can start a conversation via the embedded widget
- Pre-chat identity capture creates/matches a CRM contact
- An agent can respond in the inbox and see updates in real time
- Context drawer shows CRM contact info and allows story creation
- Conversation can be escalated to a PM story
- Agents can use `/shortcode` to insert canned responses
- Typing indicators show bidirectionally in real time
- CSAT survey appears in widget when conversation is resolved
- Unread message overlay shows near launcher when widget is closed
- Workspace admins can configure chat settings
- Widget respects workspace settings (identity capture, branding, AI toggle, business hours)

---

## Phase 4: Docs-Aware AI Support ⬜

> **Goal:** AI auto-replies grounded in docs, with source citations and human handoff.

### 4a. AI Response Flow
- [ ] Wire `DocsSearchService.Search()` + `PublicSearch()` into new `SupportAIService`
- [ ] Customer message → docs retrieval → LLM provider → AI message with citations
- [ ] `sender_type: ai` and `message_type: ai_answer` with metadata JSONB
- [ ] Handoff detection: explicit request, low confidence, billing/account keywords
- [ ] Temporal workflow for async AI response processing

### 4b. Widget AI Experience
- [ ] `StreamingText` component for real-time AI response display (component built in Phase 1)
- [ ] AI source citations displayed inline in widget
- [ ] GIN index on `docs_contents.content_text` for production-scale search

### 4c. Email Transcript
- [ ] `POST /api/widget/support/conversations/{id}/transcript` — Section 6.7.4
- [ ] HTML email template with workspace name, display ID, messages, timestamps, citations
- [ ] Send via Postmark email client
- [ ] Widget "Email Transcript" button on resolved conversations

### 4d. CSAT Reporting
- [ ] Aggregate ratings by agent and by time period
- [ ] Feedback review dashboard

### Phase 4 Exit Criteria
- AI answers are grounded in docs (internal + external help center)
- Source citations visible in both widget and dashboard
- Handoff to human agent works on low confidence or explicit request
- AI response latency < 5s (non-streaming), streaming starts < 1s
- Visitors can email themselves a conversation transcript
- CSAT data visible in reporting

---

## Phase 5: Advanced Automation & Operations ⬜

> **Goal:** Production-grade automation, analytics, and multi-channel support.

- [ ] SLA rules and enforcement
- [ ] Advanced routing (skill-based, round-robin, load-balanced)
- [ ] Advanced RAG (chunking, embeddings, hybrid ranking)
- [ ] Analytics/reporting (response times, resolution rates, agent performance)
- [ ] Analytics pixel data feeding CRM signal detection
- [ ] Additional channels (email, social, etc.)

### Phase 5 Exit Criteria
- SLA breaches trigger alerts and escalation
- Conversations auto-route to best-fit agent
- AI accuracy improves with embedding-based retrieval
- Managers can view support performance dashboards

---

## 1. Overview

Build an Intercom Finn-like support inbox for Helpin where customers can start conversations from an embeddable website widget, receive AI-first responses grounded in Helpin Docs and externally published help center content, and seamlessly hand off to human agents inside the Helpin dashboard.

The support product should be modeled around a single **inbox conversation** object. The system should not present or evolve two separate domain concepts, `chat` and `ticket`, with a later conversion from one to the other. In Helpin, the correct coupling is:

- **Support conversation** for customer communication
- **CRM contact/company/deal** for relationship context
- **Docs and external docs** for AI grounding
- **PM story** for operational or engineering follow-through

When deeper product, engineering, or ops work is required, the conversation should escalate into a **PM story**, not into a second support artifact.

### Goals

- AI-first support for web widget conversations
- Shared inbox for human agents inside the Helpin dashboard
- Clear conversation-to-CRM identity mapping
- Clear conversation-to-story escalation workflow
- Strong reuse of Helpin Docs and external help center docs for AI support
- Reuse of the frontend/widget package work from `/root/helpin-convex-main`
- Alignment with the current Helpin stack: React 19, Vite, TanStack Router/Query, Go, Chi, GORM, WebSocket publisher

### Non-Goals for Phase 1

- Voice/video/chat calls
- WhatsApp, Slack, SMS, or other new external support channels
- Campaigns or proactive outbound support messages
- Advanced SLA engine as a primary launch dependency
- Deep vector infrastructure as a Day 1 blocker
- Enterprise routing/workforce management
- Multi-customer group threads

### Product Decisions

1. The canonical support object is **conversation**, not separate `chat` and `ticket` models.
2. The dashboard surface is **Support Inbox**, not a generic ticket table.
3. Escalation is **conversation -> PM story**, not chat -> ticket conversion.
4. External help center docs are not only for public docs delivery; they are also part of Helpin's retrieval strategy for AI support.
5. The implementation must follow **Helpin's current architecture**, not the reference repo's Fiber backend patterns.

---

## 2. SDK Migration Plan

The reference repo `/root/helpin-convex-main` contains useful package and widget work that should be reused where it accelerates delivery. It should be treated as a **frontend and packaging reference**, not as the backend implementation baseline.

### 2.1 File Inventory: `/root/helpin-convex-main`

| Source Path | Current Value | Use In Helpin |
|---|---|---|
| `packages/shared/src/types/*.ts` | Shared types: `Conversation`, `Message` (with `AiSource`, `Attachment`), `WidgetConfig`, `Workspace` (with `WorkspaceBranding`) | Reuse as a starting point, then align field names to Helpin's conversation-first vocabulary and Go model fields |
| `packages/widget-core/src/types.ts` | `WidgetAdapter` interface contract with methods: `getMessages()`, `sendMessage()`, `startConversation()`, `markAsRead()`, `sendTypingIndicator()`, `getConfig()` | Reuse adapter interface; implement against Helpin REST + WebSocket APIs |
| `packages/widget-core/src/components/*.tsx` | 11 stub components (all return `null`): `ChatWindow`, `MessageList`, `MessageBubble`, `ComposeBar`, `WidgetHeader`, `WidgetLauncher`, `PreChatForm`, `QuickReplies`, `TypingIndicator`, `CsatRating`, `StreamingText` | Reuse component boundaries and naming; implement all components for real |
| `packages/widget-core/src/styles/widget.css` | CSS variable definitions with `helpin-` namespace prefix; all class rules empty | Reuse variable structure and namespace; implement actual styles |
| `packages/helpin-js/src/` | Production-grade client-side analytics SDK: pageview tracking, user identification, UTM capture, scroll depth, retry queue with offline persistence, multiple transports (XHR, Fetch, Beacon, HTTPS) | Reuse as-is for analytics tracking; extend with chat widget initialization so installing the SDK auto-mounts the support widget |
| `packages/helpin-react/src/` | React Context + hooks for analytics SDK (`HelpinProvider`, `useHelpin`, `usePageView`) | Reuse and extend with chat widget hooks (`useSupportWidget`, `useConversation`) |
| `packages/helpin-nextjs/src/` | Next.js SSR-safe wrapper: `HelpinProvider` with null-safe client, `useHelpin` with no-op fallback, `middlewareEnv.ts` for Edge Middleware (anonymous ID cookies, IP extraction, server-side event context) | Reuse for Next.js customers who embed the Helpin widget; extend with SSR-safe chat widget hooks |
| `apps/widget/src/main.tsx` | Widget bootstrap: creates `<div id="helpin-chatbox">` container, configured as Vite IIFE build (`pixel.js`) using Preact | Reuse build approach; implement full widget mount with adapter wiring |
| `apps/widget/src/adapters/WebSocketWidgetAdapter.ts` | Implements `WidgetAdapter` interface; all methods are no-op stubs | Reimplement against Helpin REST + WebSocket APIs |
| `packages/ui/` | BROKEN package — exports from non-existent file (`src/index.ts` references missing components). Build fails. | **Do NOT copy.** Helpin uses shadcn/ui directly; this package provides no value. |
| `backend/internal/realtime/protocol.go` | Realtime protocol ideas | Reference only |
| `backend/internal/handler/*.go` | Fiber handlers | Reference only; do not port directly |
| `backend/internal/rag/*.go` | RAG concepts | Reference only for later phases |

### 2.2 Migration Intent

The package work should create a dual-purpose SDK layer for the Helpin suite: **analytics tracking** and **support chat widget**. Installing the Helpin SDK on a customer's site should provide both capabilities from a single script tag or npm install.

**Package responsibilities:**

- `@helpin-ai/shared` — Shared TypeScript types for conversations, messages, widget config, AI sources, branding. Used by all other packages.
- `@helpin-ai/widget-core` — Framework-agnostic chat widget component library with a pluggable `WidgetAdapter` interface. Contains headless components (ChatWindow, PreChatForm, MessageList, etc.) and CSS with `helpin-` namespace.
- `@helpin-ai/sdk-js` — Browser SDK that combines analytics tracking (pageviews, events, user identification, UTM capture, scroll depth) with chat widget initialization. When initialized with a `widget_key`, the SDK auto-mounts the support widget. Analytics data feeds CRM signal detection.
- `@helpin-ai/react` — React Context + hooks for both analytics (`useHelpin`, `usePageView`) and chat (`useSupportWidget`, `useConversation`).
- `@helpin-ai/nextjs` — Next.js SSR-safe wrapper with Edge Middleware helpers. Provides the same analytics + chat hooks with null-safe fallbacks for server rendering.
- `@helpin-ai/widget-embed` — Standalone IIFE bundle (`pixel.js`) built with Vite + Preact. This is the script tag customers embed on their website. It bootstraps the widget, creates a shadow DOM container, and connects via the `WidgetAdapter`.

### 2.3 Required Adaptations

#### Shared Types

Update the reference types from:

- `conversation` meaning a generic support object
- `ticket` assumptions in Helpin server code

to a unified Helpin vocabulary:

- `conversation`
- `message`
- `widget_session`
- `story_escalation`
- `ai_source`

#### Widget Core

Keep:

- package shape
- `WidgetAdapter` interface contract
- headless component boundaries
- CSS variable structure with `helpin-` namespace

**Component Implementation Plan:**

All 11 components currently return `null`. Each must be fully implemented:

| Component | Responsibility | Key Features |
|---|---|---|
| `WidgetLauncher` | Floating button to open/close widget | Workspace branding colors, unread badge, position config |
| `ChatWindow` | Main widget container | Open/close animation, responsive sizing, shadow DOM isolation |
| `WidgetHeader` | Top bar with workspace name/logo | Close button, "Powered by Helpin" optional branding |
| `PreChatForm` | Email + name capture before first message | Validates based on workspace settings (`require_email`, `require_name`), triggers CRM contact match/create |
| `MessageList` | Scrollable message thread | Auto-scroll, date separators, load-more for history |
| `MessageBubble` | Individual message rendering | Customer vs AI vs agent styling, internal note exclusion, timestamp |
| `ComposeBar` | Message input + send button | Enter-to-send, shift+enter for newline, disabled during AI processing |
| `StreamingText` | AI response streaming display | Token-by-token rendering, typing animation, source citations inline |
| `TypingIndicator` | Shows when agent/AI is composing | Animated dots, "Agent is typing..." label |
| `QuickReplies` | Suggested response buttons | AI-suggested follow-ups, "Talk to a person" action |
| `CsatRating` | Post-resolution satisfaction survey | Star or emoji rating, optional comment, triggers on conversation close |

**Implementation priorities for MVP:** `WidgetLauncher`, `ChatWindow`, `WidgetHeader`, `PreChatForm`, `MessageList`, `MessageBubble`, `ComposeBar`, `QuickReplies` (with "Talk to a person" action). `StreamingText`, `TypingIndicator`, and `CsatRating` can follow in Phase 4.

#### SDKs (`@helpin-ai/sdk-js`, `@helpin-ai/react`, `@helpin-ai/nextjs`)

The current `helpin-js` SDK is a production-grade analytics pixel (forked from usermaven-js) with pageview tracking, user identification, UTM capture, scroll depth, retry queues, and multiple transports. It works but has no chat/widget functionality.

**What to keep as-is:**
- Event tracking pipeline (`track`, `id`, `lead`, `group`)
- Automatic pageview tracking with SPA support
- Retry queue with offline persistence
- Multiple transports (XHR, Fetch, Beacon)
- Cross-domain linking
- Third-party cookie capture (`_ga`, `_fbp`, etc.)
- Privacy controls (`strict`/`keep`/`comply`)

**What to add:**
- `initWidget(config)` method on the client that fetches widget config and mounts the chat widget
- Auto-widget-mount when `widget_key` is present in SDK config
- `openWidget()` / `closeWidget()` / `toggleWidget()` programmatic controls
- `onConversationStarted` / `onMessageReceived` callback hooks
- Widget WebSocket connection management (separate from analytics transport)
- Identity bridging: analytics `id()` user data passed to widget session for CRM matching

**React wrapper additions (`@helpin-ai/react`):**
- `useSupportWidget()` hook returning `{ open, close, toggle, isOpen, unreadCount }`
- `useConversation()` hook for programmatic message access
- `HelpinProvider` extended to accept `widgetKey` prop

**Next.js wrapper (`@helpin-ai/nextjs`):**
- Same as React wrapper but SSR-safe (no-op on server, hydrates on client)
- Preserve existing Edge Middleware helpers (`getAnonymousId`, `getSourceIp`, `describeClient`)
- These helpers are valuable for Next.js customers who want server-side analytics tracking

### 2.4 Package Structure and Bundling

The SDK packages should live in a **standalone `packages/` directory** at the repo root, not inside `frontend/`. The Helpin dashboard (`frontend/`) is a standalone Vite SPA with no monorepo setup. The SDK packages need their own build pipeline because:

- `@helpin-ai/widget-embed` produces an IIFE bundle (`pixel.js`) for external websites — it cannot share the dashboard's Vite config
- `@helpin-ai/sdk-js` is a universal JavaScript library (browser + Node) with its own transport layer
- `@helpin-ai/react` and `@helpin-ai/nextjs` are npm-publishable packages with their own peer dependencies

**Target structure:**

```
packages/                        # SDK monorepo (pnpm workspaces + Turborepo)
  shared/                        # @helpin-ai/shared — types only
  widget-core/                   # @helpin-ai/widget-core — headless components + adapter interface
  sdk-js/                        # @helpin-ai/sdk-js — analytics + widget init
  react/                         # @helpin-ai/react — React hooks + provider
  nextjs/                        # @helpin-ai/nextjs — Next.js SSR-safe wrapper
  widget-embed/                  # @helpin-ai/widget-embed — IIFE bundle (pixel.js)
pnpm-workspace.yaml              # workspace: packages/*
turbo.json                       # build orchestration
```

**Initial package copy (using linux cp):**

```bash
# From the repo root (/root/teampulse):
mkdir -p packages

cp -r /root/helpin-convex-main/packages/shared       packages/shared
cp -r /root/helpin-convex-main/packages/widget-core   packages/widget-core
cp -r /root/helpin-convex-main/packages/helpin-js      packages/sdk-js
cp -r /root/helpin-convex-main/packages/helpin-react   packages/react
cp -r /root/helpin-convex-main/packages/helpin-nextjs  packages/nextjs

# widget-embed is new — scaffold from apps/widget reference:
mkdir -p packages/widget-embed/src
cp /root/helpin-convex-main/apps/widget/src/main.tsx           packages/widget-embed/src/
cp /root/helpin-convex-main/apps/widget/src/adapters/WebSocketWidgetAdapter.ts packages/widget-embed/src/
cp /root/helpin-convex-main/apps/widget/vite.config.ts         packages/widget-embed/

# Copy workspace config as starting point:
cp /root/helpin-convex-main/pnpm-workspace.yaml .
cp /root/helpin-convex-main/turbo.json .
```

After copy, adapt each package to Helpin naming and API conventions before building.

**Bundling order:**

1. copy packages from `/root/helpin-convex-main` into `packages/` (cp commands above)
2. set up pnpm workspace + Turborepo at repo root
3. adapt shared types to Helpin naming
4. implement widget-core components
5. extend sdk-js with chat widget initialization + JS API
6. build widget-embed IIFE bundle (`pixel.js`)
7. wire Go APIs for widget endpoints
8. integrate inbox UI in dashboard

### 2.5 Backend Constraint

The Helpin backend uses:

- Go
- Chi router
- GORM
- current service/repository/handler layering

Therefore:

- do not plan backend work around Fiber
- do not copy reference backend files directly
- do use the reference repo for frontend packages, protocol shape, and widget architecture

### 2.6 SDK JavaScript API Methods

The Helpin SDK should expose a JavaScript API via a global `window.Helpin()` command function. This provides a familiar developer experience and allows customers to programmatically control the widget.

#### Lifecycle Methods

| Method | Description |
|---|---|
| `Helpin('boot', config)` | Initialize the SDK and mount the widget. Accepts `widget_key`, optional user identity (`email`, `user_id`, `name`), and settings. For SPAs where the user may not be logged in at page load. |
| `Helpin('shutdown')` | End the current session, clear cookies/localStorage, and unmount the widget. Call on user logout to prevent conversation leakage on shared devices. |
| `Helpin('update')` | Trigger a check for new messages and update user data without a page refresh. Can be called with no arguments (just check for messages) or with a user data object to update identity. Throttled to 20 calls per 30 minutes. |
| `Helpin('update', userData)` | Update user identity fields (email, name, custom attributes). If the user is not yet identified, this creates or matches a CRM contact. |

#### Widget Visibility Methods

| Method | Description |
|---|---|
| `Helpin('show')` | Open the Messenger panel. If there are existing conversations, shows the conversation list. If none, shows the new conversation view. |
| `Helpin('hide')` | Close the Messenger panel. Does not hide the launcher button. |
| `Helpin('showMessages')` | Open the Messenger directly to the conversation list. |
| `Helpin('showNewMessage')` | Open the Messenger with a new conversation composer. |
| `Helpin('showNewMessage', content)` | Open the Messenger with a pre-populated message in the composer. |
| `Helpin('showConversation', conversationId)` | Open a specific conversation by ID. Opens the Messenger first if closed. |
| `Helpin('showArticle', articleId)` | Display a help center article inline within the Messenger. Opens the Messenger first if closed. |

#### Event Callback Methods

| Method | Description |
|---|---|
| `Helpin('onShow', callback)` | Register a callback fired when the Messenger opens. |
| `Helpin('onHide', callback)` | Register a callback fired when the Messenger closes. |
| `Helpin('onUnreadCountChange', callback)` | Register a callback fired with `(unreadCount)` whenever unread message count changes. Fires immediately on registration with the current count. Useful for rendering a badge on a custom launcher. |
| `Helpin('onUserEmailSupplied', callback)` | Register a callback fired when a visitor enters their email in the pre-chat form. Useful for triggering CRM actions or analytics. |

#### Analytics Methods (existing from `helpin-js`)

| Method | Description |
|---|---|
| `Helpin('trackEvent', name, metadata?)` | Track a custom event associated with the current user. Metadata is an optional key-value object. Events feed CRM signal detection. |
| `Helpin('getVisitorId')` | Return the anonymous visitor ID. Can be used to retrieve the visitor via REST API or for cross-domain linking. |

#### Script Tag Installation

```html
<!-- Basic installation -->
<script async src="https://cdn.helpin.ai/pixel.js" data-widget-key="wk_xxx"></script>

<!-- With user identity (for logged-in pages) -->
<script>
  window.helpinSettings = {
    widget_key: 'wk_xxx',
    email: user.email,
    name: user.name,
    user_id: user.id,
    created_at: user.createdAt
  };
</script>
<script async src="https://cdn.helpin.ai/pixel.js"></script>
```

#### SPA Installation (Boot pattern)

```javascript
// Load the SDK (script tag or npm install)
// Then boot when user data is available:
Helpin('boot', {
  widget_key: 'wk_xxx',
  email: 'jane@example.com',
  name: 'Jane Doe',
  user_id: '12345'
});

// On route change in SPA:
Helpin('update');

// On logout:
Helpin('shutdown');
```

#### NPM Installation

```javascript
// @helpin-ai/react
import { HelpinProvider, useSupportWidget } from '@helpin-ai/react';

function App() {
  return (
    <HelpinProvider widgetKey="wk_xxx" user={{ email, name, userId }}>
      <MyApp />
    </HelpinProvider>
  );
}

function SupportButton() {
  const { show, unreadCount } = useSupportWidget();
  return <button onClick={show}>Support {unreadCount > 0 && `(${unreadCount})`}</button>;
}
```

```javascript
// @helpin-ai/nextjs (SSR-safe)
import { HelpinProvider, useSupportWidget } from '@helpin-ai/nextjs';
// Same API as @helpin-ai/react but with no-op fallbacks on server
```

#### Pre-load Command Queue

The SDK should support a command queue pattern so calls made before the script loads are queued and replayed:

```javascript
window.Helpin = window.Helpin || function() {
  (window.Helpin.q = window.Helpin.q || []).push(arguments);
};
Helpin('boot', { widget_key: 'wk_xxx' });
// ^ This queues the boot call; the real SDK replays it on load
```

The existing `helpin-js` SDK already has this queue mechanism (`i.q = []; i.c = function(args){i.q.push(args)}`). It should be extended to support the new chat-related methods alongside the existing analytics methods.

### 2.7 SDK Endpoint Architecture

The SDK serves two distinct purposes — **analytics tracking** and **chat conversations** — with separate backend services. Rather than exposing two different hostnames to the SDK, both services sit behind a **single SDK domain** (`sdk.helpin.ai`) with **ingress-nginx path-based routing** splitting traffic at the infrastructure level.

#### Data Flow

| Path Prefix | Backend Service | Storage | Data Characteristics |
|---|---|---|---|
| `/v1/events/*` | Rust-based events pipeline (future; Go API initially) | ClickHouse | High-volume, append-only, sessionization + aggregation |
| `/v1/conversations/*`, `/v1/widget/*`, `/v1/ws` | Go API server (`helpin-server-svc`) | PostgreSQL / Neon | Transactional, low-volume, ACID, real-time delivery |

These are **separate data paths by design**, but the SDK doesn't need to know that. From the SDK's perspective there is one host and the path determines the destination.

#### SDK Config

The existing `helpin-js` SDK has a `trackingHost` config field (default: `t.helpin.ai`). This collapses into a single `host` field:

```typescript
interface Config {
  key: string;            // widget_key (wk_xxx)
  host?: string;          // SDK endpoint (default: 'sdk.helpin.ai')
  // ... existing analytics config fields
}
```

One field. No customer confusion about which URL goes where.

#### Endpoint Resolution

When the SDK boots:

1. Fetch widget config: `GET https://{host}/v1/widget/config?key={widget_key}`
2. The config response can override `host` for regional or on-prem deployments
3. Analytics events POST to `https://{host}/v1/events`
4. Chat REST calls go to `https://{host}/v1/conversations/*` (create, list messages, send message)
5. WebSocket for real-time chat opens to `wss://{host}/v1/ws?session_token=xxx`

All `/v1/...` paths are rewritten by ingress-nginx to the Go server's internal `/api/...` routes (see Ingress Configuration below). The SDK never sees the internal paths.

#### Why Single Domain with Ingress Routing

A dual-endpoint approach (separate `events.helpin.ai` + `api.prod.helpin.ai` hostnames) was considered and rejected in favor of ingress-level routing:

- **No extra hop**: ingress-nginx is already in the request path for every API call. Path-based routing adds zero latency — it's the same hop that already terminates TLS and forwards to backend services.
- **Zero incremental complexity**: Helpin already runs `pp-nginx` ingress class in the `helpin` namespace. Adding path rules to the existing Ingress resource is a config change, not a new service.
- **Same failure domain**: ingress-nginx going down already takes down the Go API. Routing SDK traffic through it doesn't expand the blast radius.
- **Simpler CORS**: One origin means one set of CORS rules. No cross-origin issues between analytics and chat.
- **Simpler SDK**: One `host` config field instead of two. Less surface area for customer misconfiguration.
- **Simpler self-hosted**: On-prem customers configure one URL, not two.
- **Cookie/auth scope**: Single domain means session tokens and cookies share scope naturally.
- **Independent scaling**: Still works — ingress-nginx routes to separate k8s Services, each with independent HPA scaling configs.
- **Graceful migration**: Before the Rust events pipeline exists, ingress routes `/v1/events/*` to the Go API server. When the Rust service is ready, update the Ingress backend — the SDK never changes.

#### Ingress Configuration

The SDK calls `/v1/...` paths but the Go server registers Chi routes at `/api/...`. ingress-nginx handles this with a separate Ingress resource for `sdk.helpin.ai` that uses `rewrite-target` to map the public SDK paths to internal server paths. The existing `ingress-helpin` resource for `api.prod.helpin.ai` and `helpcenter.helpin.ai` stays unchanged.

**New file: `k8s/prod/ingress-sdk.yaml`**

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: ingress-helpin-sdk
  namespace: helpin
  annotations:
    ingress.kubernetes.io/force-ssl-redirect: "true"
    nginx.ingress.kubernetes.io/rewrite-target: /api/widget/support/$2
    nginx.ingress.kubernetes.io/proxy-read-timeout: "3600"
    nginx.ingress.kubernetes.io/proxy-send-timeout: "3600"
    nginx.ingress.kubernetes.io/enable-cors: "true"
    nginx.ingress.kubernetes.io/cors-allow-origin: "*"
    nginx.ingress.kubernetes.io/cors-allow-methods: "GET, POST, PUT, DELETE, OPTIONS"
    nginx.ingress.kubernetes.io/cors-allow-headers: "Content-Type, Authorization, X-Session-Token"
spec:
  ingressClassName: pp-nginx
  tls:
  - hosts:
    - sdk.helpin.ai
    secretName: cert-prod-helpin-wildcard
  rules:
  - host: sdk.helpin.ai
    http:
      paths:
      # /v1/widget/config → /api/widget/support/config
      # /v1/conversations/* → /api/widget/support/conversations/*
      # /v1/ws → /api/widget/support/ws
      - path: /v1(/|$)(.*)
        pathType: ImplementationSpecific
        backend:
          service:
            name: helpin-server-svc
            port:
              number: 8080
---
# Events ingress — separate resource because it needs a different rewrite target.
# Initially points to helpin-server-svc; swap backend when Rust pipeline is ready.
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: ingress-helpin-sdk-events
  namespace: helpin
  annotations:
    ingress.kubernetes.io/force-ssl-redirect: "true"
    nginx.ingress.kubernetes.io/rewrite-target: /api/events/$2
    nginx.ingress.kubernetes.io/enable-cors: "true"
    nginx.ingress.kubernetes.io/cors-allow-origin: "*"
    nginx.ingress.kubernetes.io/cors-allow-methods: "GET, POST, OPTIONS"
    nginx.ingress.kubernetes.io/cors-allow-headers: "Content-Type"
spec:
  ingressClassName: pp-nginx
  tls:
  - hosts:
    - sdk.helpin.ai
    secretName: cert-prod-helpin-wildcard
  rules:
  - host: sdk.helpin.ai
    http:
      paths:
      - path: /v1/events(/|$)(.*)
        pathType: ImplementationSpecific
        backend:
          service:
            name: helpin-server-svc    # Swap to events-pipeline-svc when ready
            port:
              number: 8080
```

**Path mapping summary:**

| SDK calls | Ingress rewrites to | Backend |
|---|---|---|
| `GET /v1/widget/config?key=...` | `GET /api/widget/support/config?key=...` | Go API |
| `POST /v1/conversations` | `POST /api/widget/support/conversations` | Go API |
| `GET /v1/conversations/:id/messages` | `GET /api/widget/support/conversations/:id/messages` | Go API |
| `POST /v1/conversations/:id/messages` | `POST /api/widget/support/conversations/:id/messages` | Go API |
| `WS /v1/ws?session_token=...` | `WS /api/widget/support/ws?session_token=...` | Go API |
| `POST /v1/events` | `POST /api/events` | Go API (later Rust) |

This means the Go server's Chi routes stay at `/api/widget/support/...` and `/api/events/...` — no backend route changes needed. The SDK only ever sees `/v1/...` paths.

Staging uses `k8s/stage/ingress-sdk.yaml` with `sdk.stage.helpin.ai` and `cert-stage-helpin-wildcard`.

**Annotations explained:**
- `rewrite-target`: Maps the captured path group to the internal route structure
- `proxy-read-timeout` / `proxy-send-timeout` at 3600s: Required for WebSocket connections — without these, ingress-nginx closes idle connections after the default 60s
- CORS annotations: The widget runs on arbitrary customer domains, so `cors-allow-origin: "*"` is required. The events Ingress has a tighter `cors-allow-headers` since it only needs `Content-Type`

#### Script Tag Example

```html
<script>
  window.helpinSettings = {
    widget_key: 'wk_xxx',
    email: user.email,
    name: user.name
  };
</script>
<script async src="https://cdn.helpin.ai/pixel.js"></script>
```

No host configuration needed — `sdk.helpin.ai` is the default. Self-hosted deployments can override with `host: 'sdk.mycompany.com'`.

#### Events Pipeline Context

A Rust-based events pipeline will be added separately to handle analytics event ingestion, sessionization, and aggregation. This pipeline:

- Receives events at `sdk.helpin.ai/v1/events/*` (routed by ingress-nginx)
- Performs sessionization (30-minute inactivity window)
- Lands processed data in **ClickHouse** for analytics queries
- Is entirely independent of the Go API server and PostgreSQL

**Before the Rust pipeline exists**, the ingress routes `/v1/events/*` to `helpin-server-svc` (the Go API). The Go server stores events in a lightweight table or forwards them. When the Rust service is ready, the migration is a one-line Ingress backend swap — the SDK and all customer installations are unaffected.

Conversation data never touches the events pipeline. Messages, typing indicators, and read receipts flow through `/v1/conversations/*` and `/v1/ws` to the Go API server, stored in **PostgreSQL / Neon**.

---

## 3. Architecture

### 3.1 System Architecture Summary

Support is not an isolated module. It is a suite-level surface integrated with:

- **CRM** for contacts, companies, deals, and lead handling
- **PM** for escalation into stories and downstream execution
- **Docs** for knowledge retrieval, internal references, and external publishing
- **Notifications** for future inbox/email alerting and assignment signals
- **Agents** for AI support replies, drafts, triage, and future autonomous workflows
- **Realtime** via the existing WebSocket publisher

### 3.2 High-Level System Diagram

```text
Customer Website                            Helpin Platform
┌─────────────────────────┐                ┌────────────────────────────────────┐
│ helpin-widget embed     │                │ Dashboard (React + Vite)          │
│ - launcher              │                │ - Support Inbox                   │
│ - pre-chat capture      │                │ - CRM surfaces                    │
│ - AI/human thread       │                │ - PM story detail                 │
│ - session persistence   │                │ - Docs / Help Center              │
└────────────┬────────────┘                └──────────────┬─────────────────────┘
             │ REST + WebSocket                             │
             ▼                                              ▼
      ┌────────────────────────────────────────────────────────────────────┐
      │ Go API (Chi + GORM + WebSocket Publisher)                         │
      │ - Support handlers/services/repos                                 │
      │ - Widget public endpoints                                         │
      │ - CRM services                                                    │
      │ - PM story services                                               │
      │ - Docs / Helpcenter services                                      │
      │ - Agent services / Temporal workflows                             │
      └───────────────────────┬───────────────────────────────┬────────────┘
                              │                               │
                              ▼                               ▼
                        PostgreSQL / Neon               LLM Provider(s)
```

### 3.3 Frontend Architecture

The dashboard implementation must align with the current frontend:

- React 19
- Vite
- TanStack Router
- TanStack Query
- shared `frontend/src` component and route patterns

Relevant current evidence in the repo:

- support route: `frontend/src/routes/_authenticated/w/$slug/support.tsx`
- PM support redirect: `frontend/src/routes/_authenticated/w/$slug/pm/support.tsx`
- support page UI: `frontend/src/pages/pm/Support.tsx`

This means the support inbox is already a suite-level navigation surface and should stay that way.

### 3.4 Backend Architecture

The backend implementation must align with Helpin's handler → service → repository layering.

Support should use a `support_inbox` prefix for its Go files to clearly scope the module within the codebase. The current `support.go` files should be renamed to reflect the conversation-first model:

**Current files → Target files:**

| Layer | Current | Target |
|---|---|---|
| Model | `server/internal/model/support.go` | `server/internal/model/support_inbox.go` |
| Repository | `server/internal/repository/support.go` | `server/internal/repository/support_inbox.go` |
| Service | `server/internal/service/support.go` | `server/internal/service/support_inbox.go` |
| Service (new) | — | `server/internal/service/support_inbox_ai.go` |
| Handler | `server/internal/handler/support.go` | `server/internal/handler/support_inbox.go` |
| Handler | `server/internal/handler/widget.go` | `server/internal/handler/support_inbox_widget.go` |

**Struct renames within the files:**

| Current | Target |
|---|---|
| `SupportTicket` | `SupportConversation` |
| `SupportMessage` | `SupportConversationMessage` |
| `SupportWidgetInstallation` | `SupportInboxInstallation` |
| `SupportWidgetSession` | `SupportInboxSession` |
| `SupportService` | `SupportInboxService` |
| `SupportHandler` | `SupportInboxHandler` |
| `WidgetHandler` | `SupportInboxWidgetHandler` |
| `SupportTicketRepository` | `SupportConversationRepository` |
| `SupportMessageRepository` | `SupportConversationMessageRepository` |
| `WidgetInstallationRepository` | `SupportInboxInstallationRepository` |
| `WidgetSessionRepository` | `SupportInboxSessionRepository` |

The PRD should evolve the existing support implementation by renaming in place rather than introducing parallel files.

### 3.5 Realtime Architecture

Helpin already has a WebSocket event publisher:

- `server/internal/websocket/publisher.go`

Support currently publishes events for:

- support ticket created/updated
- support message created
- agent-run updates

The future conversation-first model should continue this pattern with renamed entities:

- `support_conversation`
- `support_message`
- `support_agent_run`

### 3.6 Auth and Access Model

Support has two access surfaces:

#### Internal dashboard access

- authenticated users
- workspace-scoped
- permission-controlled through the existing authorization layer

**Current state:** Support routes piggyback on PM permissions (`PermPMRead` / `PermPMEdit`). There are no dedicated support permissions in `server/internal/authorization/permissions.go`. This should be corrected:

| New Permission | Constant | Usage |
|---|---|---|
| `support.read` | `PermSupportRead` | List/view conversations, messages |
| `support.edit` | `PermSupportEdit` | Create/reply/assign/status change |
| `support.admin` | `PermSupportAdmin` | Widget settings, installation config, SLA rules |

These must be added to `AllPermissions()`, the role-permission matrix, and the router middleware.

#### Public widget access

- no JWT required
- public widget key
- short-lived session token
- limited message-scope operations only

### 3.7 Suite Integration Principle

Helpin Support must behave like a suite module:

- it should appear as a native app surface
- its data should be linkable via existing cross-object association patterns
- it should reuse shared infra for audit/activity, notifications, websocket, and agents
- it should not invent parallel systems when PM/CRM/Docs already exist

---

## 4. Data Model

### 4.1 Canonical Domain Decision

The current repo persists support as:

- `support_tickets` → table
- `support_messages` → table
- `support_widget_installations` → table
- `support_widget_sessions` → table
- Go files: `model/support.go`, `repository/support.go`, `service/support.go`, `handler/support.go`, `handler/widget.go`

This PRD updates the target model to be **conversation-first** with a `support_inbox` prefix for clear module scoping:

**Target database tables:**

- `support_conversations`
- `support_conversation_messages`
- `support_inbox_installations`
- `support_inbox_sessions`

**Target Go files:**

- `model/support_inbox.go`
- `repository/support_inbox.go`
- `service/support_inbox.go`
- `service/support_inbox_ai.go`
- `handler/support_inbox.go`
- `handler/support_inbox_widget.go`

### 4.2 Why Ticket + Chat Is Wrong for Helpin

If Helpin keeps:

- live chat as one concept
- tickets as another concept

it creates long-term duplication in:

- lifecycle/status logic
- UI terminology
- reporting
- CRM links
- docs links
- PM escalation logic
- agent runtime targeting

The correct product and data model is a single conversation object that can begin in the widget and continue in the inbox.

### 4.3 Conversation State Model

Recommended MVP statuses:

- `open`
- `waiting`
- `resolved`
- `closed`

Optional later states:

- `snoozed`
- `spam`

If the existing `in_progress` state remains during migration, it can temporarily map to `open` + assignment semantics, but the preferred long-term state vocabulary is simpler and inbox-oriented.

### 4.4 Core Entities

#### A. Support Conversation

Suggested target table:

- `support_conversations`

Fields:

- `id`
- `workspace_id`
- `display_id`
- `subject`
- `status`
- `priority`
- `source` (`widget`, `internal`, `email`, `api`)
- `channel` (`web_widget`, `dashboard`, `email`)
- `customer_name`
- `customer_email`
- `opened_by_user_id`
- `assigned_agent_id`
- `crm_contact_id`
- `primary_story_id`
- `first_response_mode` (`ai`, `human`)
- `ai_handoff_reason`
- `last_message_at`
- `resolved_at`
- `closed_at`
- `created_at`
- `updated_at`

#### B. Support Conversation Message

Suggested target table:

- `support_conversation_messages`

Fields:

- `id`
- `workspace_id`
- `conversation_id`
- `sender_type` (`customer`, `user`, `agent`, `ai`, `system`)
- `sender_user_id`
- `sender_agent_id`
- `sender_display_name`
- `content`
- `message_type` (`public`, `internal_note`, `system_event`, `ai_answer`, `csat_survey`)
- `is_internal`
- `metadata` JSONB
- `created_at`
- `updated_at`

The `metadata` JSON should support:

- AI retrieval sources
- model/provider metadata
- confidence score
- widget browser/session metadata
- handoff reason
- structured system events
- CSAT survey data (`csat_type`, `rating`, `feedback`, `submitted_at`) for `csat_survey` messages

#### C. Support Inbox Installation

Suggested target table:

- `support_inbox_installations`

Fields:

- `id`
- `workspace_id`
- `widget_key`
- `secret_key`
- `settings` JSONB
- `active`
- `created_at`
- `updated_at`

#### D. Support Inbox Session

Suggested target table:

- `support_inbox_sessions`

Fields:

- `id`
- `workspace_id`
- `conversation_id`
- `session_token`
- `customer_name`
- `customer_email`
- `crm_contact_id`
- `anonymous_identity_captured`
- `expires_at`
- `created_at`

### 4.5 CRM Relationships

Each conversation should support:

- one primary CRM contact
- optional association to company/deal via `crm_associations`

Helpin already has CRM association patterns and should extend them instead of introducing support-only custom link tables for everything.

### 4.6 PM Relationships

Each conversation should support:

- one `primary_story_id` for the main execution path
- optional future associations to multiple stories where necessary

MVP recommendation:

- support one primary story link
- keep broader associations as a later extension

### 4.7 Docs Relationships

Relevant docs should be represented through:

- `docs_links` where persistent object linking matters
- message-level AI source metadata for retrieval citations

### 4.8 Current-to-Target Rename Mapping

#### Database Tables

| Current | Target |
|---|---|
| `support_tickets` | `support_conversations` |
| `support_messages` | `support_conversation_messages` |
| `support_widget_installations` | `support_inbox_installations` |
| `support_widget_sessions` | `support_inbox_sessions` |

#### Go Files

| Current | Target |
|---|---|
| `model/support.go` | `model/support_inbox.go` |
| `repository/support.go` | `repository/support_inbox.go` |
| `service/support.go` | `service/support_inbox.go` |
| — (new) | `service/support_inbox_ai.go` |
| `handler/support.go` | `handler/support_inbox.go` |
| `handler/widget.go` | `handler/support_inbox_widget.go` |

#### Go Structs and Constants

| Current | Target |
|---|---|
| `SupportTicket` | `SupportConversation` |
| `SupportMessage` | `SupportConversationMessage` |
| `SupportWidgetInstallation` | `SupportInboxInstallation` |
| `SupportWidgetSession` | `SupportInboxSession` |
| `SupportService` | `SupportInboxService` |
| `SupportHandler` | `SupportInboxHandler` |
| `WidgetHandler` | `SupportInboxWidgetHandler` |
| `SupportTicketRepository` | `SupportConversationRepository` |
| `SupportMessageRepository` | `SupportConversationMessageRepository` |
| `WidgetInstallationRepository` | `SupportInboxInstallationRepository` |
| `WidgetSessionRepository` | `SupportInboxSessionRepository` |
| `CRMObjectSupportTicket` | `CRMObjectSupportConversation` |
| `LinkedObjectSupportTicket` | `LinkedObjectSupportConversation` |

#### Frontend Types and Services

| Current | Target |
|---|---|
| `SupportTicket` | `SupportConversation` |
| `SupportMessage.ticket_id` | `SupportConversationMessage.conversation_id` |
| `TicketStatus` | `ConversationStatus` |
| `TicketPriority` | `ConversationPriority` |
| `supportService.listTickets()` | `supportInboxService.listConversations()` |
| `supportService.getTicket()` | `supportInboxService.getConversation()` |
| `queryKeys.support.tickets()` | `queryKeys.support.conversations()` |
| `queryKeys.support.ticket()` | `queryKeys.support.conversation()` |

### 4.9 Migration Strategy

Recommended migration path:

1. rename domain language in UI, API, services, and docs first
2. add compatibility adapters where necessary
3. perform table/entity renames early while support is still relatively contained
4. remove ticket-first naming after inbox APIs are stable

This is the right point to clean up naming before support grows more deeply across the suite.

---

## 5. Inbox UX and Workflow

### 5.1 Dashboard Surface

Agents should work from a native Helpin **Support Inbox** page with a **two-pane default layout**:

- **left pane**: queues, filters, conversation list
- **main pane**: conversation thread with reply composer

A **slide-out drawer** (right side) opens on demand for context and details. This avoids permanently consuming horizontal space for context that is only needed intermittently. The drawer should be triggered by:

- clicking the customer name / CRM badge in the conversation header
- clicking a dedicated "Details" or info icon button
- keyboard shortcut

On mobile, the drawer becomes a full-screen overlay.

#### Inbox Layout — Default Two-Pane

```
+---------------------------------------------------------------------+
| Support Inbox                                        [+ New]  [?]   |
+----------------------------+----------------------------------------+
| [Search conversations...  ]|  #142 · Password reset not working     |
|                            |  Jane Doe <jane@acme.co>    [Details >]|
| Filters: [All v] [Open v] |  Status: Open  Priority: High          |
|          [Assigned to me]  |  Assigned: @alex                       |
+----------------------------+----------------------------------------+
| #142 Jane Doe        2m   |                                        |
|   Password reset not wor.. |  [Jane Doe · customer · 2m ago]       |
|   Open · High              |  I keep clicking "Reset Password"     |
|----------------------------+  but nothing happens. I've tried       |
| #139 Bob Smith       15m  |  three different browsers.             |
|   Can't export CSV         |                                        |
|   Open · Medium            |  [AI · ai_answer · 1m ago]            |
|----------------------------+  I found a few articles that might     |
| #137 Acme Corp       1h   |  help:                                |
|   Billing question         |  - "Password Reset Guide" (docs)     |
|   Waiting · Medium         |  - "Account Recovery FAQ" (helpcenter)|
|----------------------------+                                        |
| #134 Sarah Lee       3h   |  Is the issue happening on the main   |
|   API rate limiting        |  login page or the settings page?     |
|   In Progress · Low        |                                        |
|----------------------------+  [System · 30s ago]                    |
| #131 Dave K.         1d   |  AI confidence: 0.72 — suggested      |
|   Feature request          |  human review                         |
|   Resolved · Low           |                                        |
|----------------------------+                                        |
|                            |                                        |
|                            |                                        |
|                            +----------------------------------------+
|                            | [x Internal note]                     |
|                            | [Type a reply...                     ]|
|                            |                      [AI Draft] [Send]|
+----------------------------+----------------------------------------+
```

#### Inbox Layout — With Context Drawer Open

```
+----------------------------+-------------------------+--------------+
| [Search conversations...  ]| #142 · Password reset   | DETAILS   [x]|
|                            | Jane Doe   [Details >]  |              |
| Filters: [All v] [Open v] | Open · High · @alex     | CONTACT      |
+----------------------------+-------------------------+ Jane Doe     |
| #142 Jane Doe        2m   |                         | jane@acme.co |
|   Password reset not wor.. | [Jane · 2m ago]        | Lifecycle:   |
|   Open · High              | I keep clicking "Reset  |   Subscriber |
|----------------------------+ Password" but nothing   | Source:      |
| #139 Bob Smith       15m  | happens.                |   Widget     |
|   Can't export CSV         |                         | [View in CRM]|
|   Open · Medium            | [AI · 1m ago]           |--------------|
|----------------------------+ I found a few articles  | COMPANY      |
| #137 Acme Corp       1h   | that might help:        | Acme Corp    |
|   Billing question         | - Password Reset Guide  | [View deal]  |
|   Waiting · Medium         |                         |--------------|
|----------------------------+                         | LINKED STORY |
| #134 Sarah Lee       3h   |                         | (none)       |
|   API rate limiting        |                         | [Create story|
|   In Progress · Low        |                         |  from conv.] |
|----------------------------+                         |--------------|
|                            |                         | AI SOURCES   |
|                            |                         | - Password   |
|                            |                         |   Reset Guide|
|                            |                         | - Account    |
|                            |                         |   Recovery   |
|                            +-------------------------+--------------|
|                            | [Type a reply...       ]| METADATA     |
|                            |          [AI Draft][Send]| Source: widget|
+----------------------------+-------------------------+ Created: 2m  |
                                                       +--------------+
```

#### Inbox Layout — Mobile (Thread View)

```
+-----------------------------+
| [<] #142 Password reset     |
|     Jane Doe · Open · High  |
+-----------------------------+
|                             |
| [Jane Doe · 2m ago]        |
| I keep clicking "Reset     |
| Password" but nothing      |
| happens. I've tried three  |
| different browsers.        |
|                             |
| [AI · 1m ago]              |
| I found a few articles     |
| that might help:           |
| - "Password Reset Guide"  |
| - "Account Recovery FAQ"  |
|                             |
| Is the issue happening on  |
| the main login page or the |
| settings page?             |
|                             |
| [System · 30s ago]         |
| AI confidence: 0.72 —      |
| suggested human review     |
|                             |
|                             |
+-----------------------------+
| [x Internal note]          |
| [Type a reply...           ]|
|              [AI Draft][Send]|
+-----------------------------+
```

### 5.2 Conversation List

Each row should show:

- display ID
- subject
- customer name/email
- latest activity time
- status
- priority
- assignment state
- AI/human indicator

### 5.3 Conversation Thread

The thread should support:

- customer messages
- agent messages
- AI messages
- internal notes
- system events
- CSAT survey messages (inline rating + feedback — see 6.7.2)
- typing indicators at bottom of thread (see 6.7.3)

### 5.4 Context Drawer

The context drawer slides out from the right edge when opened. It should contain:

- **CRM contact summary**: name, email, lifecycle stage, lead status, source
- **Associated company and deal** where available (via `crm_associations`)
- **Linked PM story**: title, status, link to story detail; button to create story if none linked
- **AI source references**: docs used for the most recent AI answer, with links
- **Conversation metadata**: source (widget/internal/email), channel, created time, first response mode
- **Conversation controls**: status, priority, assignee dropdowns (editable inline)
- **Associations panel**: reuse existing `AssociationsPanel` component with `support_conversation` object type

The drawer should be dismissible by clicking outside, pressing Escape, or clicking the close button. State (open/closed) should persist within the session but not across page navigations.

### 5.5 Mobile Behavior

The current support page already includes mobile thread switching patterns (`mobileShowThread` state with `ArrowLeft` back button). The PRD should preserve and extend mobile-first behavior for:

- list-to-thread transitions (existing pattern)
- reply composer access (existing pattern)
- context drawer opens as full-screen overlay on mobile
- swipe-to-dismiss drawer on mobile

### 5.6 Internal Notes

Internal notes are required in MVP because support in Helpin is collaborative and context-heavy. These should remain message-like records on the same conversation thread but rendered separately from customer-visible messages.

---

## 6. Widget Experience

### 6.1 Widget Capabilities

The embeddable widget should support:

- launcher button
- welcome state
- pre-chat identity capture
- AI-first response flow
- conversation history
- talk-to-human path
- session persistence
- unread message overlay (see 6.7.5)
- typing indicators (see 6.7.3)
- CSAT survey inline (see 6.7.2)
- email transcript on resolved conversations (see 6.7.4)

### 6.2 Anonymous Identity Capture

For anonymous users, the widget should ask:

1. **email**
2. **name**

This is the preferred control pattern because it:

- identifies the customer early
- enables CRM matching/creation
- supports follow-up and lead handling
- matches the desired behavior

### 6.3 Workspace Controls

Workspace admins should be able to configure:

- require email before chatting
- require name after email
- auto-create CRM contact on capture
- default lifecycle stage on create
- auto-promote to lead or not
- AI enabled by default
- display of "Talk to a person"

Recommended MVP defaults:

- require email: `true`
- require name: `true`
- auto-create CRM contact: `true`
- lifecycle stage on auto-create: `subscriber`
- optional promotion path to `lead`

### 6.4 Session Behavior

Public widget access should remain based on:

- `widget_key`
- `session_token`
- short-lived session records

The widget session should later bind to the conversation record once the first message creates or joins the canonical thread.

### 6.5 Chat Settings Pages

Chat settings are split across **3 sub-pages** under a "Support Settings" group in the workspace settings sidebar. This follows the same pattern as CRM Settings (Pipelines, Email Accounts, Autonomy) — multiple related pages under one group header.

All settings are stored in the `SupportInboxInstallation.Settings` JSONB column and passed to the widget via `GET /api/widget/support/config`.

#### Settings Route Registration

Add to `SETTINGS_SECTIONS` in `frontend/src/pages/Settings.tsx`:

```typescript
{
  id: 'chat-general',
  label: 'General',
  description: 'Widget installation, identity capture, and CRM integration.',
  icon: MessageSquare,
  group: 'Support Settings',
},
{
  id: 'chat-ai',
  label: 'AI & Routing',
  description: 'Configure AI auto-reply, handoff behavior, and business hours.',
  icon: Bot,
  group: 'Support Settings',
},
{
  id: 'chat-appearance',
  label: 'Appearance',
  description: 'Customize widget branding, launcher position, and styling.',
  icon: Palette,
  group: 'Support Settings',
},
{
  id: 'chat-responses',
  label: 'Canned Responses',
  description: 'Manage saved reply templates for quick agent responses.',
  icon: Zap,
  group: 'Support Settings',
},
```

**Sidebar rendering:**

```
Settings Sidebar
├─ Workspace
│  ├─ General
│  ├─ Members
│  └─ Teams
├─ Project Settings
│  ├─ Workflows
│  ├─ Labels
│  ├─ ...
├─ CRM Settings
│  ├─ Pipelines
│  ├─ Email Accounts
│  └─ Autonomy
├─ Support Settings          ← new group
│  ├─ General                ← /w/:slug/settings/chat-general
│  ├─ AI & Routing           ← /w/:slug/settings/chat-ai
│  ├─ Appearance             ← /w/:slug/settings/chat-appearance
│  └─ Canned Responses       ← /w/:slug/settings/chat-responses
└─ ...
```

**Components:**
- `frontend/src/components/settings/ChatGeneralTab.tsx`
- `frontend/src/components/settings/ChatAITab.tsx`
- `frontend/src/components/settings/ChatAppearanceTab.tsx`

---

#### Page 1: General (`/w/:slug/settings/chat-general`)

Widget installation, identity capture, and CRM integration.

```
/w/acme/settings/chat-general
+---------------------------------------------------------------------+
| General                                                             |
| Widget installation, identity capture, and CRM integration.         |
+---------------------------------------------------------------------+
|                                                                     |
| +---------------------------------------------------------------+  |
| | WIDGET INSTALLATION                                            |  |
| | Your widget key for embedding on your website.                 |  |
| |---------------------------------------------------------------|  |
| |                                                                |  |
| | Widget Key                                                     |  |
| | [wk_a3f8b2c1d4e5...                          ] [Copy] [Regen] |  |
| |                                                                |  |
| | Embed Code                                                     |  |
| | +-----------------------------------------------------------+ |  |
| | | <script async                                             | |  |
| | |   src="https://cdn.helpin.ai/pixel.js"                    | |  |
| | |   data-widget-key="wk_a3f8b2c1d4e5">                     | |  |
| | | </script>                                 [Copy snippet]  | |  |
| | +-----------------------------------------------------------+ |  |
| |                                                                |  |
| | Widget Status                                                  |  |
| |   Active                                           [o]        |  |
| |   Enable or disable the chat widget on your site               |  |
| +---------------------------------------------------------------+  |
|                                                                     |
| +---------------------------------------------------------------+  |
| | IDENTITY CAPTURE                                               |  |
| | Control what information is collected before a chat starts.    |  |
| |---------------------------------------------------------------|  |
| |                                                                |  |
| | Require Email                                      [o]        |  |
| |   Visitors must enter their email before starting              |  |
| |   a conversation.                                              |  |
| |                                                                |  |
| | ─────────────────────────────────────────────────────────────  |  |
| |                                                                |  |
| | Require Name                                       [o]        |  |
| |   Ask for the visitor's name after email capture.              |  |
| |   Only shown if "Require Email" is enabled.                    |  |
| |                                                                |  |
| | ─────────────────────────────────────────────────────────────  |  |
| |                                                                |  |
| | Welcome Message                                                |  |
| | [Hi! How can we help you today?                              ] |  |
| |   The greeting shown when the widget opens.                    |  |
| +---------------------------------------------------------------+  |
|                                                                     |
| +---------------------------------------------------------------+  |
| | CRM INTEGRATION                                                |  |
| | How captured visitor identities connect to your CRM.           |  |
| |---------------------------------------------------------------|  |
| |                                                                |  |
| | Auto-Create CRM Contact                            [o]        |  |
| |   Automatically create a CRM contact when a visitor            |  |
| |   provides their email. If a contact already exists,           |  |
| |   the conversation links to the existing record.               |  |
| |                                                                |  |
| | ─────────────────────────────────────────────────────────────  |  |
| |                                                                |  |
| | Default Lifecycle Stage              [Subscriber       v]     |  |
| |   The lifecycle stage assigned to auto-created contacts.       |  |
| |   Options: Subscriber, Lead, Opportunity                       |  |
| |                                                                |  |
| | ─────────────────────────────────────────────────────────────  |  |
| |                                                                |  |
| | Auto-Promote to Lead                               [ ]        |  |
| |   Automatically upgrade contacts from subscriber to            |  |
| |   lead after their first conversation is resolved.             |  |
| +---------------------------------------------------------------+  |
|                                                                     |
|                                         [Save]                      |
+---------------------------------------------------------------------+
```

---

#### Page 2: AI & Routing (`/w/:slug/settings/chat-ai`)

AI auto-reply, handoff behavior, and business hours.

```
/w/acme/settings/chat-ai
+---------------------------------------------------------------------+
| AI & Routing                                                        |
| Configure AI auto-reply, handoff behavior, and business hours.      |
+---------------------------------------------------------------------+
|                                                                     |
| +---------------------------------------------------------------+  |
| | AI AUTO-REPLY                                                  |  |
| | Configure how the AI assistant handles new conversations.      |  |
| |---------------------------------------------------------------|  |
| |                                                                |  |
| | Enable AI Auto-Reply                               [o]        |  |
| |   AI automatically responds to new conversations using        |  |
| |   your Docs and Help Center content.                           |  |
| |                                                                |  |
| | ─────────────────────────────────────────────────────────────  |  |
| |                                                                |  |
| | Confidence Threshold                 [0.7                  ]  |  |
| |   Conversations with AI confidence below this score            |  |
| |   are flagged for human review. Range: 0.0 - 1.0              |  |
| |                                                                |  |
| | ─────────────────────────────────────────────────────────────  |  |
| |                                                                |  |
| | Show "Talk to a Person"                            [o]        |  |
| |   Display a button in the widget that lets visitors            |  |
| |   request a human agent at any time.                           |  |
| +---------------------------------------------------------------+  |
|                                                                     |
| +---------------------------------------------------------------+  |
| | HANDOFF ROUTING                                                |  |
| | What happens when AI hands off to a human agent.               |  |
| |---------------------------------------------------------------|  |
| |                                                                |  |
| | Handoff Behavior                     [Assign to team   v]     |  |
| |   - Unassigned: conversation goes to inbox unassigned          |  |
| |   - Assign to team: route to a specific team                   |  |
| |   - Round robin: distribute among online agents                |  |
| |                                                                |  |
| | ─────────────────────────────────────────────────────────────  |  |
| |                                                                |  |
| | Handoff Team                         [Support Team     v]     |  |
| |   The team that receives AI handoff conversations.             |  |
| |   Only shown when handoff behavior is "Assign to team".        |  |
| +---------------------------------------------------------------+  |
|                                                                     |
| +---------------------------------------------------------------+  |
| | BUSINESS HOURS                                                 |  |
| | Set when your team is available for live support.              |  |
| |---------------------------------------------------------------|  |
| |                                                                |  |
| | Enable Business Hours                              [ ]        |  |
| |   When disabled, the widget is always shown as available.      |  |
| |                                                                |  |
| | ─────────────────────────────────────────────────────────────  |  |
| |                                                                |  |
| | Timezone                             [America/New_York v]     |  |
| |                                                                |  |
| | Schedule                                                       |  |
| |   Mon  [09:00] - [17:00]  [o]                                 |  |
| |   Tue  [09:00] - [17:00]  [o]                                 |  |
| |   Wed  [09:00] - [17:00]  [o]                                 |  |
| |   Thu  [09:00] - [17:00]  [o]                                 |  |
| |   Fri  [09:00] - [17:00]  [o]                                 |  |
| |   Sat  [     ] - [     ]  [ ]                                 |  |
| |   Sun  [     ] - [     ]  [ ]                                 |  |
| |                                                                |  |
| | Outside Hours Message                                          |  |
| | [We're currently offline. Leave a message and we'll           ]|  |
| | [get back to you!                                             ]|  |
| +---------------------------------------------------------------+  |
|                                                                     |
|                                         [Save]                      |
+---------------------------------------------------------------------+
```

---

#### Page 3: Appearance (`/w/:slug/settings/chat-appearance`)

Widget branding, launcher position, and styling.

```
/w/acme/settings/chat-appearance
+---------------------------------------------------------------------+
| Appearance                                                          |
| Customize widget branding, launcher position, and styling.          |
+---------------------------------------------------------------------+
|                                                                     |
| +---------------------------------------------------------------+  |
| | BRANDING                                                       |  |
| | Match the widget to your brand.                                |  |
| |---------------------------------------------------------------|  |
| |                                                                |  |
| | Brand Color                          [#6366F1            ] [] |  |
| |   The primary color used for the widget launcher,              |  |
| |   header background, and send button.                          |  |
| |                                                                |  |
| | ─────────────────────────────────────────────────────────────  |  |
| |                                                                |  |
| | Show Helpin Branding                               [o]        |  |
| |   Display "Powered by Helpin" in the widget footer.            |  |
| +---------------------------------------------------------------+  |
|                                                                     |
| +---------------------------------------------------------------+  |
| | LAUNCHER                                                       |  |
| | Configure the floating chat button on your site.               |  |
| |---------------------------------------------------------------|  |
| |                                                                |  |
| | Position                             [Bottom Right     v]     |  |
| |   Where the launcher button appears on the page.               |  |
| |   Options: Bottom Right, Bottom Left                           |  |
| |                                                                |  |
| | ─────────────────────────────────────────────────────────────  |  |
| |                                                                |  |
| | Icon                                 [Chat Bubble      v]     |  |
| |   The icon shown on the launcher button.                       |  |
| |   Options: Chat Bubble, Question Mark, Help                    |  |
| +---------------------------------------------------------------+  |
|                                                                     |
| +---------------------------------------------------------------+  |
| | PREVIEW                                                        |  |
| | Live preview of your widget with current settings.             |  |
| |---------------------------------------------------------------|  |
| |                                                                |  |
| |   +---------------------------------------------+             |  |
| |   | [Acme Corp Logo]  Acme Corp              [x] |             |  |
| |   |---------------------------------------------|             |  |
| |   |                                             |             |  |
| |   | Hi! How can we help you today?              |             |  |
| |   |                                             |             |  |
| |   |                                             |             |  |
| |   |                                             |             |  |
| |   |---------------------------------------------|             |  |
| |   | [Type a message...                   ] [>]  |             |  |
| |   |---------------------------------------------|             |  |
| |   |          Powered by Helpin                  |             |  |
| |   +---------------------------------------------+             |  |
| |                                                                |  |
| |                                     [ (chat bubble icon) ]     |  |
| |                                     ← launcher preview         |  |
| +---------------------------------------------------------------+  |
|                                                                     |
|                                         [Save]                      |
+---------------------------------------------------------------------+
```

---

#### Full Settings Schema

All settings stored in `SupportInboxInstallation.Settings` JSONB:

**Widget Installation** (General page)

| Field | Type | Default | Description |
|---|---|---|---|
| `active` | bool | `true` | Master switch — widget hidden when disabled |

**Identity Capture** (General page)

| Field | Type | Default | Description |
|---|---|---|---|
| `require_email_before_chat` | bool | `true` | Require email before first message |
| `require_name_after_email` | bool | `true` | Ask for name after email (only if email required) |
| `welcome_message` | string | `"Hi! How can we help you today?"` | Greeting shown when widget opens |

**CRM Integration** (General page)

| Field | Type | Default | Description |
|---|---|---|---|
| `auto_create_crm_contact` | bool | `true` | Create CRM contact on email capture |
| `default_lifecycle_stage` | string | `"subscriber"` | Lifecycle stage for auto-created contacts. Values: `subscriber`, `lead`, `opportunity` |
| `auto_promote_to_lead` | bool | `false` | Promote subscriber to lead on first resolved conversation |

**AI Auto-Reply** (AI & Routing page)

| Field | Type | Default | Description |
|---|---|---|---|
| `ai_enabled` | bool | `true` | AI auto-replies to new conversations |
| `ai_confidence_threshold` | float | `0.7` | Below this score, flag for human review |
| `show_talk_to_human` | bool | `true` | Show "Talk to a person" button in widget |

**Handoff Routing** (AI & Routing page)

| Field | Type | Default | Description |
|---|---|---|---|
| `handoff_behavior` | string | `"unassigned"` | On AI handoff: `unassigned`, `assign_to_team`, `round_robin` |
| `handoff_team_id` | string? | `null` | Team ID for `assign_to_team` handoff (nullable) |

**Business Hours** (AI & Routing page)

| Field | Type | Default | Description |
|---|---|---|---|
| `business_hours_enabled` | bool | `false` | Enable business hours restrictions |
| `business_hours_timezone` | string | `"UTC"` | IANA timezone for schedule |
| `business_hours_schedule` | object | `{}` | Day-of-week schedule: `{ "mon": { "start": "09:00", "end": "17:00", "enabled": true }, ... }` |
| `outside_hours_message` | string | `"We're currently offline. Leave a message and we'll get back to you!"` | Message shown outside business hours |

**Branding** (Appearance page)

| Field | Type | Default | Description |
|---|---|---|---|
| `brand_color` | string | `"#6366F1"` | Primary widget color (hex) |
| `show_branding` | bool | `true` | Show "Powered by Helpin" footer |

**Launcher** (Appearance page)

| Field | Type | Default | Description |
|---|---|---|---|
| `launcher_position` | string | `"bottom_right"` | Launcher position: `bottom_right`, `bottom_left` |
| `launcher_icon` | string | `"chat_bubble"` | Icon style: `chat_bubble`, `question_mark`, `help` |

#### Widget Config API Response

The `GET /api/widget/support/config?key={widget_key}` endpoint returns the subset of settings the widget needs (no server-side secrets or internal config):

```json
{
  "workspace_name": "Acme Corp",
  "workspace_logo_url": "https://...",
  "require_email": true,
  "require_name": true,
  "welcome_message": "Hi! How can we help you today?",
  "ai_enabled": true,
  "show_talk_to_human": true,
  "brand_color": "#6366F1",
  "launcher_position": "bottom_right",
  "launcher_icon": "chat_bubble",
  "show_branding": true,
  "business_hours_enabled": false,
  "is_online": true
}
```

Fields like `handoff_behavior`, `handoff_team_id`, `auto_create_crm_contact`, `default_lifecycle_stage`, and `auto_promote_to_lead` are server-side only — they affect backend behavior but are never exposed to the widget.

### 6.6 Widget Config Caching Strategy

The widget config endpoint (`GET /v1/widget/config?key={widget_key}`) is called every time a customer's website loads the Helpin widget. This section defines how config is cached, invalidated, and how admin changes propagate to live widgets.

#### Design: Boot-Time Fetch + HTTP Cache + WebSocket Push

No Redis or dedicated caching layer needed. The strategy uses three layers:

```
Admin changes settings
        │
        ▼
┌──────────────────┐    ┌─────────────────────┐
│  PostgreSQL      │───►│ Go API server        │
│  (1 row per      │    │ GET /v1/widget/config │
│   workspace)     │    │                       │
└──────────────────┘    │ Cache-Control:        │
                        │   public, max-age=60, │
                        │   stale-while-        │
                        │   revalidate=300      │
                        └──────────┬────────────┘
                                   │
              ┌────────────────────┼────────────────────┐
              ▼                    ▼                    ▼
     ┌─────────────┐    ┌──────────────┐     ┌──────────────┐
     │ CDN edge    │    │ Browser HTTP │     │ WebSocket    │
     │ (if added   │    │ cache        │     │ push event:  │
     │  later)     │    │ (60s TTL)    │     │ config_updated│
     │ 60s TTL     │    └──────────────┘     └──────────────┘
     └─────────────┘
```

#### Layer 1: HTTP Cache Headers (passive)

The Go handler sets response headers on `GET /v1/widget/config`:

```go
func (h *WidgetHandler) GetConfig(w http.ResponseWriter, r *http.Request) {
    // ... fetch config from DB ...
    w.Header().Set("Cache-Control", "public, max-age=60, stale-while-revalidate=300")
    w.Header().Set("ETag", fmt.Sprintf(`"%s"`, config.UpdatedAt.Format(time.RFC3339Nano)))
    writeJSON(w, http.StatusOK, configResponse)
}
```

- **`max-age=60`**: Browsers and CDN cache the response for 60 seconds. Most page loads across a customer's site never hit the Go server.
- **`stale-while-revalidate=300`**: For up to 5 minutes after the 60s TTL, browsers serve the stale response immediately while fetching a fresh one in the background. Visitors never see a loading delay.
- **`ETag`**: Based on `updated_at` timestamp. If the config hasn't changed, the server returns `304 Not Modified` — no response body transferred.

This alone handles 95%+ of config requests. The `support_inbox_installations` table has **one row per workspace** — it's a primary key lookup that PostgreSQL handles in <1ms. No Redis needed.

#### Layer 2: sessionStorage (client-side)

The widget caches the config response in `sessionStorage` on first load:

```typescript
const CACHE_KEY = `helpin:config:${widgetKey}`;

async function getWidgetConfig(host: string, widgetKey: string): Promise<WidgetConfig> {
  // Check sessionStorage first
  const cached = sessionStorage.getItem(CACHE_KEY);
  if (cached) {
    const { config, fetchedAt } = JSON.parse(cached);
    // Use cached if < 60s old (matches server max-age)
    if (Date.now() - fetchedAt < 60_000) return config;
  }

  // Fetch from server (browser HTTP cache may serve this)
  const res = await fetch(`https://${host}/v1/widget/config?key=${widgetKey}`);
  const config = await res.json();

  // Cache in sessionStorage
  sessionStorage.setItem(CACHE_KEY, JSON.stringify({
    config,
    fetchedAt: Date.now(),
  }));

  return config;
}
```

- `sessionStorage` is scoped to the browser tab — cleared when the tab closes
- Identity data (`session_token`, `visitor_id`) uses `localStorage` for cross-session persistence
- Config uses `sessionStorage` because it should pick up changes on new visits

#### Layer 3: WebSocket Push (active invalidation)

When an admin changes widget settings in the dashboard, the Go server:

1. Saves to PostgreSQL
2. Publishes a WebSocket event to all connected widget clients for that workspace

**Backend** (in `SupportInboxService.UpdateSettings`):

```go
func (s *SupportInboxService) UpdateSettings(ctx context.Context, workspaceID string, req UpdateSettingsRequest) error {
    // ... validate and save to DB ...

    // Push config update to all connected widget clients
    s.wsPublisher.Broadcast(websocket.Event{
        Action:      "config_updated",
        Entity:      "support_widget_config",
        WorkspaceID: workspaceID,
    })
    return nil
}
```

**Widget** (in `WebSocketWidgetAdapter`):

```typescript
// On receiving config_updated event:
ws.onMessage((event) => {
  if (event.entity === 'support_widget_config' && event.action === 'config_updated') {
    // Clear sessionStorage cache
    sessionStorage.removeItem(CACHE_KEY);
    // Re-fetch and apply new config
    const newConfig = await getWidgetConfig(host, widgetKey);
    applyConfig(newConfig); // Re-render widget with new colors, position, etc.
  }
});
```

This means: when an admin changes the brand color in the Appearance settings page, every open widget on every customer's website updates **within seconds** — no page reload needed.

#### When Each Layer Activates

| Scenario | What happens | Latency |
|---|---|---|
| **First page load** (cold) | Fetches from Go API, caches in sessionStorage + browser HTTP cache | ~100-200ms (network) |
| **Subsequent page load** (same session, <60s) | Served from sessionStorage | 0ms (no network) |
| **Subsequent page load** (same session, 60s-5min) | Browser serves stale from HTTP cache, revalidates in background | 0ms (stale-while-revalidate) |
| **New session** (tab closed and reopened) | sessionStorage cleared, fetches from server (browser HTTP cache may still serve) | 0-200ms |
| **Admin changes settings** (widget has WS open) | WebSocket push → widget re-fetches → applies immediately | <2s |
| **Admin changes settings** (widget has no WS) | Next page load picks up changes via HTTP cache expiry | Up to 60s |

#### Why Not Redis?

| Concern | Why HTTP caching is sufficient |
|---|---|
| **Query cost** | One PK lookup on `support_inbox_installations` — <1ms in PostgreSQL |
| **Traffic volume** | `Cache-Control` headers mean browsers and CDN cache responses. Most requests never reach the server. |
| **Scale** | If thousands of sites load simultaneously, add Cloudflare or similar CDN in front of `sdk.helpin.ai`. The CDN respects `Cache-Control` headers — still no Redis. |
| **Operational cost** | Redis adds another service to deploy, monitor, back up, and maintain. HTTP caching is free and built into every browser and CDN. |
| **Invalidation** | Redis requires explicit cache invalidation logic. HTTP `max-age` + WebSocket push gives both passive expiry and active push — simpler and more reliable. |

Redis becomes justified later if Helpin needs sub-second config reads under sustained >10k req/s to the config endpoint. At that point, a CDN would be the first step, and Redis the second.

#### Industry Comparison

| Provider | Config loading | Cache strategy | Change propagation | Delay |
|---|---|---|---|---|
| **Intercom** | Boot-time fetch | localStorage + cookies | Requires `Intercom('update')` or page reload | Until next page load |
| **Crisp** | Init + WebSocket | Cookies + localStorage | WebSocket RTM push | Instant |
| **Drift** | Config baked into JS bundle | CDN with 5-min cache-busting buckets | Bundle re-fetch on next page load | Up to 5 minutes |
| **Help Scout** | Boot-time fetch | Device ID + localStorage | Client-side override API or page reload | Until next page load |
| **Zendesk** | Boot-time fetch | Cookies + localStorage + sessionStorage | Client-side JS API for dynamic changes | Until next page load |
| **Helpin** (recommended) | Boot-time fetch + HTTP cache | sessionStorage + browser HTTP cache | **WebSocket push** + HTTP cache expiry | **Instant** (WS open) / **<60s** (WS closed) |

Helpin's approach matches Crisp's instant propagation (the best in the industry) while being simpler to implement — Crisp built a dedicated RTM protocol, whereas Helpin reuses the existing WebSocket hub that's already running for chat messages.

### 6.7 Additional Widget and Inbox Features

The following features were identified from a review of the Chatwoot codebase and are standard across modern support products. They are grouped here with implementation details.

#### 6.7.1 Canned Responses

Pre-written reply templates that agents invoke by typing `/` in the reply composer. Essential for agent productivity — avoids retyping common answers.

**Data model** — new table `support_canned_responses`:

| Field | Type | Description |
|---|---|---|
| `id` | uuid | PK |
| `workspace_id` | uuid | FK |
| `short_code` | string | Trigger shortcode, unique per workspace (e.g., `greeting`, `password-reset`) |
| `title` | string | Display name shown in search results |
| `content` | text | Template body. Supports variable interpolation: `{{contact.name}}`, `{{contact.email}}`, `{{agent.name}}` |
| `created_by_id` | uuid | FK to user who created it |
| `created_at` | timestamp | |
| `updated_at` | timestamp | |

**Go files**: `model/support_inbox.go` (struct), `repository/support_inbox.go` (CRUD + search), `service/support_inbox.go` (validation), `handler/support_inbox.go` (endpoints)

**API endpoints**:
- `GET /api/support/inbox/canned-responses` — list all (with `?search=` query)
- `POST /api/support/inbox/canned-responses` — create
- `PUT /api/support/inbox/canned-responses/{id}` — update
- `DELETE /api/support/inbox/canned-responses/{id}` — delete

**Search ranking** (matching Chatwoot's approach):
1. `short_code` starts with query (highest)
2. `short_code` contains query
3. `title` or `content` contains query (lowest)

**Inbox UI**:

```
Reply composer:
+----------------------------------------------------+
| /pass                                              |
+----------------------------------------------------+
| Canned responses:                                  |
| /password-reset                                    |
|   "Hi {{contact.name}}, to reset your password..." |
| /password-change                                   |
|   "You can change your password from Settings..."  |
+----------------------------------------------------+
```

- Typing `/` in the composer triggers a searchable dropdown
- Arrow keys to navigate, Enter or click to insert
- Content replaces the `/shortcode` text in the composer
- Variables are interpolated on send (not on insert — agent can edit first)

**Settings UI**: Add a "Canned Responses" management page under Support Settings group (`/w/:slug/settings/chat-responses`). Simple CRUD table with short_code, title, content columns, and create/edit dialog.

**Phase**: MVP (Phase 3) — critical for agent productivity from day one.

#### 6.7.2 CSAT Survey

Customer satisfaction survey rendered as an inline message in the conversation thread when a conversation is resolved. Not a separate flow — it's a special message type within the existing thread.

**Trigger**: When an agent resolves a conversation, the system automatically sends a CSAT message if `csat_enabled` is `true` in workspace settings (see settings addition below).

**Message representation**: Uses existing `support_conversation_messages` table with:
- `sender_type: system`
- `message_type: csat_survey`
- `metadata` JSONB: `{ "csat_type": "emoji" | "star", "rating": null, "feedback": null, "submitted_at": null }`

When the customer responds, the same message row is updated with their rating and feedback.

**Data model** — add `message_type` enum value `csat_survey` to the message model.

No separate `csat_survey_responses` table needed for MVP — the response data lives in the message's `metadata` JSONB. This keeps things simple and avoids a join table for what is conceptually a single interaction.

**Widget rendering**:

```
+------------------------------------------------+
| [System · just now]                            |
|                                                |
| How would you rate your experience?            |
|                                                |
| [ :( ]  [ :| ]  [ :) ]  [ :D ]  [ <3 ]       |
|                                                |
| (after selection:)                             |
| You rated: :D                                  |
| [Any additional feedback?               ]      |
|                           [Submit feedback]    |
+------------------------------------------------+
```

- 5-point emoji scale (matches Chatwoot's default)
- After selecting a rating, an optional text feedback field appears
- Submits via `PUT /api/widget/support/messages/{id}` updating the metadata
- Prevents re-submission once submitted
- The `CsatRating` widget-core component (already listed in section 2.3) handles this

**Settings addition** — add to `SupportInboxInstallation.Settings` JSONB:

| Field | Type | Default | Description |
|---|---|---|---|
| `csat_enabled` | bool | `true` | Send CSAT survey on conversation resolution |

Add this toggle to the **AI & Routing** settings page (`ChatAITab.tsx`) in a new "Customer Satisfaction" card.

**Phase**: Phase 3 (data model + widget component) / Phase 4 (reporting dashboard).

#### 6.7.3 Typing Indicators

Real-time typing indicators showing when an agent is composing a reply (agent → widget) and when a customer is typing (widget → dashboard).

**Backend mechanism**: Uses the existing WebSocket publisher. No database writes — typing events are ephemeral.

**Events**:

| Direction | Event | Entity | Metadata |
|---|---|---|---|
| Agent typing | `typing_on` / `typing_off` | `support_conversation` | `{ "user_id": "...", "display_name": "Alex" }` |
| Customer typing | `typing_on` / `typing_off` | `support_conversation` | `{ "session_id": "..." }` |

**Agent → Widget flow**:
1. Agent starts typing in reply composer → frontend debounces (300ms) and calls `POST /api/support/inbox/conversations/{id}/typing` with `{ "typing": true }`
2. Handler publishes `typing_on` WebSocket event scoped to the conversation
3. Widget receives event → shows `TypingIndicator` component (bouncing dots)
4. Auto-expires after 30s if no follow-up `typing_on` event

**Widget → Dashboard flow**:
1. Customer starts typing → widget calls `POST /v1/conversations/{id}/typing` with `{ "typing": true }`
2. Handler publishes `typing_on` WebSocket event scoped to the conversation
3. Dashboard `useRealtimeSync` receives event → shows typing indicator below message list
4. Auto-expires after 30s

**API endpoints**:
- `POST /api/support/inbox/conversations/{id}/typing` — dashboard (JWT auth)
- `POST /v1/conversations/{id}/typing` → rewrites to `POST /api/widget/support/conversations/{id}/typing` — widget (session token auth)

**Phase**: MVP (Phase 3) — the `TypingIndicator` widget-core component is already planned; this adds the backend wiring.

#### 6.7.4 Email Transcript

Allows visitors to email themselves a copy of the conversation transcript. Simple feature, useful for reference.

**Widget UI**: When a conversation is resolved, the widget shows a "Email Transcript" button alongside "Start New Conversation":

```
+------------------------------------------------+
| This conversation has been resolved.           |
|                                                |
| [Start New Conversation]  [Email Transcript]   |
+------------------------------------------------+
```

Clicking "Email Transcript" sends the full conversation text to the customer's email address (captured during pre-chat). If no email was captured, the button is hidden.

**Backend**:
- `POST /api/widget/support/conversations/{id}/transcript` — widget endpoint (session token auth)
- The handler formats all public messages (excluding internal notes) into a clean email template
- Sends via Helpin's existing Postmark email client (`internal/email/`)
- Includes: workspace name, conversation ID, all messages with timestamps and sender names, AI source citations if any

**Phase**: Phase 4 — nice to have, not launch-blocking. Low implementation effort since Postmark email is already wired.

#### 6.7.5 Unread Message Overlay

When the widget is closed and an agent (or AI) sends a reply, the widget should show a preview of the unread message near the launcher button — not just a badge count.

**Widget rendering**:

```
                          +------------------------------------+
                          | Alex from Acme Corp:               |
                          | I've reset your password. You      |
                          | should receive an email shortly... |
                          |                              [x]   |
                          +------------------------------------+
                                              [ (bubble icon) 1 ]
```

- Shows the latest unread message as a floating card near the launcher
- Includes sender name and a truncated message preview (max 120 chars)
- Dismiss button `[x]` hides the overlay without opening the widget
- Clicking the overlay opens the widget to that conversation
- Multiple unread messages show count badge on launcher; overlay shows only the latest
- Overlay auto-hides after 15 seconds if not interacted with

**Implementation**:
- The WebSocket connection stays open even when the widget panel is closed (only the UI is hidden, not the connection)
- On `support_conversation_message` event with `sender_type != customer`, increment unread count and show overlay
- Store unread state in `sessionStorage` (cleared when widget is opened)
- The `onUnreadCountChange` SDK callback (section 2.6) fires whenever this count changes

**Phase**: MVP (Phase 3) — important for engagement; ensures visitors notice agent replies.

---

## 7. CRM and Identity Workflow

### 7.1 Contact Matching

When email is captured:

- search existing CRM contacts by email
- if found, link the conversation to that contact
- if not found and workspace settings allow, create a new contact

### 7.2 Initial Contact Lifecycle

Today the support service auto-creates contacts with:

- `lifecycle_stage = subscriber`
- `lead_status = new`
- `source = support`

That behavior is directionally correct for MVP, but the product should expose admin control over whether inbox-captured contacts become:

- no CRM record
- `subscriber`
- `lead`

### 7.3 Manual Promotion

Agents should be able to promote a captured contact to `lead` directly from the inbox or CRM surface.

### 7.4 Future CRM Extensions

Later phases can add:

- company inference from email domain
- deal matching or creation suggestions
- signal detection from support conversations
- CRM suggestions powered by support content

### 7.5 Why CRM Integration Matters for Helpin

Helpin is building a complete suite, so support should not stop at case handling. Inbox identity should naturally feed the relationship system:

- support inquiry -> CRM identity
- CRM identity -> account context
- account context -> PM follow-through

---

## 8. PM Story Escalation Workflow

### 8.1 Primary Escalation Action

The primary operational escalation is:

- **Create Story from Conversation**

Not:

- convert chat to ticket
- duplicate the issue into a second support object

### 8.2 Story Creation Behavior

When an agent creates a story from a conversation, the system should:

- create a PM story in a selected workflow/state
- copy a concise summary from the conversation
- optionally attach transcript excerpts
- preserve customer and CRM context
- link the story back to the conversation

### 8.3 Division of Responsibility

After story creation:

- the **conversation** remains the customer communication system
- the **story** becomes the execution system

This separation is essential and fits Helpin's PM architecture cleanly.

### 8.4 Relationship Model

In MVP:

- a conversation has one primary story link

Later:

- broader associations to multiple stories can be supported through the existing association model

### 8.5 Agent and Workflow Tie-In

Support already has agent-run support in the current codebase. The future model should continue to allow:

- assigned support agents
- draft replies for human approval
- later automation around story creation or classification

But those runs should target the **conversation** domain model once renaming is complete.

---

## 9. Docs, Help Center, and AI Knowledge

### 9.1 Knowledge Sources

Support AI should retrieve from:

- internal Docs content
- external help center docs published from external-capable spaces
- public help center article metadata where useful

### 9.2 Why External Docs Matter

External docs are not just a publishing feature. In Helpin they matter for:

- self-serve support
- public help center
- grounding support AI
- future retrieval/RAG strategy

This should be explicit in the PRD because external docs are already part of the product direction.

### 9.3 Existing Docs Architecture

Helpin already has:

- docs spaces
- docs collections
- docs documents
- docs content
- docs versions
- docs links
- help center config
- help center article publishing
- public article resolution by domain/subdomain/slug

The support product should build on this, not create a separate knowledge base stack.

### 9.4 Retrieval Strategy

#### Phase 1 retrieval

Use pragmatic retrieval first:

- PostgreSQL text search on `docs_contents.content_text`
- joins to document metadata
- optional filtering to published external docs and allowed internal docs

#### Phase 2 retrieval

Helpin uses Neon PostgreSQL, which supports the `pgvector` extension natively. This makes Phase 2 retrieval achievable without introducing a separate vector database:

- document chunking (split `docs_contents.content_text` into overlapping segments)
- embedding generation via LLM provider (using existing `internal/llm/` interface)
- store embeddings in a `docs_content_embeddings` table with `vector(1536)` column (pgvector)
- hybrid retrieval: combine `to_tsvector` lexical search with `<=>` cosine distance vector search
- re-ranking with cross-encoder or LLM-based scoring
- stronger citation confidence with chunk-level provenance
- broader retrieval sources (internal docs, external help center, conversation history)

### 9.5 AI Output Requirements

AI answers should:

- be grounded in docs
- provide source snippets where possible
- hand off cleanly to humans when confidence is low
- never silently fabricate product behavior

### 9.6 Human Handoff Rules

The system should hand off to a human when:

- the user explicitly asks for a person
- confidence is low
- billing/account-sensitive actions are requested
- engineering/product bugs require investigation
- policy/risk boundaries require human review

---

## 10. API Design

### 10.1 Internal Dashboard APIs

Recommended target internal API shape:

- `GET /api/support/inbox/conversations`
- `POST /api/support/inbox/conversations`
- `GET /api/support/inbox/conversations/{id}`
- `PATCH /api/support/inbox/conversations/{id}`
- `GET /api/support/inbox/conversations/{id}/messages`
- `POST /api/support/inbox/conversations/{id}/messages`
- `POST /api/support/inbox/conversations/{id}/typing` (typing indicator — see 6.7.3)
- `POST /api/support/inbox/conversations/{id}/transcript` (email transcript — see 6.7.4)
- `GET /api/support/inbox/canned-responses` (with `?search=` — see 6.7.1)
- `POST /api/support/inbox/canned-responses`
- `PUT /api/support/inbox/canned-responses/{id}`
- `DELETE /api/support/inbox/canned-responses/{id}`
- `POST /api/support/inbox/conversations/{id}/assign`
- `POST /api/support/inbox/conversations/{id}/status`
- `POST /api/support/inbox/conversations/{id}/create-story`
- `GET /api/support/inbox/conversations/{id}/associations`

### 10.2 Public Widget APIs

The existing public endpoint family can remain conceptually similar:

- `GET /api/widget/support/config?widget_key=...`
- `POST /api/widget/support/session`
- `GET /api/widget/support/messages?session_token=...`
- `POST /api/widget/support/messages`

Under the hood, these should move to conversation-first logic.

### 10.3 Compatibility Layer

During migration, temporary compatibility aliases may be needed for:

- `/support/tickets/*`
- existing frontend types
- existing CRM support-ticket endpoints

### 10.4 Association APIs

Support should integrate with the current grouped association model so conversations can surface:

- linked story
- linked CRM objects
- linked docs

### 10.5 Story Escalation API

Add a dedicated action endpoint:

- `POST /api/support/inbox/conversations/{id}/create-story`

Payload should support:

- workflow ID
- workflow state ID
- optional team ID
- story type
- optional assignee/requester defaults

### 10.6 Settings APIs

Support settings should include:

- widget branding
- identity capture rules
- CRM auto-create behavior
- lead conversion default
- AI enablement and handoff controls

---

## 11. Realtime, Notifications, and Agent Workflows

### 11.1 Realtime

Support should continue using the existing WebSocket publisher pattern (`server/internal/websocket/publisher.go` → `Hub.Broadcast`).

#### Backend Event Publishing

The Go support service should publish `websocket.Event` structs for all state changes. The existing `Event` struct supports `Action`, `Entity`, `EntityID`, `WorkspaceID`, `ActorID`, `ParentType`, and `ParentID` — which is sufficient for all support events.

**Events to publish:**

| Action | Entity | ParentType | ParentID | Trigger |
|---|---|---|---|---|
| `created` | `support_conversation` | — | — | New conversation from widget or dashboard |
| `updated` | `support_conversation` | — | — | Status change, assignment change, priority change |
| `created` | `support_conversation_message` | `support_conversation` | `{conversation_id}` | New message (customer, agent, AI, or system) |
| `updated` | `support_conversation` | — | — | Story linked/unlinked |
| `created` | `support_conversation_message` | `support_conversation` | `{conversation_id}` | AI reply drafted (with `sender_type: ai`) |
| `updated` | `support_conversation` | — | — | AI handoff (status or assignment change) |

#### Frontend Realtime Sync

The existing `useRealtimeSync` hook (`frontend/src/hooks/useRealtimeSync.ts`) handles WebSocket events by invalidating TanStack Query caches. It currently supports PM, Docs, notifications, and agent runs but has **no support entity handling**.

**Required additions to `useRealtimeSync`:**

```
// In the onEvent callback, add:
} else if (event.entity === 'support_conversation') {
  queryClient.invalidateQueries({ queryKey: queryKeys.support.conversations(workspaceId) })
  queryClient.invalidateQueries({ queryKey: queryKeys.support.conversation(workspaceId, event.entity_id) })
} else if (event.entity === 'support_conversation_message') {
  if (event.parent_id) {
    queryClient.invalidateQueries({ queryKey: queryKeys.support.messages(workspaceId, event.parent_id) })
    queryClient.invalidateQueries({ queryKey: queryKeys.support.conversation(workspaceId, event.parent_id) })
  }
}
```

This ensures:
- The conversation list refreshes when any conversation changes
- The active conversation thread refreshes when new messages arrive (from any source: customer, agent, AI)
- Multiple agents viewing the same conversation see updates in real time

#### Widget Realtime

The embeddable widget connects to the same WebSocket endpoint but uses session-token auth instead of JWT. Widget clients should receive only events scoped to their active conversation. The widget adapter should:

- Connect via `GET /api/ws?session_token={token}&workspace_id={id}`
- Listen for `support_conversation_message` events where `parent_id` matches the active conversation
- Append new messages to the local message list without requiring a full refetch
- Show typing indicators when an `agent_typing` event is received

### 11.2 Notifications

Helpin already has a notification architecture built around:

- entity-centric notifications
- event tables
- per-channel delivery state

Support should not launch by implementing a full support-specific notification system, but the data model should leave room for:

- assignment notifications
- mentions/internal note notifications
- unresolved queue alerts

### 11.3 Agents

Helpin already supports agent runs and support-targeted agent behavior. The support inbox should leverage this for:

- draft reply generation
- triage assistance
- suggested status updates
- future summarization and categorization

Current code already supports human approval of drafted support replies. That pattern should be preserved and moved to conversation-first targeting.

---

## 12. Implementation Plan

> **Note:** Phase progress is tracked at the top of this document. See the **Phase Progress Tracker** section for current status with checkboxes. The subsections below describe the original deliverables and exit criteria for each phase.

### 12.1 Phase 0: Product and Naming Alignment ✅

Deliverables:

- approve conversation-first support model
- approve story escalation as the primary workflow
- approve docs/external docs as AI knowledge sources
- approve Chi/GORM architecture as canonical

### 12.2 Phase 1: Package Setup, SDK Migration, and Widget-Core Implementation ✅

Deliverables:

- create `packages/` directory with pnpm workspace + Turborepo at repo root
- copy and adapt package structure from `/root/helpin-convex-main`
- adapt `@helpin-ai/shared` types to Helpin conversation-first naming
- extend `@helpin-ai/sdk-js` with `initWidget()` and widget lifecycle methods
- extend `@helpin-ai/react` with `useSupportWidget()` and `useConversation()` hooks
- adapt `@helpin-ai/nextjs` with SSR-safe chat widget hooks
- implement widget-core MVP components: `WidgetLauncher`, `ChatWindow`, `WidgetHeader`, `PreChatForm`, `MessageList`, `MessageBubble`, `ComposeBar`, `QuickReplies`
- implement `WebSocketWidgetAdapter` against Helpin REST + WebSocket APIs
- build `@helpin-ai/widget-embed` IIFE bundle (`pixel.js`) with Preact and shadow DOM isolation

Exit criteria:

- all packages build successfully via `turbo build`
- `pixel.js` bundle exists and is < 80KB gzipped
- widget embed loads on a test page, renders launcher, opens chat window
- analytics tracking continues to work as before

### 12.3 Phase 2: Data Modeling and Go APIs ✅

Deliverables:

- conversation-first model refactor (rename tables, add missing fields including `resolved_at`, `closed_at`, `channel`, `message_type`, `metadata` JSONB)
- add `csat_survey` to `message_type` enum; add CSAT metadata fields to message `metadata` JSONB schema
- `support_canned_responses` table: `short_code`, `title`, `content`, `created_by_id` (see section 6.7.1)
- canned responses CRUD API: `GET/POST/PUT/DELETE /api/support/inbox/canned-responses` with ranked search
- repository/service/handler updates with new `/inbox/conversations` endpoints
- typing indicator endpoint: `POST /api/support/inbox/conversations/{id}/typing` — publishes ephemeral WebSocket event, no DB write
- widget session/message endpoints updated for conversation model
- CRM identity capture logic with workspace settings
- `CreateStory` endpoint (creates new PM story from conversation, not just linking)
- WebSocket event publishing with `support_conversation` and `support_conversation_message` entity names
- Widget WebSocket auth path (session token alongside JWT)
- Chat settings backend: expand `SupportInboxInstallation.Settings` JSONB schema with full settings fields (identity capture, CRM integration, AI behavior, widget appearance, business hours, `csat_enabled` — see sections 6.5, 6.7.2)
- Chat settings API: `GET /api/support/inbox/installations` (read), `PATCH /api/support/inbox/installations` (update), `POST /api/support/inbox/installations/regenerate-key` (regenerate widget key)
- Widget config endpoint: `GET /api/widget/support/config?key={widget_key}` returns public-facing subset of settings (see section 6.5 Widget Config API Response)
- Settings validation in `SupportInboxService`: validate `ai_confidence_threshold` range (0.0–1.0), validate `handoff_team_id` exists when `handoff_behavior` is `assign_to_team`, validate hex color format for `brand_color`

Exit criteria:

- end-to-end conversation create/read/update/message flows work
- CRM contact matching and auto-creation works with configurable defaults
- story creation from conversation works
- realtime events publish correctly for all support state changes
- widget session creates conversation on first message
- chat settings can be read and updated via API
- widget config endpoint returns correct settings subset for a given widget key

### 12.4 Phase 3: Inbox and Widget MVP

Deliverables:

- dashboard inbox page with two-pane layout (list + thread) — see section 5.1 ASCII layouts
- slide-out context drawer (CRM contact, associations, story link, metadata)
- widget MVP with pre-chat capture, message thread, and "Talk to a person" action
- assignment/status flows with auto-set `resolved_at`/`closed_at`
- internal notes in conversation thread
- human handoff path from widget
- canned responses: `/shortcode` composer trigger with ranked search dropdown (see 6.7.1)
- canned responses management page under Support Settings (`/w/:slug/settings/chat-responses`)
- typing indicators: agent ↔ customer bidirectional via WebSocket (see 6.7.3)
- CSAT survey: auto-send `csat_survey` message on resolution, `CsatRating` widget component (see 6.7.2)
- unread message overlay: floating preview card near launcher when widget is closed (see 6.7.5)
- realtime sync in `useRealtimeSync.ts` for support entities
- frontend terminology migration (ticket → conversation throughout)
- Chat Settings — 3 sub-pages under "Support Settings" group (see section 6.5 for ASCII layouts):
  - `ChatGeneralTab.tsx`: Widget Installation + Identity Capture + CRM Integration cards
  - `ChatAITab.tsx`: AI Auto-Reply + Handoff Routing + Business Hours + CSAT toggle cards
  - `ChatAppearanceTab.tsx`: Branding + Launcher + live widget Preview cards
- Register `chat-general`, `chat-ai`, `chat-appearance` sections in `SETTINGS_SECTIONS`
- Widget key display with copy-to-clipboard, embed code snippet, and regenerate key
- Settings save flow via `PATCH /api/support/inbox/installations`

Exit criteria:

- a website visitor can start a conversation via the embedded widget
- pre-chat identity capture creates/matches a CRM contact
- an agent can respond in the inbox and see updates in real time
- the context drawer shows CRM contact info and allows story creation
- the conversation can be escalated to a PM story
- multiple agents viewing the same conversation see updates live
- agents can use `/shortcode` to insert canned responses in the reply composer
- typing indicators show in both widget and dashboard in real time
- CSAT survey appears in widget when conversation is resolved
- unread message overlay shows near launcher when widget is closed
- workspace admins can configure chat settings from `/w/:slug/settings/chat`
- widget respects workspace settings (identity capture rules, branding, AI toggle, business hours)

### 12.5 Phase 4: Docs-Aware AI Support

Deliverables:

- wire existing `DocsSearchService.Search()` and `DocsSearchService.PublicSearch()` into new `SupportAIService`
- AI response flow: customer message → docs retrieval → LLM provider (via existing `internal/llm/` interface) → AI message with source citations
- `sender_type: ai` and `message_type: ai_answer` message creation with `metadata` JSONB (doc IDs, titles, snippets, confidence)
- handoff detection logic (explicit user request, low confidence, billing/account keywords)
- Temporal workflow for async AI response processing
- `StreamingText` widget-core component (TypingIndicator already shipped in Phase 3)
- GIN index on `docs_contents.content_text` for production-scale search
- widget displays AI source citations inline
- email transcript: `POST /api/widget/support/conversations/{id}/transcript` sends formatted transcript via Postmark (see 6.7.4)
- CSAT reporting dashboard: aggregate ratings by agent, by time period, feedback review

Exit criteria:

- AI answers are grounded in docs (internal + externally published help center)
- source citations are visible in both widget and dashboard
- handoff to human agent works reliably on low confidence or explicit request
- AI response latency is acceptable (< 5s for non-streaming, streaming starts < 1s)
- visitors can email themselves a conversation transcript
- CSAT data is visible in reporting

### 12.6 Phase 5: Advanced Automation and Operations

Deliverables:

- SLA rules
- richer routing
- more advanced RAG (chunking, embeddings, hybrid ranking)
- analytics/reporting (response times, resolution rates)
- analytics pixel data feeding CRM signal detection
- additional channels

---

## 13. Detailed MVP Requirements

### 13.1 Widget Requirements

- embeddable script/widget bundle
- configurable launcher and branding
- email then name capture
- AI-first reply
- human fallback
- session persistence

### 13.2 Inbox Requirements

- list of conversations
- filter by status
- view conversation thread
- send public reply
- send internal note
- assign agent
- update status
- create story from conversation
- view associations/context

### 13.3 CRM Requirements

- match contact by email
- create contact when configured
- support subscriber/lead default logic
- display contact context in inbox

### 13.4 Docs/AI Requirements

- retrieve from docs corpus
- retrieve from externally published docs
- attach sources to AI answers
- hand off when uncertain

### 13.5 Architecture Requirements

- React/Vite/TanStack frontend
- Go/Chi/GORM backend
- reuse existing websocket publisher
- avoid Fiber-specific design dependencies

---

## 14. Known Gaps from Current Implementation

> **Note:** Action items marked `[x]` have been completed in the indicated phase. Remaining `[ ]` items are assigned to future phases. See the **Phase Progress Tracker** at the top for a consolidated view.

The current codebase has a working MVP based on ticket terminology. The following gaps must be addressed to meet PRD requirements:

### 14.1 Data Model Gaps

The current model (`support_tickets`, `support_messages`) uses ticket terminology. Required updates:

- **Missing conversation fields**: `channel`, `first_response_mode`, `ai_handoff_reason`, `last_message_at`, `resolved_at`, `closed_at`
- **Missing message fields**: `message_type` (`public`, `internal_note`, `system_event`, `ai_answer`), `metadata` JSONB for AI sources/confidence scores
- **Missing table renames**: `support_tickets` → `support_conversations`, `support_messages` → `support_conversation_messages`
- **Widget tables**: `support_widget_installations` → `support_inbox_installations`, `support_widget_sessions` → `support_inbox_sessions`

**Action Items:**
- [x] ~~Rename `server/internal/model/support.go` → `server/internal/model/support_inbox.go`~~ — Kept as `support.go` but renamed structs (Phase 2)
- [x] Add missing fields to `SupportConversation`: `channel`, `first_response_mode`, `ai_handoff_reason`, `last_message_at`, `resolved_at`, `closed_at` (Phase 2)
- [x] Add missing fields to `SupportConversationMessage`: `message_type`, `metadata` JSONB (Phase 2)
- [x] Create SQL migration to rename tables and add columns — `038_support_conversations.sql` (Phase 2)
- [ ] Update `CRMObjectSupportTicket` constant to `CRMObjectSupportConversation` in `server/internal/model/crm_association.go` → Phase 3
- [ ] Rename repository, service, and handler files and structs per section 3.4 and 4.8 mapping → Phase 3
- [ ] Update `UpdateStatus` in `server/internal/service/support_inbox.go` to auto-set `resolved_at` when status transitions to `resolved` and `closed_at` when status transitions to `closed` → Phase 3

### 14.2 API Endpoint Gaps

Current endpoints use `/api/support/tickets/*`. Required updates:
- Add `/api/support/inbox/conversations` endpoints
- Add `/api/support/inbox/conversations/{id}/create-story` for creating new PM stories (not just linking existing)
- Current `LinkStory` only links to existing stories

**Action Items:**
- [ ] Rename `server/internal/handler/support.go` → `server/internal/handler/support_inbox.go`; rename `server/internal/handler/widget.go` → `server/internal/handler/support_inbox_widget.go`
- [ ] Add new endpoints with `/inbox/conversations` route in renamed handler
- [ ] Add `CreateStoryFromConversation` method to `server/internal/service/support_inbox.go` that creates a new PM story, copies conversation summary, and links back via `primary_story_id`
- [ ] Update router in `server/internal/router/router.go` to register new endpoints under `/api/support/inbox/conversations`
- [ ] Keep legacy `/support/tickets/*` endpoints with deprecation notices for backward compatibility

### 14.3 Widget Gaps

Widget implementation is incomplete:
- No pre-chat identity capture enforcement (email required, name required)
- No AI-first response flow
- No branding/theme configuration in widget settings
- No welcome state / launcher customization
- Still creates "ticket" on first message (should be conversation)

**Action Items:**
- [ ] Implement pre-chat form in widget with email/name capture based on workspace settings
- [ ] Add AI response flow that queries docs service and streams responses
- [ ] Add "Talk to Human" button that triggers handoff
- [ ] Add branding config to `SupportInboxInstallation.Settings` JSONB
- [ ] Update widget to create `support_conversation` instead of `support_ticket`

### 14.4 AI/Docs Integration Gaps

Docs retrieval infrastructure already exists but is not wired to support:
- `DocsSearchService.Search()` performs full-text search on `docs_contents.content_text` using PostgreSQL `to_tsvector`/`to_tsquery` with weighted ranking (title=A, content=B)
- `DocsSearchService.PublicSearch()` searches externally published help center articles (filters by `public_published_at IS NOT NULL`)
- Plain text is auto-extracted from TipTap JSON on every content save via `extractPlainText()`

Not implemented:
- No wiring of existing docs search into support AI response flow
- No confidence scoring or source citations in messages
- No `sender_type: ai` messages
- No human handoff logic (AI → human)
- No message metadata for AI sources

**Action Items:**
- [ ] Wire existing `DocsSearchService.Search()` and `DocsSearchService.PublicSearch()` into a new `SupportInboxAIService` in `server/internal/service/support_inbox_ai.go`
- [ ] Build AI response flow: receive customer message → search docs → send context + message to LLM provider (via existing `internal/llm/` interface) → create AI message
- [ ] Implement AI message creation with `sender_type: ai` and `message_type: ai_answer`
- [ ] Add `metadata` JSONB to messages for AI sources (doc ID, title, snippet, confidence score)
- [ ] Add handoff detection logic: explicit user request ("talk to a person"), low LLM confidence, billing/account keywords
- [ ] Create Temporal workflow for async AI response processing to avoid blocking the widget message endpoint
- [ ] Add GIN index on `docs_contents.content_text` for production-scale search performance: `CREATE INDEX idx_docs_content_fts ON docs_contents USING GIN(to_tsvector('english', content_text))`

### 14.5 Package Migration ✅ COMPLETE (Phase 1)

PRD specifies packages to copy from `/root/helpin-convex-main`:
- `@helpin-ai/shared`, `@helpin-ai/widget-core`, `@helpin-ai/sdk-js`, `@helpin-ai/react`, `@helpin-ai/nextjs`, `@helpin-ai/widget-embed`

**All packages created and adapted in Phase 1.**

**Action Items:**

Setup:
- [x] Create `packages/` directory at repo root (Phase 1)
- [x] Add `pnpm-workspace.yaml` at repo root with `packages: ["packages/*"]` (Phase 1)
- [x] Add `turbo.json` at repo root for build orchestration (Phase 1)
- [x] Each package gets its own `package.json`, `tsconfig.json`, and Vite/Rollup build config (Phase 1)

Package adaptation (after copy):
- [x] `packages/shared/` — aligned types with Go model fields (Phase 1)
- [x] `packages/widget-core/` — all 11 components implemented with tests (Phase 1)
- [x] `packages/sdk-js/` — JS API, WebSocket management, analytics pipeline (Phase 1)
- [x] `packages/react/` — analytics hooks + support widget hooks (Phase 1)
- [x] `packages/nextjs/` — SSR-safe wrapper with no-op server fallbacks (Phase 1)
- [x] `packages/widget-embed/` — Vite IIFE build producing `pixel.js` with shadow DOM (Phase 1)

Widget-core implementation:
- [x] Implement `WidgetLauncher`: floating button with workspace branding, unread badge (Phase 1)
- [x] Implement `ChatWindow`: container with open/close animation, shadow DOM isolation (Phase 1)
- [x] Implement `WidgetHeader`: workspace name/logo, close button (Phase 1)
- [x] Implement `PreChatForm`: email + name inputs with validation (Phase 1)
- [x] Implement `MessageList`: scrollable thread with date separators, smart auto-scroll (Phase 1)
- [x] Implement `MessageBubble`: customer/AI/agent message styling, XSS-safe (Phase 1)
- [x] Implement `ComposeBar`: text input with send button, enter-to-send (Phase 1)
- [x] Implement `QuickReplies`: quick reply buttons with ARIA roles (Phase 1)
- [ ] Implement `WebSocketWidgetAdapter`: connect to Helpin REST + WebSocket APIs → Phase 3

### 14.6 Workspace Settings Gaps

The current `SupportInboxInstallation.Settings` JSONB is a bare struct with no defined fields. The full settings schema (section 6.5) requires 20+ fields across 6 categories:

**Missing settings fields** (full schema and defaults in section 6.5):
- Identity Capture: `require_email_before_chat`, `require_name_after_email`, `welcome_message`
- CRM Integration: `auto_create_crm_contact`, `default_lifecycle_stage`, `auto_promote_to_lead`
- AI Behavior: `ai_enabled`, `ai_confidence_threshold`, `show_talk_to_human`, `handoff_behavior`, `handoff_team_id`
- Widget Appearance: `brand_color`, `launcher_position`, `launcher_icon`, `show_branding`
- Business Hours: `business_hours_enabled`, `business_hours_timezone`, `business_hours_schedule`, `outside_hours_message`

**Missing frontend:**
- No Chat Settings pages exist — need 3 tab components (see section 6.5 for ASCII layouts per page)
- No `chat-general`, `chat-ai`, `chat-appearance` sections registered in `SETTINGS_SECTIONS`

**Action Items (Backend):**
- [ ] Define `SupportInboxSettings` Go struct with all 20+ fields and JSON tags in `server/internal/model/support_inbox.go`
- [ ] Add settings validation in `server/internal/service/support_inbox.go`: threshold range (0.0–1.0), handoff team exists when `assign_to_team`, hex color format, business hours schedule structure
- [ ] Add `GET /api/support/inbox/installations` endpoint (returns full settings for dashboard)
- [ ] Add `PATCH /api/support/inbox/installations` endpoint (partial update — all 3 settings pages save to this endpoint)
- [ ] Add `POST /api/support/inbox/installations/regenerate-key` endpoint (regenerate widget key)
- [ ] Update `GET /api/widget/support/config` to return public-facing subset (see section 6.5 Widget Config API Response)
- [ ] Add `is_online` computation: check `business_hours_enabled` + `business_hours_schedule` against current time in configured timezone

**Action Items (Frontend — 3 settings pages):**
- [ ] Create `frontend/src/components/settings/ChatGeneralTab.tsx` — 3 Cards: Widget Installation (key + embed snippet + active toggle), Identity Capture (email + name + welcome message), CRM Integration (auto-create + lifecycle stage + auto-promote)
- [ ] Create `frontend/src/components/settings/ChatAITab.tsx` — 3 Cards: AI Auto-Reply (enabled + threshold + talk-to-human), Handoff Routing (behavior + team), Business Hours (enabled + timezone + schedule + outside message)
- [ ] Create `frontend/src/components/settings/ChatAppearanceTab.tsx` — 3 Cards: Branding (color + show branding), Launcher (position + icon), Preview (live widget preview with current settings)
- [ ] Register 3 sections in `SETTINGS_SECTIONS` under "Support Settings" group: `chat-general` (General), `chat-ai` (AI & Routing), `chat-appearance` (Appearance)
- [ ] Add `renderSection()` cases in `Settings.tsx` for all 3 new section IDs
- [ ] Update `SettingsSection` type union to include `'chat-general' | 'chat-ai' | 'chat-appearance'`
- [ ] Add TanStack Query hooks: `useChatSettings(workspaceId)`, `useUpdateChatSettings(workspaceId)`, `useRegenerateWidgetKey(workspaceId)` in `frontend/src/hooks/queries/`
- [ ] Widget key display with copy-to-clipboard button and auto-generated embed snippet
- [ ] Barrel export all 3 new tabs from `frontend/src/components/settings/index.ts`

### 14.7 CRM Association Gaps

Current: direct `crm_contact_id` on ticket.
Required: support company/deal associations via `crm_associations` table.

**Action Items:**
- [ ] Add `CRMObjectSupportConversation` constant in `server/internal/model/crm_association.go`
- [ ] Update association queries in `server/internal/service/associations.go` to support conversations
- [ ] Add company/deal association UI in `frontend/src/pages/pm/Support.tsx` context panel
- [ ] Update `AssociationsPanel` to work with `support_conversation` object type

### 14.8 Realtime Events and Frontend Sync

**Backend:** Currently publishes `support_ticket`, `support_message`. Required: rename to `support_conversation`, `support_conversation_message`.

**Frontend:** `useRealtimeSync.ts` currently handles PM entities (`story`, `epic`, `sprint`, `objective`), Docs entities (`docs_document`, `docs_space`, `docs_collection`), `notification`, and `agent_run`. It has **zero support entity handling** — support events are received via WebSocket but silently ignored.

**Action Items (Backend):**
- [ ] Update all `websocket.Event` entity names in `server/internal/service/support_inbox.go`: `"support_ticket"` → `"support_conversation"`, `"support_message"` → `"support_conversation_message"`
- [ ] Publish events for all state changes: conversation create/update, message create, assignment change, story link
- [ ] Add widget WebSocket auth path: accept `session_token` query param in `GET /api/ws` alongside existing JWT `token` param
- [ ] Scope widget WebSocket clients to receive only events for their active conversation (filter by `ParentID` or `EntityID` match)

**Action Items (Frontend — Dashboard):**
- [ ] Add `support_conversation` and `support_conversation_message` handling to `useRealtimeSync.ts` `onEvent` callback
- [ ] On `support_conversation` events: invalidate `queryKeys.support.conversations(workspaceId)` and `queryKeys.support.conversation(workspaceId, event.entity_id)`
- [ ] On `support_conversation_message` events: invalidate `queryKeys.support.messages(workspaceId, event.parent_id)` and `queryKeys.support.conversation(workspaceId, event.parent_id)` (to update `last_message_at` in list)
- [ ] Update `queryKeys.ts`: rename `tickets` → `conversations`, `ticket` → `conversation`, `ticketAssociations` → `conversationAssociations`, `messages` key to use `conversation_id`
- [ ] Dispatch DOM custom events (`support_conversation-created`, `support_conversation_message-created`) for component listeners

**Action Items (Frontend — Widget):**
- [ ] Implement WebSocket connection in `WebSocketWidgetAdapter` using session token auth
- [ ] On `support_conversation_message` events matching active conversation: append message to local list
- [ ] Show typing indicator on agent/AI typing events
- [ ] Reconnect with exponential backoff on connection drop

### 14.9 Frontend Terminology

SupportPage.tsx still uses "New Ticket" and ticket terminology throughout. Requires conversation-first terminology update.

**Action Items:**
- [ ] Update `frontend/src/pages/pm/Support.tsx`: Change "New Ticket" button to "New Conversation"
- [ ] Update all UI labels: "ticket" → "conversation", "tickets" → "conversations"
- [ ] Update `frontend/src/lib/pmTypes.ts`: Rename `SupportTicket` to `SupportConversation`, `SupportMessage.ticket_id` to `conversation_id`, `TicketStatus` to `ConversationStatus`, `TicketPriority` to `ConversationPriority`
- [ ] Update `frontend/src/lib/services/supportService.ts`: Rename methods (`listTickets` → `listConversations`, etc.) and point to new API endpoints
- [ ] Update `frontend/src/lib/queryKeys.ts`: Rename keys (`tickets` → `conversations`, `ticket` → `conversation`, `ticketAssociations` → `conversationAssociations`)

### 14.10 Widget Embed Build Pipeline ✅ COMPLETE (Phase 1)

The `@helpin-ai/widget-embed` package produces a single `pixel.js` IIFE bundle for embedding via `<script>` tag.

**Action Items:**
- [x] Create `packages/widget-embed/` with Vite IIFE build config (Phase 1)
- [x] Use Preact for lightweight rendering (Phase 1)
- [x] `main.tsx` reads `data-widget-key`, creates shadow DOM container, mounts widget (Phase 1)
- [x] CSS isolation via shadow DOM (Phase 1)
- [x] Widget script loadable async (Phase 1)
- [ ] Add CDN deployment step to CI/CD pipeline for `pixel.js` versioned releases → Phase 3
- [x] Widget bundle size target met: 0.25 KB gzipped (Phase 1)

### 14.11 Inbox Layout Upgrade

Current `SupportPage.tsx` is a two-pane layout (list + thread) without a context panel. PRD specifies a two-pane default with a slide-out context drawer.

**Action Items:**
- [ ] Add a "Details" icon button to the conversation header in `frontend/src/pages/pm/Support.tsx`
- [ ] Implement context drawer using shadcn `Sheet` component (slides from right)
- [ ] Populate drawer with: CRM contact summary, company/deal associations, linked PM story (or "Create Story" button), AI source references, conversation metadata, inline-editable status/priority/assignee
- [ ] On mobile: drawer opens as full-screen overlay
- [ ] Drawer state (open/closed) managed via local component state, not persisted

### 14.12 RBAC Permissions

Support routes currently use PM permissions (`PermPMRead` / `PermPMEdit`). No dedicated support permissions exist.

**Action Items:**
- [ ] Add `PermSupportRead`, `PermSupportEdit`, `PermSupportAdmin` constants to `server/internal/authorization/permissions.go`
- [ ] Add new permissions to `AllPermissions()` slice
- [ ] Update the role-permission matrix to grant support permissions (viewer: read, member: read+edit, manager+: read+edit+admin)
- [ ] Update `server/internal/router/router.go` to use `PermSupportRead` / `PermSupportEdit` / `PermSupportAdmin` on support routes instead of `PermPMRead` / `PermPMEdit`
- [ ] Update frontend permission type union in `frontend/src/lib/types.ts` to include `support.read`, `support.edit`, `support.admin`

### 14.13 DI Wiring in main.go

`server/cmd/api/main.go` wires all support repositories, services, and handlers. All renames must be reflected here.

**Action Items:**
- [ ] Rename repository constructors: `NewSupportTicketRepository` → `NewSupportConversationRepository`, `NewSupportMessageRepository` → `NewSupportConversationMessageRepository`, `NewWidgetInstallationRepository` → `NewSupportInboxInstallationRepository`, `NewWidgetSessionRepository` → `NewSupportInboxSessionRepository`
- [ ] Rename service constructor: `NewSupportService` → `NewSupportInboxService`; add `NewSupportInboxAIService` wiring with `DocsSearchService` and LLM provider
- [ ] Rename handler constructors: `NewSupportHandler` → `NewSupportInboxHandler`, `NewWidgetHandler` → `NewSupportInboxWidgetHandler`
- [ ] Update `AutoMigrate` calls to use renamed model structs
- [ ] Update `agentService` wiring (it receives `supportTicketRepo` and `supportMessageRepo` — rename references)
- [ ] Update `associationsService` wiring (it receives `supportTicketRepo` — rename reference)

### 14.14 Agent Run Integration

The existing agent run system already supports support targets. `RunTicketAgent` creates runs with `targetType: "support_ticket"` and `approvalState: "pending"` (support runs always require human approval). `ApproveRun` posts the agent's draft reply as a `SupportMessage` with `sender_type: "agent"`.

**Action Items:**
- [ ] Rename `RunTicketAgent` → `RunConversationAgent` in `server/internal/service/agent.go`
- [ ] Update `targetType` from `"support_ticket"` to `"support_conversation"` throughout agent service and Temporal activities
- [ ] Update approval flow: `ApproveRun` should post to `SupportConversationMessage` (renamed table/struct)
- [ ] Update `validateAgentTarget` to accept `"support_conversation"` instead of `"support_ticket"`
- [ ] Update Temporal queue constant `QueueAgentSupport` description/comments to reference conversations
- [ ] Update `ListTicketMessages` activity function name to `ListConversationMessages` in `server/internal/temporalapp/activities.go`
- [ ] Update `frontend/src/pages/pm/Support.tsx`: agent run UI already works, but update the run filter from `target_type === 'support_ticket'` to `target_type === 'support_conversation'`
- [ ] Update `frontend/src/components/pm/AssociationsPanel.tsx`: change `AssociationsObjectType` to include `'support_conversation'` instead of `'support_ticket'`; update tab config

### 14.15 Docs Link Object Type

The `docs_links` model already supports `support_ticket` as a `LinkedObjectType` via `LinkedObjectSupportTicket = "support_ticket"`. This needs renaming.

**Action Items:**
- [ ] Rename `LinkedObjectSupportTicket` → `LinkedObjectSupportConversation` with value `"support_conversation"` in `server/internal/model/docs.go`
- [ ] Create migration to update existing `docs_links` rows: `UPDATE docs_links SET linked_object_type = 'support_conversation' WHERE linked_object_type = 'support_ticket'`

### 14.16 Canned Responses, CSAT, Typing, Transcript, Unread Overlay

Features identified from Chatwoot codebase review (section 6.7). None exist in the current implementation.

**Action Items (Canned Responses — Backend ✅ Phase 2, Frontend → Phase 3):**
- [x] Create `SupportCannedResponse` model with `short_code`, `title`, `content`, `created_by_id` (Phase 2)
- [x] Add GORM migration for `support_canned_responses` table with unique index on `(workspace_id, short_code)` (Phase 2)
- [x] Add repository methods: `List`, `Search` (ranked), `Create`, `Update`, `Delete` (Phase 2)
- [x] Add 4 handler endpoints: `GET/POST/PUT/DELETE /api/support/inbox/canned-responses` (Phase 2)
- [ ] Frontend: canned response management page at `/w/:slug/settings/chat-responses` → Phase 3
- [ ] Frontend: `/` trigger in reply composer — debounced search, dropdown, Enter to insert → Phase 3
- [ ] Support variable interpolation on send: `{{contact.name}}`, `{{contact.email}}`, `{{agent.name}}` → Phase 3

**Action Items (CSAT Survey — Phase 3):**
- [ ] Add `csat_survey` to `message_type` enum in `server/internal/model/support_inbox.go`
- [ ] Add `csat_enabled` field to `SupportInboxSettings` struct (default: `true`)
- [ ] On conversation resolution: if `csat_enabled`, auto-create a system message with `message_type: csat_survey` and `metadata: { csat_type: "emoji", rating: null, feedback: null }`
- [ ] Add `PUT /api/widget/support/messages/{id}` endpoint for CSAT submission (updates metadata with rating + feedback + submitted_at)
- [ ] Implement `CsatRating` widget-core component: 5-point emoji scale, optional feedback text, submit button, prevent re-submission
- [ ] Add CSAT toggle to `ChatAITab.tsx` settings page in a "Customer Satisfaction" card

**Action Items (Typing Indicators — Backend ✅ Phase 2, Frontend → Phase 3):**
- [x] Add `POST /api/support/inbox/conversations/{id}/typing` handler (JWT auth, dashboard) (Phase 2)
- [ ] Add `POST /api/widget/support/conversations/{id}/typing` handler (session token auth, widget) → Phase 3
- [x] Handler publishes ephemeral `typing_on`/`typing_off` WebSocket events — no database write (Phase 2)
- [ ] Frontend: debounce typing detection (300ms) in reply composer, call typing endpoint → Phase 3
- [ ] Frontend: show typing indicator in conversation thread (agent name + bouncing dots) → Phase 3
- [ ] Widget: show `TypingIndicator` component when `typing_on` event received, auto-dismiss after 30s → Phase 3
- [ ] Add `typing_on`/`typing_off` to `useRealtimeSync.ts` event handling → Phase 3

**Action Items (Email Transcript — Phase 4):**
- [ ] Add `POST /api/widget/support/conversations/{id}/transcript` handler (session token auth)
- [ ] Format all public messages (exclude internal notes) into HTML email template
- [ ] Include: workspace name, conversation display ID, messages with timestamps + sender names, AI source citations
- [ ] Send via existing Postmark email client (`server/internal/email/`)
- [ ] Widget: show "Email Transcript" button on resolved conversations (only if customer email is known)

**Action Items (Unread Message Overlay — Phase 3):**
- [ ] Widget: when panel is closed and WS receives `support_conversation_message` with `sender_type != customer`, show floating preview card near launcher
- [ ] Card content: sender name, truncated message (120 chars max), dismiss `[x]` button
- [ ] Click overlay → open widget to that conversation
- [ ] Auto-hide after 15 seconds if not interacted with
- [ ] Track unread count in `sessionStorage`, clear when widget panel opens
- [ ] Fire `onUnreadCountChange` SDK callback (already defined in section 2.6)

---

## 15. Risks and Mitigations

### Risk: Mixed terminology during migration

Mitigation:

- move new product language to `conversation` immediately
- maintain temporary API compatibility only where required

### Risk: Support grows as a silo

Mitigation:

- specify CRM, PM, Docs, Notifications, and Agents integration explicitly

### Risk: RAG scope delays MVP

Mitigation:

- use PostgreSQL/text search retrieval first
- phase deeper vector retrieval later

### Risk: Frontend package reuse drifts from Helpin UI conventions

Mitigation:

- reuse package structure and adapters
- implement UI natively within Helpin design/system patterns

### Risk: Story escalation becomes duplicate issue tracking

Mitigation:

- keep conversation as communication layer
- keep story as execution layer
- do not create a second support object

---

## 16. Acceptance Criteria

- The PRD uses a single canonical support object: **conversation**.
- Support is specified as part of the Helpin suite, with explicit PM, CRM, Docs, and Agent integration.
- Anonymous widget users are captured with **email first, then name**.
- Workspace admins can control whether captured users become subscribers or leads.
- The PRD explicitly states that escalation is to **PM story**, not support-ticket conversion.
- The PRD explicitly states that external docs are part of the AI retrieval strategy.
- The PRD aligns the implementation with Helpin's current architecture instead of Fiber.
- The PRD preserves an original-style comprehensive structure rather than a short replacement memo.

---

## 17. Final Recommendation

Helpin should proceed with a **conversation-first support inbox** that sits naturally inside the broader suite.

This is the cleanest long-term model because:

- customer communication stays in one thread
- CRM identity is captured and enriched early
- AI is grounded in Helpin Docs and external help center content
- execution work is tracked in PM stories
- the implementation stays aligned with the architecture Helpin already has

That gives Helpin a support product that behaves like a native part of the platform rather than a bolted-on chat tool.
