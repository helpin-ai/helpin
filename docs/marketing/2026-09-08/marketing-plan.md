# Helpin marketing plan

Prepared September 8, 2026. Planning window: September 8–December 6, 2026.

## Recommendation

Lead with **customer support connected to product delivery and knowledge for lean SaaS teams**. Use the broader connected-workspace story as the expansion narrative. Make the first sales motion a guided pilot of one support workflow, supported by a real product demonstration, customer evidence, practical articles, and founder-led distribution.

Use Contentpen for research, briefs, drafting, editorial review, and the content calendar. Use the existing Next.js website as the publishing destination and ContentStudio for repurposing approved content. Start with a simple editorial export; automate publishing only after the first two articles establish a reliable content format.

The primary objective is **qualified workspaces that activate and become retained paying customers**. Publishing volume, impressions, and AI visibility are supporting indicators.

## Basis and limits

Reviewed the marketing source in `helpin-main-merge/website`, main commit `ba418f0b5`, related deployment and analytics source, the live homepage and pricing page, and public Contentpen documentation. Tested live routes and rendered the homepage in Chromium at 1440px and 390px. The product history review from the preceding task informs which workflows to investigate; commits alone are not proof a feature is enabled for every customer.

No private analytics, Search Console, CRM pipeline, customer interviews, Contentpen workspace, keyword-volume data, or actual CAC/retention data were available. Budgets, ICP, channel mix, and targets below are planning hypotheses. Search phrases are candidates, not verified keyword opportunities. Public vendor descriptions establish advertised capabilities, not account entitlement or tested integrations.

This is a plan and working asset pack. No site changes, account configuration, advertising purchases, publishing, or outreach have been performed.

## 1. Website review

### What to retain

- Distinct visual identity, consistent brand treatment, and a responsive homepage. No horizontal overflow or broken images were observed in the two sampled viewports; this is not a full accessibility or performance audit.
- Clear trial and demo paths; signup leads to `app.helpin.ai/register`, demo to the existing Cal.com booking page.
- A workflow illustration that links support, tasks, docs, and customer context. This can become the centerpiece of a recorded demonstration.
- Pricing organized around workspace plans, with a no-card trial and unlimited seats. Keep commercial details sourced from the current plan definitions.
- Canonical URLs, page descriptions, Open Graph images, and Twitter metadata already exist. Keep these foundations as routes expand.

### Findings and priority

| Priority | Observed evidence | Marketing implication | Action and acceptance condition |
|---|---|---|---|
| P0 | Homepage promises broad value to every team and displays a large agent roster | Visitors must infer which problem to start with | Lead with one ICP and outcome; five target buyers can explain who it is for, the first workflow, and the next step after a short review |
| P0 | Homepage API-key FAQ conflicts with included-AI pricing language | Raises setup and cost uncertainty | One approved explanation of included usage, optional provider connection, limits, and extra usage across home/pricing/app |
| P0 | Home promises broad import availability and says setup takes under 10 minutes | Migration effort and capability may be overstated | Publish a verified import matrix and distinguish basic setup from completed migration; attach evidence to any timing claim |
| P0 | Agent roster and animated scenario imply many autonomous workflows | Product breadth can outpace demonstrable behavior | Mark ready-to-use, configurable, and forthcoming capabilities distinctly; demonstrate the chosen workflow in production-like conditions |
| P0 | Existing CTA tracking helper appears on home/pricing; navbar/footer CTAs use direct links | Attribution may depend on inconsistent autocapture | Test all CTA locations; implement one event contract and verify delivery through paid conversion |
| P1 | `/blog`, `/customers`, `/security`, `/sitemap.xml` returned 404 on September 8 | Few owned pages answer evaluation questions or capture specific search intent | Launch the first three destinations and a sitemap; return real 404s for absent pages; ensure published URLs are crawlable |
| P1 | Main navigation offers Agents and Pricing; footer mainly offers legal links | No clear product, proof, or learning path | Add Product, Solutions, Customers, Resources, Pricing progressively as pages exist |
| P1 | Logo strip identifies portfolio brands but does not substantiate outcomes | Logos cannot answer implementation and value questions | Produce two documented customer/portfolio stories; accurately disclose internal or affiliated use where relevant |
| P1 | Broad cost-replacement comparison and security statements | Skeptical buyers need scoped evidence | Add dated assumptions, actual workflow scope, verified security details, and limitations; do not promise complete tool parity |
| P1 | Homepage has no rendered JSON-LD in the sample | Structured entity information can be improved | Add accurate Organization/WebSite data; add Article/BreadcrumbList for articles; do not invent reviews or ratings |
| P2 | Large client-component homepage with animation | Potential performance cost; impact unmeasured | Measure field/lab performance before optimizing; defer optional scripts and media based on results |

`robots.txt` returned 200 with a content-signals explanatory preamble; the inspected response did not provide a sitemap reference. Do not describe the site as blocked from search. Add explicit crawler rules appropriate to the intended public routes and review edge-generated behavior.

Source anchors: `website/src/app/page.tsx` (hero, roster, FAQs, CTA helper), `website/src/app/pricing/page.tsx` (plans/claims), `website/src/components/Navbar.tsx`, `Footer.tsx`, `website/src/lib/metadata.ts`, `website/next.config.ts`, `.github/workflows/deploy-website.yml`. Public copy: [homepage](https://helpin.ai/), [pricing](https://helpin.ai/pricing).

## 2. Initial audience and positioning

### Proposed initial customer profile

English-speaking B2B SaaS companies with roughly 5–50 employees, founder/product-led purchasing, and a small support team. Their customer conversations, bugs, documentation, and account context are split across tools. They have enough support demand to feel coordination costs but can pilot a new workflow without an enterprise-wide migration.

Buyer: founder, COO, or head of support/customer success. Champion: support lead or product manager. Technical evaluator: engineering lead. Trigger: increasing support volume, missed customer follow-ups, repeated documentation gaps, or a tool renewal.

Prioritize prospects with two or more of these signals: repeated support-to-engineering handoffs; duplicate manual updates; no reliable connection from a ticket to a task; support knowledge that is hard to maintain; willingness to trial one inbox. Deprioritize enterprise replacement projects and accounts whose mandatory integrations/security requirements are unverified.

Validate with 8–10 conversations in the first two weeks. Ask for the last actual incident, current workflow, cost of delay, buying trigger, required integrations, and what would prevent a pilot. Change the segment if buyers consistently value a different workflow more strongly.

### Message proposal

**Category descriptor:** AI workspace for SaaS support and product teams.

**Draft headline:** Turn customer conversations into connected product work.

**Draft subheading:** Bring your inbox, customer context, tasks, and help docs into one workspace. Give your team AI assistance and control over the actions agents take.

**Primary CTA:** Start free trial.

**Secondary CTA:** See the workflow. Open a short product walkthrough; retain Book a demo alongside evaluation/pricing content.

**Proof needed:** a real conversation, its linked task, its related help article, and a visible human approval step where relevant. Show actual supported transitions rather than suggesting every transition happens automatically.

**Expansion narrative:** Start with support and product handoffs. Add CRM, meeting intelligence, planning, and broader agent workflows as the team gains confidence.

### Competitive framing

Intercom markets AI across the customer journey, HubSpot markets agents tied to CRM, and ClickUp markets AI within its work platform. Therefore, “we have AI agents” is not a defensible differentiator. The proposed differentiation is a demonstrable connected workflow, approachable adoption, and workspace economics for the chosen segment. This is positioning to test, not a claim of market uniqueness. Sources: [Intercom](https://www.intercom.com/help/en/articles/12508017-fin-as-a-customer-agent-for-service-sales-and-more), [HubSpot](https://www.hubspot.com/products/artificial-intelligence/breeze-ai-agents), [ClickUp](https://clickup.com/brain).

Comparison pages must say where Helpin is a fit and where the other option is a better fit. Check current official product/pricing sources and perform the relevant workflow before publishing. Avoid generic “best tool” rankings in which Helpin automatically wins.

## 3. Website conversion plan

### Homepage sequence

1. Specific audience/outcome, trial CTA, and short demo CTA.
2. Authentic product recording showing the first workflow in 60–90 seconds, with captions and transcript.
3. Proof strip with accurately described customer/portfolio usage.
4. Three benefits: retain customer context, create actionable product work, make knowledge accessible. Show product evidence under each.
5. Three-step pilot: connect one inbox, add a small approved knowledge set, run a supervised workflow. Describe prerequisites explicitly.
6. Relevant controls: permissions, approvals, AI usage visibility, and supported integrations.
7. One measured case study and an implementation story.
8. Plan overview and a clear route to detailed pricing.
9. FAQs answering migration, setup, AI costs, control, and fit; closing CTA.

Keep the full agent catalog as a deeper page, with accurate availability labels. Do not put every module and dozens of agent descriptions ahead of the first proof point.

### Recommended information architecture

| URL | Buyer question | Core contents | CTA | Phase |
|---|---|---|---|---|
| `/` | Is this for my team? | Positioning, workflow, proof, pilot | Trial / workflow video | 1 |
| `/product/customer-support` | Can it handle my inbox? | Real inbox/widget flows, channels, handoff, limits | Trial | 1 |
| `/solutions/saas-teams` | Does it solve our coordination problem? | Persona workflow, pilot, adoption plan | Guided demo | 1 |
| `/pricing` | What will I pay? | Plans, included usage, worked examples, fit | Trial | 1 |
| `/demo` | What actually happens? | Recording, transcript, booking option | Trial / booking | 1 |
| `/customers/contentstudio` | Does it work in a real company? | Verified before/after process and evidence | Demo | 2 |
| `/customers` | Who uses it? | Approved stories; no empty directory | Case study | 2 |
| `/security` | Can we evaluate trust and controls? | Verified facts and contact path | Ask a question | 1 |
| `/integrations` | Can I connect or migrate? | Native integration vs import vs planned; limitations | Pilot | 2 |
| `/blog` and `/blog/[slug]` | How do I improve this workflow? | Practical guides and original examples | Relevant product page | 1 |
| `/compare/helpin-vs-intercom` | Is switching appropriate? | Tested support workflows, economics, migration limits | Demo | 2 |
| `/compare/helpin-vs-clickup` | How does connected support differ? | Tested collaboration/support scope | Demo | 3 |
| `/product/ai-agents`, `/product/knowledge-base` | How do these supporting capabilities work? | Focused demos and controls | Trial | 2 |

Defer separate CRM/meeting and broad PM acquisition campaigns until the first audience shows activation. Those capabilities remain visible in product navigation and sales material.

### Pricing and proof work

Use the current commercial model as source of truth. Explain what an allowance means with examples approved by product/billing; do not fabricate a fixed number of AI replies. A savings calculator should use buyer-entered current spend and show assumptions, migration effort, and un-replaced tools. Validate “unlimited seats” and all plan restrictions against actual entitlements before publishing.

For a case study, record: customer type, prior tools/process, pilot scope, dates, conversation sample size, baseline and after metrics, operational changes, direct customer quote with approval, limitations. Candidate metrics: handoff time, repeat-question handling time, time to first useful reply, and percentage of support-derived work linked to an owner. Do not invent improvements because a portfolio company is available.

## 4. Blog, SEO, and AI discovery

### Editorial focus

Begin with four closely related clusters:

| Cluster | Candidate search language | Reader intent | Unique Helpin contribution |
|---|---|---|---|
| Support → product | support escalation workflow; turn customer feedback into tasks | Fix a recurring process | Real ticket-to-task example and downloadable template |
| Support knowledge | knowledge base gaps; AI support knowledge base setup | Improve answer quality | Demonstrated article review and coverage workflow |
| AI support operations | human handoff for AI support; evaluate AI support agents | Adopt AI responsibly | Evaluation scorecard, approval examples, failure cases |
| Consolidation/evaluation | shared inbox for SaaS; Intercom alternative for small SaaS | Evaluate purchase/change | Current comparison and a phased migration checklist |

Use Contentpen/SERP research and Search Console to validate phrasing and intent. Prioritize by customer relevance (0–3), proximity to an activation action (0–3), available original evidence (0–3), and research confidence (0–3), minus production effort (0–3). This is an editorial scoring method, not a forecast of ranking difficulty.

Avoid broad AI news, generic productivity listicles, mass glossary generation, and thin variations for every industry. Publish fewer pieces if original evidence is unavailable. Google emphasizes original, useful, people-first content; automation itself does not substitute for that value. [Google guidance](https://developers.google.com/search/docs/fundamentals/creating-helpful-content).

### Production standard

Each article answers one question, identifies its intended reader, explains a usable process, includes original screenshots/examples or interview evidence, names an author and reviewer, and links to a relevant next step. Add sources near factual claims, a meaningful updated date, and clear limitations. Use headings and tables where useful, not as a formula for padding word count. No fixed length requirement; target enough depth to complete the job.

Plan for **20 core content assets** over weeks 3–12: 16 practical/editorial blog pieces, 2 comparisons, and 2 case-study slots. Four additional topics are an unscheduled backlog. The calendar identifies evidence dependencies; delay a case study if proof is not ready. Website foundation pages are a separate workstream.

Internal linking: each article links to its cluster landing page, one relevant product/solution page, and 2–3 genuinely related articles once live. Comparison pages link to pricing, migration guidance, and proof. Use descriptive anchors; do not publish links to planned 404 pages. Combine overlapping topics rather than creating competing near-duplicate pages.

### Technical acceptance conditions

- Keep the blog at `helpin.ai/blog` with its canonical content on that domain; match website navigation/design.
- Generate static HTML for article text, unique titles/descriptions, canonical URLs, and meaningful headings.
- Publish sitemap entries only for canonical public URLs; verify production robots and edge behavior.
- Provide Article and BreadcrumbList structured data consistent with visible content. Organization/WebSite metadata should be factual. No ranking or rich-result guarantees.
- Article fields: title, slug, summary, body, author, reviewer, published/updated dates, category, canonical, hero image and alt text, SEO fields, source references, related content, CTA, status, revision.
- Host durable image assets; do not rely on temporary generation URLs. Validate responsive images, links, headings, and keyboard/mobile behavior.
- Exclude draft previews and internal search/filter duplicates from indexing as appropriate. Preserve changed URLs with redirects.
- Use Search Console to inspect initial URLs and actual indexing; a `site:` search alone is not an index inventory.
- Measure performance with real data. The current viewport check is not a Core Web Vitals assessment.

For AI discovery, use clear entity descriptions, accessible factual content, original evidence, and well-structured answers. Monitor a fixed small set of buyer questions monthly and record the engine/date/citation, but do not promise mentions. Treat this as an extension of content quality and crawlability, not a separate bulk-content campaign. [Google AI-search guidance](https://developers.google.com/search/docs/fundamentals/ai-optimization-guide).

## 5. Contentpen operating model

Public Contentpen pages advertise research/writing, calendar and approval workflows, CMS publishing integrations, Search Console integration, and ContentStudio distribution. Next.js is not listed as a native publishing destination in the reviewed integration catalog. Webhook documentation describes events, but an authenticated full-content export contract was not verified. Sources: [integrations](https://contentpen.ai/integrations), [publishing](https://contentpen.ai/content-publishing), [planning](https://contentpen.ai/content-planning-and-scheduling).

### Setup and weekly process — proposed, not configured

1. Create a Helpin workspace and brand/source pack: approved positioning, audience, product facts, plan facts, supported integrations, screenshots, terminology, examples, and prohibited/unverified claims.
2. Create four cluster folders. Import or manually map the supplied calendar into the account's supported fields. The CSV is a planning interchange file, not a claimed native Contentpen import schema.
3. Use the account's approval-based workflow. Working stages: idea → researched → brief approved → draft → product review → editorial review → ready → published → refresh. Map these to available Contentpen statuses; keep any unsupported checklist fields in the article brief.
4. Research the reader question and search intent. Editor approves angle and evidence before generation.
5. Generate from the supplied brief and approved sources. Product expert adds the actual workflow, limitations, and screenshot evidence.
6. Editor checks factual support, originality, readability, links, CTA, and overlap with existing content. Return unsupported statements for correction.
7. Publish through the chosen transport only after preview review. Verify the final public URL, metadata, images, sitemap, and analytics.
8. Repurpose the approved article into a demo clip, founder post, checklist, and opt-in digest item. Schedule appropriate social assets through ContentStudio after checking the account connection.
9. At 30 and 60 days, review query/intent fit, indexing, qualified visits, and activation contribution. Refresh based on evidence, not arbitrary date changes.

### Publishing architecture decision

**Recommended initial path:** Contentpen draft → editorial approval → reviewed content export into the Next.js content directory → preview build → production release. Use Markdown/content data rendered by controlled templates; do not evaluate untrusted generated MDX code. Two weekly exports are acceptable while the format is being proven.

The site uses `output: 'export'` and deploys to Cloudflare Pages. It cannot receive a webhook inside a static Next.js route. If automating, place the receiver in a separate service/Worker, authenticate requests, deduplicate by article ID/revision, validate/sanitize content, store durable media, and create a reviewable Git change. Use limited credentials and keep unapproved content out of production. Schedule publication through a documented build/release mechanism; merely setting a date in a static file does not publish it later.

**Week-2 integration spike:** inspect the actual Contentpen account/API or supported export; verify delivery of title, body, metadata, image URLs, status and stable ID; test create, revision, scheduled release, unpublish, duplicate event and failed publish. Do not assume a generation webhook includes approved article content.

**Fallback:** if export/automation is insufficient, use Contentpen's Ghost or WordPress connection with that CMS as the content source for a Next.js blog. This adds operational cost; select it only if editors need its workflow. Render on `/blog` and define build/update behavior. Do not move the entire marketing site merely to gain a blog integration.

## 6. Acquisition and distribution

### Allocation of effort

First month: approximately 40% website/proof/measurement, 25% customer discovery and guided pilots, 25% content production, 10% distribution. After foundations: 35% content, 30% direct discovery/pilots, 20% distribution/partnerships, 15% experiments and conversion improvements. These are staffing allocations, not media spend.

| Channel | Execution | Cadence | Success measure |
|---|---|---|---|
| Founder LinkedIn | Show one real workflow, lesson or result; use product clips and candid tradeoffs | 3 useful posts/week | Qualified conversations and assisted trials |
| Owned portfolio audiences | Relevant placements on Contentpen/ContentStudio channels, contextual webinar, opt-in invitations; describe affiliation accurately | 1 scoped pilot placement, then evaluate | Activated workspaces by partner source |
| Practical SEO content | Publish the calendar; link to a useful workflow CTA | 2 reviewed assets/week from week 3 | Qualified organic trials and activation |
| Product video | One workflow recording reused on landing page, YouTube, sales follow-up and social | 1 short clip/week; 1 fuller demo/month | Demo engagement and downstream activation |
| Customer/partner proof | Interview design partners; co-host a specific workflow session | 2 case studies and 2 sessions across 90 days | Attended demos, trials and fit |
| Communities | Answer relevant support/SaaS questions with complete advice; link only when useful and allowed | 2–3 contributions/week | Relevant conversations, not link count |
| Email lifecycle | Contextual onboarding and educational digest to eligible subscribers | Triggered onboarding; fortnightly digest | Activation and retained usage |
| Paid search | Small exact/phrase high-intent test after tracking and landing-page readiness | Optional from week 7 | Cost per activated workspace; cohort CAC |

No audience-wide blast across owned products. Segment by company type and role; account for subscription preferences and prior relationship. An owned audience is a distribution opportunity, not automatic product-market fit. No sending is authorized or performed by this document.

### Launch campaign

Offer a **guided support-to-product pilot**: one inbox, an approved knowledge set, and one escalation workflow. Publish a walkthrough, template, and case study around the same process. Target prospects who can evaluate that workflow, not every user of the sister products.

Demo agenda: 3 minutes of current process discovery; 7 minutes of actual workflow; 5 minutes of setup/integration constraints; 5 minutes of pilot success criteria. Use synthetic or approved anonymized examples. Do not imply approval-dependent actions are always autonomous.

### Lifecycle sequence

| Trigger/time | Purpose | Suggested message angle | Exit/suppression |
|---|---|---|---|
| Workspace created | Select first use case | “Start with one inbox and one workflow” | Skip steps already completed |
| Day 1, inbox missing | Remove setup friction | “Connect your first channel” with precise instructions | Stop after connection |
| Day 3, no meaningful agent action | Reach first value | “Try an AI-assisted support workflow” | Suppress once activated |
| After first successful workflow | Invite collaboration | “Bring the teammate who owns the next step” | Avoid repeat sends |
| Day 7, active pilot | Evaluate benefit | Checklist of baseline vs observed results | Contextual to completed actions |
| Days 11–13 of 14-day trial | Explain decision | What continues on each plan; offer help | Stop on payment/opt-out |
| Post-payment | Retention and expansion | Review adoption, then add one adjacent workflow | Based on actual engagement |

Reuse the existing lifecycle infrastructure rather than building a second messaging pipeline. Confirm actual events and eligibility before implementing these proposed messages.

## 7. Measurement, targets, and economics

### Instrumentation

Source currently includes Usermaven and Customer.io, plus a CTA tracking helper. Presence in source does not prove delivered events. An earlier browser sample reported a Customer.io resource blocked by ORB; recheck its response and delivery before relying on it.

Create an end-to-end test from anonymous visit through signup, workspace, activation and paid subscription. Preserve first-touch and last-touch attribution across `helpin.ai` → `app.helpin.ai`; attribute at workspace/account level and deduplicate identities. Keep personal or conversation content out of URLs/event properties.

| Event or metric | Definition | Authority |
|---|---|---|
| `website_start_trial_clicked` | Trial CTA click with page and placement | Existing name; verify all locations |
| `website_book_demo_clicked` | Booking CTA click | Existing name; distinguish from completion |
| `demo_booked`, `demo_attended` | Confirmed booking / attendance | Proposed canonical events from booking/sales records |
| `workspace_created` | Successful workspace creation | Map to existing server event |
| `support_channel_connected` | First usable inbox/channel | Proposed; verify backend success |
| `knowledge_ready` | Approved usable knowledge exists | Proposed; exclude empty uploads |
| `support_workflow_completed` | A meaningful successful assistance/handoff action | Proposed; exclude failed runs/test spam |
| Activated workspace | Channel connected, usable knowledge, one successful workflow, and a second active member within 7 days | Proposed ICP-specific definition; validate in interviews |
| Paid workspace | First confirmed paid subscription | Billing server record; not checkout click |
| Week-4 retained | Activated workspace uses the core workflow in week 4 | Cohort measure |

Track time-to-first-value, qualified trial→activation, activation→paid, demo→pilot, churn reasons, and acquisition cost. Define qualified trial by ICP fit and actual buying/evaluation intent, not every signup.

### First-90-day targets — internal planning, not forecasts

Operational: instrument the funnel, complete 8–10 buyer interviews, recruit 5 guided pilots, publish 6 core commercial/trust pages, produce 20 content assets, secure 2 evidence-backed stories, and document weekly learning. Case-study count depends on customer approval.

After a two-week baseline, use **30–60 qualified trials**, **10–20 activated workspaces**, and **3–6 paid workspaces** as provisional validation targets, not commitments. Cohorts arriving near day 90 will not have completed retention observation. Segment owned-audience, direct, organic and paid acquisition; do not attribute all trials to the blog.

Illustrative funnel only: 1,000 qualified visits × 3% trial rate × 40% activation × 25% activated-to-paid = 3 new paying workspaces. None of those rates has been measured here. Replace them with observed rates before setting a growth forecast.

Budget decisions should use contribution margin: allowable CAC = monthly revenue × contribution-margin percentage × desired payback months. Include AI serving and support costs. Example only: $99 × 70% × 6 months ≈ $416 CAC. If 10% of paid-media trials pay, allowable cost per trial is about $42, before other acquisition labor. This is an illustration, not an assertion about Helpin's margins.

### Review and stop rules

Weekly: delivery QA, qualified trials, first-value blockers, channel feedback. Monthly: cohort conversion/retention, content-assisted pipeline, pricing objections, and spend.

- If events cannot be reconciled to actual workspaces, fix measurement before increasing spend.
- If fewer than 3 of the first 10 qualified pilot workspaces activate, investigate onboarding/fit before buying more traffic.
- With limited traffic, prioritize interviews and sequential tests over underpowered A/B tests. Predefine outcomes and record traffic mix changes.
- For a paid test, use a fixed loss budget and weekly review; pause an ad group after the agreed spend limit without qualified actions. Do not wait for statistically precise CAC from a tiny sample.
- After 60–90 days, consolidate articles with overlapping intent; refresh high-impression/low-click or high-visit/low-activation pages using actual data.

## 8. Team, budget, and delivery schedule

### Lean resourcing assumption

One marketing owner/editor: 12–16 hours/week. Founder/product SME: 3–4 hours/week. Designer/video contributor: 2–4 hours/week. Developer: 3–5 days for initial site/blog foundations and 2–4 hours/week thereafter; integration automation is a separately estimated spike. Contentpen reduces drafting effort, not expert review or evidence collection. If fewer than 10 total marketing hours/week are available, publish one core article/week and postpone paid acquisition and secondary comparisons.

Illustrative incremental monthly cash budget, excluding existing staff/tools and taxes: $500–1,000 design/video assistance, $300–700 research/customer-story support, $200–500 hosting/other tools if needed, and $0–1,000 optional paid tests after readiness. Total $1,000–3,200/month; confirm actual internal capacity and Contentpen entitlements before adopting this budget. No purchase is proposed as already approved.

### Roadmap

| Period | Main work | Owner | Exit condition |
|---|---|---|---|
| Week 1: Sep 8–13 | Baseline funnel; 4–5 interviews; claim audit; positioning draft; source pack | Marketing + founder | Clear ICP hypothesis, tracked gaps, approved product facts |
| Week 2: Sep 14–20 | Remaining interviews; home/pricing updates; `/demo`, `/security`, blog template and sitemap; Contentpen spike | Marketing + developer | Preview QA passes; one complete draft→public test |
| Weeks 3–4: Sep 21–Oct 4 | Support/solution pages; first four content assets; first pilot cohort; founder distribution | Marketing + product | Pilot onboarding measured; all published links work |
| Weeks 5–6: Oct 5–18 | First case study slot; integration/import page; one comparison; first webinar | Marketing + SME | Evidence approved; customer objections reflected in pages |
| Weeks 7–8: Oct 19–Nov 1 | Refine onboarding; repeat strongest distribution; optional bounded paid test | Marketing + product | Funnel reliable and initial pilots demonstrate first value |
| Weeks 9–10: Nov 2–15 | Second case study slot; second comparison; first content refreshes | Marketing + SME | Channel cohort report and updated messaging |
| Weeks 11–12: Nov 16–29 | Continue proven topics; second workflow session; plan next quarter | Marketing + founder | Evidence-based continue/change decisions |
| Nov 30–Dec 6 | Final scheduled assets, 90-day review and backlog prioritization | Marketing owner | Report conversion/retention and cost, including incomplete cohorts |

### Prioritized experiments

1. Specific workflow headline vs current broad message: evaluate comprehension first, then qualified trial behavior.
2. Recorded workflow beside trial CTA vs explanation alone: measure demo→trial/activation, not only video views.
3. Guided pilot vs self-serve invitation for portfolio SaaS buyers: compare activation and founder time required.
4. Evidence-rich practical guide vs comparison page distribution: compare qualified conversations and assisted activation.
5. Pricing examples and migration scope: measure recurring sales objections and completion of evaluation steps.

Dependencies: marketing owner, current feature/plan facts, pilot access, approved customer evidence, blog publishing transport, validated events, and Search Console access. These dependencies are implementation tasks, not reasons to delay preparing the content backlog.

## 9. Immediate next actions

1. Assign the marketing owner and validate the initial SaaS audience with real buyers.
2. Resolve the API-key/AI allowance copy contradiction and audit the strongest import/automation claims.
3. Record the first authentic support-to-product demo and define the guided pilot.
4. Implement the blog foundation, sitemap, and the first support/solution/trust pages.
5. Load the source pack, three starter briefs, and calendar into Contentpen's supported workflow.
6. Publish the first reviewed practical article, verify acquisition→activation tracking, and distribute it to a narrow relevant audience.

Companion files: `content-calendar.csv`, `contentpen-playbook.md`, and `website-backlog.csv`. These are reviewable planning assets, not scheduled publications or deployed changes.
