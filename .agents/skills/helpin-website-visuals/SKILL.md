---
name: helpin-website-visuals
description: Create consistent Helpin marketing website product imagery and animated workflow scenes. Use when adding or revising website product pages, generated demo screenshots, bento illustrations, and website motion; excludes the application UI and unrelated websites.
---

# Helpin website visuals

Create product-led visuals for Helpin's marketing website. Keep each scene grounded in a customer task, using the website's established green, white, and forest palette. This skill governs marketing presentation; it does not define product capabilities.

## Find the current references

Resolve paths below from the Helpin repository root (the directory containing `website/` and `AGENTS.md`). If invoked from the parent workspace, locate that repository first.

- Brand and typography: `website/src/app/new/layout.tsx` and `new.css`.
- Existing product imagery: `website/public/new/product/`.
- Canonical demo identity: `output/imagegen/product-orbitdesk/` when present. Those corrected references take precedence over older images still bearing Helpin Studio or using OrbitDesk as the customer company.
- Palette authorities: the homepage, `website/src/app/new/products/customer-support/`, and `website/src/app/new/products/ai-agents/`.
- Read [palette and scene references](references/palette-and-scenes.md) before styling a section or choosing a product preview. It maps approved surface roles and HTML animation examples to their source files.
- Animated illustration references: `website/src/app/new/_components/AgentControlArt.tsx`, `CustomerRecordBento.tsx`, and `AskAgentBento.tsx`.
- Playback and reveals: `useBentoPlayback.ts` and `ScrollReveal.tsx` in that components directory.

Inspect the closest approved visual and its consuming code before starting. Existing references guide composition; do not copy a screenshot's accidental text or identity inconsistencies.

## Choose the medium

- Use the available image-generation tool for new product screenshots, raster illustrations, or edits to existing generated images. Use the installed `imagegen` skill for its tool workflow when available.
- Prefer shared React/DOM/SVG previews for platform UI and workflows that benefit from exact text, responsive layout, and meaningful animation. Inspect the application components first; reuse their field order, panel structure, typography, and icons. Keep marketing demos isolated from application auth, APIs, and visitor chat sessions. Preserve existing logo assets and icon systems.
- Reuse the same HTML product shell when two sections show the same workspace, as Support does for its inbox and floating Ask Agent. Use raster imagery where it adds value; do not replace an approved HTML preview with an image by default. Motion should show a change: context found, a draft prepared, a teammate assigned, a task linked.
- The user's established image-model preference is GPT Image 2.5. Select it when the available workflow supports and confirms it. A built-in tool without a model selector cannot establish the model version. Disclose that limitation and follow the user's instruction; do not label unknown provenance as GPT Image 2.5. Built-in generation was authorized for the first Customer Support draft, not as a permanent override of future explicit model requests.

## Shared visual contract

Use `website/src/app/new/new.css` as the token authority. Use white and cool gray for light sections, emerald for accents, near-black for dark marketing sections, and the approved forest surfaces for dark workflow art. Reuse the semantic roles in [palette and scene references](references/palette-and-scenes.md); avoid inventing a separate olive, lime, or warm-gray palette for each product page.

Keep the full Projects Kanban/task previews in their approved charcoal/sage palette; see the explicit exception in the palette reference. Do not wash those screens in the forest-green workflow colors.

Keep three layers distinct: section background, illustration stage, and product UI. Use restrained borders and generous spacing. Semantic task states, tags, avatar colors, warnings, and code syntax retain their meaning; a brand alignment is not a reason to make all UI colors green. Instrument Sans is the marketing font; exact platform previews use the app’s Inter font. Small technical labels use JetBrains Mono.

Demo identity, unless the user specifies another scenario:

- **OrbitDesk** is the workspace/product, with a forest-green square and white O monogram.
- **Northstar Labs** is the customer company.
- **Maya Chen**, `maya@northstar.example`, is the customer.
- **Sam Rivera** is the teammate; use the existing avatar or sage SR initials as appropriate.
- **Helpin AI** and **Ask Agent** retain their product names.
- Continue the section’s established story: Support uses Maya’s incomplete CSV export and EXP-142; Projects uses Slack alerts for failed syncs and ORB-491. Older SSO demos remain valid in their original sections but are not the default for new work. Keep customer names, task IDs, owners, and statuses consistent across a sequence. Distinguish proposed work, approved changes, merged code, and sent customer updates.

## Images

Read [the image recipe](references/image-recipe.md) when generating or editing a bitmap. Inspect every input and label its role: edit target, layout reference, or identity reference. Keep real product structure recognizable and qualify generated screenshots as illustrative demos.

Store selected deliverables in `website/public/new/<page>/`, with versioned names, and source/prompt/provenance in `output/imagegen/<page>/`. If the output folder is ignored, put the reproducible prompt/provenance alongside the skill or in another tracked task-appropriate source location; do not rely solely on an ignored artifact. Use actual pixel widths in filenames. Never overwrite approved assets just to test an alternative.

Export responsive WebP variants without upscaling, preserve aspect ratio, specify intrinsic dimensions and `srcSet`/`sizes`, and inspect the result at its display size. Prioritize the hero image; lazy-load lower imagery. Keep the full-size image available when small text benefits from inspection.

## Animation

Read [the motion recipe](references/motion-recipe.md) when adding a scene. Reuse the existing playback hook and scroll-reveal system; do not install a second animation library for these scenes.

The default/static state must tell the whole story with no JavaScript or with reduced motion. Animation progressively illustrates it; it is never required to discover the message or operate a real control. Keep automated changes out of live announcements and give each illustrated sequence one useful accessible description.

## Content and delivery checks

- Match each claim to current product code, docs, and release scope. Community's default support/docs/agents surface does not imply availability of every cloud workflow. Do not import competitor channels, performance numbers, testimonials, or product claims.
- Inspect desktop, tablet, and 390px mobile renders. Check 320px when content is dense. Verify no page overflow, unreadable overlays, missing images, or clipped controls. Ensure image labels agree with nearby page copy.
- Exercise animation visibility, pause/replay, reduced motion, keyboard focus, and FAQ/anchor navigation. Check hydration and console errors.
- Run the website type check and relevant existing checks. The Next configuration isolates development in `.next-dev` and production in `.next`. Keep that separation: with static export, `NEXT_DIST_DIR` changes the export destination but does not isolate the production build cache. After any build beside a running preview, request its CSS and JavaScript URLs and check their HTTP status and applied styles, not just the HTML response.
- Report a working preview link, generated asset and prompt locations, the tool/model provenance actually known, and any unverified limitation. Do not imply deployment or production availability from a local preview.
