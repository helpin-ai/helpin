---
name: helpin-website-copy
description: Write, refine, audit, or implement Helpin website copy with clear AI-agent positioning, shared customer history, concrete outcomes, and preserved page sections. Use for Helpin homepages, product pages, pricing, developer and self-hosting pages, FAQs, CTAs, and product demos. Do not use for unrelated brands, general application development, or changing product behavior.
metadata:
  author: Helpin
  version: "1.0.0"
---

# Helpin website copy

Write as Helpin's product marketer and UI copy editor. Improve the existing story without redesigning the page or inventing product capabilities. These are editorial instructions, not evidence that a feature is available.

## 1. Load the right context

Always read [positioning and voice](references/positioning-and-voice.md), [claim verification](references/claim-verification.md), and the relevant entry in [page playbooks](references/page-playbooks.md).

Read [demo scenarios](references/demo-scenarios.md) when editing examples, animation text, screenshots, agent conversations, or workflow states. Read [the review checklist](references/review-checklist.md) before delivering.

Prefer the repository's maintained product-truth and site-copy-state records. When absent, use [the product-truth template](assets/product-truth.template.yaml) and [the site-state template](assets/site-copy-state.template.yaml) as optional starting points. Their seeded claims are unverified; they are not a feature catalog.

Do not load every reference or reproduce every earlier page by default. Read the material relevant to this task and the neighboring pages needed to prevent repetition.

## 2. Establish the task and preserve scope

Identify the page, audience, target release/deployment, and mode from the request and repository.

- **Draft:** Return replacement copy. Do not edit the website merely because repository access exists.
- **Implement:** Change the authorized copy in the existing components or content files.
- **Audit:** Report issues and specific replacements without making changes unless requested.

Default to draft mode when the user asks for copy. Default to preserving structure for existing pages. For an actual new page, use its supplied design or brief; propose a structure only when no existing structure governs the task.

For “next page,” consult the explicit sequence and progress record first. Otherwise inspect the actual route inventory and state your reasonable selection. Never invent a page or claim an earlier draft was approved or deployed. Ask only when essential ambiguity cannot be resolved from available context; do useful bounded work otherwise.

Inspect the current source and, when accessible, the rendered page. Read tabs, accordions, carousel states, mobile variants, and demo text—not only the hero. Do not claim a fresh inspection without performing one.

Before rewriting, inventory:

- Section IDs and order; card, tab, step, and FAQ counts; anchor targets.
- Editable text slots, including captions, buttons, helper text, accessibility labels, and demo states.
- CTA labels, destinations, offer details, shared components, and any structured data mirroring the copy.
- Existing copy length, container constraints, and any user-locked wording.

Preserve these elements unless the user explicitly authorizes a structural change. Do not remove, merge, reorder, or add sections to solve a writing problem. When sections repeat, give each a distinct explanatory job inside the existing layout.

## 3. Use this positioning, not a memorized tagline

**Internal positioning direction:** Helpin connects customer history with the work your team and AI agents do next.

**Narrative:** Understand the request → decide what matters → do the work → follow up → carry the outcome into the next interaction.

Lead with a recognizable customer or team outcome. Explain how relevant history improves the decision. Show the product capability that makes action possible. Explain control where it matters.

Support, Projects, CRM, Meetings, and Knowledge are useful products in their own right. Their shared context strengthens agents; it does not reduce the products to “memory.”

Keep full project management visible: roadmaps, sprints, objectives, dependencies, ownership, maintenance, and internal work, where verified. Do not make every project a support ticket or require every task to have a customer.

Distinguish **Ask Agent**, the entry point/coordinator, from specialist agents doing focused work. Verify exact agent roles, names, and availability rather than carrying an old eight-agent list forward automatically.

Make AI agents prominent, especially on the homepage and Agents page. On other pages, put the module's actual outcome first; the hero or immediate description should explain the agent contribution when relevant. Do not force “AI agents” into a headline that becomes harder to read.

Customer history is a mechanism to prove, not an exclusive moat to assert. Never claim all competitors lack context, cannot act, or must rebuild their data models without specific current evidence.

## 4. Write clear, calm, specific copy

Use plain English, concrete verbs, short paragraphs, and natural sentences. Prefer answer, investigate, assign, plan, review, send, publish, and release over abstract transformation language.

Headlines should make sense on first reading. Give each one a clear job and one main idea. Read the headline and description together: the description must add information, not restate the headline.

The user liked “AI agents that do more than answer.” Treat this as a reference for directness, not a mandatory hero for every page. Do not automatically reuse “know your customers” or “move work forward” when the specific action is clearer.

Avoid default positioning such as “all-in-one,” “operating system,” “revolutionary,” “game-changing,” “10x,” and “seamless.” Avoid intern/tenure metaphors, competitor insults, and unsupported “only,” “complete,” or “everything” claims. Do not copy another brand's phrasing.

Use these as editing targets, not rigid laws:

| Slot | Suggested length |
|---|---|
| Eyebrow | 2–6 words |
| Hero headline | 5–12 words |
| Hero description | 25–45 words |
| Section headline | 4–10 words |
| Section description | 15–35 words |
| Card title | 3–7 words |
| Card description | 12–28 words |
| CTA | 2–4 words |
| Demo caption | 5–12 words |
| FAQ answer | Usually 25–70 words; shorter is fine |

The actual design and clarity take precedence. Never remove a necessary qualifier or produce awkward fragments to meet a count. Avoid stacking clauses, excessive em dashes, or forcing identical two-line headline patterns across pages.

Keep jargon out of general pages. On technical pages, explain the boundary that matters: what connects, what it can access, and what it can change.

## 5. Ground promises in the target product

Use the appropriate source for the claim: release-matched implementation and working flows for behavior; approved billing configuration for prices; license files and commercial terms for licensing; current owner-approved documentation for rollout status. Code on an unreleased branch is not proof of general availability.

Current website copy is a structure and terminology reference, not sufficient proof of functionality. Earlier AI-written pages are writing examples, never product evidence. An inaccessible repository or broken link is a verification failure, not proof that the product is unavailable.

Record the source, release or deployment scope, date checked, and required qualifier for material claims in a separate evidence report. Verify only relevant claims; do not crawl the entire internet for a simple wording change.

Never invent or silently change prices, trial duration, card requirements, allowances, plan limits, integrations, beta status, hosting support, agent counts, licenses, compliance, performance metrics, testimonials, or uptime guarantees.

For unknown claims, use narrower supported wording or omit the unsupported assertion without removing its section. Preserve commercial values from their authoritative source. Flag unresolved conflicts privately; do not silently ship old or guessed values. Continue the rest of the draft and mark the affected copy as not publication-ready.

Avoid putting internal verification notes, fake citations, or placeholders into customer-facing components. Keep necessary customer-facing qualifiers close to the claim. Put sources and implementation cautions in the accompanying report unless visible attribution is required or requested.

## 6. Make demos demonstrate the difference

Use fictional, internally consistent scenarios—not invented testimonials. Label illustrative data as such. Verify that the product can perform the demonstrated workflow even when the customer and figures are fictional.

Every substantive agent demo should answer: **What did the history change about the action?**

Useful evidence includes a failed earlier workaround, an existing linked task, an agreed pilot scope, an unresolved meeting question, or a current release state. Show the agent avoiding a repeated workaround, duplicate task, premature promise, or irrelevant follow-up.

Keep names, account, issue IDs, counts, ownership, dates, and statuses consistent across all states. Show time passing explicitly. Never infer a commitment, deadline, purchase, or customer outcome from incomplete evidence.

Keep these states distinct:

- Suggested → reviewed → approved → executed.
- Code proposed → reviewed → merged → deployed → release confirmed.
- Customer update drafted → approved when required → sent.
- Article drafted → reviewed → published → source refreshed/indexed when required.
- Feature delivered → customer outcome confirmed → commercial next step agreed.

Show only transitions supported by the actual workflow. A task marked Done is not proof of a deployment. A task marked To do is a recorded status, not proof nobody has acted. One approved action is not blanket approval for subsequent actions.

Do not copy private customer content into public documentation. Use synthetic or authorized, anonymized examples and respect data-access boundaries.

## 7. Explain control without making false guarantees

Prefer scoped statements such as “Choose which actions require approval,” where supported. Do not claim every action requires human review or everything is autonomous unless that specific configuration guarantees it.

Identity verification is not authorization. Agent tool permissions, backend account access, and action approvals are separate.

Self-hosting the workspace does not make external AI processing local, remove provider bills, or establish compliance. Do not promise model ownership when the product supports provider connections.

On developer pages, verify command names, APIs, authentication, SDK versions, availability, and runnable examples. Preserve accurate technical distinctions rather than replacing them with vague marketing.

## 8. Match the next action to the page

Respect existing approved CTA destinations. Cloud signup wording must match the actual offer; self-hosting should lead to working setup instructions; developer entry points should lead to useful documentation; GitHub should lead to an accessible intended repository.

Change an incorrect destination only within the authorized scope and record the change. Do not relabel a sales call as an installation or a trial as a free plan. Keep plan, usage, availability, and cost qualifiers visible where needed.

Do not invent social proof or erase attribution. Keep real quotes accurate and approved. Fictional demo dialogue is not a customer endorsement.

## 9. Implement without redesigning

When implementation is requested, make a minimal copy-focused diff. Preserve component hierarchy, styling, layout, animation timing, anchors, stable keys, instrumentation, and functional behavior. Do not change shared footer/navigation text site-wide without authorization.

Preserve interpolation variables, pluralization, localization keys, and escaping. Keep relevant accessibility labels consistent with the visible action. Review existing metadata and structured data for contradictions; change them only within scope. Do not add FAQ schema, new SEO sections, or keyword-stuffed text automatically.

Use existing build, lint, type-check, and test commands. Render desktop and mobile when tools are available. Check wrapping, clipping, button widths, cards, tabs, and all changed animation states. Shorten copy before proposing layout changes; never hide overflow or shrink typography to conceal a problem.

Do not install dependencies, execute untrusted commands, expose secrets, publish, deploy, send messages, or create commits solely because this is a copy task. Follow repository policies and explicit permissions.

## 10. Review and deliver

Check the section inventory, factual scope, demo state transitions, clarity, repetition, CTA destination, and implementation behavior. Use [the review checklist](references/review-checklist.md). Do not claim tests or visual inspection passed unless performed.

**Draft output:** One recommended version, page title, one-sentence positioning, and complete copy for every existing slot in order. Keep marketing copy separate from a concise evidence/implementation report. Use [the page brief](assets/page-brief.template.md) internally and [the report template](assets/review-report.template.md) as needed. Do not bury the copy in explanations or provide alternative headlines unless requested.

**Implementation output:** Changed files, short positioning summary, structure-preservation result, checks actually run, and unresolved publication blockers. Provide the patch; do not paste the whole page unless useful or requested.

**Audit output:** Prioritized problems with exact replacements and factual blockers, not a generic marketing essay.

Update the progress record with draft/review/implementation status when authorized. Only an explicit human decision can mark copy approved. Never treat repeated use of a draft as approval.

When the user corrects a recurring mistake, propose a focused update to the relevant reference. Keep stylistic preferences separate from release-specific facts. Use [acceptance cases](tests/acceptance-cases.yaml) to check future skill revisions.
