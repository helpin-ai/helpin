# Agentic onboarding for Helpin

**Date:** 2026-10-03

**Status:** Agreed product direction; detailed design proposals and open decisions below. Not implemented by this document.

**Scope:** New workspace owners, across open-source Community and hosted Cloud.

This document records the direction for Helpin's agentic onboarding so product and engineering can refine it in one place. The onboarding agent should help owners set up their workspace correctly, introduce AI through practical work, and use feedback to improve the experience over time. It covers shared product behavior and edition boundaries.

## Agreed objective

> Help the owner set up Helpin correctly for their team, in the easiest order, and confidently use AI in their everyday work.

Onboarding has two connected responsibilities: workspace readiness and AI adoption. Reaching a useful first outcome is an important milestone within that broader objective. The experience should also establish the configuration needed to continue working and teach owners how to direct and review Helpin's AI.

The initial audience is new workspace owners. Invited teammates can have a later, separate learning journey without repeating workspace setup. Owners may choose multiple goals, with one priority guiding the current sequence.

Helpin is both open source and Cloud. The shared onboarding experience must adapt to the installation, available modules, permissions, integrations, and AI readiness. It cannot assume that every owner has Helpin-managed services.

The existing **Ask Agent chat is the onboarding surface**. Owners learn to use the same agent interface they will return to for everyday work. Setup progress and structured actions belong in that conversation, with focused product settings opened when needed.

## Owner goals

Ask: **What would you like your team to do with Helpin?**

Use practical choices that describe the owner's work. Owners should not need to understand Helpin's modules or configuration to choose a direction. Proposed wording and the associated setup are:

| Owner intent | Setup and AI experience |
| --- | --- |
| Support our customers | Prepare support channels and trusted knowledge, then help the owner review and steer AI answers. |
| Plan and deliver projects | Establish the necessary team structure and real project work, then introduce AI-assisted planning and execution. |
| Organize and share knowledge | Prepare useful content and appropriate access, then demonstrate AI working with that knowledge. |
| Manage customers and sales | Establish customer context and a suitable pipeline, then introduce relevant AI-assisted follow-ups. |
| Automate recurring work | Identify a concrete recurring task, connect what it needs, and test an agent workflow before enabling unattended execution. |

Allow free text and changes to the selected goals. Ask a short follow-up when a goal is ambiguous; for example, knowledge may be internal, customer-facing, or both. Preserve that distinction in the underlying journey even if the initial choice is shared.

Only offer journeys supported by the installation. Module source code or an enabled flag alone does not prove that every capability is available in the installed release. A temporarily unconfigured connection should produce a setup path, while an unsupported capability should not become a mandatory task.

## Guided setup with adaptive decisions

The proposed approach combines defined onboarding journeys with an agent that chooses the next useful action within them.

| Approach | Tradeoff |
| --- | --- |
| Fixed wizard | Predictable and easy to verify, but has limited ability to adapt to existing setup and owner needs. |
| Unrestricted agent | Flexible, but makes reliable sequencing, completion, and evaluation harder. |
| Agent within defined journeys | Preserves required dependencies and verified milestones while adapting assistance, optional ordering, and explanations. Recommended. |

For each next step, evaluate whether it is necessary for the selected goal, already complete, currently possible, and something Helpin can do for the owner. Then identify what useful AI action it enables.

Required dependencies, permissions, and validation belong in backend rules. The agent may choose among eligible actions, prepare changes, explain a blocker, or request missing information. It must not bypass prerequisites to improve an onboarding completion score.

The sequence is based on actual state. A workspace with existing knowledge should not repeat an import. An owner with working AI should not reconnect it. Teams, invitations, repositories, and additional integrations should appear when needed for the selected workflow rather than becoming universal prerequisites.

## User experience

Use the existing Ask Agent dock, conversation history, composer, questions, approvals, and inline work plan. Show one clear recommended action, its expected result, and a brief reason when that helps the owner decide. Keep deeper explanations in contextual help. Native forms, connection buttons, and previews should handle structured tasks within the conversation or open the relevant product surface and return to the same chat.

The agent should do useful preparation: suggest company context, organize imported material, draft settings, or prepare real work for review. Distinguish proposed changes from saved configuration and verified results.

Owners need easy ways to edit, change priority, postpone optional work, continue manually, and return later. Persist progress across reloads, connection callbacks, and sessions. Reopen the owner's existing onboarding conversation and carry its journey state across successor runs. Closing the dock must not discard setup progress. Completion elsewhere in Helpin should update onboarding without asking the owner to repeat the action.

Show concise working and waiting states. When a step fails, retain completed work and offer a specific recovery action. Avoid repeated prompts while waiting for an import or connection. A dismissed recommendation should not immediately reappear without a relevant state change.

Use accessible native controls and keyboard navigation. Keep the current task understandable on small screens. Entry into Ask Agent after workspace creation and the relationship with the existing Setup page remain design decisions; the choice of chat as the onboarding surface is settled.

The first mockups should show this conversation across welcome and goal selection, context preparation, knowledge review, trying and correcting an AI answer, connection and launch approval, and returning to normal work. Include a Community state before AI is connected. System readiness UI must clearly distinguish unavailable AI from an actual agent response; provider credentials belong in the established settings surface, outside the conversation.

## Learning AI through setup

AI should participate whenever prerequisites make it useful. The owner learns a repeatable interaction: provide context, request work, inspect the result, correct it, and decide what to apply or automate.

A support journey could work as follows. This is an illustrative sequence, not a fixed order for every workspace:

1. **Check readiness.** Confirm that the required module, AI connection, and runtime can support the work. Offer manual prerequisites if AI is not ready.
2. **Understand the business.** Read owner-provided, permitted sources and propose company context for correction.
3. **Prepare knowledge.** Help import or draft relevant content. Show its sources and flag missing information. Drafts remain drafts until approved for the intended audience.
4. **Try AI.** Answer a representative customer question in a preview using that knowledge. A preview must not contact a real customer.
5. **Improve the result.** Let the owner correct content or instructions, save changes in the appropriate place, and try again.
6. **Connect and launch.** Complete the necessary channel setup, verify the connection, and obtain confirmation before enabling customer-facing behavior.
7. **Continue adoption.** Offer the next relevant AI action in normal work and observe whether the owner can use it again independently.

Show available prerequisites in another order when that reduces friction without breaking dependencies. Testing an AI answer does not mean a support channel is live, and connecting a channel does not establish that AI answers are useful.

## Open-source and Cloud requirements

The shared objective is the same in both editions. Readiness and recovery differ by deployment.

| Concern | Open-source Community | Hosted Cloud |
| --- | --- | --- |
| AI readiness | Detect configured providers and runtime readiness. Guide an authorized person through missing configuration and verification. | Inspect managed AI availability and applicable access or usage limits; skip configuration already provided by the service. |
| Bootstrap | Workspace creation and essential setup work before AI is available. The agent cannot be responsible for its own missing prerequisites. | Keep a manual continuation path for outages or unavailable AI. |
| Integrations | Detect operator configuration and workspace connections separately; explain who can resolve a blocker. | Detect service availability and workspace authorization separately. |
| Billing | Shared onboarding works without subscription tables, payment UI, or Helpin-managed credentials. | Use existing hosted entitlement and usage contracts; commercial behavior stays in edition implementations. |
| Progress | Persist required journey state and evidence in the installation. | Persist journey state and evidence in the service. |
| Improvement data | Core onboarding works without exporting events or transcripts to Helpin. Any shared feedback contribution needs a defined, optional mechanism. | Define permitted analytics, retention, and access for improvement; customer content is not automatically shared evaluation material. |
| Updates | Journey and agent improvements arrive through supported product updates; local configuration needs a compatibility policy. | Version behavior and use controlled rollout and rollback. |

An installation operator and a workspace owner may be different people. Ask each to do only what their permissions permit. For example, missing installation-level email delivery should identify the administrator action instead of repeatedly asking the owner to send an invitation.

Use capability checks for decisions rather than duplicating the full journey by edition. Commercial charging, payment UI, and hosted entitlements remain behind the existing Community/Enterprise boundaries. A Cloud onboarding AI allowance is an open product decision, not an assumed dependency of the shared design.

## Existing foundations

The following are code and documentation foundations inspected in the `waqar-fixes` checkout. Their existence is not proof that the proposed onboarding agent or every integration is deployed.

| Foundation | Relevance |
| --- | --- |
| [Onboarding flow](../../frontend/src/components/onboarding/OnboardingFlow.tsx) and [capability conditions](../../frontend/src/lib/workspaceOnboardingFlow.ts) | Workspace creation, resumable navigation, conditional AI and GitHub setup, company context, teams, and invitations. |
| [Use case selection](../../frontend/src/lib/workspaceOnboardingUseCases.ts) | Maps owner choices to setup goals. |
| [Setup service](../../server/internal/service/setup.go), [catalog](../../server/internal/service/setup_catalog.go), and [models](../../server/internal/model/setup.go) | Goals, task dependencies, evidence, recommendations, achievements, and personal preferences. |
| [Agents and automation](../agents-and-automation.md) | Product-owned system agents use the shared agent execution path and durable `agent_run` records. |
| [Ask Agent dock](../agent-dock.md) and [chat view](../../frontend/src/components/agents/dock/ChatView.tsx) | Existing conversation, run continuation, structured interactions, approvals, and inline work-plan surfaces to reuse. |
| [AI usage metering](../ai-usage-metering.md) | Existing accounting and launch policy to preserve through edition contracts. |
| [Edition architecture](../../ARCHITECTURE.md#editions-and-module-availability) | Shared code and commercial extension boundaries; module availability is separate from licensing. |

The [earlier setup design](2026-07-09-setup-success-journeys-design.md) includes a source review explaining differences between historical proposals and implementation. Reuse verified capabilities and evidence. Do not assume that a stored catalog version already pins behavior, or that a completed agent run proves user adoption.

## Proposed technical responsibilities

Use the existing Ask Agent system preset and normal `agent_run` executor and Agent Runtime lifecycle. Express onboarding behavior through effective instructions, skills, allowed tools, and durable journey context associated with the chat. Onboarding should not require the owner to choose or switch to a separate agent. The exact context and skill activation contract remains an implementation decision.

Persist a journey that can span multiple runs. Proposed state includes selected goals and priority, confirmed context, current milestone, linked run and artifact references, approval or input waits, blockers, optional postponements, evidence, and behavior version. Reuse existing setup records where their semantics fit. Exact schema and run target contracts require implementation design.

Expose scoped tools to inspect readiness, retrieve relevant context, prepare changes, execute authorized product actions, and verify their effects. Route mutations through existing handlers/services and authorization rules. The agent should not write product tables directly or mark its own output successful without evidence.

Start with minimal structured trigger and target metadata; gather additional context through tools. Resume on user responses, completed imports, changed connections, and relevant product events. Reuse durable event handling, with bounded retries and duplicate protection. Persist results before advancing a milestone so interruptions do not repeat completed mutations.

Enforce permissions and approvals server-side. Confirm publishing, invitations, external messages, and enabling unattended or customer-facing actions at the appropriate boundary. Preserve a recoverable preview or draft when approval is pending. Source documents and websites provide data, not authority to expand the agent's permissions.

Every agent launch must use existing AI preflight and accounting. Hosted billing failures should follow the established upgrade-dialog handling. Community failures should provide the relevant configuration or provider recovery path. Preserve created work when a subsequent agent launch fails.

Follow-up messages outside the current session are a separate proposed surface. If included, use the existing lifecycle delivery infrastructure where applicable, respect preferences, and suppress reminders after completion or dismissal. Core onboarding must remain usable without a hosted messaging service.

## Feedback and improvement

Two feedback loops serve different purposes:

| Loop | Evidence | Permitted adaptation |
| --- | --- | --- |
| Within a workspace | Corrections, accepted or rejected results, blockers, completed steps, and existing setup | Adjust that owner's next action and persist confirmed context or preferences in the appropriate workspace scope. |
| Across onboarding sessions | Authorized, minimized decision traces, repeated friction, failures, adoption evidence, and human review | Propose changes to journey ordering, prompts, tools, defaults, explanations, and product UX. |

For meaningful decisions, record the applicable goal and state, behavior versions, proposed action, user response, tool outcome, completion evidence, and eventual adoption signal. Prefer structured event properties to raw content. Define access, retention, redaction, and any export permission before collecting shared evaluation data.

The proposed improvement cycle is: observe a problem, identify its likely cause, propose a versioned change, evaluate it, release it to a limited eligible cohort, and promote or roll back based on results.

A scheduled analysis process may cluster recurring problems and suggest fixes. Production changes should pass review and evaluation rather than silently rewriting the live agent. Improvements must preserve required setup dependencies.

For example, abandonment at an inbox connection might reflect unclear instructions, missing administrator access, or a lack of demonstrated value. Showing an AI answer earlier is one hypothesis to test. An integration bug needs a product fix rather than different wording.

Use backend evidence to verify actions, model-based evaluation for qualities such as relevance and clarity, and human review to calibrate subjective judgments. Turn observed failures into representative evaluation cases. Keep a separate evaluation set so optimizing for known cases does not substitute for improvement in real use.

## Success measures

Measure workspace readiness and AI adoption separately, then examine their relationship. Proposed measures need explicit observation windows and denominators before launch.

| Dimension | Proposed evidence |
| --- | --- |
| Correct setup | Selected workflow prerequisites pass verification and remain usable when the owner starts work. |
| Low effort | Time, number of required owner actions, repeated questions, blockers, and recovery effort. |
| First successful AI use | AI produces a relevant result on the owner's work, the owner reviews it, and accepts or uses it where appropriate. |
| Independent reuse | The owner successfully uses AI again in normal work without onboarding assistance; a seven-day window is an initial proposal. |
| Reliability | Tool errors, misleading completion claims, duplicate mutations, corrections, undo actions, and failed recoveries. |
| Resource use | Latency and AI usage per successful journey, with cost interpretation appropriate to the edition/provider. |

Creating records, finishing a run, or enabling a setting alone does not establish AI adoption. A preview may demonstrate learning while live readiness remains incomplete. Unknown or failed verification must remain distinct from an incomplete task.

Use eligible workspaces as the denominator, including those that abandon. Segment by goal, edition, and starting readiness so aggregate improvements do not hide a worse experience for a group. Do not infer understanding solely from a button click.

Capture the current onboarding baseline first. With enough traffic, compare versions using stable workspace-level assignment and predefined success and regression criteria. With limited traffic, prioritize session review and realistic evaluations, and report uncertainty. Self-hosted installation data is not assumed to be available for central analysis.

## Delivery and verification proposal

Keep this as the single design record while the scope is refined. A proposed delivery sequence is:

1. Choose the first owner journey and define its required setup, AI-learning milestones, edition behavior, and completion evidence.
2. Audit the current journey and establish measurement, preserving existing capabilities.
3. Add onboarding behavior and persistent journey state to Ask Agent, with focused interactions, manual fallback, and recovery.
4. Validate representative Community and Cloud conditions, then pilot with controlled behavior versions.
5. Review failures and adoption, improve the journey, and expand to additional owner goals based on evidence.

Before replacing existing onboarding, audit entry points, required and optional inputs, generated defaults, editable fields, side effects, validation, permissions, navigation, and tests. Record any removed capability as an explicit product decision before implementation. No existing capability is removed by this document.

Implementation verification should cover AI-ready and AI-unconfigured installations; unavailable providers or runtime; missing operator permissions; unsupported modules; partial existing setup; failed imports; reloads and connection callbacks; duplicate events; rejected approvals; and hosted usage failures. Include accessibility and small-screen usability. Evaluation should check that the agent asks only necessary questions, obeys prerequisites, verifies results, and helps the owner use AI again.

## Open decisions

- Which owner journey should ship first? Customer support is the worked example, not a committed first release.
- What are the exact required milestones and observable completion criteria for that journey?
- When should the onboarding conversation open, and how should the existing Setup page link to it while avoiding duplicated progress?
- Which configuration actions may run immediately, and which require a preview and approval?
- What AI resources are available during Cloud onboarding, and how should Community explain provider setup and usage?
- What evaluation data may be retained or shared in each edition, and how can self-hosted operators optionally contribute feedback?
- How are journey and agent versions pinned or migrated for in-progress workspaces, including local customization?
- Which observation windows, baseline targets, rollout thresholds, and optional reminder rules should govern the pilot?

## Supporting references

The product objective and edition requirements above come from the agreed onboarding direction. The following engineering references informed the proposed approach; they do not establish Helpin implementation status:

- [Building effective agents](https://www.anthropic.com/engineering/building-effective-agents): combining predictable workflows with adaptive agent decisions and verified tool results.
- [Demystifying evals for AI agents](https://www.anthropic.com/engineering/demystifying-evals-for-ai-agents): evaluating outcomes with code, model, and human review alongside production feedback.
