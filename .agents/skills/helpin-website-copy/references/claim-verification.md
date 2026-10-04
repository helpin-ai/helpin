# Claim verification

Use this reference to decide whether a claim is ready for the intended release. Verify the claims affected by the task; a small wording edit does not require a whole-product audit.

## Evidence by claim

| Claim | Appropriate evidence | Insufficient by itself |
| --- | --- | --- |
| Product behavior | Release-matched handler/component plus a supported workflow or maintained contract | Old marketing copy or unreleased code |
| Price, allowance, trial, billing unit | Approved billing configuration and commercial owner decision | A previous draft |
| License | Applicable license files and commercial terms | An “open source” badge |
| Availability and plan gates | Target deployment configuration and current rollout decision | A route existing in development |
| CLI, SDK, API | Intended package/release signatures, authentication contract, runnable example when possible | A plausible command name |
| Performance or compliance | Scoped measurements or applicable formal evidence | Framework choice or self-hosting |
| Social proof | Approved attribution and quote | Fictional demo dialogue |

Record claim, source path or URL, revision/release, deployment scope, date checked, status, qualifier, and remaining conflict. Do not store secrets or private customer excerpts in the evidence register.

## Claims about other products

Comparison copy carries the most legal and trust risk. Before naming another product:

- Use the product's own pricing pages and documentation, checked on a recorded date. Reviews and third-party roundups can point you somewhere but are not evidence on their own.
- Quote list prices with the billing period and currency, and say what the comparison leaves out (add-ons, usage, taxes).
- Don't say a product lacks something unless its documentation confirms it. Prefer describing what Helpin does. When the other product has a similar feature, credit it: for example, Intercom and Zendesk both draft help-center content from support conversations, and Zendesk accepts your own OpenAI key through Marketplace apps or its API.
- Treat competitor facts as stale after about 90 days, and re-check them before any edit that touches them.
- Refer to other products by name only; never use their logos or brand marks.

## Conflicts and unknowns

A broken public link means the link could not be verified; it does not establish that the feature is unavailable. A product owner's offering decision may differ from an older release guide. Preserve the decision and identify the release conflict privately rather than silently narrowing or expanding the offering.

Use statuses such as unverified, verified, conflicted, and stale. Verification applies to a defined release and environment, not forever. If a claim cannot be resolved, write a narrower supported sentence and mark the affected slot in the report as not publication-ready. Keep the section and continue independent work.

## Boundaries worth checking

- Public widget identity, session API authentication, and MCP permissions are different mechanisms.
- Identity verification does not authorize unrestricted account access.
- External MCP tools used by Helpin and public MCP used by another AI client have opposite access directions.
- An approval is scoped to its action. Release, publishing, and customer messages can have separate gates.
- Full module inclusion is distinct from hosted plan capacity, provider services, and enabled integrations.
- Self-hosting establishes deployment control, not model ownership or local processing by external providers.

These are verification criteria, not mandatory disclaimer copy. Respect authorized copy removals while avoiding a contradictory promise elsewhere.

## Reporting and publication

Put internal evidence and missing information in the accompanying report. Keep only necessary visitor-facing qualifications in the page. Do not add fake citation tokens from a pasted draft. Publication, deployments, remote edits, and messages require the authorization appropriate to those actions; a copy request alone does not supply it.
