# New Home Hero Design

## Goal

Create a standalone, high-impact hero at `/new-home` while leaving the existing homepage at `/` unchanged. The hero must immediately explain that Helpin is one connected work operating system where AI agents coordinate project management, support, CRM, and documentation.

## Visual thesis

A warm editorial canvas becomes a living operating system: quiet work signals travel through an emerald Helpin core and leave as coordinated, completed outcomes. The result should feel precise, dimensional, and premium rather than futuristic or noisy.

## Content

- Eyebrow: `The AI operating system for work`
- Headline, with an intentional line break: `One system. Every team.` then `AI agents that move work forward.`
- Supporting copy: `Helpin connects project management, customer support, CRM, and docs—giving your team and AI agents the shared context to plan, resolve, follow up, and ship.`
- Primary CTA: `Start free trial`
- Secondary CTA: `Watch it work`, implemented as a button that restarts the animation. On stacked mobile layouts it first scrolls the visualization into view, then restarts it.
- Small reassurance: no credit card required.

## Composition

The hero fills at least the first viewport beneath the existing Helpin navigation. At desktop widths, its minimum height is `calc(100dvh - 72px)` and its content fits without vertical scrolling at 1440×900 and 1280×720. Copy is left-aligned in a narrow, calm column. The right side is dominated by a bespoke animated system visualization, not a dashboard screenshot or a collection of cards.

The visualization has three layers:

1. Incoming work signals from Projects, Support, CRM, and Docs.
2. A central Helpin core that gathers context and visibly reasons across it.
3. Outgoing outcomes that resolve, update, advance, or ship work.

Desktop uses an asymmetric two-column composition with the visualization extending beyond the conventional content grid. Below 768px, mobile stacks copy above a simplified system: four compact labeled sources arranged around the same central core and four short outcome labels beneath it. The copy and top of the visual should establish the concept in the first viewport at 390×844; the full visualization may continue below the fold. On 320px-wide screens and short mobile landscape viewports, content flows naturally with no clipping or forced viewport-height constraint.

## Motion direction

The animation runs once as an 8.8-second narrative and then holds its resolved state:

1. `0–900ms`: eyebrow, headline lines, body, and actions enter with 90ms stagger, 700ms duration, and `cubic-bezier(.16,1,.3,1)` easing.
2. `900–3300ms`: Project Management, Support, CRM, and Docs sources wake 300ms apart; each sends one luminous packet along a curved inbound path over 900ms.
3. `3300–4400ms`: the core contracts to 0.96 for 180ms, returns with spring-like overshoot to 1.03, and emits one restrained emerald ring that fades by 4400ms.
4. `4400–7600ms`: outbound packets leave 240ms apart and resolve `Plan updated`, `Ticket resolved`, `Deal advanced`, and `Docs synced`; each outcome uses a 420ms mask reveal.
5. `7600–8800ms`: route highlights fade to their resting opacity and the system holds a complete, motionless end state.

The desktop path layout uses four source nodes on a loose left arc, a large core slightly right of center, and four outcomes on a tighter right arc. Curves converge without crossing labels. Motion uses transform and opacity wherever possible. SVG paths provide the routes and CSS keyframes drive the deterministic sequence. Pointer movement adds no more than six pixels of depth on fine-pointer desktop devices. The restart mechanism changes a React key so the client visualization remounts from its exact initial state. Restart is ignored while the user prefers reduced motion.

With `prefers-reduced-motion: reduce`, all copy staggering, packet travel, core contraction/pulse, outcome reveals, idle effects, and pointer parallax are disabled. The complete final state renders immediately.

## Architecture

- Add `website/src/app/new-home/page.tsx` as the route entry.
- Keep the route page small and compose the visual from local, purpose-specific components.
- Use a client `LivingSystemHero` component for restart state, pointer depth, `IntersectionObserver`, and `visibilitychange`; presentational SVG/node components remain stateless.
- Add a route-scoped CSS module for complex animation timing and responsive behavior so global homepage styles remain untouched.
- Reuse the existing logo, typography, color tokens, signup URL, and global navigation.
- Update the shared footer to return `null` only at `/new-home`, making this route explicitly navbar + hero only.
- Do not modify `website/src/app/page.tsx`.

## Interaction and accessibility

- The visualization is explanatory but nonessential; meaningful product information remains in text.
- Decorative SVG layers are hidden from assistive technology.
- CTA focus states are visible and keyboard accessible.
- The restart control has an explicit accessible label.
- Because the sequence runs once, leaving the viewport or hiding the document pauses it; returning resumes from the same point. No continuous idle or loop remains to require a pause control.
- Responsive layout is tested at common desktop and mobile sizes.

## Verification

- Confirm `/` is unchanged.
- Confirm `/new-home` renders without console or hydration errors.
- Run website typecheck and production build.
- Test at 1440×900, 1280×720, 390×844, 320×568, and a short mobile-landscape viewport; verify 200% zoom without content loss.
- Verify keyboard-only operation, visible focus, contrast, and the full reduced-motion static state.
- Confirm animation loop, restart control, CTA links, and first-viewport fit.

## Out of scope

- Reworking the rest of the homepage.
- Changing the shared navbar. The footer receives only a route-specific visibility exception for `/new-home`.
- Adding new dependencies or image assets.
- Replacing `/` with the experiment.
