# Demand attribution fixes

> Historical plan, source-compared on 2026-09-17. For contributors evaluating
> attribution work: search-event changes and widget company fallback exist, but
> the original trust guarantee, email scope, and measured exit criteria are not
> established as complete by this checkout.

## Current implementation and remaining gaps

All three event emitters now populate a normalized client-supplied anonymous ID:
[public search](../../server/internal/handler/docs.go),
[help-center answers](../../server/internal/handler/docs_helpcenter_answers.go),
and [widget help search](../../server/internal/service/support_inbox_widget.go).
They do not populate `WidgetSessionID` in these event calls. The
[normalizer](../../server/internal/service/support_events.go) trims the token
and rejects empty values or values over 128 bytes. This is browser attribution,
not proof of identity. Answer recording includes cache hits; requests rejected
before an answer response, such as budget-blocked requests, do not reach that call.
No production measurement here establishes the proposed 95% coverage target.

The [company resolver](../../server/internal/service/support_inbox.go) matches
an existing company by email domain when widget identity has no usable declared
company. It filters a fixed list of free/disposable providers and never creates
a company through that fallback. The widget identify transaction records
`company_match_method = email_domain`, while retaining the person's original
`identity_method` and `identity_trust`. This differs from the proposed
probabilistic `identity_trust` plus `identity_method = email_domain` representation.

**The original context-only safety guarantee is not established.** The
[identity model](../../server/internal/model/support_inbox.go) explicitly separates
company provenance from person trust, but
[behavioral identity resolution](../../server/internal/repository/crm_signal_rule.go)
returns company ID with the person's trust without checking `CompanyMatchMethod`.
The [rule evaluator](../../server/internal/service/crm_signal_rule_evaluator.go)
uses verified evidence trust in activation eligibility. These paths do not prove
that an inferred company is always barred from activation; the plan's “no new
guard is needed” claim needs implementation review before being relied on.

The widget fallback does not establish email-channel coverage. The inspected
[inbound email creation path](../../server/internal/service/email_fallback.go)
still attaches a matched/created contact without invoking the company-domain
resolver. The [resolver tests](../../server/internal/service/support_inbox_email_domain_company_test.go)
cover widget matching and exclusions, not before/after production attribution rates.

## Original proposal and assessment

The counts, line numbers, vendor prices, competitive claims, and traffic assumptions
below are the August assessment, not current measurements or verified market facts.

**Original status (2026-08-30):** ready to build
**Date:** 2026-08-30

Two defects that make support demand unattributable to accounts. Both are small.
Both are worth fixing on their own merits, independent of any feature built on
top of them.

## Defect 1 — search and answer events carry no identity

`SupportEventInput` already has `AnonymousID` and `WidgetSessionID`
(`server/internal/service/support_events.go:23-24`). None of the three
`widget_search_performed` emitters populate them:

- `server/internal/handler/docs.go:2142`
- `server/internal/handler/docs_helpcenter_answers.go:58`
- `server/internal/service/support_inbox_widget.go:1374`

Measured in the local dev database:

| Event | Rows | With any identity |
|---|---|---|
| `widget_search_performed` | 71 | **0** |
| `ai_handoff_triggered` | 18 | 18 conversation, 0 anonymous/session |
| `support_gap_evidence` | 7 | **0** conversation, session, document, or article |

Every question a customer asks is currently recorded anonymously, so support
coverage evidence cannot be traced back to who asked.

**Fix:** pass the values already in scope at each call site. The widget service
has the session; the two handlers can read it from the request.

Also remove the `if !response.Cached` guard at
`handler/docs_helpcenter_answers.go:58`. Cached answers record no event at all,
so every repeat ask — the strongest evidence that a question matters — is
silently dropped.

## Defect 2 — conversations rarely resolve to a company

`resolveWidgetCompanyPayload` returns `nil` when the customer's identify call
omits a `company` object (`server/internal/service/support_inbox.go:3413-3416`),
and there is no email-domain fallback anywhere. The only company `GetByDomain`
calls are the widget payload path (`support_inbox.go:3429`) and the separate
enrichment flow (`crm_enrichment.go:608`).

Consequences:

- A contact identified as `jane@acme.com` is **not** linked to the Acme company
  record.
- Email conversations never get a company at all — only `crm_contact_id`
  (`support_inbox.go:1466`, `email_fallback.go:3446`). `SetCRMCompanyIfUnset` has
  exactly two call sites, both inside the widget identify transaction
  (`support_inbox_widget.go:199,313`).

Traffic is widget-dominated today and email is landing soon, which makes this
worth fixing **before** email ships rather than backfilling afterwards.

**Fix:** on contact match/create, resolve company by email domain via
`CRMCompanyRepository.GetByDomain`, behind a free/disposable-provider blocklist
(gmail, outlook, yahoo, proton, icloud, mailinator, …). Never *create* a company
from a free domain — match only. Record the result on `CRMIdentityLink` with
`identity_trust = probabilistic` and a new `identity_method = email_domain`.

**Safety:** the existing signal activation gate already refuses to route
notifications or create tasks from evidence below the policy's identity-trust
requirement, so domain-inferred links are context-only by default and cannot act
on their own. No new guard is needed.

## Exit criteria

- ≥95% of new `widget_search_performed` rows carry at least one identity field.
- Repeat asks produce events (event count exceeds distinct cached answers).
- Company resolution rate on new conversations is reported before and after, split
  by widget vs email.

## What was cut, and why

This document originally proposed "Priced Demand" — clustering every customer
question into topics and ranking them by the pipeline and ARR of the accounts
asking, across six phases with new rollup tables, versioned interpretation
mappings, and a ported drilldown layer. It was cut after vetting.

- **The job is already served cheaply.** Savio sorts feature requests by Account
  MRR and Opportunity Value pulled from HubSpot/Salesforce — essentially the
  proposed screen — starting at **$39/month**. Canny has revenue-weighted voting.
  This is feature money, not product money.
- **The proposed delivery form is the category's known failure mode.** A sorted
  topics table is a dashboard, and voice-of-customer dashboards are documented as
  going stale and unused.
- **Scale floor points the wrong way.** Pricing a topic needs several hundred
  conversations per month per workspace. The feature would switch on only for the
  largest workspaces, which are the ones there are fewest of.
- **The complexity was not warranted.** The repository already carries 273 tables
  and 287k lines of Go built in six months, against a dataset of 26 conversations
  and 2 deals. Adding a sixth-phase subsystem before the existing machinery has
  met real users made the problem worse, not better.

The narrower idea that did survive vetting — churn and expansion signals derived
from unstructured support content, where ChurnZero, Vitally, and Gainsight have a
documented gap — remains worth revisiting. **If it is picked up, it should ship
as one or two shadow-mode rules on the existing signal spine, not as a new
subsystem.** Shadow mode plus the existing per-rule precision report measures
whether anyone acts on it, at near-zero cost. Build the rest only if they do.

Do not re-propose the full version without production evidence on three points:
the share of workspaces clearing the conversation-volume floor, the act-rate on
shadow signals, and whether export/cancellation-class questions actually
correlate with churn.
