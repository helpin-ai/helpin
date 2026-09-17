# Widget Feature Parity Analysis: Helpin vs Crisp.chat

> Historical comparison from March 2026, not a current feature inventory.
> For example, widget-core now exports an emoji catalog, image lightbox, and
> help-center browsing views that this table marks absent or limited. Use
> [widget architecture](widget-architecture.md) and the
> [component reference](../packages/widget-core/README.md) for current code.


**Date:** 2026-03-22  
**Reference:** Crisp.chat Widget v4.4.4 (db1a904, March 10, 2026)  
**Analysis Scope:** Customer support chat widget capabilities

---

## Executive Summary

Crisp.chat is a mature, feature-rich chat widget with ~27 major feature categories. Helpin's SDK (`@helpin-ai/sdk-js`) is well-architected with WebSocket real-time messaging, AI indicators, and session management, but lacks many consumer expectations for modern support widgets. The most critical gaps are **online/offline status**, **identity verification (OTP)**, **push notifications**, and **emoji/GIF pickers**.

---

## Feature Parity Matrix

| Feature | Crisp.chat | Helpin Widget | Status | Priority |
|---------|-----------|---------------|--------|----------|
| Real-time messaging (WebSocket) | ✅ | ✅ (SDK) / ❌ (legacy polling) | Good | — |
| Pre-chat form (email/name capture) | ✅ | ✅ | Good | — |
| Typing indicators (bidirectional) | ✅ | ✅ | Good | — |
| AI thinking indicator | ✅ | ✅ | Good | — |
| CSAT rating | ✅ | ✅ | Good | — |
| Multiple conversations list | ✅ | ✅ | Good | — |
| File attachments | ✅ | ⚠️ Partial | Medium | Medium |
| Message status (sent/delivered/read) | ✅ | ⚠️ Sending only | Medium | Medium |
| Emoji picker | ✅ | ❌ | High | **HIGH** |
| GIF picker | ✅ | ❌ | High | **HIGH** |
| Audio messages (voice recording) | ✅ | ❌ | High | **HIGH** |
| Video playback | ✅ | ❌ | Low | Low |
| Image magnify/lightbox | ✅ | ❌ | Medium | Medium |
| Code syntax highlighting | ✅ | ❌ | Medium | Medium |
| Message editing | ✅ | ❌ | Medium | Medium |
| Message translation | ✅ | ❌ | Low | Low |
| Quick replies (suggested actions) | ✅ | ⚠️ Partial | Medium | Medium |
| Help center article search/browse | ✅ | ⚠️ showArticle only | Medium | **HIGH** |
| Phone number capture | ✅ | ⚠️ preChatForm only | Medium | Medium |
| Identity verification (OTP code) | ✅ | ❌ | High | **HIGH** |
| Online/offline status display | ✅ | ❌ | High | **HIGH** |
| Push notifications | ✅ | ❌ | High | **HIGH** |
| Sound notifications toggle | ✅ | ⚠️ On by default, no toggle | Low | Low |
| Carousels/rich content | ✅ | ❌ | Medium | Medium |
| Interactive pickers | ✅ | ❌ | Low | Low |
| Message link previews | ✅ | ⚠️ Plain links only | Medium | Medium |
| Internal notes (agent-only) | ✅ | ❌ | Medium | Medium |
| Dark/light theme | ✅ | ⚠️ Limited | Low | Low |
| Widget position customization | ✅ | ⚠️ Bottom corners only | Low | Low |
| Overlay mode (Ctrl+K) | ✅ | ❌ | Medium | Medium |
| Keyboard shortcuts | ✅ | ❌ | Low | Low |
| GDPR/cookie consent | ✅ | ❌ | Medium | Medium |
| Screen sharing/co-browsing | ✅ | ❌ | Low | Low |
| Game during wait | ✅ | ❌ | Low | Low |
| Conversation search | ✅ | ❌ | Medium | Medium |
| AI sources/citations display | ✅ | ⚠️ Metadata only | Medium | Medium |
| Markdown/rich text rendering | ✅ | ⚠️ Basic | Low | Low |
| Rich message forms (field inputs) | ✅ | ❌ | Medium | Medium |

---

## Priority Breakdown

### ULTRA-HIGH Priority

These features represent fundamental expectations for modern support widgets. Their absence creates immediate trust and usability issues.

| Feature | Why It Matters |
|---------|---------------|
| **Online/offline status** | Users don't know if agents are available; leads to abandonment or frustration when expecting instant replies |
| **Identity verification (OTP)** | Critical for lead quality, prevents fake accounts, enables trusted communication |
| **Push notifications** | Users miss messages when widget is closed; the widget becomes invisible when not in focus |

### HIGH Priority

These features are established consumer expectations and significant gaps vs Crisp.chat.

| Feature | Why It Matters |
|---------|---------------|
| **Emoji picker** | Baseline UX expectation; text-only input feels crippled |
| **GIF picker** | Baseline UX expectation; heavily used in consumer-facing support |
| **Audio/voice messages** | Growing expectation; useful for hands-free communication |
| **Help center article search/browse** | Core self-service functionality; reduces agent load by enabling DIY resolution |
| **Identity verification** | Crisp has email+phone OTP; Helpin has neither |

### MEDIUM Priority

Important quality-of-life improvements that differentiate good from great widgets.

| Feature | Why It Matters |
|---------|---------------|
| **File upload improvements** | Progress indicators, drag-drop, preview thumbnails |
| **Message status (read receipts)** | Users don't know if their messages were seen |
| **Code syntax highlighting** | Critical for tech support use cases |
| **Message editing** | Common expectation; prevents awkward "edit: xyz" workarounds |
| **Quick replies improvements** | Rich interactive buttons for guided conversations |
| **Image magnify/lightbox** | Better media viewing experience |
| **Conversation search** | Better UX for returning users with history |
| **Markdown/rich text** | Better message formatting for longer responses |
| **Carousels** | Rich content display for products, galleries, etc. |
| **Internal notes** | Agent-to-agent communication within conversations |

### LOW Priority

Nice-to-have but not critical differentiators.

| Feature | Why It Matters |
|---------|---------------|
| **Video playback** | Niche use case; most support is text/image |
| **Message translation** | Useful for multilingual support; not core to MVP |
| **Interactive pickers** | Crisp-specific; uncommon outside their product |
| **Keyboard shortcuts** | Power user feature; not expected in support widgets |
| **Overlay mode (Ctrl+K)** | Useful but requires discovery |
| **Game during wait** | Novelty feature; not expected |
| **Screen sharing/co-browsing** | Complex to implement; enterprise use case |
| **Sound toggle** | Nice but not critical |
| **Dark/light theme** | Already partially supported |

---

## Architectural Notes

### Current State

Helpin's widget exists in two forms:

1. **SDK (`packages/sdk-js/src/core/widget.ts`)** — Well-architected with:
   - WebSocket real-time communication with exponential backoff
   - Session management and persistence
   - Multiple conversation views
   - AI thinking indicators
   - Typing indicators
   - Pre-chat form
   - CSAT rating
   - Shadow DOM CSS isolation

2. **Legacy Embed (`widget/src/helpin-widget.js`)** — Uses:
   - Long-polling (3s open, 15s closed) instead of WebSocket
   - Limited styling/theming
   - Basic message rendering
   - Missing most rich features

### Key Insight

The biggest gap vs Crisp.chat isn't any single feature — it's the **absence of a cohesive rich content experience** including:

- Emoji and GIF pickers (standard expectations)
- Media previews and lightbox viewing
- Interactive carousels
- Proper link previews
- Code syntax highlighting
- Rich quick replies

These have become baseline expectations for consumer-facing chat widgets. Without them, Helpin's widget feels like a 2015-era live chat rather than a modern support experience.

---

## Crisp.chat Feature Categories (27 total)

For reference, Crisp.chat's feature set spans:

1. Real-time messaging with rich media
2. File/image/video/GIF upload and preview
3. Voice and audio messages
4. Emoji and GIF pickers
5. AI-powered automation and bots
6. Knowledge base/helpdesk integration
7. Co-browsing and screen sharing
8. Full internationalization (i18n)
9. Theme and branding customization
10. Event hooks for third-party integration
11. GDPR/compliance features
12. Plugin architecture for extensibility
13. Video/audio calls
14. Message editing and translation
15. Code highlighting (10+ languages)
16. Online/offline presence
17. Push notifications
18. Identity verification (email/phone OTP)
19. Rating and feedback system
20. Waiting entertainment (games)
21. Search functionality (global + helpdesk)
22. Message retry on failure
23. Rich content carousels
24. Interactive pickers
25. Link previews
26. Quick replies
27. Overlay mode (Ctrl+K trigger)

---

## Recommendations

1. **Phase 1 (Ultra-High):** Add online/offline status, push notifications, and identity verification (OTP)
2. **Phase 2 (High):** Add emoji picker, GIF picker, audio messages, and help center search
3. **Phase 3 (Medium):** Add file upload improvements, message status, code highlighting, quick replies enhancements
4. **Phase 4 (Low):** Add carousels, lightbox, overlay mode, keyboard shortcuts
