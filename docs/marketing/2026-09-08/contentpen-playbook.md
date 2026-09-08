# Contentpen production playbook for Helpin

September 8, 2026. Proposed operating instructions; no workspace or publishing connection has been configured.

## 1. Brand and source pack

Create a Helpin workspace. Select an approval-based editorial workflow if available in the account. Contentpen publicly describes author/reviewer roles, scheduling, and approval workflows; verify the exact controls in your account. [Planning documentation](https://contentpen.ai/content-planning-and-scheduling).

Use these settings or attach them to each brief when a dedicated setting is unavailable:

- Audience: founders, support leads, and product teams at lean B2B SaaS companies.
- Positioning hypothesis: customer support connected to product work and knowledge.
- Voice: direct, specific, practical, candid about tradeoffs. Write for someone responsible for getting work done.
- English language; use the terminology approved by the product team.
- Product names: Helpin, Ask Agent, Contentpen, ContentStudio. Use “tasks” consistently; do not introduce legacy “stories” terminology without context.
- Preferred evidence: actual product walkthroughs, anonymized support examples, approved interviews, current official documentation, measured results with dates and methodology.
- Avoid: invented statistics, fake quotes, unsupported superlatives, generic AI introductions, repetitive definitions, implied SOC 2/ISO certifications, guaranteed rankings, guaranteed AI accuracy, and blanket replacement claims.
- Pricing: retrieve from the current approved plan record for each article; do not hard-code pricing into an evergreen brand prompt.
- Capability: distinguish built-in behavior, configured automation, human-reviewed action, and unavailable/future capability. Do not turn an agent name into a promise of verified execution.

Attach a source manifest to each assignment:

| Field | Required value |
|---|---|
| Product fact | Exact approved statement and limitations |
| Evidence | URL, screenshot or recorded workflow reference |
| Reviewer | Product owner who checked it |
| Verified date | Date checked against the current product |
| Availability | Plan, configuration, environment or rollout restrictions |
| Permission | Whether customer data/quote/media may be used publicly |

Use the repository and release notes to find potential evidence, not as a substitute for verifying customer-facing behavior. Do not upload private tickets, customer identifiers, credentials, or internal-only product data as generic writing context.

## 2. Brief template

Copy this into a Contentpen assignment and fill it before generating:

```text
Working title:
Content ID / cluster:
Audience and situation:
One question this piece answers:
Search intent / candidate phrase:
Intent validation notes (actual SERP review; volume optional, never invented):
What the reader should be able to do afterward:
Original insight or evidence:
Approved product facts and limitations:
Sources with dates:
Required outline:
Screenshot / example / downloadable asset:
Primary CTA and destination:
Internal links (live URLs only):
Author:
Product reviewer:
Editorial reviewer:
Draft date / proposed publish date:
Refresh trigger:
Unresolved facts:
```

## 3. Reusable drafting prompt

```text
Write an actionable article for the audience in this approved brief.
Use only the supplied product facts and attributable sources for factual claims.
Separate general process advice from behavior demonstrated in Helpin.

Open with the reader's problem and a useful answer. Explain the actual steps,
include the supplied worked example, and address when the method is not a fit.
Use concise headings, examples and tables only when they clarify the process.
Do not pad to a word count or repeat keywords mechanically.

Do not invent product capabilities, results, customer quotes, citations,
search volumes, rankings, certifications or competitor prices.
Flag missing facts as [VERIFY: specific question] in the working draft.
Use screenshots only from approved assets. Never create fictional UI evidence.

Use the brief's primary CTA at a relevant point. Suggest internal links only
from the supplied live URL list. Keep Helpin promotion proportional to the topic.

Return:
1. Suggested title, slug, SEO title and description.
2. Article draft.
3. Proposed image alt text and captions.
4. Claim/source table and unresolved verification items.
5. Three derivative social angles; no automatic scheduling or publication.
```

## 4. Starter briefs

### H01 — A support-to-engineering escalation workflow that preserves context

**Reader:** support lead in a small SaaS company whose developers repeatedly ask for missing details.

**Candidate intent:** “support escalation workflow”; validate against current search results before finalizing title.

**Reader outcome:** implement a reusable escalation packet and know when to create a task.

**Angle:** a ticket link is not a complete handoff. Preserve impact, reproduction, evidence, ownership, next action, and the customer update plan.

**Outline:**

1. A short, anonymized example of an incomplete escalation.
2. Define escalation criteria: severity, affected users, workaround, and uncertainty.
3. The handoff packet: problem, expected/actual result, reproduction, environment, customer impact, evidence, owner, communication commitment.
4. Worked example using synthetic data, labeled as such.
5. How to maintain the link between conversation and product task.
6. Closing the loop: verify a fix and decide who updates the customer and knowledge.
7. Failure modes: duplicate tickets, unverified severity, private data in tasks, unattended ownership.
8. Downloadable checklist and next step.

**Original evidence:** support/product lead interview; actual Helpin conversation-to-task flow; 3 screenshots reviewed by product. Only claim automatic behavior if demonstrated.

**Asset:** escalation checklist as accessible HTML plus downloadable Markdown.

**CTA:** see the support workflow; link to `/product/customer-support` only once live.

**Internal links:** knowledge-gap guide H02 and workflow evaluation H03 when available.

**Review gate:** someone unfamiliar with the process can complete the template from the example. Product verifies task creation/linking and any status synchronization mentioned.

### H02 — Turn recurring support questions into a useful knowledge backlog

**Reader:** founder/support lead facing repeated questions and stale help content.

**Candidate intent:** “knowledge base gaps”; needs live intent validation.

**Reader outcome:** audit a small conversation sample and prioritize updates without producing redundant articles.

**Angle:** frequency alone is not enough; include severity, account breadth, existing coverage, and whether the underlying product needs fixing.

**Outline:**

1. Why another generic FAQ may not resolve repeated support demand.
2. Select a defined sample and remove sensitive information.
3. Group recurring needs; separate unclear docs, missing docs, usability bugs, and account-specific requests.
4. Score a worked example with a transparent, author-proposed rubric.
5. Choose update-existing, create-new, change-product, or no-action.
6. Assign reviewer/owner and define completion criteria.
7. Review usefulness with actual support evidence over time.
8. Show how Helpin's current coverage review supports the demonstrated steps, with rollout limitations.

**Original evidence:** approved synthetic sample table or authorized anonymized sample, reviewed against current coverage UI. No claim of reduced ticket volume without measurements.

**Asset:** gap-prioritization worksheet.

**CTA:** view the knowledge workflow or start a trial, depending on available pages.

**Review gate:** do not portray finding generation as automatic article publication. Confirm current review and activation behavior.

### H03 — How to evaluate an AI support agent before giving it more autonomy

**Reader:** SaaS founder/support lead deciding whether to use AI on customer conversations.

**Candidate intent:** “evaluate AI customer support”; validation pending.

**Reader outcome:** build a small, repeatable evaluation set and choose human-review boundaries.

**Angle:** evaluate usefulness and correct escalation, not merely whether a response sounds fluent.

**Outline:**

1. Start with the work the agent may perform and what remains human-owned.
2. Build an evaluation set: straightforward docs question, missing knowledge, billing exception, ambiguous bug, account-specific data, and sensitive request.
3. Define a scorecard: factual correctness, evidence use, policy compliance, action correctness, escalation quality, latency, and cost visibility.
4. Show a hypothetical scored example clearly labeled as illustrative.
5. Run in supervised mode and review errors with a named owner.
6. Define deployment gates and rollback/disable criteria.
7. Re-evaluate after product, policy or model changes.
8. Demonstrate the verified Helpin review/control experience.

**Original evidence:** actual control screenshots, a product-reviewed evaluation worksheet, and a recorded supervised run. Do not present suggested scoring thresholds as industry benchmarks.

**Asset:** evaluation scorecard.

**CTA:** book a guided pilot using your team's evaluation criteria.

**Review gate:** no claims that every action is approval-gated unless tested; no assertion that a high test score guarantees safe production behavior.

## 5. Case-study interview guide

Ask what the team actually did before Helpin, what triggered evaluation, what changed in the pilot, which integrations remained, where setup was difficult, and what result can be substantiated. Request dated baseline/after data with comparable scope and sample sizes. Ask for failures or limitations as well as benefits.

Proposed story: ContentStudio's support-to-product process. Ownership/availability does not imply permission to publish customer data or a success outcome. Describe it as an affiliated/portfolio implementation where applicable. Have the quoted person approve the final wording and numerical claims.

If metrics are unavailable, publish a process walkthrough with clear qualitative attribution. Do not retrofit a percentage improvement.

## 6. Editorial and publishing checklist

**Product reviewer:** every product claim verified; limitations and plan conditions included; screenshots current; no invented roadmap certainty.

**Editor:** one intent; useful original contribution; readable opening; sources verified; no unresolved `[VERIFY]` tags; comparison fair; dates and authors accurate; no copied passages.

**Publisher:** draft preview approved; unique canonical; durable images and alt text; correct metadata; mobile/keyboard review; all links work; sitemap entry; CTA event verified; author/reviewer present; content status and revision recorded.

**After publication:** confirm public 200, readable HTML, no accidental noindex, correct canonical, no broken assets; record final URL; request/check indexing appropriately; schedule repurposing only after final URL is verified.

For updates and corrections, preserve a clear source of truth between Contentpen and Git. Assign ownership of the latest revision; reconcile edits before pushing a newer copy. For unpublishing, record redirect/404 behavior and remove the page from sitemap and related links.

## 7. Distribution recipe per approved article

- Founder post: one operational observation, a concrete example, and a question that invites relevant experience.
- Company post: brief value statement and link to the useful artifact.
- Video: 45–90 second real workflow clip, captions, and one action.
- Digest: two-sentence reason to read; segment by reader interest.
- Sales: one relevant section or checklist for a live evaluation conversation.

Contentpen publicly advertises ContentStudio integration for social distribution; verify connection, supported accounts, previews, and approval behavior before using it. [Publishing documentation](https://contentpen.ai/content-publishing).

Do not automatically publish identical text to every channel. Adapt format and context, avoid unnecessary tagging, and do not send outreach without the owner's authorization.

## 8. Calendar handling

`content-calendar.csv` contains 20 proposed core assets and 4 unscheduled backlog ideas. Dates are tentative and depend on website readiness and evidence. Fields include intent, evidence, CTA, owner and review gate. Check the Contentpen account's supported import format; map fields or paste individual briefs. This file has not been imported, and its schema is not represented as a native Contentpen import contract.

The marketing owner reviews the next two weeks every Monday. Prefer moving an unsupported piece out of the schedule over publishing generic filler to preserve cadence.
