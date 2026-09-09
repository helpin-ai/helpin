---
name: helpin-design-system
description: Design, implement, refactor, or review product UI under Helpin's frontend/ using the Quiet Hairline design system and Helpin's centralized React components. Do not use for the website, help-center app, widgets, SDKs, or mobile/desktop shells.
---

# Helpin Design System

Build Helpin product surfaces in the Quiet Hairline language: a warm, document-like interface whose structure comes from hairlines, whitespace, type weight, and restrained semantic color.

## Before editing UI

1. Read [references/quiet-hairline.md](references/quiet-hairline.md).
2. When implementing or reviewing code, also read [references/helpin-components.md](references/helpin-components.md).
3. Inspect the nearest existing page and the centralized component being reused before changing its API or appearance.

## Working rules

- Quiet Hairline is the visual authority. Generic frontend-design advice yields when it conflicts.
- Existing Helpin components remain the authority for behavior, accessibility, routing, permissions, data flow, and domain logic.
- Prefer the centralized Quiet components and established domain components over copying Tailwind class strings into pages.
- For option-selection dropdowns, use `QuietDropdown` or an existing adapter built on it. For list filters, show the shared editable applied-filter pills below the toolbar, as Tasks and Epics do. Read [Dropdowns and applied filters](references/helpin-components.md#dropdowns-and-applied-filters) for component selection and behavior.
- For human-authored conversation messages, comments, and email outside Support, compose with the centralized `QuietConversationComposer` primitives. `ReplyComposer` remains the heavily used Support reference implementation; preserve its domain behavior and mirror intentional visual-contract changes between it and the primitives.
- Keep search visually distinct from data entry: use the centralized `QuietSearchInput` throughout product surfaces; it owns the Skill Catalog's compact bordered treatment and leading icon. Never use `QuietUnderlineInput` for search. Preserve behavior-owned `CommandInput`, editor search/replace, and content-preview search controls.
- Use semantic Quiet tokens. Do not hard-code a near-match when a token exists.
- Preserve visible keyboard focus with the system's warm 2px focus treatment. "No focus ring" means no default blue ring, not no focus indicator.
- Preserve dark mode and current responsive application behavior.
- Do not introduce a second icon library. Use `@/lib/icons` or `@/lib/pmIcons`.
- Treat derived content as incomplete without provenance.

## Completion check

Verify populated, empty, loading, error, disabled, overflow, narrow-width, keyboard, light-mode, and dark-mode states. Remove decorative containers, chips, metrics, and icons that do not help the user understand or act.
