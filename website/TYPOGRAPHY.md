# Website typography, color, and spacing

The helpin.ai marketing site (`src/app/(site)/` and `src/app/pricing/`) styles everything inside the `.hp3` scope. Tokens live in `src/app/(site)/new.css`; the inner-page system is in `src/app/(site)/_components/platform/`. Treat those files as the source of truth; this page summarizes them. For building pages, use the `helpin-website-pages` skill; for copy, `helpin-website-copy`.

## Fonts

| Token | Font | Use |
| --- | --- | --- |
| `--sans` | Instrument Sans (400, 500, 600, 700) | All text |
| `--mono` | JetBrains Mono (400, 500) | Small technical labels, table group labels, code |

Both load through `next/font` in `src/app/(site)/_components/MarketingShell.tsx`. Exact product previews use the app's own UI font.

## Type scale

| Element | Size | Line height | Tracking | Color |
| --- | --- | --- | --- | --- |
| Body | 17px | 1.6 | normal | `--ink` |
| Homepage hero h1 (`.hero h1`) | `clamp(2.4rem, 4.4vw, 3.5rem)` | 1.05 | -0.028em | `--ink` |
| Inner-page hero h1 (`.platform-hero-copy h1`) | `clamp(2.7rem, 4.4vw, 4.05rem)` | 1.08 | -0.045em | `--on-dark`, accent span `--on-dark-accent` |
| Section h2 (`.sec-head h2`) | `clamp(1.75rem, 3vw, 2.5rem)`; `clamp(1.95rem, 3.4vw, 2.8rem)` on inner pages | 1.15 | -0.035em | `--ink` |
| Card h3 | 16.5–22px | 1.3 | -0.02em | `--ink` |
| Hero lede | 17–19px | 1.6–1.85 | normal | `--t2`, or `#b9cbbf` on dark |
| Section lede | 16px | 1.85 | normal | `--t2` |
| Eyebrow (`.eyebrow`) | 12px, weight 600, uppercase | normal | 0.08em | `--em` |
| Mono label | 10.5–12px, uppercase | normal | 0.06–0.08em | `--t3` or `--em` |
| Fine print | 12.5–13px | 1.7 | normal | `--t3` |
| Button (`.btn`) | 15px, weight 600 | normal | normal | See buttons |

All headings use weight 600 and `text-wrap: balance`.

## Color

| Token | Value | Use |
| --- | --- | --- |
| `--ink` | `#131514` | Headings, body text, primary buttons |
| `--t2` | `#4F5A55` | Secondary text, ledes |
| `--t3` | `#66716C` | Captions, fine print, labels |
| `--border` / `--border-2` | `#E4E9E6` / `#CFD6D2` | Hairlines, card borders, input borders |
| `--bg` / `--bg2` | `#FFFFFF` / `#F6F8F7` | Page and soft section backgrounds |
| `--em` / `--em-h` / `--em-t` | `#0F7A50` / `#0B6340` / `#E7F4ED` | Accent, hover, tint |
| `--surface-dark` | `#090909` | Dark heroes and dark bands |
| `--art-dark`, `--art-dark-soft`, `--art-dark-raised`, `--art-dark-border` | forest charcoals | Dark workflow art and cards |
| `--on-dark` / `--on-dark-muted` / `--on-dark-accent` | `#EDF3EF` / `#AEBDB5` / `#9CDBB3` | Text and accents on dark |
| `--amber*` | amber set | Warnings, “coming soon”, and “partly” states only |

## Buttons

- `.btn .btn-primary`: ink with white text on light surfaces; light green `#d5efc8` with dark green text on dark heroes and the dark nav.
- `.btn .btn-secondary`: outline variant.
- Radius 8px, height 40px (36px in the nav).

## Spacing and layout

| Element | Value |
| --- | --- |
| Section padding | 96px top and bottom on desktop; 56–64px on phones |
| Section divider | 1px `--border` top border |
| Container (`.wrap`) | 1120px with 24px side padding; inner pages widen to 1280px |
| Section head to content | 48px |
| Card radius | 14–18px |
| Breakpoints | 1100px, 960px, 720px (or 680px), 600px |
