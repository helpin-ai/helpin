# Historical Note: Product-Spec Planning Pipeline PRD

This PRD is retained for historical context.

It describes an older staged planning pipeline with stricter class-driven target mapping and hidden planner orchestration that no longer matches the active runtime.

## Current Planner Model

The shipped direct-run planner model is:

1. start one interactive run against the epic
2. ask clarifying questions inline
3. draft the PRD inline
4. get approval inline in chat
5. persist the approved spec through tools
6. plan stories inline
7. get approval inline in chat
8. create stories through tools

## Current Source Of Truth

Use this doc instead:

- `docs/AGENTS_AND_AUTOMATION.md`
