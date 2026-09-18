# Paragraph Spacing in Documentation/Help Center Tools

> **Status: historical design research.** Vendor measurements below were not
> reverified in this audit and are not a specification of Helpin's CSS.
> As of the 2026-09-17 source comparison, the
> [editor stylesheet](../../frontend/src/index.css) uses paragraph bottom margin
> `1.05rem` and line height `1.7`; the
> [help-center stylesheet](../../help-center/src/app.css) uses `1.1rem` and `1.7`.
> Both have more-specific contextual rules. Inspect rendered pages when assessing
> visual parity; these declarations alone do not prove identical layout.

Research comparing how major documentation platforms handle paragraph spacing in their public-facing knowledge base views.

## Comparison Table

| Tool | `<p>` margin-bottom (published view) | Line height | Editor vs Published match? | Empty paragraph handling | Source |
|------|--------------------------------------|-------------|---------------------------|--------------------------|--------|
| **HelpScout Docs** | `1.5em` (~21px at 14px base) | `1.75em` | No -- editor is simpler WYSIWYG, published uses `#fullArticle p` styles | `<div>` tags also get `margin-bottom: 1.5em` | [GitHub: doc-article-styles](https://github.com/helpscout/doc-article-styles/blob/master/dist/scss/styles.web.scss) |
| **Intercom Articles** | ~`16px`-`24px` (CSS modules, varies) | Not explicitly set on article body (inherits) | Approximately -- editor is WYSIWYG with similar spacing | Uses `<p>` tags; empty paragraphs rendered as `<br>` inside `<p>` | [Intercom CSS analysis](https://static.intercomassets.com/_next/static/css/7aa380e246027379.css) |
| **Zendesk Guide** | Browser default `1em` (15px at 15px base) | `1.6` (on `.article-content`) | Close -- Copenhagen theme applies minimal overrides; relies on browser defaults | Browser default: empty `<p>` maintains full height | [GitHub: copenhagen_theme](https://github.com/zendesk/copenhagen_theme/blob/master/style.css) |
| **Notion** | Dynamic padding system (~6-8px between text blocks; more between paragraphs after March 2026 update) | `1.5` (default); varies by font choice | Yes -- editor IS the published view (same rendering engine) | Empty blocks maintain min-height (~1 line); can be deleted but not collapsed | [Notion blog: Updating page design](https://www.notion.com/blog/updating-the-design-of-notion-pages) |
| **GitBook** | `1.14286em` (~16px) -- Tailwind Typography values | `1.71429` (Tailwind prose) | Yes -- editor is close to published (block-based WYSIWYG) | Empty paragraphs maintain line height | [GitBook CSS analysis](https://static-2v.gitbook.com/_next/static/css/) |
| **Confluence** | Browser default `1em` (~14px); users commonly override to `0` or `5px` | `1.5` (default) | Approximately -- editor (ProseMirror) adds visual spacing similar to published view | Enter creates new `<p>` with spacing; Shift+Enter creates `<br>` within same `<p>` (no extra spacing) | [Atlassian KB: heading spacing](https://support.atlassian.com/confluence/kb/how-to-remove-spacing-under-a-heading/) |

## Detailed Findings

### 1. HelpScout Docs

**Source**: Official open-source stylesheet at [helpscout/doc-article-styles](https://github.com/helpscout/doc-article-styles)

Key CSS rules from `#fullArticle` (the published article container):
```css
#fullArticle {
  color: #4f5d6b;
  font-family: 'Roboto', 'Helvetica', sans-serif;
  font-size: 14px;
  line-height: 1.75em;
}

#fullArticle p {
  margin-bottom: 1.5em;   /* ~21px */
  margin-top: 1em;         /* ~14px */
}

#fullArticle > div {
  margin-bottom: 1.5em;
  margin-top: 1em;
}
```

- Paragraphs AND top-level divs both get `1.5em` bottom margin
- The `1.5em` at `14px` base = **21px** effective paragraph spacing
- Asymmetric: more space below (1.5em) than above (1em)
- This is the Beacon/help widget rendering; the public Docs site allows full CSS customization

### 2. Intercom Articles

**Source**: CSS analysis of `static.intercomassets.com` stylesheets + HTML inspection

- Intercom uses **CSS modules** with hashed class names, making static analysis difficult
- The published help center uses Tailwind-like utility classes (e.g., `mb-6` = `margin-bottom: 1.5rem` = **24px**)
- Article body content paragraph spacing appears to be **16-24px** based on frequency analysis of their CSS (16px and 24px are the most common margin-bottom values)
- Intercom does NOT allow custom CSS on the Help Center -- styling is controlled by their theme system
- When using the Articles API, `<div>` and `<span>` tags are replaced with `<p>` tags

### 3. Zendesk Guide (Copenhagen Theme)

**Source**: Official open-source [copenhagen_theme](https://github.com/zendesk/copenhagen_theme/blob/master/style.css)

Key CSS:
```css
body {
  font-size: 15px;
  line-height: 1.5;
}

.article-content {
  line-height: 1.6;
  margin: 40px 0;
  word-wrap: break-word;
}

/* No explicit .article-body p rule! */
/* Only override: */
.article-body > p:last-child {
  margin-bottom: 0;
}
```

- **No custom paragraph margin** -- relies entirely on browser default (`1em` = **15px** at their 15px base font)
- The only `p`-specific rule removes margin from the last paragraph
- Article content wrapper gets `line-height: 1.6`
- Lists get explicit `margin: 20px 0 20px 20px`
- The Copenhagen theme is highly customizable (SCSS variables), so actual sites vary widely

### 4. Notion Published Pages

**Source**: [Notion blog: "Updating the design of Notion pages"](https://www.notion.com/blog/updating-the-design-of-notion-pages) (March 2026)

- Notion uses a **dynamic block-based spacing system**, not traditional CSS `<p>` margins
- Each block gets padding (not margin) determined by its **adjacent neighbors**
- **Before March 2026 update**: Paragraph spacing was minimal (~3px padding per block), making paragraphs feel compressed. Users widely complained about tight spacing.
- **After March 2026 update**: Paragraph blocks get more breathing room, while list items remain compact
  - Text blocks adjacent to other text blocks: **increased padding** (visually ~12-16px gap)
  - List items adjacent to list items: **reduced padding** (compact chunking)
- Line height: `1.5` for default font, varies slightly for Serif and Mono options
- Editor and published view are **identical** -- Notion renders the same way in both contexts
- Empty blocks maintain minimum height equal to one line of text

### 5. GitBook

**Source**: CSS analysis of `static-2v.gitbook.com` stylesheets

- GitBook uses **Tailwind CSS Typography plugin** (`@tailwindcss/typography`) values
- The most frequent `margin-bottom` values in their CSS:
  - `1.14286em` (45 occurrences) -- this is `16px / 14px`, the Tailwind prose default for `<p>` tags
  - `1.71429em` (30 occurrences) -- used for headings
  - `.571429em` (27 occurrences) -- tighter spacing for nested elements
- Effective paragraph spacing: **~16px** at their base font size
- GitBook's editor is block-based WYSIWYG and closely matches the published output
- Published documentation sites can be customized with space-level CSS

### 6. Confluence

**Source**: [Atlassian KB](https://support.atlassian.com/confluence/kb/how-to-remove-spacing-under-a-heading/), [CONFCLOUD-34468](https://jira.atlassian.com/browse/CONFCLOUD-34468)

- Confluence uses **browser default** `<p>` margins (`1em` = ~14-16px depending on base font)
- The editor (ProseMirror-based) and published view use similar but not identical CSS
- Enter key creates a new `<p>` tag with full paragraph spacing
- Shift+Enter creates a `<br>` within the same `<p>` (single-line spacing)
- Users can override with custom CSS: `#main-content p { margin: 0px 0px 5px 0px; }`
- Confluence Cloud does NOT support custom CSS at the space level (Data Center does)
- Long-standing feature request (CONFCLOUD-34468, 26 upvotes) to add paragraph spacing controls

## Summary of Patterns

1. **Most tools use 14-24px paragraph spacing** (roughly 1em to 1.5em)
2. **HelpScout is the most generous** at 1.5em (21px) -- designed for readability in help content
3. **Zendesk and Confluence rely on browser defaults** (~1em = 15-16px)
4. **GitBook uses Tailwind Typography defaults** (~16px, well-tested for documentation)
5. **Notion is unique** with its dynamic neighbor-aware padding system
6. **Intercom falls in the 16-24px range** with utility class-based spacing
7. **Editor-published parity varies**: Notion has perfect parity; others have approximate parity
8. **Empty paragraphs**: All tools maintain some height for empty paragraphs (typically one line height)

## Historical recommendation for help-center implementation

Based on this research, a **16-20px margin-bottom** on `<p>` tags in published help center articles is the industry standard sweet spot:
- `1em` (browser default) for minimal styling overhead
- `1.25em` for slightly more breathing room (GitBook-like)
- `1.5em` for maximum readability

A line-height of `1.5` to `1.75` on article body content is universal across all tools.
