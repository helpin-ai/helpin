# Helpin motion recipe

Use motion to explain a short workflow. Match the existing website's calm pacing and use React DOM/SVG plus CSS for accurate text, connectors, highlights, and checkmarks.

## Lifecycle

Use `useBentoPlayback` from `website/src/app/new/_components/`. It starts just before a scene enters view (currently a 120px root margin), stops once offscreen or the document is hidden, respects reduced motion, and supplies a cycle key. Do not replace this with a 50% visibility cutoff: it caused restarts and jitter near section boundaries. Check the current hook implementation before changing it. Small bentos may use short cycles; full inbox/board stories use longer cycles (roughly 19–30 seconds) so the finished state can be read.

- Render a complete, readable static scene first.
- Apply animation selectors only under `data-playing="true"`.
- Use the returned cycle key on the illustrated scene, not on focusable controls.
- Leave a settled end state before replaying. Avoid introducing promises of speed through fake counters or typing races.
- For ongoing automatic replay, provide an accessible stop/pause control. Stopping can reveal the complete static scene rather than freezing half-visible text; playing restarts the sequence. Keep this control outside remounted scene content.
- Add a CSS reduced-motion rule as well as hook behavior. No animation, disappearing information, or moving overlays in that mode.

## Choose the right reference

Read the scene table in [palette and scene references](palette-and-scenes.md). Use the existing HTML Kanban and Support inbox implementations for full product previews, and the homepage bento implementations for compact capability cards.

- Conversation or AI draft: a fast, readable typewriter effect; reserve the message area so text does not move the page. Keep automated typing out of live regions.
- Kanban: move the task between states and let other cards move to occupy the space. Avoid an empty permanent slot where the animated card will arrive. Keep counts and agent activity consistent with the state.
- Sprints: move selected backlog tasks into a commitment, then show progress/closeout and carryover. Keep closed-sprint history visible.
- Roadmap: reveal scheduled epic bars along their actual date positions and highlight the relevant owner or dependency.
- Ask Agent: open the floating panel, reveal work/tool steps in order, then show the human review boundary. Preserve the underlying workspace context.
- Approvals/completion: use the approved green state, a restrained checkmark, and the reviewer avatar. Do not imply sending or merging before approval.

Auto-loop after the completed state has settled. Keep a small pause/play control outside any keyed scene subtree; do not add a separate “Replay this example” button or example-name label. Use stage captions only when they help explain the sequence. Do not apply typewriting to task tables, headings, or every bento.

## Timing and composition

Default to opacity plus 8–18px translation over roughly 350–650ms with `cubic-bezier(.22,1,.36,1)`. Space semantic steps around one second apart. Reserve path drawing for direction or completion. Use restrained highlights and checkmarks; do not make whole panels bounce, blur critical text, or animate page height. Use fixed or reserved stage geometry without leaving blank slots inside product lists. On mobile, reflow or hide secondary chrome while keeping the primary conversation, task, and agent result readable; do not shrink a desktop screenshot to microscopic text.

For SVGs, use a fixed viewBox, `useId` for unique clip/mask/title IDs, and `title`/`desc`. For DOM scenes, keep semantic text or provide a complete `role="img"` description and hide its redundant decoration. Do not present a simulated send/approve button as a real action.

Use the existing `ScrollReveal` for entrance animations. Add only the necessary section selectors. It keeps content visible without JavaScript, responds to reduced motion, and cancels animation around keyboard focus. Do not layer a second entrance animation on a scene's playback.

## Check observable behavior

In a real browser: offscreen means `data-playing=false`; bringing the scene into view starts it; stopping reveals a readable stationary state; restarting works; reduced motion disables animation; moving focus to the control does not lose focus after a cycle. Verify the same story is legible on mobile without hover.
