# Helpin Website Typography & Spacing Standards

## Typography Scale

| Element | Size | Weight | Leading | Tracking | Color |
|---------|------|--------|---------|----------|-------|
| **Hero h1** | `clamp(2.25rem,5vw,4.75rem)` | `font-bold` | `leading-[1.0]` | `tracking-[-0.04em]` | `text-foreground` |
| **Section h2** | `clamp(1.875rem,3.5vw,3rem)` | `font-bold` | `leading-[1.08]` | `tracking-tight` | `text-foreground` |
| **Sub-section h3** | `text-[clamp(1.5rem,2.6vw,2.25rem)]` | `font-bold` | `leading-[1.12]` | `tracking-tight` | `text-foreground` |
| **Card title** | `text-lg` (18px) | `font-bold` | default | default | `text-foreground` |
| **Body text** | `text-[17px]` | `font-normal` | `leading-relaxed` | default | `text-muted-foreground` |
| **Body text (small)** | `text-[15px]` | `font-normal` | `leading-relaxed` | default | `text-muted-foreground` |
| **Feature list** | `text-[14px]` | `font-normal` | default | default | `text-muted-foreground` |
| **FAQ question** | `text-[18px]` | `font-semibold` | default | default | `text-foreground` |
| **FAQ answer** | `text-[17px]` | `font-normal` | `leading-relaxed` | default | `text-muted-foreground` |
| **Label/eyebrow** | `text-[10px]` | `font-medium` | default | `tracking-widest` | `text-muted-foreground` uppercase |
| **Caption/fine print** | `text-[13px]` | `font-normal` | default | default | `text-muted-foreground/50` |
| **Button primary** | `text-[15px]` | `font-semibold` | default | default | `text-background` |
| **Nav link** | `text-[15px]` | `font-medium` | default | default | `text-muted-foreground` |

## Bold / Emphasis in Body Text

Use `<strong className="text-foreground font-semibold">` for inline emphasis within body text.

## Spacing

| Element | Value |
|---------|-------|
| **Section padding (desktop)** | `py-32` (8rem) |
| **Section padding (mobile)** | `py-16` (4rem) |
| **Heading to body gap** | `mb-6` (1.5rem) — standard everywhere |
| **Body to CTA gap** | `mb-10` |
| **Between sections** | `border-t border-border` |
| **Max content width** | `max-w-7xl` (80rem) |
| **Max text width (centered)** | `max-w-3xl` (48rem) |
| **Max text width (body)** | `max-w-2xl` (42rem) or `max-w-lg` (32rem) |
| **Horizontal padding** | `px-6 lg:px-8` |

## Colors

| Token | Usage |
|-------|-------|
| `text-foreground` | Headings, strong text |
| `text-muted-foreground` | Body text, descriptions |
| `text-muted-foreground/50` | Fine print, captions |
| `text-pop` | Accent text, links, highlights |
| `var(--color-pop)` | Accent color (emerald green) |
| `var(--color-pop-light)` | Accent background (8% opacity) |
| `var(--color-background)` | Page background |
| `var(--color-border)` | Borders, dividers |

## Icon Styling (Agent/Module pattern)

```
<div className="w-10 h-10 rounded-xl flex items-center justify-center"
  style={{ background: colorBg }}>
  <Icon className="w-[18px] h-[18px]" style={{ color }} />
</div>
```

## Fonts

| Variable | Font | Usage |
|----------|------|-------|
| `--font-sans` | Plus Jakarta Sans | All text |
| `--font-display` | Instrument Serif | Editorial accents (hero second line) |
| `--font-mono` | JetBrains Mono | Code/technical |
