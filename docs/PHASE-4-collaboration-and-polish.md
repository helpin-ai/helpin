# Phase 4: Collaboration & Polish

**Timeline**: Weeks 13-16
**Goal**: Full collaboration toolkit and UX polish.
**Status**: Not Started
**Depends on**: Phase 3 complete

---

## Overview

This phase adds Docs (rich-text documents linked to work items), file attachments, WIP limits enforcement, dependency graph visualization, @mention notifications, keyboard shortcuts, and GitHub integration. By the end of Phase 4, the PM module is a complete, polished project management system.

---

## Task Breakdown

### 4.1 Database Migrations

#### Task 4.1.1: Create PM Docs tables
- **Status**: [ ] Not Started
- **Description**: Create migration for `pm_docs` and `pm_doc_links` tables
- **Details**:
  - `pm_docs`: id, workspace_id, title, content (text/markdown), author_id, pinned, archived, access_level (workspace/team/private), team_id, timestamps
  - `pm_doc_links`: id, doc_id, entity_type (story/epic/iteration/objective), entity_id, created_at. UNIQUE(doc_id, entity_type, entity_id)
  - Apply `set_updated_at()` trigger
- **Files**: `server/migrations/025_pm_docs.sql`

#### Task 4.1.2: Create PM Attachments table
- **Status**: [ ] Not Started
- **Description**: Create migration for `pm_attachments` table
- **Details**:
  - `pm_attachments`: id, entity_type (story/epic/comment/doc), entity_id, uploaded_by, filename, file_size, content_type, storage_path, thumbnail_path, created_at
  - Index on (entity_type, entity_id)
- **Files**: `server/migrations/026_pm_attachments.sql`

---

### 4.2 Backend — Docs

#### Task 4.2.1: Doc models
- **Status**: [ ] Not Started
- **Description**: Go structs for Doc, DocLink
- **Details**:
  - GORM models with JSON tags
  - CreateDocRequest, UpdateDocRequest
  - CreateDocLinkRequest
  - DocWithLinks (includes linked entities with their current status)
- **Files**: `server/internal/model/pm_doc.go`

#### Task 4.2.2: Doc repository
- **Status**: [ ] Not Started
- **Description**: Data access layer for docs
- **Methods**:
  - `List(ctx, workspaceID, filters)` — filter by author, team, access_level, pinned, archived
  - `GetByID(ctx, id)` — with links
  - `Create(ctx, doc)`, `Update`, `Delete`
  - `CreateLink(ctx, docID, entityType, entityID)` — link doc to entity
  - `DeleteLink(ctx, linkID)` — unlink
  - `ListByEntity(ctx, entityType, entityID)` — docs linked to a specific entity
  - `Pin(ctx, id)`, `Unpin(ctx, id)` — toggle pin status
- **Files**: `server/internal/repository/pm_doc.go`

#### Task 4.2.3: Doc service
- **Status**: [ ] Not Started
- **Description**: Business logic for docs
- **Logic**:
  - Access control: private docs visible only to author, team docs to team members, workspace docs to all
  - Only author or admin can edit/delete
  - When linking to entity, verify entity exists
  - Log activity when doc is linked/unlinked
  - Extract @mentions from content for notifications
- **Files**: `server/internal/service/pm_doc.go`

#### Task 4.2.4: Doc handler
- **Status**: [ ] Not Started
- **Description**: HTTP handlers for doc CRUD
- **Endpoints**:
  - `GET /api/pm/docs` — list docs (filterable)
  - `POST /api/pm/docs` — create doc
  - `GET /api/pm/docs/{id}` — get doc with links
  - `PUT /api/pm/docs/{id}` — update doc
  - `DELETE /api/pm/docs/{id}` — delete doc
  - `POST /api/pm/docs/{id}/links` — link to entity
  - `DELETE /api/pm/docs/{id}/links/{linkId}` — unlink
  - `PUT /api/pm/docs/{id}/pin` — toggle pin
- **Files**: `server/internal/handler/pm_doc.go`

---

### 4.3 Backend — Attachments

#### Task 4.3.1: Attachment models
- **Status**: [ ] Not Started
- **Description**: Go structs for Attachment
- **Files**: `server/internal/model/pm_attachment.go`

#### Task 4.3.2: Attachment repository
- **Status**: [ ] Not Started
- **Description**: Data access layer for attachments
- **Methods**:
  - `ListByEntity(ctx, entityType, entityID)` — list attachments
  - `GetByID(ctx, id)` — get attachment metadata
  - `Create(ctx, attachment)` — save metadata
  - `Delete(ctx, id)` — delete metadata (and trigger file cleanup)
- **Files**: `server/internal/repository/pm_attachment.go`

#### Task 4.3.3: Attachment service
- **Status**: [ ] Not Started
- **Description**: Business logic for file uploads
- **Logic**:
  - Validate file size (max 50MB)
  - Generate unique storage path: `{workspace_id}/{entity_type}/{entity_id}/{uuid}_{filename}`
  - Store file to local filesystem or S3-compatible storage
  - Generate thumbnail for images (optional, can be deferred)
  - On delete: remove file from storage + delete DB record
  - Log activity when attachment added/removed
- **Files**: `server/internal/service/pm_attachment.go`

#### Task 4.3.4: Attachment handler
- **Status**: [ ] Not Started
- **Description**: HTTP handlers for file upload/download
- **Endpoints**:
  - `POST /api/pm/attachments` — multipart file upload (fields: entity_type, entity_id, file)
  - `GET /api/pm/attachments/{id}` — download/stream file
  - `GET /api/pm/attachments/{id}/thumbnail` — get thumbnail (images only)
  - `DELETE /api/pm/attachments/{id}` — delete file
- **Files**: `server/internal/handler/pm_attachment.go`

---

### 4.4 Backend — WIP Limits Enforcement

#### Task 4.4.1: WIP limit validation in story move
- **Status**: [ ] Not Started
- **Description**: Enforce WIP limits when moving stories between states
- **Logic**:
  - When a story is moved to a state with a WIP limit:
    - Count current stories in that state
    - If count >= wip_limit, return a warning (soft limit) or error (hard limit)
    - Response includes: `wip_warning: true`, `state_count`, `wip_limit`
  - Frontend decides whether to block or warn based on response
  - WIP limits are per-workflow-state (already defined in Phase 1 schema)
- **Files**: Update `server/internal/service/pm_story.go`

---

### 4.5 Backend — @Mention Notification Enhancement

#### Task 4.5.1: Mention extraction and notification creation
- **Status**: [ ] Not Started
- **Description**: Parse @mentions from comments, descriptions, and docs to create notifications
- **Logic**:
  - Regex to extract `@username` patterns from text
  - Resolve usernames to user IDs within workspace
  - Support `@team-name` → expand to all team member IDs
  - Create `mentioned` notification for each mentioned user
  - Deduplicate: don't notify if user is the actor
  - Deduplicate: don't create duplicate notification for same entity+user
  - Called from: comment service (on create/edit), story service (on description update), doc service (on content update)
- **Files**: `server/internal/service/pm_mention.go`

---

### 4.6 Backend — Dependency Graph Data

#### Task 4.6.1: Dependency graph endpoint
- **Status**: [ ] Not Started
- **Description**: API endpoint returning structured dependency graph data
- **Endpoints**:
  - `GET /api/pm/epics/{id}/dependencies` — all story links within an epic
  - `GET /api/pm/iterations/{id}/dependencies` — all story links within an iteration
- **Response**: List of nodes (stories) and edges (links) for graph rendering
  ```json
  {
    "nodes": [
      { "id": "story-uuid", "display_id": 123, "name": "...", "state": "...", "state_type": "started" }
    ],
    "edges": [
      { "source": "story-uuid-1", "target": "story-uuid-2", "type": "blocks" }
    ]
  }
  ```
- **Files**: Add to `server/internal/handler/pm_epic.go`, `server/internal/handler/pm_iteration.go`

---

### 4.7 Frontend — Docs

#### Task 4.7.1: Docs list page
- **Status**: [ ] Not Started
- **Description**: Page listing all documents
- **Details**:
  - Route: `/w/{slug}/pm/docs`
  - List/grid of docs: title, author, last updated, access level icon, pinned indicator
  - Filter by: author, team, access level
  - Sort by: updated, created, title
  - Pinned docs shown at top
  - Create Doc button
  - Search docs by title
- **Files**: `frontend/src/pages/pm/Docs.tsx`

#### Task 4.7.2: Doc Editor page
- **Status**: [ ] Not Started
- **Description**: Rich text editor for documents
- **Details**:
  - Route: `/w/{slug}/pm/docs/{id}`
  - Title: editable inline (large heading)
  - Content: Rich text editor with:
    - Headings (H1-H6)
    - Bold, italic, strikethrough, underline
    - Ordered and unordered lists
    - Code blocks with syntax highlighting
    - Inline code
    - Blockquotes
    - Tables (basic)
    - Links
    - Images (from attachments)
    - @mention autocomplete
    - Markdown shortcuts (type `#` for heading, `- ` for list, etc.)
  - Sidebar:
    - Access level selector (workspace/team/private)
    - Team selector (for team-scoped)
    - Pin/Unpin toggle
    - Linked entities: list with live status
    - "Link to..." button → search and select entity
  - Auto-save on content change (debounced 1-2 seconds)
  - Comment section at bottom
  - Use TipTap editor (headless, extensible, React-native)
- **Files**: `frontend/src/pages/pm/DocEditor.tsx`

#### Task 4.7.3: Rich Text Editor component
- **Status**: [ ] Not Started
- **Description**: Reusable TipTap-based rich text editor
- **Details**:
  - Toolbar: formatting buttons, heading selector, list buttons, code block, table, link, image
  - Floating menu for selected text formatting
  - Slash commands (`/heading`, `/list`, `/code`, `/table`)
  - @mention extension with user/team autocomplete
  - Markdown import/export
  - Image upload (via attachment service)
  - Configurable: toolbar shown/hidden, features enabled/disabled
  - Reusable in: Doc Editor, Story description (upgrade from textarea), Epic description
- **Files**: `frontend/src/components/pm/RichTextEditor.tsx`

#### Task 4.7.4: Doc links panel in entity detail pages
- **Status**: [ ] Not Started
- **Description**: Show linked docs in story, epic, iteration, objective detail pages
- **Details**:
  - "Docs" section showing linked documents with title and link
  - "Link Doc" button to search and link existing doc
  - "Create Doc" shortcut to create new doc pre-linked to this entity
  - Each linked doc: title, author, access icon, click to navigate
- **Files**: `frontend/src/components/pm/DocLinksPanel.tsx`

#### Task 4.7.5: Pinned docs in sidebar navigation
- **Status**: [ ] Not Started
- **Description**: Show pinned docs in the workspace sidebar
- **Details**:
  - Under "Docs" nav item, show pinned docs as sub-items
  - Click → navigate directly to doc editor
  - Max 5-10 pinned docs visible, "View all" link for more
- **Files**: Update sidebar navigation component

---

### 4.8 Frontend — File Attachments

#### Task 4.8.1: Attachments section in Story Detail Panel
- **Status**: [ ] Not Started
- **Description**: View and upload file attachments on stories
- **Details**:
  - "Attachments" section in Story Detail Panel
  - Drop zone: drag and drop files onto the section
  - Upload button: click to select files
  - File list: filename, size, type icon, uploaded by, date
  - Image files: show inline thumbnail preview
  - Click file: download
  - Delete attachment (X button, confirm)
  - Upload progress indicator
  - File size limit warning (50MB)
- **Files**: `frontend/src/components/pm/AttachmentSection.tsx`

#### Task 4.8.2: Inline image paste in description/comments
- **Status**: [ ] Not Started
- **Description**: Paste images from clipboard into text areas
- **Details**:
  - Listen for paste events in description editor and comment input
  - If clipboard contains image, auto-upload as attachment
  - Insert markdown image reference `![](attachment_url)` into text
  - Show upload progress inline
- **Files**: Integration in `RichTextEditor.tsx` and comment input

---

### 4.9 Frontend — WIP Limits

#### Task 4.9.1: WIP limit indicators on board columns
- **Status**: [ ] Not Started
- **Description**: Visual indicators for WIP limits on Kanban board
- **Details**:
  - Column header shows: "State Name (3/5)" where 3 is current count, 5 is limit
  - Normal: gray count text
  - At limit: amber/yellow count text and subtle background
  - Over limit: red count text and warning background
  - Tooltip explaining the limit
  - When dragging a story to an over-limit column, show warning overlay
- **Files**: Update `frontend/src/components/pm/KanbanBoard.tsx`

#### Task 4.9.2: WIP limit configuration
- **Status**: [ ] Not Started
- **Description**: Configure WIP limits per workflow state in settings
- **Details**:
  - In Workflow settings: each state has a WIP limit input (number or empty for unlimited)
  - Save updates WIP limit on the workflow state
- **Files**: Update PM Settings page (workflow section)

---

### 4.10 Frontend — Dependency Graph Visualization

#### Task 4.10.1: Dependency graph component
- **Status**: [ ] Not Started
- **Description**: Visual dependency graph for stories in an epic or iteration
- **Details**:
  - Tab in Epic Detail and Iteration Detail pages: "Dependencies"
  - Render directed graph:
    - Nodes: story cards (display ID, title, state badge)
    - Edges: arrows showing blocking relationships
    - Arrow style: solid for blocks, dashed for relates_to
  - Color coding: unstarted (gray), started (blue), done (green), blocked (red border)
  - Layout: left-to-right directed graph (dagre layout)
  - Click node → navigate to story
  - Zoom and pan controls
  - Use `reactflow` library or `dagre` + custom SVG
- **Files**: `frontend/src/components/pm/DependencyGraph.tsx`

---

### 4.11 Frontend — Keyboard Shortcuts

#### Task 4.11.1: Keyboard shortcut system
- **Status**: [ ] Not Started
- **Description**: Global keyboard shortcuts for common PM actions
- **Shortcuts**:
  - `c` — Create new story (when not in text input)
  - `n` — Open notification dropdown
  - `/` or `Cmd+K` — Focus search bar
  - `b` — Switch to board view
  - `l` — Switch to list view
  - `Esc` — Close story detail panel / modal
  - `j` / `k` — Navigate between stories (down/up) in list view
  - `Enter` — Open selected story
  - `?` — Show keyboard shortcuts help
- **Details**:
  - Don't trigger shortcuts when user is typing in an input/textarea
  - Show shortcuts in a help modal
  - Use `useHotkeys` hook or custom handler
- **Files**: `frontend/src/hooks/usePMKeyboardShortcuts.ts`, `frontend/src/components/pm/KeyboardShortcutsHelp.tsx`

---

### 4.12 Backend — GitHub Integration

#### Task 4.12.1: GitHub webhook receiver
- **Status**: [ ] Not Started
- **Description**: Receive GitHub webhook events for branches, PRs, and commits
- **Details**:
  - Endpoint: `POST /api/pm/integrations/github/webhook`
  - Handle events:
    - `push` — extract story IDs from commit messages (`[tp-123]`)
    - `pull_request` — extract story IDs from PR title/body/branch name
    - `pull_request_review` — track review status
  - Parse branch naming convention: `tp-{display_id}-description`
  - Store linked VCS data on stories (PR URL, branch name, commit SHAs)
  - Webhook secret validation for security
- **Files**: `server/internal/handler/pm_github.go`, `server/internal/service/pm_github.go`

#### Task 4.12.2: GitHub integration settings
- **Status**: [ ] Not Started
- **Description**: Configure GitHub integration per workspace
- **Details**:
  - Settings page: GitHub Integration tab
  - Configure:
    - Webhook URL (show for copy)
    - Webhook secret (generate/regenerate)
    - Repository URL(s)
    - Auto-state transitions: enable/disable
    - State mapping: PR opened → "In Review", PR merged → "Done"
  - Store config in workspace settings or new `pm_integrations` table
- **Files**: `server/internal/model/pm_integration.go`, PM Settings page update

#### Task 4.12.3: VCS info display on story cards
- **Status**: [ ] Not Started
- **Description**: Show linked branches, PRs, and commits on stories
- **Details**:
  - Story Detail Panel: "Development" section
  - Show: linked branch name, PR status (open/merged/closed), commit count
  - PR status badge: open (green), merged (purple), closed (red)
  - Click PR → link to GitHub
  - Story card on board: small Git icon when VCS data exists
- **Files**: `frontend/src/components/pm/VCSSection.tsx`

---

### 4.13 Frontend — Upgrade Story Description to Rich Editor

#### Task 4.13.1: Replace textarea with Rich Text Editor in Story Detail
- **Status**: [ ] Not Started
- **Description**: Upgrade story description from plain textarea to TipTap rich editor
- **Details**:
  - Use the same RichTextEditor component built for Docs
  - Reduced toolbar (no table needed for story descriptions)
  - @mention support
  - Image paste/upload support
  - Auto-save on change (debounced)
  - Render markdown content for existing stories
- **Files**: Update `frontend/src/components/pm/StoryDetailPanel.tsx`

---

### 4.14 Frontend API Services (Phase 4 additions)

#### Task 4.14.1: Doc service
- **Files**: `frontend/src/lib/services/pmDocService.ts`

#### Task 4.14.2: Attachment service
- **Files**: `frontend/src/lib/services/pmAttachmentService.ts`

---

### 4.15 UX Polish

#### Task 4.15.1: Loading states and skeletons
- **Status**: [ ] Not Started
- **Description**: Add proper loading states to all PM pages
- **Details**:
  - Skeleton loaders for: board columns, story cards, table rows, timeline bars
  - Skeleton for story detail panel sections
  - Page-level loading spinner during initial data fetch
  - Inline loading indicators for actions (save, move, etc.)

#### Task 4.15.2: Empty states
- **Status**: [ ] Not Started
- **Description**: Friendly empty states for all list/board views
- **Details**:
  - Board with no stories: "No stories yet. Create your first story to get started."
  - Epics list empty: "No epics created. Epics help organize large initiatives."
  - Iterations empty: "No iterations yet. Create an iteration to start sprint planning."
  - Reports with no data: "Not enough data to show charts. Complete some stories first."
  - Each empty state has a CTA button to create the relevant entity

#### Task 4.15.3: Error handling and toasts
- **Status**: [ ] Not Started
- **Description**: Consistent error handling across PM features
- **Details**:
  - API errors show toast notification with message
  - Optimistic updates revert with error toast on failure
  - Network errors show retry option
  - Form validation errors highlighted inline
  - Success toasts for: create, update, delete, bulk operations

#### Task 4.15.4: Responsive design adjustments
- **Status**: [ ] Not Started
- **Description**: Ensure PM views work on smaller screens
- **Details**:
  - Board view: horizontal scroll on small screens
  - Table view: prioritize key columns, hide secondary on small screens
  - Story Detail Panel: full-width on mobile
  - Timeline: horizontal scroll with touch support
  - Sidebar: collapsible on small screens

---

## Definition of Done

Phase 4 is complete when:
- [ ] Docs can be created, edited with rich text, and linked to work items
- [ ] File attachments can be uploaded, viewed, and downloaded on stories
- [ ] WIP limits show visual indicators on the board and warn on violation
- [ ] Dependency graph visualizes blocking relationships in epic/iteration views
- [ ] @mention notifications work in comments, descriptions, and docs
- [ ] Keyboard shortcuts work for common actions
- [ ] GitHub webhook receives events and links PRs/branches/commits to stories
- [ ] Story description uses rich text editor with image support
- [ ] All pages have proper loading, empty, and error states
- [ ] Application works on standard desktop screen sizes

---

## Dependencies

- Phase 3 must be complete
- `tiptap` (rich text editor): `@tiptap/react`, `@tiptap/starter-kit`, extensions
- `reactflow` or `dagre` for dependency graph
- GitHub webhook setup (configured per repository)
- Object storage for file attachments (local filesystem initially, S3 later)

---

## Risk & Mitigations

| Risk | Mitigation |
|------|-----------|
| Rich text editor complexity | Use TipTap — mature, extensible, good React support |
| File storage costs | Local filesystem initially, migrate to S3 when needed |
| GitHub webhook security | Validate webhook signatures, store secret securely via Doppler |
| Dependency graph layout | Use dagre for automatic layout, fall back to simple list if graph is too complex |
| Keyboard shortcut conflicts | Only trigger when no input focused, use standard shortcuts where possible |
