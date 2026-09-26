# Support custom view sidebar and list surface implementation plan

**Goal:** Make custom-view creation discoverable in the support sidebar and align the conversation-list background with the clearer details sidebar.

**Architecture:** Keep the Custom views section visible in the support rail and add a create action. Use transient inbox state to open the existing live filter panel from the sidebar; the current save dialog creates the view, then selects it. Use the details panel's `bg-muted/30` surface token for the conversation list.

**Tech stack:** React, Zustand, TanStack Router and Query, Tailwind CSS, Vitest.

- [x] Add a failing rail test for the persistent section, plus button, and empty-state create action.
- [x] Add a failing list test for opening the filter flow from sidebar state and saving a view.
- [x] Implement the rail action and transient filter-flow state, with route navigation from other support pages.
- [x] Reuse the existing filter panel and save dialog; select the created view and keep the new view available to route hydration.
- [x] Compare the panel background classes and align the list with the details sidebar.
- [x] Run focused tests, type checking, lint, and diff checks; commit only task files.
