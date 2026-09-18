# PM Types Folder Guide

This folder is the domain-split home for the old `@/lib/pmTypes` monolith.

## Goal

- Keep type definitions grouped by domain.
- Keep `../pmTypes.ts` as a compatibility barrel.
- Avoid reintroducing a 2000+ line catch-all type file.

## Source Of Truth

- `../pmTypes.ts`
  - Barrel exports only.
  - Do not add concrete type definitions there again.
- This folder
  - Owns the actual PM/support/agent/delivery type definitions.

## File Ownership

- `project.ts`
  - Core PM entities and requests.
  - Workflows, labels, tasks, epics, sprints, task templates, recurring, comments, attachments, associations.
- `objectives.ts`
  - Objectives, key results, objective requests.
- `automations.ts`
  - PM automations, automation rules, saved views.
- `agents.ts`
  - Agent definitions, agent runs, presets, runner health, planning artifacts.
- `support.ts`
  - Support conversations, messages, inboxes, mailbox members, email routes, AI preview/rewrite, support settings/content.
- `delivery.ts`
  - Git integrations, repositories, delivery targets, PR/branch metadata.
- `orchestration.ts`
  - Agent handoffs, structured questions, tool catalog.
- `visitor.ts`
  - Support visitor context payloads.

Additional exported domains:

- `codingSession.ts`: coding-session interaction and artifact contracts.
- `skills.ts`: agent skill references and skill-catalog records.
- `taskInsights.ts`: task detail views and activity/update entries.

## Editing Rules

- Put new types in the domain file that owns the feature, not in `pmTypes.ts`.
- If a type is shared across domains, prefer:
  - keeping it near the primary domain owner
  - importing it with `import type`
  - avoiding duplicate definitions
- Use `import type` for cross-file references inside this folder.
- Keep runtime code out of this folder. These files should stay type-only.
- If a new domain grows large enough, create a new file and re-export it from `../pmTypes.ts`.

## Import Rules

- App code may continue importing from `@/lib/pmTypes` for compatibility.
- New internal cross-domain references inside this folder should import directly from sibling files.
- Avoid circular references. If two domains start depending on each other heavily, extract the truly shared types into a smaller shared file instead of layering more cycles.

## Good Change Pattern

1. Add or update the concrete type in the right domain file.
2. Add any needed `import type` references.
3. Re-export the file from `../pmTypes.ts` if needed.
4. Run type verification.

## Verification

```bash
cd frontend
pnpm exec tsc -b
pnpm run build
```
