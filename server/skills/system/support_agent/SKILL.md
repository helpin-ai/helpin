---
name: support_triage_response
description: Live support conversation handling - grounded replies, escalation judgment, and sub-agent usage.
metadata:
  title: Support Triage and Response
  supported_runtimes:
    - native_sdk
---

## Workspace support guidance

This skill supplies workspace-specific support guidance. Echo's system instructions own knowledge grounding, privacy, tool permissions, acknowledgments, continuation, and handover mechanics. Do not redefine those rules here. Product policies and facts must come from configured knowledge, not from assumptions in this skill.

## Investigation workflow

Use the workspace's configured knowledge for product questions. For a customer-specific issue, identify the affected account, workspace, resource and time window from existing conversation context. Use only tools actually available in this run and within their documented coverage. If no workspace-specific diagnostic coverage is defined, do not assume any logs or account tools exist.

Workspace maintainers should document:
- Which issue types each connected diagnostic tool covers and what it cannot establish.
- The verified identity and scope required before each lookup, required inputs, useful filters, and when results are insufficient.
- Product-specific distinctions needed to interpret results, including dependencies on external providers.
- Appropriate team or inbox destinations for issues that need human action, when verified mappings exist.

Do not invent tool names, arguments, permissions, repository mappings, or product capabilities. A tool mentioned in a skill is not automatically available. If a necessary lookup is unavailable or outside coverage, use the system's clarification or handover rules.

## External-provider questions

Workspace guidance may identify relevant external providers and the circumstances that need official provider research. Keep provider capabilities, this product's supported behavior, and evidence about the customer's incident separate. Use the system's permitted research flow; do not add alternate launch, approval, or reply mechanics here.

## Background investigation context

Document verified feature-to-repository mappings and concrete code questions that would justify the system's optional post-handover investigation. Do not require repository inspection during a customer-facing turn. Missing repository mappings are uncertainty, not permission to guess. A handover must not wait for optional research.

## Internal follow-up recommendations

When evidence warrants it, recommend a bug-fix task for a supported defect, feature work for a verified unmet need, or a documentation update for missing, outdated, or contradictory guidance. Explain the evidence and uncertainty in the handover brief or background investigation note. Recommend combining categories only when independently justified. Missing documentation alone does not establish that a feature is unsupported or defective.

Check existing linked work when that information is already available or during permitted background investigation; do not delay the customer turn to search internal tasks. Link a verified existing task instead of suggesting a duplicate. Recommendations do not authorize automatic task creation or promise implementation, priority, or delivery dates.
