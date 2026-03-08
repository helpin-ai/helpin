# Phase 4 Frontend: Collaboration & Polish

**Timeline**: After Phase 4 Backend is complete
**Goal**: Docs with rich editor, attachments, WIP limits, dependency graphs, @mentions, keyboard shortcuts, GitHub integration display, UX polish.
**Status**: Not Started
**Depends on**: Phase 3 Frontend complete, Phase 4 Backend complete

---

## Overview

This phase builds the remaining collaboration features and polishes the entire PM module: rich text Docs with entity linking, file attachments, WIP limit enforcement on the board, dependency graph visualization, keyboard shortcuts, and the GitHub integration display. It also covers UX polish including loading states, empty states, error handling, and responsive design.

---

## Task Breakdown

### F4.1 API Services & Types

#### Task F4.1.1: Update PM types for Phase 4 entities
- **Status**: [ ] Not Started
- **Description**: Add TypeScript types for docs, attachments, and VCS data
- **Details**:
  - Interfaces: `Doc`, `DocWithLinks`, `DocLink`, `Attachment`, `DependencyGraph`, `DependencyNode`, `DependencyEdge`, `VCSData`, `LinkedPR`, `LinkedBranch`
  - Request types: `CreateDocRequest`, `UpdateDocRequest`, `CreateDocLinkRequest`, `UploadAttachmentRequest`
  - Enums: `DocAccessLevel` (workspace/team/private)
- **Files**: Update `frontend/src/lib/pmTypes.ts`

#### Task F4.1.2: Doc API service
- **Status**: [ ] Not Started
- **Description**: API client for doc endpoints
- **Methods**:
  - `listDocs(filters?)` → `GET /api/pm/docs`
  - `getDoc(id)` → `GET /api/pm/docs/{id}`
  - `createDoc(data)` → `POST /api/pm/docs`
  - `updateDoc(id, data)` → `PUT /api/pm/docs/{id}`
  - `deleteDoc(id)` → `DELETE /api/pm/docs/{id}`
  - `createLink(docId, data)` → `POST /api/pm/docs/{id}/links`
  - `deleteLink(docId, linkId)` → `DELETE /api/pm/docs/{id}/links/{linkId}`
  - `togglePin(docId)` → `PUT /api/pm/docs/{id}/pin`
- **Files**: `frontend/src/lib/services/pmDocService.ts`

#### Task F4.1.3: Attachment API service
- **Status**: [ ] Not Started
- **Description**: API client for file upload/download
- **Methods**:
  - `uploadAttachment(entityType, entityId, file)` → `POST /api/pm/attachments` (multipart)
  - `getAttachment(id)` → `GET /api/pm/attachments/{id}` (download)
  - `getThumbnail(id)` → `GET /api/pm/attachments/{id}/thumbnail`
  - `deleteAttachment(id)` → `DELETE /api/pm/attachments/{id}`
- **Files**: `frontend/src/lib/services/pmAttachmentService.ts`

---

### F4.2 Routing (Phase 4 additions)

#### Task F4.2.1: Add Docs routes
- **Status**: [ ] Not Started
- **Routes**:
  ```
  pm/
  ├── docs/
  │   ├── index.tsx    → Docs list page
  │   └── $docId.tsx   → Doc editor page
  └── settings.tsx     → (already exists, add new tabs if needed)
  ```
- **Files**:
  - `frontend/src/routes/_authenticated/w.$slug.pm/docs/index.tsx`
  - `frontend/src/routes/_authenticated/w.$slug.pm/docs/$docId.tsx`

#### Task F4.2.2: Update sidebar navigation
- **Status**: [ ] Not Started
- **Description**: Add Docs link and pinned docs to sidebar
- **Details**:
  - Add **Docs** nav item — icon: `FileText` → `/w/{slug}/pm/docs`
  - Sub-items: pinned docs (up to 5), with "View all" link
  - Pinned docs fetched on sidebar mount
- **Files**: Update sidebar navigation component

---

### F4.3 Rich Text Editor

#### Task F4.3.1: TipTap Rich Text Editor component
- **Status**: [ ] Not Started
- **Description**: Reusable rich text editor built on TipTap
- **Details**:
  - **Extensions to include**:
    - `StarterKit` (bold, italic, strike, heading, bullet list, ordered list, blockquote, code, code block, horizontal rule)
    - `Placeholder` (configurable placeholder text)
    - `Link` (auto-detect URLs, click to edit)
    - `Image` (upload via attachment service, paste from clipboard)
    - `Table` (basic table with add/remove rows/columns)
    - `TaskList` + `TaskItem` (checkbox lists)
    - `Mention` (custom extension for @username/@team autocomplete)
    - `Typography` (smart quotes, em dashes)
    - `CharacterCount` (optional, for length limits)
  - **Toolbar**:
    - Row 1: Bold, Italic, Strike | H1, H2, H3 | Bullet List, Ordered List, Task List | Blockquote, Code, Code Block
    - Row 2: Table, Link, Image Upload | Undo, Redo
    - Toolbar hidden when `toolbar: false` prop
  - **@Mention autocomplete**:
    - Trigger on `@` character
    - Dropdown shows matching users and teams
    - Select → inserts `@username` styled as a chip/link
    - Data source: workspace members and teams
  - **Image handling**:
    - Upload button → file picker → upload via attachment service → insert image markdown
    - Paste from clipboard → auto-upload → insert
    - Drag and drop image files → auto-upload → insert
    - Show upload progress indicator
  - **Slash commands** (optional, nice-to-have):
    - Type `/` to show command palette
    - Commands: Heading 1-3, Bullet List, Numbered List, Task List, Code Block, Table, Image
  - **Props**:
    - `content: string` — initial content (HTML or Markdown)
    - `onChange: (content: string) => void` — content change handler
    - `placeholder?: string`
    - `toolbar?: boolean` (default true)
    - `editable?: boolean` (default true, false for read-only render)
    - `autofocus?: boolean`
    - `maxHeight?: string` — for contained editing areas
  - **Output**: HTML string (stored in DB, rendered in views)
- **Files**: `frontend/src/components/pm/RichTextEditor.tsx`

#### Task F4.3.2: Mention extension
- **Status**: [ ] Not Started
- **Description**: Custom TipTap extension for @mentions
- **Details**:
  - Extends TipTap's `Mention` extension
  - Custom suggestion plugin:
    - On `@` keystroke, show floating dropdown
    - Filter workspace members and teams by typed text
    - Show: avatar + name for users, team icon + name for teams
    - Up/Down arrow to navigate, Enter to select
    - Escape to dismiss
  - Renders as styled chip/badge in editor
  - Stored as `<span data-mention-id="uuid" data-mention-type="user">@username</span>` in HTML
- **Files**: `frontend/src/components/pm/editor/MentionExtension.tsx`

---

### F4.4 Docs Pages

#### Task F4.4.1: Docs list page
- **Status**: [ ] Not Started
- **Description**: Page listing all documents
- **Details**:
  - Route: `/w/{slug}/pm/docs`
  - **Page header**: Title "Docs", "Create Doc" button
  - **Pinned section**: Pinned docs at top (if any), with pin icon
  - **Doc list** (card grid or table):
    - Each doc: Title, Author avatar + name, Last updated (relative time), Access icon (globe/team/lock)
    - Hover: shows first ~100 chars of content as preview
  - **Filters**: Author (user picker), Team (for team-scoped), Access Level
  - **Sort**: Updated (default), Created, Title (A-Z)
  - **Search**: Filter docs by title
  - **Empty state**: "No docs yet. Create your first document to start collaborating."
  - Click doc → navigate to Doc Editor page
- **Files**: `frontend/src/pages/pm/Docs.tsx`

#### Task F4.4.2: Doc Editor page
- **Status**: [ ] Not Started
- **Description**: Full page document editor
- **Details**:
  - Route: `/w/{slug}/pm/docs/{docId}`
  - **Layout**: Two areas — main editor (left ~70%) + sidebar (right ~30%)
  - **Main area**:
    - Title: Large editable text (auto-save on blur/debounce)
    - Content: RichTextEditor component with full toolbar
    - Auto-save: Debounce 1.5 seconds after last keystroke. Show "Saving..." / "Saved" indicator in header.
  - **Sidebar**:
    - **Access Level**: Dropdown (Workspace / Team / Private)
    - **Team**: Dropdown (visible when access = team)
    - **Pin/Unpin**: Toggle button
    - **Author**: Display only (avatar + name)
    - **Created**: Date
    - **Updated**: Date + "X minutes ago"
    - **Linked Entities** section:
      - List of linked stories, epics, iterations, objectives
      - Each shows: type icon, name, status badge
      - "Link to..." button → entity type selector → search and select
      - Unlink button (X) per linked entity
    - **Danger Zone**: Delete doc button (confirm dialog)
  - **Comments section**: Below editor content, same CommentSection component
  - **Breadcrumb**: Docs > Doc Title
  - **New doc flow**: `/w/{slug}/pm/docs/new` → create via API → redirect to `/w/{slug}/pm/docs/{id}`
- **Files**: `frontend/src/pages/pm/DocEditor.tsx`

#### Task F4.4.3: Doc Links panel for entity detail pages
- **Status**: [ ] Not Started
- **Description**: Show linked docs in story, epic, iteration, objective detail pages
- **Details**:
  - Reusable component: `DocLinksPanel`
  - Section title: "Docs" with count
  - List linked docs: title (link), author, updated date
  - "Link Doc" button → search docs → link
  - "Create Doc" button → creates new doc pre-linked to this entity
  - Empty state: "No docs linked"
  - Add to: StoryDetailPanel, EpicDetail, IterationDetail, ObjectiveDetail
- **Files**: `frontend/src/components/pm/DocLinksPanel.tsx`

---

### F4.5 File Attachments

#### Task F4.5.1: Attachment section in Story Detail Panel
- **Status**: [ ] Not Started
- **Description**: File upload and display for stories
- **Details**:
  - Section title: "Attachments" with count
  - **Drop zone**: Dashed border area, "Drop files here or click to upload"
  - **Upload button**: Click to open file picker
  - **Upload progress**: Progress bar per file being uploaded
  - **File list**: Each attachment shows:
    - File type icon (image, pdf, doc, code, generic)
    - Filename (truncated)
    - File size (formatted: "1.2 MB")
    - Uploaded by (avatar)
    - Uploaded date (relative)
    - Actions: Download (click filename), Delete (X button with confirm)
  - **Image preview**: Image files show thumbnail inline (~100x100px). Click to view full size in lightbox/modal.
  - **File size validation**: Show error for files > 50MB
  - **Multiple file upload**: Support selecting multiple files at once
  - **Drag and drop**: Accept files dragged onto the section
  - Also add to: EpicDetail (comments section)
- **Files**: `frontend/src/components/pm/AttachmentSection.tsx`

#### Task F4.5.2: Clipboard image paste in Rich Text Editor
- **Status**: [ ] Not Started
- **Description**: Paste images from clipboard into the editor
- **Details**:
  - Listen for `paste` event in TipTap editor
  - If clipboard contains image data (image/png, image/jpeg):
    - Create file from clipboard data
    - Upload via attachment service
    - On success: insert `<img src="attachment_url">` into editor
    - Show inline loading placeholder while uploading
  - Works in: Doc editor, Story description (after upgrade), Comments
- **Files**: Integration in `frontend/src/components/pm/RichTextEditor.tsx`

---

### F4.6 WIP Limits on Board

#### Task F4.6.1: WIP limit visual indicators
- **Status**: [ ] Not Started
- **Description**: Show WIP limit status on Kanban board columns
- **Details**:
  - **Column header update**: Show "State Name (3/5)" when WIP limit is set
  - **States**:
    - Under limit: Gray text "3/5"
    - At limit: Amber/yellow text and subtle amber column border
    - Over limit: Red text, red column border, warning icon
  - **Drag behavior**: When dragging a story over a column that would exceed its WIP limit:
    - Column border turns amber/red
    - Tooltip: "This column has a WIP limit of 5. Adding this story will exceed the limit."
    - Still allow the drop (soft limit) — but show confirmation toast
  - **API response handling**: If move API returns `wip_warning: true`, show toast: "WIP limit exceeded for 'In Review' (6/5)"
- **Files**: Update `frontend/src/components/pm/KanbanBoard.tsx`

---

### F4.7 Dependency Graph Visualization

#### Task F4.7.1: Dependency Graph component
- **Status**: [ ] Not Started
- **Description**: Visual directed graph of story dependencies
- **Details**:
  - **Location**: "Dependencies" tab in Epic Detail and Iteration Detail pages
  - **Library**: `reactflow` (for node-based graph rendering)
  - **Nodes**: Story cards (compact):
    - Display ID (TP-123)
    - Title (truncated)
    - State badge (colored pill)
    - Type icon
  - **Edges**: Arrows between stories
    - `blocks` → solid arrow, red
    - `relates_to` → dashed arrow, gray
    - `duplicates` → dotted arrow, gray
  - **Layout**: Auto-layout using `dagre` (left-to-right directed graph)
  - **Node colors**:
    - Backlog/Unstarted: gray background
    - Started: blue background
    - Done: green background
    - Blocked: red border
  - **Interactions**:
    - Click node → open Story Detail Panel
    - Zoom in/out (scroll wheel)
    - Pan (click and drag background)
    - Fit to view button
  - **Controls**: Zoom buttons (top-right corner), Fit button, Fullscreen button
  - **Empty state**: "No dependencies found. Add blocking relationships between stories to see the dependency graph."
  - **Large graphs**: If >50 nodes, show warning and option to filter
- **Files**: `frontend/src/components/pm/DependencyGraph.tsx`

---

### F4.8 Upgrade Story Description

#### Task F4.8.1: Replace story description textarea with Rich Text Editor
- **Status**: [ ] Not Started
- **Description**: Upgrade from plain textarea to TipTap editor in Story Detail Panel
- **Details**:
  - Replace `StoryDescription` component internals
  - Use `RichTextEditor` with reduced toolbar (no table)
  - Props: `toolbar: true`, `placeholder: "Add a description..."`
  - Content stored as HTML in `story.description`
  - Backward compatible: existing plain text descriptions render as paragraphs
  - Auto-save on change (debounced)
  - @mention support in descriptions
  - Image paste support
- **Files**: Update `frontend/src/components/pm/StoryDescription.tsx`

---

### F4.9 GitHub Integration Display

#### Task F4.9.1: VCS section in Story Detail Panel
- **Status**: [ ] Not Started
- **Description**: Show linked branches, PRs, and commits on stories
- **Details**:
  - Section title: "Development" (with Git icon)
  - **Branches**: List linked branch names with copy button
  - **Pull Requests**: Each PR shows:
    - Status icon: Open (green circle), Merged (purple merged icon), Closed (red circle)
    - PR title (link to GitHub)
    - PR number (#123)
    - Repository name
    - Created date
  - **Commits**: Count badge "3 commits" (expandable to show list)
  - **Empty state**: "No development activity linked. Use `[tp-123]` in commits or `tp-123-` in branch names."
  - **Board indicator**: Small Git icon on story card when VCS data exists
- **Files**: `frontend/src/components/pm/VCSSection.tsx`

---

### F4.10 Keyboard Shortcuts

#### Task F4.10.1: Keyboard shortcut handler
- **Status**: [ ] Not Started
- **Description**: Global keyboard shortcuts for PM module
- **Shortcuts**:
  | Key | Action | Context |
  |-----|--------|---------|
  | `c` | Open Create Story modal | Not in text input |
  | `n` | Toggle notification dropdown | Not in text input |
  | `/` or `Cmd+K` | Focus search bar | Not in text input |
  | `b` | Switch to Board view | Stories page |
  | `l` | Switch to List view | Stories page |
  | `Esc` | Close Story Detail Panel / modal | Panel or modal open |
  | `j` | Select next story | List view |
  | `k` | Select previous story | List view |
  | `Enter` | Open selected story | Story selected |
  | `?` | Show keyboard shortcuts help | Not in text input |
- **Implementation**:
  - Custom `usePMKeyboardShortcuts` hook
  - Check `document.activeElement` — skip when in input/textarea/contenteditable
  - Register on PM page mount, cleanup on unmount
  - Use `useEffect` with keyboard event listener
- **Files**: `frontend/src/hooks/usePMKeyboardShortcuts.ts`

#### Task F4.10.2: Keyboard shortcuts help modal
- **Status**: [ ] Not Started
- **Description**: Modal showing all available shortcuts
- **Details**:
  - Triggered by `?` key or help menu
  - Two-column layout: Key | Action
  - Grouped: Navigation, Views, Actions, Story Detail
  - Close with Esc or click outside
  - Use shadcn Dialog
- **Files**: `frontend/src/components/pm/KeyboardShortcutsHelp.tsx`

---

### F4.11 UX Polish

#### Task F4.11.1: Loading skeletons
- **Status**: [ ] Not Started
- **Description**: Skeleton loading states for all PM pages
- **Details**:
  - **Board view**: Skeleton columns with placeholder cards (pulsing gray rectangles)
  - **Table view**: Skeleton rows with column-width rectangles
  - **Story Detail Panel**: Section-by-section skeletons (title bar, description area, sidebar fields)
  - **Epic/Iteration list**: Skeleton cards with progress bar placeholders
  - **Timeline**: Skeleton bars on grid
  - **Charts**: Skeleton chart area with axes outlines
  - Use shadcn `Skeleton` component
- **Files**: `frontend/src/components/pm/skeletons/` (directory with skeleton components per view)

#### Task F4.11.2: Empty states
- **Status**: [ ] Not Started
- **Description**: Friendly empty states for all views
- **Details**:
  - Each empty state has: illustration/icon, message, description, CTA button
  - **Stories board**: "No stories yet" + "Create your first story" button
  - **Stories with filters**: "No stories match your filters" + "Clear filters" button
  - **Epics**: "No epics yet" + "Create an epic to organize related stories"
  - **Iterations**: "No iterations yet" + "Create an iteration for sprint planning"
  - **Objectives**: "No objectives yet" + "Create an objective to align work with goals"
  - **Docs**: "No docs yet" + "Create your first document"
  - **Reports**: "Not enough data" + "Complete some stories in an iteration to see charts"
  - **Comments**: "No comments yet. Start the conversation."
  - **Attachments**: "No files attached"
  - **Search results**: "No results for '{query}'"
- **Files**: `frontend/src/components/pm/EmptyState.tsx` (reusable with props)

#### Task F4.11.3: Error handling and toast notifications
- **Status**: [ ] Not Started
- **Description**: Consistent error handling across PM module
- **Details**:
  - **API errors**: Catch in service layer, show toast with message
  - **Toast library**: Use sonner (already in shadcn) or shadcn toast
  - **Toast types**:
    - Success (green): "Story created", "Moved to In Review", "5 stories updated"
    - Error (red): "Failed to create story. Please try again.", "Could not move story. Reverting."
    - Warning (amber): "WIP limit exceeded for 'In Review'"
    - Info (blue): "Story TP-123 is blocked by TP-100"
  - **Optimistic update rollback**: On API error, revert UI change + show error toast
  - **Network errors**: "Connection lost. Retrying..." with retry button
  - **Form validation**: Inline error messages below fields (red text)
  - **Global error boundary**: Catch React errors, show "Something went wrong" with reload button
- **Files**: `frontend/src/lib/pmErrorHandler.ts`, update service files

#### Task F4.11.4: Responsive design
- **Status**: [ ] Not Started
- **Description**: Ensure PM views work on standard screen sizes
- **Details**:
  - **Breakpoints**: 1024px (min for full experience), 768px (tablet), 640px (mobile — limited)
  - **Board view**: Horizontal scroll at all sizes. Columns min-width 280px.
  - **Table view**: Hide secondary columns (labels, due date, updated) below 1024px. Priority columns: ID, Title, State, Owner.
  - **Story Detail Panel**: Full-width overlay below 1024px (instead of side panel)
  - **Timeline**: Horizontal scroll, touch-friendly drag
  - **Sidebar navigation**: Collapsible to icon-only mode below 1024px
  - **Modals**: Max-width with padding, scrollable on small screens
  - **Charts**: Responsive resize with container. Min-height 300px.
  - Test on: 1920px, 1440px, 1280px, 1024px screens
- **Files**: Across all components (responsive Tailwind classes)

---

### F4.12 Install Dependencies

#### Task F4.12.1: Install npm packages for Phase 4
- **Status**: [ ] Not Started
- **Packages**:
  - `@tiptap/react` — TipTap React integration
  - `@tiptap/starter-kit` — Base extensions bundle
  - `@tiptap/extension-link` — Link handling
  - `@tiptap/extension-image` — Image embedding
  - `@tiptap/extension-table` + `@tiptap/extension-table-row` + `@tiptap/extension-table-header` + `@tiptap/extension-table-cell` — Table support
  - `@tiptap/extension-placeholder` — Placeholder text
  - `@tiptap/extension-mention` — @mention support
  - `@tiptap/extension-task-list` + `@tiptap/extension-task-item` — Checkbox lists
  - `@tiptap/extension-typography` — Smart typography
  - `@tiptap/extension-character-count` — Character counting
  - `reactflow` — Node-based graph for dependency visualization
  - `dagre` — Graph layout algorithm
  - `@types/dagre` — TypeScript types for dagre

---

## Definition of Done

Phase 4 Frontend is complete when:
- [ ] Rich Text Editor (TipTap) works with formatting, @mentions, images, tables
- [ ] Docs list page displays documents with filters and search
- [ ] Doc editor page supports full rich text editing with auto-save
- [ ] Docs can be linked to stories, epics, iterations, objectives
- [ ] Pinned docs appear in sidebar navigation
- [ ] File attachments can be uploaded, previewed (images), downloaded, and deleted
- [ ] Image paste from clipboard works in editor
- [ ] WIP limits show visual indicators on board (under/at/over)
- [ ] Dependency graph renders nodes and edges with auto-layout
- [ ] Graph supports zoom, pan, and click-to-navigate
- [ ] Story description uses rich text editor
- [ ] GitHub integration shows branches, PRs, commits on stories
- [ ] Keyboard shortcuts work for all mapped actions
- [ ] Shortcuts help modal shows all available shortcuts
- [ ] Loading skeletons show on all pages during data fetch
- [ ] Empty states display for all empty views with CTAs
- [ ] Error toasts show for API failures with retry options
- [ ] Optimistic updates revert correctly on errors
- [ ] Layout works on screens 1024px and above
- [ ] No console errors or TypeScript warnings

---

## Component Inventory (Full PM Module)

After Phase 4, the complete component tree:

```
PM Module Components
├── Navigation
│   └── PM Sidebar Section (with pinned docs)
├── Board View
│   ├── KanbanBoard (with WIP indicators)
│   ├── BoardColumn
│   ├── StoryCard
│   └── QuickCreateStory
├── Table View
│   ├── StoryTable
│   └── BulkActionBar
├── Story Detail
│   ├── StoryDetailPanel
│   ├── StoryDetailSidebar (all field selectors)
│   ├── StoryDescription (Rich Text)
│   ├── SubTaskSection
│   ├── ChecklistSection
│   ├── RelationshipsSection
│   ├── CustomFieldsSection
│   ├── AttachmentSection
│   ├── VCSSection
│   ├── DocLinksPanel
│   ├── CommentSection
│   └── ActivityLog
├── Modals
│   ├── CreateStoryModal (with templates)
│   ├── EpicModal
│   ├── IterationModal
│   ├── ObjectiveModal
│   └── SaveViewDialog
├── Shared Pickers
│   ├── UserPicker
│   ├── LabelPicker
│   ├── EntityPickers (Epic, Iteration, Team)
│   ├── PrioritySelector
│   └── EstimateSelector
├── Filter System
│   ├── FilterBar
│   ├── SpacesTabs
│   └── Filter value selectors
├── Pages
│   ├── Stories (Board + Table)
│   ├── Epics + EpicDetail
│   ├── Iterations + IterationDetail
│   ├── Objectives + ObjectiveDetail
│   ├── Roadmap (Timeline)
│   ├── Reports
│   ├── Docs + DocEditor
│   └── PMSettings
├── Charts
│   ├── VelocityChart
│   ├── BurndownChart
│   ├── CumulativeFlowDiagram
│   ├── CycleTimeChart
│   └── LeadTimeChart
├── Rich Text
│   ├── RichTextEditor (TipTap)
│   └── MentionExtension
├── Visualization
│   ├── DependencyGraph
│   └── TimelineBar + TimelineGrid
├── UX
│   ├── EmptyState
│   ├── Skeletons
│   ├── KeyboardShortcutsHelp
│   └── NotificationDropdown
└── Search
    └── SearchBar
```
