# New Home Hero Design

## Goal

Create a standalone, high-impact hero at `/new-home` while leaving the existing homepage at `/` unchanged. The hero must immediately explain that Helpin is one connected work operating system where AI agents coordinate project management, support, CRM, and documentation.

## Visual thesis

A warm editorial canvas becomes a living operating system: quiet work signals travel through an emerald Helpin core and leave as coordinated, completed outcomes. The result should feel precise, dimensional, and premium rather than futuristic or noisy.

## Content

- Eyebrow: `The AI operating system for work`
- Headline: `One system. Every team.` / `AI agents that move work forward.`
- Supporting copy: one short sentence naming Projects, Support, CRM, and Docs and explaining that Helpin connects their context.
- Primary CTA: `Start free trial`
- Secondary CTA: `Watch it work`, which restarts or focuses the animation rather than navigating away.
- Small reassurance: no credit card required.

## Composition

The hero fills the first viewport beneath the existing Helpin navigation. Copy is left-aligned in a narrow, calm column. The right side is dominated by a bespoke animated system visualization, not a dashboard screenshot or a collection of cards.

The visualization has three layers:

1. Incoming work signals from Projects, Support, CRM, and Docs.
2. A central Helpin core that gathers context and visibly reasons across it.
3. Outgoing outcomes that resolve, update, advance, or ship work.

Desktop uses an asymmetric two-column composition with the visualization extending beyond the conventional content grid. Mobile stacks copy above a simplified but fully legible version of the system.

## Motion direction

The animation runs as a calm 10–12 second narrative loop:

1. The hero copy enters in a staggered sequence.
2. Four module signals wake in sequence and send luminous packets toward the core.
3. The core gathers the packets, contracts slightly, and emits a restrained emerald pulse.
4. Packets travel outward to outcome labels such as `Plan updated`, `Ticket resolved`, `Deal advanced`, and `Docs synced`.
5. The system settles into a breathing idle state before looping.

Motion uses transform and opacity wherever possible. SVG paths provide the routes and CSS keyframes drive the deterministic loop. Pointer movement adds only a few pixels of depth on desktop. The `Watch it work` control restarts the narrative. `prefers-reduced-motion` removes travel and parallax while keeping a polished static end state.

## Architecture

- Add `website/src/app/new-home/page.tsx` as the route entry.
- Keep the route page small and compose the visual from local, purpose-specific components.
- Add a route-scoped CSS module for complex animation timing and responsive behavior so global homepage styles remain untouched.
- Reuse the existing logo, typography, color tokens, signup URL, and global navigation.
- Do not modify `website/src/app/page.tsx`.

## Interaction and accessibility

- The visualization is explanatory but nonessential; meaningful product information remains in text.
- Decorative SVG layers are hidden from assistive technology.
- CTA focus states are visible and keyboard accessible.
- The restart control has an explicit accessible label.
- Animation pauses when the hero is outside the viewport or the document is hidden.
- Responsive layout is tested at common desktop and mobile sizes.

## Verification

- Confirm `/` is unchanged.
- Confirm `/new-home` renders without console or hydration errors.
- Run website typecheck and production build.
- Test desktop, mobile, and reduced-motion states.
- Confirm animation loop, restart control, CTA links, and first-viewport fit.

## Out of scope

- Reworking the rest of the homepage.
- Changing the shared navbar or footer.
- Adding new dependencies or image assets.
- Replacing `/` with the experiment.
