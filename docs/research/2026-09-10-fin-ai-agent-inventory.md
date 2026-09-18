# Intercom Fin AI Agent research inventory: September 10, 2026

This historical research inventory records the September 10 assessment of Fin and other support products. Its original author cited official pages and named third parties; those citations are research leads, not a current certification of every capability. Items tagged UNVERIFIED were not confirmed in the original review.

## Review boundary: September 18, 2026

The corporate announcements were rechecked against primary sources. Intercom announced the Fin company name on [May 12](https://www.intercom.com/blog/today-intercom-becomes-fin/). Salesforce subsequently announced that the acquisition **completed on September 10, 2026**, superseding the earlier expected-close wording. [Salesforce completion announcement](https://www.salesforce.com/news/press-releases/2026/09/10/salesforce-completes-acquisition-of-fin/).

The remaining feature counts, pricing, provider support, certifications, release dates, and competitor classifications below retain their original research date and were not comprehensively revalidated in this documentation pass. Vendor performance and security statements are vendor claims, not independent measurements or Helpin guarantees. Recheck a linked primary source before using a specific comparison in public documentation; missing evidence in this inventory does not prove a competitor lacks a feature.

The strategic takeaway is to compare complete support workflows and measurable outcomes. This source review does not establish an exhaustive ranking of open-source products or prove that only two products support autonomous local-model replies.

## 0. Corporate context
- Intercom renamed itself "Fin" on 2026-05-12; the helpdesk keeps the Intercom name ("Intercom 2"). https://www.intercom.com/blog/today-intercom-becomes-fin/
- Salesforce announced an agreement on June 15 and [completion on September 10, 2026](https://www.salesforce.com/news/press-releases/2026/09/10/salesforce-completes-acquisition-of-fin/). The original expected closing date is superseded.
- Help content mirrored on intercom.com/help and fin.ai/help.

## 1. Core engine
- "Fin AI Engine": refine query (rewrite, check Workflows/Custom Answers, safety filter) → generate (retrieve from content, data, actions; generate or disambiguate/execute) → validate accuracy/safety. https://www.intercom.com/help/en/articles/9929230-the-fin-ai-engine
- fin.ai/ai-engine lists 7 stages and proprietary models `fin-cx-retrieval`, `fin-cx-reranker`, **Fin Apex 1.0** as final answer generator; OWASP LLM Top 10 protections; claims 76% average resolution rate, 99.97% uptime. https://fin.ai/ai-engine
- Fin Apex 1.0 (2026-03-26) post-trained on CS interaction data; replaced GPT + Sonnet 4.0 as core answering model; claims +2.8% resolution, 0.6s faster TTFT, 65% fewer hallucinations vs Sonnet 4.6. https://www.intercom.com/blog/announcing-fin-apex-the-age-of-vertical-models-is-here/
- Apex Flash powers Fin Voice 2 (June 2026). https://fin.ai/voice
- Multi-vendor: "Fin uses OpenAI, Anthropic, Google, and Intercom models, switching automatically." https://fin.ai/trust-reliability
- Fin 2 (2024-25, "66% across 6,000+ customers"); Fin 3 (2025-10-15, Pioneer 2025: Procedures, Simulations, Slack/Discord, Voice, Insights). https://www.intercom.com/blog/whats-new-with-fin-3/
- Fin API Platform (Apex, Apex Flash, Retrieval, Reranker, RAG APIs; $250k+/yr) and Fin Agent API for embedding Fin in your own UI. https://fin.ai/api-platform

### Content sources (Knowledge Hub)
- Public articles (native or Zendesk-synced), internal articles (Confluence, Guru, Notion), snippets, macros (Copilot only), websites (weekly sync), PDFs, Zendesk/Salesforce Knowledge/Freshdesk KBs, past conversations (Copilot only, 4 months), API private data. https://www.intercom.com/help/en/articles/9440354-knowledge-sources-to-power-ai-agents-and-self-serve-support
- Per-content audience targeting, bulk actions for Fin availability/folder/languages. https://www.intercom.com/help/en/articles/7120684-fin-ai-agent-explained
- Google Docs as a source: UNVERIFIED.
- Custom Answers unavailable to customers who joined on/after 2025-03-19. https://www.intercom.com/help/en/articles/7837520-using-fin-ai-agent-with-custom-answers

### Multilingual, tone, guidance
- 45 languages with auto-detect and real-time translation fallback. https://www.intercom.com/help/en/articles/8322387-set-up-fin-ai-agent-s-multilingual-support
- Tone: Friendly, Neutral, Matter-of-fact, Professional, Humorous. Length: Concise (~30% shorter), Standard, Thorough. Pronoun formality per language. https://www.intercom.com/help/en/articles/13177409-customize-fin-ai-agent-tone-of-voice-and-answer-length
- Guidance: categories Communication Style, Context & Clarification, Content & Sources, Spam, Other; max 100 live items, 2,500 chars each; can escalate but cannot tag/route/update attributes; audience-targetable; preview panel; per-guidance usage and resolution metrics; version history (Apr 2026). https://www.intercom.com/help/en/articles/10210126-provide-fin-ai-agent-with-specific-guidance
- Fin Attributes: classifies issue type, sentiment, urgency, spam for Workflows/reports. https://www.intercom.com/help/en/articles/11680403-how-to-create-fin-attributes
- Previews: test as a user/lead/audience, choose brand/language/channel, event log with reasoning and sources. https://www.intercom.com/help/en/articles/12599471-use-fin-previews
- Identity: receives name/email/locale etc. with identity verification; Email OTP (6-digit, 3 attempts) gating data connectors or as a Workflow step; JWT user tokens for authenticated API calls. https://www.intercom.com/help/en/articles/10741459-secure-data-connectors-with-one-time-passcode
- Fin Memory (July 2026): recalls past conversations across channels. https://fin.ai/updates

## 2. Actions / Tasks / Procedures
- Data connectors (formerly Custom Actions): HTTP calls, fixed/dynamic tokens, JWT user tokens, data transformation (Python code blocks), health dashboard; no retry on timeout/error; triggered from a 3–5 sentence NL description. https://www.intercom.com/help/en/articles/9916507-fin-and-data-connectors-faqs
- MCP connectors: templates for Linear, Stripe, Shopify, Zapier, Snowflake, HubSpot, Salesforce, Marketo; custom MCP via URL + OAuth 2.0/token. https://intercom.com/help/en/articles/11461635-powering-fin-with-your-external-tools-using-mcp-connectors
- Fin Tasks sunset for new builds from 2026-03-12 in favour of Procedures. https://www.intercom.com/help/en/articles/9569407-fin-tasks-and-data-connectors-explained
- Fin Procedures: document editor with NL instruction, condition, code, connector/MCP, handoff, sub-procedure, wait, wait-for-webhook (July 2026) steps; AI drafts from outline; intent-triggered plus event and manual inbox triggers; sequential execution; automatic procedure switching; structured extraction from uploaded PDFs/images; per-procedure reporting; Proactive Procedures (Aug 2026). https://www.intercom.com/help/en/articles/12495167-fin-procedures-explained
- Built-in confirmation before writes: UNVERIFIED (implemented via instruction/condition steps). On connector failure Fin "does not automatically fall back to saying it doesn't have the information."

## 3. Handover, escalation, resolution accounting
- Default triggers: asks for human, frustration, loops, negative feedback; safety handoffs (self-harm, minors, jailbreaks, high-risk medical/legal/financial). https://www.intercom.com/help/en/articles/12396892-manage-fin-ai-agent-s-escalation-guidance-and-rules
- Escalation Rules (attribute) and Escalation Guidance (NL) decide when; Workflows decide what next (route, message, collect info, create ticket).
- "Let Fin handle" step: expectation setting, gather info before handover, inactivity follow-up (4 min chat; 1h–30d email, default 24h), CSAT toggle, auto-close. https://www.intercom.com/help/en/articles/10032299-use-fin-ai-agent-in-workflows
- External helpdesk handover via data connector ticket creation. https://www.intercom.com/help/en/articles/7995955-hand-over-fin-ai-agent-conversations-to-another-support-tool
- Escalation Reporting attributes (Aug 2026). https://fin.ai/updates
- Outcomes: Resolution (confirmed, or assumed after 24h silence following an actual answer), Procedure handoff, Qualification, Disqualification. One charge per conversation max; not billed for explicit human requests, rule escalations, procedure failures, abandonment after a question; resolution deducted if customer returns to the same conversation later. https://www.intercom.com/help/en/articles/8205718-fin-ai-agent-outcomes
- Metrics: Resolution rate and Involvement rate on the Performance dashboard. https://www.intercom.com/help/en/articles/10576273-measure-customer-service-with-ai-insights-built-for-the-ai-agent-era

## 4. Channels
- Messenger (web/iOS/Android), Email, Phone (Fin Voice), WhatsApp, SMS, Instagram, Facebook Messenger, Slack, Discord, Telegram (July 2026), Fin Agent API, Workflows, outbound via FullStory/Pendo. https://fin.ai/channels
- Email: forwarding + authenticated domain; reads full thread; inline citations; follow-up 1h–7d; CC/reply-all rules. https://www.intercom.com/help/en/articles/9356221-deploy-fin-ai-agent-over-email
- Fin Voice: 28 languages, transfer/callback/voicemail, SIP/PSTN with major CCaaS; sales-only, custom pricing. https://www.intercom.com/help/en/articles/10697275-deploy-fin-voice
- Fin for platforms: standalone on Zendesk, Salesforce Service Cloud, HubSpot, Freshdesk/Freshchat. https://www.intercom.com/help/en/articles/10118495-fin-for-platforms-explained

## 5. Fin AI Copilot
- Inbox sidebar: cited answers, process guidance, rewrite/translate, fact-check; sources incl. past conversations; 10 free conversations/teammate/month; unlimited $29/agent/mo annual or $35 monthly. https://www.intercom.com/help/en/articles/9121374-copilot-explained

## 6. Insights, testing, reporting
- Base: Performance dashboard. Pro add-on: CX Score, Topics Explorer, Trends, Recommendations (gaps, contradictions, duplicates), Incident detection, Monitors (auto-QA), Custom Scorecards. https://www.intercom.com/help/en/articles/11394959-use-ai-powered-content-recommendations-to-improve-fin
- Unresolved questions clustered by topic (≥10/month). https://www.intercom.com/help/en/articles/8890980-dig-into-fin-ai-agent-unresolved-questions
- Testing: Previews (manual), Batch tests (50 questions, CSV, human-rated), Simulations (AI-simulated multi-turn vs a Procedure, 250–12,500 runs/month). https://www.intercom.com/help/en/articles/14077180-simulations-vs-batch-tests-vs-previews
- Evals and Releases (2026-08-13): scenario sets scored by deterministic checks + LLM judge; staging workspace with ramp, A/B, rollback. https://www.intercom.com/blog/announcing-evals-and-releases/
- Benchmarks: vendor claims 76% (12,000+ customers) vs 66% in Oct 2025; third-party tests 38–73%. Treat as vendor or small-sample. https://builts.ai/blog/intercom-fin-ai-review/

## 7. Trust, safety, compliance
- Answers only from your content; claims <1% hallucination; escalates below confidence. https://www.intercom.com/help/en/articles/7837535-fin-ai-agent-faqs
- LLM sub-processors under zero-data-retention, no training; Fin may use anonymised data for fine-tuning with opt-out. https://www.intercom.com/blog/data-privacy-security-ai-chatbots/
- Hosting US/EU/AU (AU AI processing US-based). SOC 2 II, ISO 27001/27018/27701/42001, AIUC-1, HIPAA (enterprise), 99.8% SLA. https://fin.ai/trust-reliability
- PII: PAN redaction with Luhn; up to 10 custom regex rules. https://www.intercom.com/help/en/articles/13925174-redacting-sensitive-data-in-conversations-with-custom-rules
- Audit: conversation events record guidance/content used; admin activity logs API.

## 8. Pricing (2026)
- $0.99 per outcome; $9.99 per sales qualification; 50-outcome monthly minimum; 14-day free trial. https://fin.ai/pricing
- Seats $29/$85/$132 per seat/mo annual. Copilot $29/$35. Pro add-on $99/mo for first 1,000 conversations then $0.12→$0.06 per conversation. https://www.intercom.com/help/en/articles/13868265-pro-add-on
- Fin Voice: sales-only. Fin API Platform: $250k+/yr.

## 9. 2025–2026 roadmap items
- Customer Agent vision (Pioneer 2025), Fin for Sales, Fin for Ecommerce (Shopify), Fin Operator (GA 2026-08-06 in Pro: analyses conversations, drafts knowledge/procedures/guidance as human-approved proposals), Monitors (2026-03-25), Apex 1.0 (2026-03-26), Voice 2 (June 2026), Evals and Releases (2026-08-13), Memory, Telegram, Proactive Procedures, Incident Detection (July–Aug 2026). https://www.intercom.com/blog/introducing-operator/ , https://fin.ai/updates
- "Fin AI Analyst", "Fin Coach", "Fin for Analytics", "Fin Overview": NOT FOUND in official sources; UNVERIFIED.

## 10. Open-source alternatives: AI agent / BYOK / local model story
- **Chatwoot (Captain)**: assistant, copilot, FAQ suggestions, memories, documents, custom tools; self-hosted Captain requires Enterprise Edition on a paid plan; BYOK via OpenAI key + custom endpoint (Ollama not named). https://www.chatwoot.com/hc/user-guide/articles/1755284287-how-to-enable-captain-on-self_hosted-installations
- **Zammad 7.0**: agent-assist only (categorisation, summaries, translation, writing); providers incl. OpenAI, Anthropic, Azure, Mistral, Ollama, custom OpenAI-compatible. https://admin-docs.zammad.org/en/7.0/ai/provider.html
- **Libredesk** (Go, AGPL-3.0, single binary): live-chat AI assistant answering from snippet KB with handoff; copilot; providers OpenAI, Anthropic, OpenRouter, Groq, Together, Azure, LiteLLM, Ollama; separate embedding provider with dimension config; "AI in beta". https://docs.libredesk.io/configuration/ai.md
- **Papercups**: maintenance mode, no AI. **Chaskiq**: rule bots, no LLM feature documented. **Helpwise**: proprietary. **FreeScout**: AI module (drafts/summaries) with many providers incl. Ollama/LM Studio; autonomous replies only via third-party module. **Tiledesk**: visual AI agent builder, Agentic RAG with Qdrant, LLM-agnostic incl. Ollama/vLLM, open-core.

## Takeaways
- Fin's moat is the surrounding system, not the LLM: knowledge sync, bounded Guidance, Procedures with deterministic steps plus simulations/evals/releases, outcome accounting with reversals, escalation reporting, insights/QA.
- The original comparison identified Libredesk and Tiledesk as local-model autonomous-agent examples. Its coverage is not exhaustive, and current edition/provider support needs verification before making exclusivity claims.
