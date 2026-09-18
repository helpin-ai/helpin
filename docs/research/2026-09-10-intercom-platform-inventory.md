# Intercom platform inventory — September 2026

This historical research inventory helps product contributors compare non-Fin-agent Intercom capabilities with Helpin. It records the September 10, 2026 investigation; it does not describe Helpin implementation or promise feature parity.

## Review status — 2026-09-18

The full inventory was reviewed for scope and provenance. External sources were not fetched again: prices, plan gates, SDK versions, release dates, security certifications, redirects, and API limits below remain dated research claims. Recheck the linked official sources before using them for a product or purchasing decision. The original report links sources by section but does not preserve fetched snapshots or claim-level citations, so its assertions cannot all be independently reproduced from this file alone. “Not found” means not found during that investigation, not proof that a feature does not exist.

## Original inventory

**Original method note.** The original researcher reported that the inventory came from pages fetched during the September 10 investigation. Intercom's marketing site has been partly folded into fin.ai (intercom.com/messenger now 308-redirects to fin.ai; /helpdesk, /outbound, /reporting, /workflows, /channels product pages 404). The help center (intercom.com/help), developer docs, pricing page, and changelog remain the authoritative sources and are what I relied on. Items marked UNVERIFIED could not be confirmed from a fetched page.

---

## 1. Messenger (web widget, iOS/Android SDKs)

Sources: https://www.intercom.com/help/en/articles/6612588-messenger-explained · https://www.intercom.com/help/en/articles/6612589-set-up-and-customize-the-messenger · https://www.intercom.com/help/en/articles/6612597-messenger-faqs · https://www.intercom.com/help/en/articles/9319961-updates-to-the-messenger · https://www.intercom.com/help/en/articles/6705301-use-the-messenger-in-your-mobile-app · https://www.intercom.com/help/en/collections/10723236-channels

**Spaces (the Messenger's tabbed structure)**
- Six spaces: **Home**, **Messages** (permanent, cannot be disabled), **Tickets**, **Help** (in-context help center), **News** (opt-in feed), **Tasks** (checklists; requires Checklists enabled). Spaces can be reordered by drag-and-drop; Home stays at top but can be disabled. Space names cannot be edited.
- Option to **bypass Home and launch directly into a conversation** (needs iOS SDK 18.6.3+ / Android 15.15.0+).

**Home / apps**
- Home is a card-based screen built from **apps** (at least one app required). Named first-party apps: Tasks (checklists), Content Showcase (blog/video cards), Get a Demo (lead qualification questions), External Links (up to 10 static URLs), Calendly, Chili Piper, Google Analytics, Ask a Question. Third-party Canvas Kit apps can also be placed here (see §9). https://www.intercom.com/help/en/articles/1827291-customize-your-messenger-home-with-apps
- Welcome message and team introduction, localizable in up to 45 languages; teammate avatars toggle; "Start conversation" CTA with 6 presets, separate CTAs for visitors vs. users and for Fin vs. teammate conversations.
- Conversation restrictions: disable new conversations entirely, or **require an article search before starting a conversation**.

**Styling and launcher**
- Light / Dark / Match-system theme (dark mode shipped Oct 28, 2025), separate logos per theme, home background and action colors, header gradients/background images, auto-import branding from your domain.
- **Multi-brand Messenger**: separate styles per brand, each linked to its own Help Center (Expert plan).
- Launcher: show/hide by audience rules, left/right position, side and bottom spacing (position/spacing on Advanced/Expert only). Custom launcher via JS (method list in §9). Messenger window size cannot be changed; no fullscreen mode.
- "Spotlight Messenger": alternate experience for Fin for Sales, targetable per audience.

**Conversation experience**
- File attachments and images (bulk image upload, preview carousels, grid layouts — Feb 23, 2026); **attachment upload can be restricted to users matching custom attributes** (June 10, 2026).
- Emoji, GIFs (GIFs plan-gated; disabling requires support), message reactions, code blocks with syntax highlighting/line numbers/copy button, borderless conversations on desktop.
- **Voice transcription** (dictate a message, July 22, 2026).
- **Live queue position** shown to waiting customers (June 12, 2026).
- **AI-generated conversation titles** shown in the customer's history (Feb 20, 2026).
- Clear labels for who is handling the conversation (Fin / workflow bot / human) and visible handover.
- Conversation ratings (CSAT) with five emoji plus free-text follow-up, triggered by a simple-automation toggle or via Workflows. https://www.intercom.com/help/en/articles/7872853-measure-customer-satisfaction-with-conversation-ratings
- Sound notifications toggle; "user conversation history visibility" controls.

**Reply time and hours**
- Reply-time expectations driven by workspace office hours; option to show office hours only after team assignment (team-level hours are Expert plan). Special notices (delays, status) multilingual with formatting/hyperlinks. Privacy-policy notice at conversation start (Jan 27, 2025).

**Identity / security**
- **JWT-based Messenger security** is the current recommended mechanism; the older HMAC "Identity Verification" is marked **[Deprecated]** in the help center but still works. Short token expiry, secure cookies, configurable session duration, **trusted domains** list with wildcards. https://www.intercom.com/help/en/collections/384-security-privacy
- Error-report transmission can be disabled for web/iOS/Android.

**Multilingual / visibility**
- Auto-detect browser/device language, up to 45 languages. Visibility rules on launcher via audience filters. HIPAA option to hide message content in mobile push.

**Mobile SDKs**
- iOS (CocoaPods / SPM / manual; iOS 15+), Android (Gradle; API 23+), React Native (0.59+, includes an **Expo config plugin**), Cordova/PhoneGap. Flutter and Unity: **not found on developers.intercom.com — UNVERIFIED**. https://developers.intercom.com/installing-intercom/react-native/installation
- Mobile features: push notifications (with images, multi-brand push, disable for lost devices), Mobile Carousels, Surveys, Help Center/collections in-app, article suggestions, unread badges, ticket tracking, bottom-sheet native presentation, unidentified-user support, launch Articles/Carousels from a button. https://www.intercom.com/help/en/collections/2094798-mobile-sdks
- Known gap: Messenger (browser) calls are not supported in mobile SDKs; no native Messenger-usage report.

---

## 2. Inbox / Helpdesk

Sources: https://www.intercom.com/help/en/articles/10223008-setting-up-the-inbox · https://www.intercom.com/help/en/articles/6258745-the-inbox-explained · https://www.intercom.com/help/en/articles/8838656-inbox-faqs · https://www.intercom.com/help/en/collections/3497068-inbox

**Structure and views**
- Team inboxes (Advanced/Expert), **Inbox Views** with saved filters and custom folders; views can be searched and duplicated (Feb 25, 2026); **export a view as CSV** preserving filters/columns/sort (Sep 10, 2026).
- Chat layout and table layout (toggle `L`), configurable columns, dark/light/system theme, UI in English/French/German/Brazilian Portuguese/Spanish, timestamps in customer's timezone.
- Search by keyword/tag/user/assignee/date, filter by custom attributes; cannot filter multiple conversation states at once.
- Bulk actions on up to 1,000 conversations; "bulk action on every conversation matching your search" runs in background (July 28, 2026).
- Apps sidebar (Similar conversations, Recent conversations, Recent page views, Jira, Salesforce, Shopify, custom Canvas Kit apps); **role-based sidebar templates** (June 5, 2026).

**Assignment and workload**
- Manual assignment, **round robin** (Advanced+), **balanced assignment** (Expert; assigns to active teammate with fewest open conversations, then longest-waiting), **skills-based routing** with load-balancing fallback (July 13, 2026). https://www.intercom.com/help/en/articles/6553774-balanced-assignment-deep-dive
- Assignment limits at teammate, workspace, per-inbox (Feb 23, 2026), and per-channel (email vs Messenger, July 16, 2026) levels; teammates can pick which channels they take without splitting the inbox (Aug 3, 2026).
- Priority ordering by conversation priority, SLA breach time, wait time, start time, inbox priority. **Five priority levels** (July 2, 2026).
- Away / Away & Reassign statuses with auto-reassignment; snoozed conversations re-balance to the inbox if the owner is at capacity; wrap-up time protection on chat/email (July 17, 2026); permissions to stop cherry-picking/dropping work (Aug 18, 2026).

**SLAs** (Expert plan) https://www.intercom.com/help/en/articles/6546152-set-slas-for-conversations-and-tickets
- Targets: first response, next response, time to close, time to resolve (tickets). Office-hours aware; timers pause on snooze / waiting-on-customer; auto-unsnooze on breach; applied via Workflows or edited directly in Settings (June/July 2026); **workflow triggers before/after SLA breach** (June 30, 2026); phone speed-of-answer SLAs (May 28, 2026); SLA report.

**Macros** https://www.intercom.com/help/en/articles/6433193-creating-and-managing-macros
- Saved replies with articles/emoji/GIFs/images/attachments and personalization variables; actions: assign, tag, snooze, close, reopen, set priority, set custom ticket state, call a data connector. Visibility: workspace / team (Expert) / personal. Folders (June 25, 2026), CSV export of macro content and usage, Macros API. Invoked via Command-K.

**Conversation handling**
- Snooze (custom times, bulk snooze, "stay snoozed through notes/assignment changes" Sep 1, 2026), close, reopen, priority, tags, conversation attributes (custom conversation data), merge conversations and tickets, conversation events timeline, spam folder (2-month retention), strip links, block users, trusted senders.
- Internal notes with @mentions (people and **teams**), **emoji reactions on notes** (June 2, 2026), separate reply/note drafts.
- **Side conversations** via email or Slack from a conversation/ticket; replies sync back; can be auto-translated and exported. https://www.intercom.com/help/en/articles/8398956-side-conversations
- Keyboard shortcuts and Command-K palette (`R` reply, `N` note, etc.; multiple keyboard layouts; not customizable).
- **Collision detection / presence indicators: not documented in the Inbox FAQs — UNVERIFIED.**
- Export multiple conversations as PDF/text (June 3, 2026); image zoom to 5x.

**Tickets** https://www.intercom.com/help/en/articles/6436600-tickets-explained · https://www.intercom.com/help/en/articles/8450754-customer-portal-explained
- Three categories: **Customer tickets** (customer sees progress), **Back-office tickets** (internal; notes/status cross-posted to the customer conversation), **Tracker tickets** (one issue linked to many conversations, broadcast updates to all affected customers).
- Custom **ticket types** with custom attributes and **ticket forms** (sendable in Messenger and, since June 10, 2026, on WhatsApp/SMS/Facebook/Instagram/email). Fin AI Autofill for title/description.
- **Ticket states**: four default categories (Submitted, In progress, Waiting on customer, Resolved) with custom-named states, multiple Submitted states, state chosen at conversion, per-state customer notifications (Aug 3, 2026), separate reply windows for tickets vs conversations.
- Convert conversation to ticket in one click (details carry over), link tickets/conversations, multibrand tickets, ticket assignment limit, tickets in reporting (lifecycle report; ticket assignments dataset Sep 8, 2026).
- **Customer/Tickets portal** inside the Help Center (Advanced+): logged-in users see tickets and, since June 30, 2026, conversations for their company; audience rules for access; filter/sort; option to restrict to tickets the user created.

**AI in the Inbox** https://www.intercom.com/help/en/articles/6955446-ai-features-available-in-the-inbox · https://www.intercom.com/help/en/articles/9121374-copilot-explained
- **Fin AI Copilot** (agent-facing): answers from help center, internal articles, macros, synced Notion/Confluence/Zendesk content, and past conversations; tone/translation of drafts; usage reporting. 10 conversations/teammate/month included on all plans; unlimited add-on $29/agent/month annual or $35 monthly.
- AI Compose (rephrase, tone, grammar, translate to 10 languages), Expand, AI Summarize (Advanced+ manual; Expert for workflow "add summary note"), AI Issue Summary, AI Autofill, Smart Replies.
- **AI Inbox Translations**: 45+ languages, inbound translated to the teammate's language, outbound translated back; 10 free translated conversations/teammate/month, unlimited with Copilot seat; glossary; quality rating thumbs.
- Fin Copilot "Similar conversations" app.

---

## 3. Channels

Sources: https://www.intercom.com/help/en/articles/9955432-channels-explained · https://www.intercom.com/help/en/collections/10723236-channels

- **Live chat / Messenger** (web + mobile) — see §1.
- **Email**: multiple support addresses per brand, automatic forwarding, DKIM/SPF/DMARC domain authentication (Entri auto-setup or manual DNS), custom outbound sender address, reply from the received-at address, signatures, custom domain for email assets, deliverability alerts, DMARC reporting via Valimail, email threading across chat/email. https://www.intercom.com/help/en/articles/9744849-connect-your-email-support-channel
- **WhatsApp**: two-way in inbox, multiple business numbers, message templates, outbound WhatsApp, "open WhatsApp from your product", use in Workflows, **voice-note replies** (May 14, 2026); billed per conversation.
- **Instagram DMs**, **Facebook Messenger**, **Discord**, **Telegram** (July 28, 2026).
- **SMS**: two-way SMS in inbox, promotional/transactional via Series, multiple numbers per country (July 17, 2026); billed per message.
- **Slack**: Slack as a native channel (community workspaces or Slack Connect), Slack notifications, broadcast to many customer Slack channels at once (Oct 14, 2025). Microsoft Teams: Fin in Teams channels article exists.
- **Phone (Intercom Phone)**: inbound/outbound calls in the inbox, IVR via Workflows, queues with hold music, callbacks, transfer/warm transfer, 3-way calls, barge and whisper (May 27, 2026), recording with consent prompts, transcription and call summaries, voicemail, answering-machine detection, caller OTP verification, CNAM, business-number caller ID, numbers in 35+ countries, porting, per-brand numbers, phone dashboard and 11+ call metrics. Balanced assignment only (no round robin). https://www.intercom.com/help/en/articles/8488925-intercom-phone-faqs
- **Messenger calls**: browser audio/video calls (no mobile SDK support).
- **Switch**: deflect phone callers to Messenger via SMS link.
- **Fin Voice** (AI phone agent; detailed evaluation outside this inventory) — pricing "contact Sales".
- All channels configurable under Settings > Channels and filterable in views; Workflows run across chat, email, SMS, WhatsApp, Instagram, Facebook, Slack, phone.

---

## 4. Help Center / Knowledge Hub

Sources: https://www.intercom.com/help/en/articles/9357912-knowledge-explained · https://www.intercom.com/help/en/articles/9440354-knowledge-sources-to-power-ai-agents-and-self-serve-support · https://www.intercom.com/help/en/articles/56644-customize-your-help-center · https://www.intercom.com/help/en/articles/56640-help-center-explained · https://www.intercom.com/help/en/articles/2982784-control-who-can-see-your-public-articles · https://www.intercom.com/help/en/articles/8827044-public-articles-faqs · https://www.intercom.com/help/en/collections/9615439-knowledge

**Knowledge Hub (single content system for Help Center, Fin, Copilot)**
- Content types: public articles, internal articles, snippets, PDFs/documents, synced websites, macros, past conversations (for Copilot), private data via API.
- Sync/import sources: Zendesk (sync or import public articles, 301 redirects on import), Confluence, Guru, Notion, Salesforce Knowledge, Freshdesk, Box, GitHub, website URLs (with proxy region selection).
- Folders (up to 10 sub-folders), universal search and filters, bulk actions (AI state, folder, audience, help-center status, language, delete), content tagging, enable/disable per Fin and per Copilot, per-content audiences, permission-scoped access, content performance reporting across Fin/Copilot/Help Center.

**Help Center**
- Collections (with descriptions for search), sections, articles; drafts before publish; article formatting, heading deep-links, related articles (up to 5), table of contents, author display.
- **Multilingual**: per-language versions of collections/sections/articles; 18 more languages added June 22, 2026 (Multilingual Help Center is Advanced+).
- **Custom domain** via CNAME with HTTPS/SSL, cookie forwarding for login-aware content; custom fonts; Google Analytics GA4; cookie consent banner.
- **SEO**: meta descriptions from article description, canonical tags, sitemap/indexing controls, manage redirects, option to block indexing.
- Styling: logo/header, colors, Google Fonts, "import your brand", Classic/Visual/Compact card styles, 1–3 columns, hero block, footer templates with up to 16 social links, favicon/social image, responsive preview; redesigned faster Help Center with persistent sidebar (July 23, 2026). No custom HTML/CSS/iframes in articles.
- **Article feedback**: happy/neutral/disappointed reactions with follow-up comment; article views/clicks/reactions/replies and "searched but not found" terms in reporting.
- **Access control**: unlisted articles; audience-targeted articles by user type, custom attributes, country, segment (Private Help Center is Advanced+); login link setting; "[Restricted]" warning to teammates.
- **Multiple Help Centers** (multibrand Help Center is Expert).
- Article search in Messenger and in mobile apps; Articles API.
- Article version history / rollback: **UNVERIFIED** (only drafts are documented).

---

## 5. Outbound / Proactive Support

Sources: https://www.intercom.com/help/en/articles/3292835-outbound-explained · https://www.intercom.com/help/en/collections/2091449-outbound · https://www.intercom.com/help/en/articles/9061648-proactive-support-plus-add-on · https://www.intercom.com/help/en/articles/4425207-series-explained · https://www.intercom.com/help/en/articles/6612245-checklists-explained · https://www.intercom.com/help/en/articles/2900887-design-your-product-tour · https://www.intercom.com/help/en/articles/8771588-chats-posts-and-banners-faqs · https://www.intercom.com/help/en/articles/5973834-intercom-s-survey-types

- **Message types**: Chats, Posts (full-screen in-app), Banners (with **Banners API** for native apps/kiosks, June 10, 2026), Tooltips (can launch tours/articles/surveys), Emails, Mobile Push (with images), SMS, WhatsApp outbound, Product Tours, Checklists, Surveys, Mobile Carousels, News items, Series.
- **Audience targeting**: dynamic vs fixed audiences, And/Or rules on person/company data, events as filters and triggers, segments, tags; scheduling; goals; control groups; A/B tests (message variations); message tags, saved views, bulk pause/delete/tag.
- **Series**: visual drag-and-drop campaign builder with Rule blocks ("try to match for X time"), Wait blocks, Tag blocks, content blocks (chats/posts, tours, carousels, emails, push, custom bots/workflows, banners, checklists, surveys), entry/exit rules, re-entry, multivariate split testing (up to 5 paths), webhooks from Series, goals, version/change history, performance summary report.
- **Product Tours**: Post, Pointer, Video Pointer steps; advance on click/typing/element click; multi-page; page + audience targeting; snooze 24h; restart; confetti; shareable URL. Desktop web only (use Carousels on mobile).
- **Checklists**: tasks that link to tours/articles/URLs, auto-resolve on events/attributes, Tasks space and Tasks app, shareable link, step-level reporting.
- **Surveys**: 8 question types (NPS, numeric scale, star, emoji, dropdown, short/long text, multiple choice), branching logic, email-embedded first question, responses stored as user attributes, advanced survey reports.
- **News**: pull-only newsfeed in Messenger (no push).
- **Mobile Carousels**: multi-page onboarding/permission prompts with goals, A/B tests, control groups.
- Chats/Posts support reactions, email collectors, CTA/URL buttons.
- **Proactive Support Plus add-on**: $99/month incl. 500 messages; unlocks Series, Checklists, News, A/B testing, webhooks, versioning, event-based messaging; usage charged for Posts, Push, Tours, Carousels, Surveys, Series messages on a tiered scale ($0.07 → $0.0175 per message). Chats, banners, tooltips, email, SMS, WhatsApp, phone do not need the add-on (email campaigns/SMS/WhatsApp billed per usage).

---

## 6. Workflows (formerly Custom Bots)

Sources: https://www.intercom.com/help/en/articles/7836459-workflows-explained · https://www.intercom.com/help/en/articles/6611595-using-the-workflows-builder · https://www.intercom.com/help/en/articles/7846212-using-branches-in-workflows · https://www.intercom.com/help/en/articles/7872724-a-b-testing-workflows-and-using-control-groups

- Visual drag-and-drop builder (Advanced+; Essential has "simple automations" toggles only); templates; versioning and duplication; reusable workflows callable from other workflows; multilingual workflows with auto-translation (Oct 31, 2025); metrics (sent/engaged/completed).
- **Triggers**: customer opens new conversation in Messenger, sends first/any message, visits a page, clicks an element, opens Messenger; teammate changes state / assigns / adds note; customer unresponsive; ticket created or state changed; SLA about to breach / breached; inbound phone call and post-call; reusable-workflow invocation.
- **Customer-facing steps**: bot message, collect data (attributes or conversation data; phone keypad/speech modes), collect customer reply, show expected reply time, send ticket form, send app, Let Fin answer, pass to reusable workflow, email OTP verification, buttons/quick replies.
- **Background actions**: apply rules, tag/untag conversation or person, assign, snooze, wait, mark priority, apply SLA, disable customer reply, close, set conversation data, add integration action, **Custom Action** (API call via Data Connectors, with Python code blocks to transform responses), create ticket, mention teammate, add summary note.
- **Branching** on person, company, message, conversation data, availability, and Fin escalation guidance; A/B tests (requires "visits a page" trigger) and control groups.
- Omnichannel: chat, email, SMS, WhatsApp, Instagram, Facebook, Slack, phone (IVR, shared IVR blocks, escalate to reusable workflows).

---

## 7. Reporting / Analytics

Sources: https://www.intercom.com/help/en/articles/200-intercom-reports-explained · https://www.intercom.com/help/en/articles/4549035-create-a-custom-report · https://www.intercom.com/help/en/collections/2094752-reports · https://www.intercom.com/help/en/articles/13868265-pro-add-on

- **12 prebuilt templates**: Calls, Conversation tags, Conversations, Surveyed CSAT, Effectiveness, Fin AI Agent, Copilot, Responsiveness, SLAs, Team inbox performance, Teammate performance, Tickets; plus Holistic overview, Tickets lifecycle, Conversation topics, Escalation metrics, Phone dashboard (Aug 20, 2026). Legacy reports (Articles, Email deliverability, Leads, Workflows, Sales) still listed.
- **Custom reports** (Advanced+): 9 chart types (KPI, column, bar, donut, line, combo, area, heatmap, table), 100+ chart templates, datasets (Conversations, Conversation actions, Conversation state, Tickets, Ticket assignments, Calls, teammate activity), aggregations (avg/median/percentile/min/max/sum), filters and breakdowns by channel/team/tag/topic/attribute, office-hours-aware metrics (workspace, team, per-teammate), custom time buckets, drill-in, folders/favorites, saved filters, internal sharing with view/edit permissions, external share links and scheduled external sharing.
- **Real-time dashboard** (Expert plan).
- **CSAT**: surveyed CSAT across humans/Fin/workflows; chatbot CSAT; option to hide CSAT from agents.
- Metrics such as Average Adjusted Handling Time, snooze duration, per-active-hour metrics.
- **Exports**: CSV of conversations/tickets, chart data export, scheduled exports to S3/Google Cloud Storage/Azure Blob (conversations; teammate/activity logs added Sep 1, 2026), **Reporting Dataset Export API** (Oct 28, 2025), "dataset filters for SQL generation with LLMs".
- **Pro add-on** ($99/month for up to 1,000 conversations, then per-conversation tiers): CX Score, Topics Explorer, Trends, Recommendations, real-time Incident Detection, Monitors (auto-QA), custom scorecards, "Operator" AI analyst.

---

## 8. Data platform

Sources: https://www.intercom.com/help/en/collections/2094808-contacts · https://www.intercom.com/help/en/articles/179-create-and-track-custom-data-attributes-cdas · https://www.intercom.com/help/en/articles/6298280-setting-up-a-custom-object · https://www.intercom.com/help/en/articles/1636975-how-to-export-your-intercom-data-for-gdpr · https://www.intercom.com/help/en/articles/4667982-review-actions-taken-in-your-workspace-with-teammate-activity-logs · https://www.intercom.com/help/en/articles/6124430-regional-data-hosting

- **People model**: visitors → leads → users (user_id/email), companies, lead/user merge with preview (Aug 6, 2026), owners, tags, qualification data, UTM tracking, CSV import (plus Mixpanel/Mailchimp/Stripe importers), export users/leads/companies.
- **Custom data attributes**: string/number/boolean/timestamp on people and companies; set via JS snippet, REST API, CSV, integrations; max 250 active CDAs, 255-char strings, archive-only.
- **Events**: up to 120 active events, 20 metadata pairs each, 90-day retention; usable as filters/triggers.
- **Segments**: rule-based dynamic groups with And/Or logic.
- **Custom Objects**: typed objects (text, number, decimal, list, boolean, date, reference) with references to People/Conversations/other objects; populated via Data Connectors (not the JS snippet); surfaced in Inbox, Workflows, Fin.
- **Data Connectors**: no-code API integrations (GET/actions), XML→JSON, Python code blocks, health monitoring, versioning, log retention 7/14 days, public management APIs, MCP connectors (Zapier, Snowflake, HubSpot, Salesforce, Marketo templates).
- **Privacy tools**: per-user "Delete data" permanent deletion, per-user/subset CSV+JSON export, conversation export to S3, PAN redaction and custom-rule redaction, limit teammate conversation access, inactive workspace deletion, opt-out of AI fine-tuning.
- **Audit**: Teammate Activity Logs on all plans, 1 year in UI, full history via Activity Logs API, CSV download, webhook topic for SIEM.
- **Regional hosting**: US (us-east-1), EU (eu-west-1 Dublin), AU (ap-southeast-2 Sydney); no cross-region migration; API hosts api.intercom.io / api.eu.intercom.io / api.au.intercom.io.

---

## 9. Integrations / Platform

Sources: https://developers.intercom.com/docs · https://developers.intercom.com/docs/references/rest-api/api.intercom.io/ · https://developers.intercom.com/docs/references/webhooks/webhook-models/ · https://developers.intercom.com/docs/canvas-kit · https://developers.intercom.com/docs/build-an-integration/learn-more/rest-apis/sdks-plugins · https://developers.intercom.com/docs/references/rest-api/errors/rate-limiting · https://www.intercom.com/help/en/collections/2094744-apps-integrations · https://www.intercom.com/security

- **App Store size**: official pages fetched do not state a count. Third-party marketplace listing claims 565 apps; other sources say 200+/400+. **UNVERIFIED.** Developer docs say public apps reach "25K+ customers".
- **Named first-party integrations** (help center): Salesforce (8 articles), HubSpot, Pipedrive, Shopify (order refunds/replacements in inbox), Stripe, Segment, Clearbit, Zapier (plus Zapier MCP connector), Statuspage, Trello, GitHub, **Jira for Tickets**, Google/Outlook Calendar, Google Meet, Marketo, Mailchimp, Campaign Monitor, Facebook Lead Ads, Typeform, Unbabel, Lokalise, Aircall, Freshdesk, Google Analytics/GA4, Amplitude, Mixpanel, Asana, Monday.com. Zendesk appears as a **content sync/import source**, not a listed app.
- **REST API v2.16** (OpenAPI spec published): Contacts, Companies, Visitors, Data Attributes, Data Events, Conversations, Messages, Emails, Calls, WhatsApp, Articles, Help Center, Internal Articles, Content Snippets, Banners, Teams, Admins, Tags, Segments, Subscription Types, Audiences, Tickets, Ticket Types, Ticket States, Ticket Type Attributes, Workflows, Macros, Jobs, Data Connectors, Data Export, Reporting Data Export, Custom Object Instances, Fin Agent API, News, Office Hours, Away Status Reasons, AI Content, IP Allowlist, Switch, Activity Logs, Contacts Activity.
- **Rate limits**: 10,000 calls/min per app, 25,000/min per workspace (Extended API limit on Expert).
- **Auth**: OAuth with scopes for public apps, access tokens for private apps, REST API IP allowlist.
- **Webhooks** signed with X-Hub-Signature; topic families: admin, article, call, company, contact, conversation, content stat, event, API activity, ticket, subscription, visitor, data connector, translation (Sep 2, 2026).
- **Canvas Kit**: server-driven UI apps in Messenger Home, conversations, messages, Workflows, and Inbox conversation details; initialize/submit/configure/live-canvas flows; sheets (iframe); HMAC-SHA256 signed.
- **Messenger JS API**: boot, update, shutdown, show/hide, showMessages, showNewMessage, showArticle, showSpace, showTicket, showConversation, showNews (listed in docs nav), startTour, startChecklist, startSurvey, trackEvent, getVisitorId, onShow/onHide/onUnreadCountChange.
- **Server SDKs**: PHP, Node.js, Ruby, Go, Java, .NET, Rails plugin. **MCP server** documented at developers.intercom.com/docs/guides/mcp.
- **Security/admin**: SAML SSO (Okta, Azure AD, OneLogin; Expert), Google sign-in, 2FA with recovery codes, SCIM provisioning incl. SCIM-group → role mapping, IP restrictions, custom session length, custom roles (one role per teammate) with granular permissions, teammate activity logs, security health check, CSP support, HIPAA support (Expert), SOC 2 Type II, ISO 27001/27701/27018/42001, AIUC-1, GDPR/CCPA, 99.8% uptime SLA.
- **Seats**: full seats per plan; **lite seats** (no reply/no Copilot/limited inbox) free: 0 Essential, 20 Advanced, 50 Expert; all seats on one plan per workspace. Multiple workspaces are separate billing entities (regional workspaces cannot be migrated).

---

## 10. Pricing

Sources: https://www.intercom.com/pricing · https://www.intercom.com/help/en/articles/9061614-fin-and-intercom-plans-explained · https://www.intercom.com/help/en/articles/8205716-seats · https://www.intercom.com/help/en/articles/9121384-copilot-included-and-unlimited-usage · https://www.intercom.com/help/en/articles/9061648-proactive-support-plus-add-on · https://www.intercom.com/help/en/articles/13868265-pro-add-on · https://www.intercom.com/help/en/articles/8488934-phone-pricing

| Item | Price |
|---|---|
| Essential | $29/seat/mo annual ($39 monthly) |
| Advanced | $85/seat/mo annual ($99 monthly); 20 lite seats |
| Expert | $132/seat/mo annual ($139 monthly); 50 lite seats |
| Fin AI Agent | $0.99 per outcome (resolution, procedure handoff, qualification, disqualification); standalone "Fin on existing helpdesk" has 50-outcome/month minimum, no seat fees |
| Copilot unlimited | $29/agent/mo annual, $35 monthly (10 conversations/mo free on all plans) |
| Proactive Support Plus | $99/mo incl. 500 messages, then tiered per message |
| Pro add-on | $99/mo for ≤1,000 conversations, then $0.12 → $0.06 per conversation |
| Phone | usage-based: numbers $1–$275/mo by type/country, inbound $0.012–$0.552/min, Messenger calls $0.03/min, recording $0.0035/min |
| Email campaigns, SMS, WhatsApp | pay-as-you-go |
| Fin Voice | contact Sales |

Plan gating (official): all plans get Fin, Messenger, shared inbox, ticketing, prebuilt reports, public Help Center, simple automations, Slack integration. **Advanced** adds Workflows, multiple team inboxes, round robin, private + multilingual Help Center, tickets portal, custom reports, Salesforce and Marketo integrations. **Expert** adds SSO/identity management, HIPAA, SLAs, multibrand Messenger and Help Center, workload management (balanced assignment), team office hours and reply times, extended API limit, real-time dashboard.

---

## 11. Notable 2025–2026 changelog additions

Source: https://www.intercom.com/changes/en (pages 1–20 fetched), plus https://www.intercom.com/changes/en/114353-multilingual-workflows-are-now-live and https://www.intercom.com/changes/en/114323-new-api-capabilities-for-deeper-integrations

**2025**
- Jan 7 — Phone: 3-way calls, warm transfer, call listening, blocking, manual assignment; numbers in 15 more countries.
- Jan 17 — Filter by ticket state in Inbox.
- Jan 27 — New Messenger (redesigned Fin answers/handover, privacy notice, launch-into-conversation).
- Mar — Fin Voice launched (per blog "Built for You Spring '25").
- Aug 15 — Branch on escalation guidance in Workflows. Aug 18 — Python code blocks in Data Connectors.
- Sep 10 — Internal articles (Notion/Guru/Confluence) as Fin knowledge. Sep 11 — Slack as native channel (beta).
- Oct 1 — Fin Audiences & Identities. Oct 14 — Slack broadcast. Oct 23 — Fin attribute detection. Oct 28 — Dark mode Messenger; API v2.14 + Reporting Dataset Export API + Macros API. Oct 31 — Multilingual Workflows.

**2026**
- Feb 20 — Email OTP step in Workflows; AI titles in Messenger. Feb 23 — Per-inbox capacity limits; bulk image attachments. Feb 25 — Duplicate/search views; 11 new call metrics. Feb 26 — Data Connector health/dependencies; ticket-portal visibility restriction. Mar 2 — Scheduled external report sharing.
- Apr 9–13 — Side conversation export; translation quality rating; ticket resolution time with office hours; draft preservation. Apr 22–24 — Fin for Sales; per-brand phone numbers; separate ticket/conversation reply settings; CSAT for outbound calls; Fin Guidance version history; macro actions (reopen, ticket state). Apr 30 — Macro usage export.
- May 5 — Data Connector redesign with versioning/APIs. May 7 — Average Adjusted Handling Time; Fin on Shopify storefront. May 14 — WhatsApp voice notes. May 27 — Barge/whisper; hide CSAT from agents. May 28 — Phone SLAs.
- Jun 2–5 — Note reactions; customer-timezone timestamps; availability filters in limits; PDF/text batch export; phone Collect Data step; call quality indicators; role-based sidebar templates. Jun 8 — Customer-facing brand name; on-demand translation languages. Jun 10 — Banners API; attachment restrictions; ticket forms on 5 channels; Fin email follow-up/preview/spam controls. Jun 12 — Live queue position; SLA settings page. Jun 17 — Shopify refunds in inbox. Jun 22 — 18 new Help Center languages. Jun 25 — Macro folders; recording consent; AMD. Jun 30 — Conversations in Customer Portal; SLA-breach triggers; per-line CSAT.
- Jul 2–3 — Five priority levels; phone workflow triggers/post-call automation; caller OTP; SLA editing in settings; Fin for Sales + HubSpot booking. Jul 8 — Side-conversation auto-translate; Fin Ecommerce revenue report. Jul 9 — Zapier MCP; snooze improvements. Jul 10 — Macro CSV export. Jul 13 — Skills-based routing. Jul 15–17 — Custom hold music; per-channel assignment limits; Customer Portal search; per-teammate office hours; multi-number SMS; wrap-up on chat/email; Calls report template. Jul 20–28 — Business caller ID; snooze metrics; voice transcription in Messenger; Help Center redesign; Fin waits for external systems; ticket details carry-over; search-wide bulk actions; **Telegram channel**.
- Aug 3–20 — Per-state ticket notifications; channel selection without splitting inbox; connector log retention; merge preview; Marketo connector; Fin memory across conversations; incident detection; take/drop permissions; mid-call hold music; Phone dashboard.
- Sep 1–10 — Snooze persistence; scheduled exports incl. teammate/activity logs; translation webhooks; Series activation reliability; Ticket assignments dataset; Snowflake MCP connector; export views as CSV; connector health charts.

---

**Unverified / not found**: App Store app count; collision detection / agent presence in the Inbox; article version history; Flutter and Unity SDKs; exact OAuth scope list; Fin Voice pricing. The original researcher attributed the remaining items to the linked pages; that attribution has not been independently revalidated in this repository review.
