# OG card artwork

Text-free artwork for the website's social preview cards. `scripts/generate-og-images.mjs`
draws all text (Instrument Sans) on a near-black canvas and places the matching artwork on the
right 480px of each card, fading its left edge into the canvas. A card without an artwork file
falls back to its drawn visual. Pricing and the legal cards use drawn visuals only.

Style: quiet and monochrome, in the spirit of Linear's marketing. Flat near-black background
(`#0A0B0B`), charcoal UI panels with hairline borders, gray placeholder bars, and one small
emerald accent. No glow, gradients, grain or decorative lines.

- Model: `gpt-image-2.5-sunburst`, 1024x1536, medium quality (September 2026)
- Prompts: `STYLE` and `SUBJECTS` in `scripts/generate-og-art.py`
- Stored as JPEG, 720x945 (the 480x630 area at 1.5x), centre-cropped, quality 86

## Regenerate a panel

1. `OPENAI_API_KEY=... python3 scripts/generate-og-art.py <slug>` writes options to `og-art-options/`.
2. Review the options and pick one. It must contain no text, and its background must stay flat.
3. Crop and compress it to `scripts/assets/og-art/<slug>.jpg` at 720x945. For example, with sharp:
   `sharp(src).resize(720, 945, { fit: 'cover' }).jpeg({ quality: 86, mozjpeg: true })`.
4. `HELPIN_OG_VARIANT=<slug> pnpm generate:og`, then check the card in `public/og/`.
5. If the card's content changed, bump `VERSION` in `generate-og-images.mjs` and the matching paths in
   `src/lib/metadata.ts`, `src/app/(site)/_components/marketing-metadata.ts` and `tests/seo-metadata.test.mjs`.
