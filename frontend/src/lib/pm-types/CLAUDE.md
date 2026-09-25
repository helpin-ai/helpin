# Claude PM Types Context

This folder is the structured replacement for the old `pmTypes.ts` monolith.

## Intent

- Preserve compatibility for existing imports from `@/lib/pmTypes`.
- Keep actual definitions split by domain so types remain findable and maintainable.
- Make future growth additive instead of dumping everything into one file.

## Architecture

- `../pmTypes.ts` is a barrel only.
- Real definitions live in this folder.
- Cross-domain references should use `import type`.

## Where new types should go

- Project management core entities:
  - `project.ts`
- Objectives / key results:
  - `objectives.ts`
- PM automation rules / views:
  - `automations.ts`
- Agents / runs / presets / planning:
  - `agents.ts`
- Support / mailboxes / forwarding / visitor-facing support payloads:
  - `support.ts`
- Git and delivery:
  - `delivery.ts`
- Tool catalog / structured question / handoff orchestration:
  - `orchestration.ts`
- Visitor context:
  - `visitor.ts`

Additional domains exported by the barrel:

- Coding-session interactions and artifacts: `codingSession.ts`
- Agent skill references and catalog: `skills.ts`
- Task detail views and update entries: `taskInsights.ts`

## Preferred workflow

1. Add the type in the domain file that owns the concept.
2. Import dependent types with `import type`.
3. Re-export through `../pmTypes.ts` if consumers should access it through the barrel.
4. Do not add concrete definitions back into `../pmTypes.ts`.

## Avoid

- Reintroducing a single giant type file.
- Duplicating the same request/response type in multiple domain files.
- Converting these files into mixed runtime/type utility modules.
- Using value imports when a type-only import is enough.

## Validation

Run:

```bash
cd frontend
pnpm exec tsc -b
pnpm run build
```
