# OG card artwork

Text-free artwork panels for the website's social preview cards. `scripts/generate-og-images.mjs`
draws all text (Instrument Sans) and places the matching panel on the right 440px of each card.
A card without an artwork file falls back to its drawn visual.

- Model: `gpt-image-2.5-sunburst`, 1024x1536, medium quality (September 2026)
- Prompts: `STYLE` and `SUBJECTS` in `scripts/generate-og-art.py`
- Stored as JPEG, 660x945 (the 440x630 panel at 1.5x), centre-cropped, quality 82

## Regenerate a panel

1. `OPENAI_API_KEY=... python3 scripts/generate-og-art.py <slug>` writes options to `og-art-options/`.
2. Review the options and pick one. It must contain no text.
3. Crop and compress it to `scripts/assets/og-art/<slug>.jpg` at 660x945. For example, with sharp:
   `sharp(src).resize(660, 945, { fit: 'cover' }).jpeg({ quality: 82, mozjpeg: true })`.
4. `HELPIN_OG_VARIANT=<slug> pnpm generate:og`, then check the card in `public/og/`.

The pricing panel is a background only; its text is drawn by `pricingVisual(true)`.
