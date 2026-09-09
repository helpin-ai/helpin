# Helpin Quiet Hairline

## The one rule

Structure comes from hairlines, whitespace, and type weight—never from boxes. Do not use bordered inputs, rounded field outlines, cards, pills, tinted status chips, filled section wells, or decorative shadows. If a border seems necessary to group content, use a 1px divider and an eyebrow label.

Functional exceptions include the centralized search field and editable applied-filter pills described below. The Objectives index intentionally retains its established card grid. Preserve their shared components rather than restyling them as plain text.

The only sanctioned elevation is an existing product overlay such as a Sheet, Dialog, Popover, command surface, or menu. Preserve its established behavior and use the quietest compatible chrome.

## Voice

The interface reads like a well-set working document, not a form. Use one or two dark buttons per screen at most; everything else is a text, icon, or underline action with a clear hover and focus state. Every number, count, status, badge, and icon must help the user scan, decide, or act.

## Light palette

| Role | Value |
| --- | --- |
| Text primary | `#1c1a17` |
| Text secondary | `#57534e` |
| Text tertiary | `#78716c` |
| Muted / placeholder / meta | `#a8a5a0` |
| Surface | `#ffffff` |
| Page backdrop | `#e9e8e5` |
| Hover | `#f5f4f2` |
| Row hover | `#faf9f7` |
| Icon well / neutral avatar | `#f0efec` |
| Divider strong | `#ecebe8` |
| Divider light | `#f2f1ee` |
| Underline field | `#ddd9d4` |
| Meta separator | `#e0ded9` |
| Empty glyph | `#d6d3ce` |
| Link / warning / blocker | `#c2410c`, hover `#9a3412` |
| Positive / linked | `#0f766e` |
| Lifecycle | `#7c5cff` |
| Dark action | `#1c1a17`, hover `#3b3733` |

Use the `quiet-*` semantic Tailwind tokens rather than these literals. In dark mode, the tokens map to Helpin's existing accessible dark semantics.

Avatar fallback pairs, selected by the centralized avatar seed, are:

- `#e8ecf7 / #4c5a86`
- `#eef0f6 / #4c5a86`
- `#e6f0ec / #3f6b58`
- `#eeeaf7 / #5b4c86`
- `#f7ece6 / #8a5433`
- `#eef2e6 / #5c6b3f`
- unresolved: `#f0efec / #78716c`

## Type

Use Helpin's centralized `font-sans` stack. Do not introduce another font. Use only weights 400, 500, and 600.

- Static route page title: `20px / 600`, letter spacing `-.018em`. Identity titles remain `24px / 600`; keep editable entity-name fields at `26px / 600` outside the compact detail-header context.
- Section or thread title: `20–21px / 600`, `-.018em`.
- Row title or subject: `13.5px / 600`, `-.008em`; this is the strongest row element.
- Body prose: `14px / 1.7`, maximum measure `680–760px`, `text-wrap: pretty`.
- UI label: use Helpin's `text-sm` token (about `13.125px` under the current 93.75% root scale); secondary: `12.5px`; meta: `11.5–12px`. Do not use `text-[13px]` for the UI-label role.
- Dropdown content: explicit `12.2px` through the centralized dropdown typography token and content wrapper.
- Eyebrow: `12px / 600`, `.06em`, uppercase, muted.
- Tone label: `11.5px / 600`, `.03em`, uppercase, meaning-colored word—not a pill.

## Lines, spacing, and radius

- Use 1px hairlines everywhere.
- Use 2px only for visible focus rules, active tab underlines, and progress bars.
- Use 3px only for an absolute left row state marker.
- Spacing steps: 2, 3, 5, 6, 7, 9, 10, 11, 12, 13, 14, 16, 18, 20, 22, 24, 26, 28, 32, and 44px.
- Radius: 6px icon actions, 7–8px dark actions, 12px genuine chat bubbles, 14px elevated overlay surfaces, 50% avatars, 10px company marks.

## Application shells

Use Helpin's existing authenticated route shell and responsive overflow behavior. The fixed backdrop/page-card prototype shell is not a production layout rule.

- Route pages use the centralized Quiet viewport and page header. Collection pages such as Tasks, Epics, and Objectives pin the shell header and full-width search/filter rows outside the results scroller, with dividers between the rows.
- Detail sheets/pages use the centralized detail layout, tabs, and rail.
- CRM detail tabs inherit the detail page surface. Full-height Tasks, Emails, and Meetings views must remain transparent rather than painting an opaque `bg-background`; bounded embedded task views may retain their own surface.
- Specialized editors and dense workflow toolbars may keep their own header composition, but must use the centralized workspace-sidebar safe inset so the collapsed navigation opener never covers controls.
- Module-specific sidebar utilities, such as CRM and Support settings shortcuts, stay bottom-pinned directly above the shared Cmd+K search footer. They do not follow the last navigation item up the rail.
- Multi-column Support inbox headers share one fixed height and divider treatment so the list title, conversation title/actions, and Details header close on one continuous horizontal hairline. In dark mode, the Inbox list and conversation thread use the same sidebar surface token. The first Inbox header also uses the centralized sidebar-safe inset so the collapsed navigation opener stays inline without covering its title.
- Below `md`, Support remains Inbox-first: selecting a row pushes the conversation route into a full-screen right sheet while the Inbox stays mounted underneath. The sheet header keeps an accessible arrow-only Inbox return action, the truncating conversation number/subject, and icon-only task, resolve, and more actions on the same hairline. Tapping the subject opens the existing Details rail as a nested full-screen sheet. Direct links close to `/support`; list-originated sheets use browser Back and restore focus to the opened row.
- Center fixed-width route content with `margin-inline: auto`; never use flex centering that makes overflow unscrollable.
- Keep the established application widths: detail rail 300–352px, task rail 300–308px, thread list about 456px, prose measure 680–760px, detail measure 600–700px.

## Core patterns

### Page and identity headers

Page headers use a 20px title, an optional compact breadcrumb/navigation row, a compact secondary description when useful, and right-aligned actions. Identity headers use a 40–44px avatar or company mark, a 24px name, a metadata line separated by 1×11px bars, and a lifecycle/status dot plus word. Editable entity-name fields remain 26px. Actions are ghost text actions plus at most one dark primary.

Detail pages that combine breadcrumbs, editable identity, state, and specialized actions use the centralized compact detail header. The first row contains ancestor breadcrumbs only. The current entity or document title appears once below at an explicit `20px` at every breakpoint. CRM identity names stay on one line and truncate rather than increasing header height; document titles may wrap when showing the full title is necessary. Metadata stays on one line under the title, with the semantic status as its final item. Actions remain on the right; transient save feedback such as “All changes saved” stays directly underneath them. On phones, header actions become round icon-only controls; their labels return from the small breakpoint upward. The shared header owns sidebar-toggle clearance, responsive geometry, accessible action labels, and its closing strong hairline; domain pages own saving, validation, permissions, and routing. In Docs, presence appears to the left of the action buttons from `sm` upward; on phones, avatar-only presence sits beneath the button row. The authenticated Docs detail page lets this header scroll with the page below `lg`, while its desktop editor retains contained scrolling.

PM detail headers follow the Epic reference: a visible back arrow beside the breadcrumb and a title control with an underline that appears on hover/focus, capped at 42rem rather than spanning the header. See [PM detail composition](helpin-components.md#pm-detail-composition) for the required wrapper and shared controls.

### Tabs and filters

Primary tab labels use an explicit `14px` in every state. This is a deliberate exception to Helpin's globally scaled type tokens: `text-sm` renders smaller than 14px, while `text-base` renders larger. Inactive and hover labels use weight 500; active labels use weight 600, primary text color, and a 2px underline. Hover changes only the text color, not the weight. Counts are inline muted text, never badges. Secondary quick-filter tabs are plain text; active is 12.5px/600 and inactive is 12.5px/400. Do not turn either level of tabs into pills.

Applied list filters follow the shared Tasks/Epics pattern: a row below the toolbar with a field label, operator, editable value, individual remove action, and Clear all. These functional pills are an explicit exception to the decorative-chip rule. Keep Owner first when present. Option selectors use the shared dropdown with 12.2px content text, centralized search spacing, and search shown only when needed for overflow or an explicit typing workflow. Group by keeps its label inside the trigger and remains separate from applied filters. See [Dropdowns and applied filters](helpin-components.md#dropdowns-and-applied-filters) for the component contract.

### Actions

- Icon action: 15px centralized icon, 6px radius, 6px padding, tertiary ink, warm hover/focus background.
- Text action: icon plus 12.5–13px label, no border or filled background.
- Dark action: use Helpin's centralized `Button` at `size="sm"` and inherit its standard compact geometry (`h-8`, `px-3`, `text-sm`, `rounded-4xl`). Quiet styling controls the warm dark color and the one-to-two-primary-actions limit; it does not override button size, spacing, or curvature.
- Detail-header action: use the centralized responsive action. Below `sm`, it is a 32px round icon control with an accessible label; from `sm` upward it returns to the appropriate text or dark-action presentation.
- Underline control: borderless with a 1px field underline that darkens on hover and becomes 2px on focus.
- Search control: use centralized `QuietSearchInput`, which retains the Skill Catalog's compact bordered field and leading icon. Search is not an underline control.

### Property rows

For PM detail rails, preserve Epic's compact `DetailMetadataRow` presentation: 12px muted property labels and a 16px icon / 72px label / remaining-value grid. Preserve semantic option colors inside the dropdown as well as in its selected trigger. The generic property-row pattern below applies to other surfaces.

Use icon → fixed label column → truncating value. Rows are 7px vertically padded and receive only a warm row hover. Empty values show the action prompt in muted text; the field itself is the affordance. Group property rows with light bottom hairlines.

Relationship picker dialogs grow responsively from a viewport-bounded mobile width to 576px at `sm`, 672px at `lg`, and 768px at `xl`. Search results scroll vertically inside the dialog; rows and their text containers use `min-width: 0`, hide horizontal overflow, and truncate long names/details. Never let unbounded result text force the dialog or row past the overlay.

### List rows

Rows stack actor/meta, title, detail, and provenance. They use 12–13px vertical padding and a light bottom divider. Never allow snippets to run into a wide horizontal metadata strip. A 3px left marker means selected/current, blocker/overdue, or positive; do not add margins to the marker.

### Sections

Sections are full-width and stacked. A section header contains an optional 15px icon, eyebrow, inline count, spacer, and quiet action. End sections with a strong divider. Do not create side-by-side section cards.

### Inputs and editors

PM descriptions use the same display, edit affordance, divider editor, and Cancel/Done components as Epic detail. These are description controls, separate from conversation composers; see [PM detail composition](helpin-components.md#pm-detail-composition).

Form inputs are borderless and transparent. Use an adjacent underline or divider as the visible focus surface. Search fields are the deliberate exception: use `QuietSearchInput` so every product search inherits the Skill Catalog's recognizable compact bordered treatment. Keep TipTap, form, select, popover, and dialog behavior in existing Helpin components; select their plain, divider, line, or borderless presentation when available.

Human-authored conversation composers are the deliberate editor exception. Support replies define the reference contract; task and epic comments and CRM email bodies/replies consume its centralized primitives: a 12px-radius hairline shell, borderless rich-text surface, full formatting toolbar, shortcut hint, AI rewrite menu when permitted, attachment affordance when supported, and one compact dark send action. The emphasized state preserves Support's established blue border exactly: `border-blue-500 dark:border-blue-400`, with no added ring or surface lift; internal Support notes retain their semantic amber border. Product areas keep their own drafts, recipients, mentions, uploads, permissions, and send behavior. Docs comments remain on their document-anchored presentation unless a product decision explicitly migrates them.

### Timeline and activity

Use a single 1px vertical line and 7px dots for step timelines. Every step starts with its outcome or finding; machinery is demoted behind disclosure in 11.5px monospace. For table-like activity, use hairline headers and stacked rows rather than cards.

### Provenance

Anything derived—AI summaries, enrichment, signals, or synchronized content—must say what it is based on. Use a `Based on` source list, a 5px positive dot plus `View source`, or source and drill-in link in the row meta line. Do not wrap provenance in a green-tinted container.

### Empty states

Empty states are left-aligned working states: a 14px/500 title, one 13px/1.6 explanation, then a useful progress rule or hairline list of what the feature watches for. End with an underline action that fixes the real blocker and plain-text status. Do not use centered illustrations, apologies, or decorative icon wells.

### Chat

Use bubbles only for genuine turn-taking. Inbound is warm hover neutral and outbound is row-hover neutral; both use 12px radius and left-aligned text. Conversation composers use the centralized composer primitives, with an avatar when the surrounding thread identifies the sender, a borderless editor, hairline toolbar, sending identity where relevant, shortcut hint, and a single dark send action.

## Production build rules

- React components and typed props replace prototype custom elements and state toggles.
- Tailwind v4 semantic utilities replace inline styles.
- `@/lib/icons` and `@/lib/pmIcons` replace CDN Lucide imports.
- Repeated rows use normal React mapping with stable domain IDs.
- Variants use props, CVA, or existing domain presentation variants.
- Prototype populated/empty toggles are never shipped as product controls.

## Anti-patterns

Do not introduce bordered data-entry inputs, rounded field boxes, outlined section cards, decorative pills/chips for ordinary status or filter tabs, tinted enrichment cards, blue focus treatments, blue send buttons, centered empty illustrations, raw function names without an outcome sentence, emoji, gradients, ornamental metrics, or more than two dark actions per screen. Preserve the shared search field and editable applied-filter exceptions above.
