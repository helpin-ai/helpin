# Helpin website palette and scene references

Use for website section colors, product previews, and animation selection. The approved visual authorities are the homepage, Customer Support, and AI Agents pages. Projects should share that palette, not establish a new one. Inspect the current source before adapting a scene; these paths are relative to the Helpin repository root.

## Color roles

The shared tokens live on `.hp3` in `website/src/app/new/new.css`.

| Layer / role | Token | Value / reference |
| --- | --- | --- |
| White section or UI panel | `--bg` | `#FFFFFF` |
| Cool-gray section or bento stage | `--bg2` | `#F6F8F7` |
| Primary light text | `--ink` | `#131514` |
| Secondary / muted light text | `--t2` / `--t3` | `#4F5A55` / `#66716C` |
| Light borders | `--border` / `--border-2` | `#E4E9E6` / `#CFD6D2` |
| Emerald action or selected state | `--em` / `--em-h` | `#0F7A50` / `#0B6340` |
| Selected / completed light surface | `--em-t` | `#E7F4ED` |
| Dark marketing section | `--surface-dark` | `#090909`; Support operations and Agents coding |
| Dark illustration base | `--art-dark` | `#0B2119`; Support operations art |
| Secondary dark UI surface | `--art-dark-soft` | `#112D20` |
| Raised dark message/card | `--art-dark-raised` | `#193A2B` |
| Dark hairline | `--art-dark-border` | `#BCE2CB26` |
| Dark text / supporting text | `--on-dark` / `--on-dark-muted` | `#EDF5EF` / `#B2C6BA` |
| Dark accent / completion | `--on-dark-accent` | `#9CDBB3` |

Keep light body text neutral, with emerald reserved for actionable or meaningful states. Avoid yellow-green stage gradients and olive text throughout whole cards. Green-tinted stages are appropriate where already established, such as the Support inbox pattern or the homepage Ask Agent section. They are accents, not separate palettes for each page.

Dark heroes may retain the subtle radial forest glow and pale green CTA used by Support. The homepage’s black customer-record section and deep forest Ask Agent section are intentional anchors; do not flatten all sections into one color. Preserve the approved alternation of white, cool gray, and dark sections.

Inside product UI, keep semantic amber risk/waiting states, purple/blue workflow states, tags, agent identities, and avatar colors. Match the actual app’s light/dark appearance when reproducing a specific screen. Use shared forest colors for stylized dark marketing workflows. Do not recolor screenshots indiscriminately.

Projects’ `products/projects/project-palette.css` maps shared section/workflow roles, while the component styles use shared tokens for Objectives and planning. The Kanban retains its documented charcoal exception; the closing delivery scene owns its section palette in `project-delivery.css`. Prefer changing a role/token in the existing stylesheet over accumulating competing color overrides.

## Kanban and full Projects UI exception

The user explicitly prefers the original charcoal/sage treatment for the Kanban board and foreground task. Preserve this palette in `products/projects/project-hero.css` and `project-focus.css`:

| Role | Value |
| --- | --- |
| Board / task canvas | `#171B18` |
| Navigation / secondary surface | `#1B211D` / `#191E1B` |
| Raised task card | `#1E2420` |
| Hairline | `#303A33` |
| Main / secondary text | `#E6ECE8` / `#A4AFA7` |

Give the outer Kanban preview container an explicit charcoal background and matching radius; transparent wrappers can expose a white page surface. Use restrained sage highlights and the original semantic state colors. Do not apply the forest workflow palette to the entire Kanban or its Ask Agent/task overlay: broad dark-green surfaces overpower this full product layout. Page consistency comes from the shared section palette and hierarchy, not recoloring every product preview identically.

## Approved implementation references

Paths below start at `website/src/app/new/`.

| Need | Reference | What to reuse |
| --- | --- | --- |
| Overall section palette | `new.css`, `products/customer-support/support.css`, `products/ai-agents/agents.css` | White/gray/black hierarchy, type, borders, accents |
| Dark compact workflow art | `products/customer-support/support-inbox-features.tsx` and `.css` | Green raised cards on forest art within a black section |
| Full HTML inbox | `products/customer-support/support-workspace.tsx` and `.css` | Shared app shell, queues, thread, details, routed tags, typing AI draft, real demo controls |
| Floating Ask Agent over product UI | The same `support-workspace` with `variant="agent"` | Reuse the inbox beneath the dock; context → linked task → Forge → human review |
| Animated Kanban + task | `products/projects/project-hero.tsx`, `project-hero-board.tsx`, `project-hero.css` | Platform panel structure, card movement, neighboring-card repositioning, agent counts |
| Board/list prioritization | `products/projects/project-focus.tsx` and `.css` | Board → list → blocked task/dependency → Ask Agent recommendation |
| Sprint and roadmap | `products/projects/planning-scenes.tsx` and `.css` | Task transfer and carryover; date-aligned epic bars |
| Selectable objective details | `products/projects/project-health.tsx`, `project-objectives.css` | Separate delivery and outcome measures, linked epic/spec/task details |
| Customer-record capability bentos | `_components/CustomerRecordBento.tsx` | Purpose-specific motion; typewriter for conversations, other motion for other capabilities |
| Agent delegation and external tools | `_components/AskAgentBento.tsx`, `products/ai-agents/agent-workflow-art.tsx` | Shared context, ordered/parallel work, completion states |
| Approval boundaries | `_components/AgentControlArt.tsx`, `products/customer-support/support-controls.tsx` | Tool permissions, review gates, clear approval state |
| Playback and section entrances | `_components/useBentoPlayback.ts`, `_components/ScrollReveal.tsx` | Visibility lifecycle, reduced motion, stable keyboard focus, subtle entrances |

For exact platform structure, inspect `frontend/src/components/support/` or `frontend/src/components/pm/` first. Marketing previews can simplify content and remove secondary chrome, but should preserve recognizable layouts and field meanings. Avoid importing live app stores/providers into the website merely to reuse a layout.

For validation, capture whole sections as well as the isolated art. Check 1440px, tablet, 390px, and 320px where dense. Validate CSS-computed surface colors, text contrast, unchanged geometry across animation stages, semantic state distinctions, replay/pause, and keyboard behavior. Screenshots alone do not prove the animations work.
