# OG Images and Metadata Design

**Date:** 2026-08-03

## Goal

Give every Helpin marketing page and app link a deliberate, accurate social preview with complete metadata, while preventing private app content and tokenized shared documents from being indexed or leaked into cached artwork.

## Current state

- The Next.js marketing website defines global social copy but no image.
- Pricing, privacy, and terms inherit homepage social metadata.
- The Vite app shell defines only a title and theme color.
- Public document shares are client rendered, so crawlers do not receive document-specific metadata in the original HTML response.

## Chosen approach

Use a shared deterministic OG image generator and page-specific metadata.

- Marketing website: static PNG assets generated at build time and explicit route metadata for home, pricing, privacy, and terms.
- App shell: one generic app image and complete static Open Graph/X metadata, with `noindex` because private/auth routes share the same HTML shell.
- Public document shares: a Netlify Edge Function fetches only the already-public document title, replaces the app shell's marked metadata block, and returns crawler-readable metadata in the original response. It does not expose document content or workspace/customer information.

## Visual system

All images are 1200×630 PNGs. They use a warm off-white canvas, black Helpin lockup, one coral accent, large short headlines, generous safe margins, and a restrained connected-work or app-workspace illustration. Images contain no CTA buttons, tiny screenshots, customer data, pricing tables, or generic AI imagery.

Variants:

- Home: “Bring every team together. Put AI agents to work.”
- Pricing: “Focus on growth, not the seat count.”
- Privacy: “Privacy at Helpin.”
- Terms: “Helpin Terms of Service.”
- App: “Your team’s work, connected.”
- Shared document: generic “Shared document — Securely shared via Helpin” artwork. The document title appears only in HTML metadata.

## Metadata contract

Each marketing page receives a unique title and description, absolute canonical URL, index/follow policy, complete Open Graph image fields, and a complete X `summary_large_image` card. Googlebot is explicitly allowed large image previews and unrestricted snippet/video previews. The app shell receives equivalent generic metadata plus `noindex, nofollow, noarchive, nosnippet` for both general robots and Googlebot; it omits a canonical link because one static canonical would be incorrect across all SPA routes. No `meta keywords` tag is added.

The public share response uses `{Document title} — Shared via Helpin`, a generic non-content description, the exact share URL, and the shared-document image. It applies `noindex, nofollow, noarchive, nosnippet` in both robots meta tags and the `X-Robots-Tag` response header.

## Public-share safety

- Fetch only the existing public share API.
- Read only `document.title`; never include document content in metadata.
- HTML-escape every dynamic value.
- Encode the share token and never log it.
- On upstream or transformation failure, preserve the generic app metadata and continue serving the SPA.
- Do not opt the edge transformation into CDN caching.

## Testing

- Metadata contract tests for every marketing route.
- PNG signature, 1200×630 dimension, and file-size checks.
- App shell assertions for complete default metadata and absence of misleading canonical/keywords tags.
- Edge-function tests for environment selection, escaping, token encoding, safe fallbacks, and privacy.
- Production builds for website and frontend.
