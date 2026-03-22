# Docs Editor Low-Risk Blocks Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add slash menu command palette and expose blockquote, code block, divider, and table blocks in the Docs editor.

**Architecture:** StarterKit already bundles blockquote, codeBlock, and horizontalRule — they just need UI exposure. Table requires installing `@tiptap/extension-table` packages. A new slash menu (`/` command) provides the primary insertion UX. The Go renderer and Help Center CSS already support all four blocks, so no backend changes are needed.

**Tech Stack:** Tiptap 3, React, TypeScript, `@tiptap/extension-table`

---

## File Map

| Action | File | Responsibility |
|--------|------|----------------|
| Create | `frontend/src/components/docs/SlashMenu.tsx` | Slash menu popup component (custom, positioned near cursor) |
| Create | `frontend/src/components/docs/slash-commands.ts` | Slash command definitions (name, icon, action) |
| Create | `frontend/src/components/docs/SlashMenuExtension.ts` | Tiptap extension that triggers slash menu on `/` |
| Modify | `frontend/src/components/docs/DocsEditor.tsx` | Register slash menu extension + table extensions, render SlashMenu |
| Modify | `frontend/package.json` | Add `@tiptap/extension-table` packages |
| Modify | `packages/widget-core/src/styles/widget.css` | Add `.helpin-article-content hr` styles |

---

### Task 1: Install table Tiptap packages

**Files:**
- Modify: `frontend/package.json`

- [ ] **Step 1: Install table extensions**

```bash
cd frontend && pnpm add @tiptap/extension-table @tiptap/extension-table-row @tiptap/extension-table-header @tiptap/extension-table-cell
```

- [ ] **Step 2: Verify install**

Run: `cd frontend && pnpm ls @tiptap/extension-table`
Expected: Shows installed version

- [ ] **Step 3: Commit**

```bash
git add frontend/package.json frontend/pnpm-lock.yaml
git commit -m "chore: add @tiptap/extension-table packages for docs editor"
```

---

### Task 2: Create slash command definitions

**Files:**
- Create: `frontend/src/components/docs/slash-commands.ts`

- [ ] **Step 1: Create the slash commands file**

```ts
import type { Editor } from '@tiptap/core';
import {
  Heading1,
  Heading2,
  Heading3,
  List,
  ListOrdered,
  Quote,
  Code2,
  Minus,
  Table,
  Image,
  type LucideIcon,
} from 'lucide-react';

export interface SlashCommand {
  title: string;
  description: string;
  icon: LucideIcon;
  action: (editor: Editor) => void;
}

export const slashCommands: SlashCommand[] = [
  {
    title: 'Heading 1',
    description: 'Large section heading',
    icon: Heading1,
    action: (editor) => editor.chain().focus().toggleHeading({ level: 1 }).run(),
  },
  {
    title: 'Heading 2',
    description: 'Medium section heading',
    icon: Heading2,
    action: (editor) => editor.chain().focus().toggleHeading({ level: 2 }).run(),
  },
  {
    title: 'Heading 3',
    description: 'Small section heading',
    icon: Heading3,
    action: (editor) => editor.chain().focus().toggleHeading({ level: 3 }).run(),
  },
  {
    title: 'Bullet List',
    description: 'Unordered list',
    icon: List,
    action: (editor) => editor.chain().focus().toggleBulletList().run(),
  },
  {
    title: 'Numbered List',
    description: 'Ordered list',
    icon: ListOrdered,
    action: (editor) => editor.chain().focus().toggleOrderedList().run(),
  },
  {
    title: 'Blockquote',
    description: 'Quote or excerpt',
    icon: Quote,
    action: (editor) => editor.chain().focus().toggleBlockquote().run(),
  },
  {
    title: 'Code Block',
    description: 'Fenced code block',
    icon: Code2,
    action: (editor) => editor.chain().focus().toggleCodeBlock().run(),
  },
  {
    title: 'Divider',
    description: 'Horizontal rule',
    icon: Minus,
    action: (editor) => editor.chain().focus().setHorizontalRule().run(),
  },
  {
    title: 'Table',
    description: 'Insert a table',
    icon: Table,
    action: (editor) =>
      editor.chain().focus().insertTable({ rows: 2, cols: 2, withHeaderRow: true }).run(),
  },
  {
    title: 'Image',
    description: 'Upload an image',
    icon: Image,
    action: () => {
      // Handled specially in SlashMenu — triggers file picker
    },
  },
];
```

- [ ] **Step 2: Commit**

```bash
git add frontend/src/components/docs/slash-commands.ts
git commit -m "feat: define slash menu command palette entries for docs editor"
```

---

### Task 3: Create the Tiptap slash menu extension

This extension listens for `/` typed at the start of an empty block or after whitespace and emits events for the React component to show/hide the menu.

**Files:**
- Create: `frontend/src/components/docs/SlashMenuExtension.ts`

- [ ] **Step 1: Create the extension file**

```ts
import { Extension } from '@tiptap/core';
import { Plugin, PluginKey } from '@tiptap/pm/state';

export interface SlashMenuState {
  open: boolean;
  from: number;
  query: string;
  selectedIndex: number;
  commandCount: number;
}

const CLOSED: SlashMenuState = { open: false, from: 0, query: '', selectedIndex: 0, commandCount: 0 };

export const slashMenuPluginKey = new PluginKey('slashMenu');

export const SlashMenuExtension = Extension.create({
  name: 'slashMenu',

  addProseMirrorPlugins() {
    return [
      new Plugin({
        key: slashMenuPluginKey,
        state: {
          init(): SlashMenuState {
            return CLOSED;
          },
          apply(tr, prev): SlashMenuState {
            const meta = tr.getMeta(slashMenuPluginKey);
            if (meta !== undefined) return { ...prev, ...meta };
            if (!prev.open) return prev;
            // If the selection moved away from the slash position, close
            const { from } = tr.selection;
            if (from < prev.from) return CLOSED;
            // Update query from text between slash and cursor
            const text = tr.doc.textBetween(prev.from, from, '\0', '\0');
            return { ...prev, query: text, selectedIndex: 0 };
          },
        },
        props: {
          handleKeyDown(view, event) {
            const state = slashMenuPluginKey.getState(view.state) as SlashMenuState;
            if (!state?.open) return false;

            if (event.key === 'Escape') {
              view.dispatch(view.state.tr.setMeta(slashMenuPluginKey, CLOSED));
              return true;
            }
            if (event.key === 'ArrowDown') {
              const next = (state.selectedIndex + 1) % Math.max(state.commandCount, 1);
              view.dispatch(view.state.tr.setMeta(slashMenuPluginKey, { selectedIndex: next }));
              return true;
            }
            if (event.key === 'ArrowUp') {
              const count = Math.max(state.commandCount, 1);
              const next = (state.selectedIndex - 1 + count) % count;
              view.dispatch(view.state.tr.setMeta(slashMenuPluginKey, { selectedIndex: next }));
              return true;
            }
            if (event.key === 'Enter') {
              // Signal the React component to execute the selected command
              view.dispatch(view.state.tr.setMeta(slashMenuPluginKey, { executeSelected: true }));
              return true;
            }
            return false;
          },
          handleTextInput(view, from, _to, text) {
            if (text !== '/') return false;
            // Only trigger at start of empty text block or after whitespace
            const { $from } = view.state.selection;
            const textBefore = $from.parent.textBetween(0, $from.parentOffset, '\0', '\0');
            if (textBefore.length === 0 || textBefore.endsWith(' ')) {
              // Schedule opening after the character is inserted
              setTimeout(() => {
                const tr = view.state.tr.setMeta(slashMenuPluginKey, {
                  open: true,
                  from: from + 1, // after the /
                  query: '',
                  selectedIndex: 0,
                });
                view.dispatch(tr);
              });
            }
            return false;
          },
        },
      }),
    ];
  },
});
```

- [ ] **Step 2: Commit**

```bash
git add frontend/src/components/docs/SlashMenuExtension.ts
git commit -m "feat: add Tiptap ProseMirror plugin for slash menu trigger"
```

---

### Task 4: Create the SlashMenu React component

**Files:**
- Create: `frontend/src/components/docs/SlashMenu.tsx`

Reference: The editor uses `useRef` for position tracking, similar to the floating toolbar pattern in DocsEditor.tsx.

- [ ] **Step 1: Create the component**

```tsx
import React, { useEffect, useRef, useCallback } from 'react';
import type { Editor } from '@tiptap/core';
import { slashMenuPluginKey, type SlashMenuState } from './SlashMenuExtension';
import { slashCommands, type SlashCommand } from './slash-commands';

interface SlashMenuProps {
  editor: Editor;
  onImageInsert?: () => void;
}

const CLOSED = { open: false, from: 0, query: '', selectedIndex: 0, commandCount: 0 };

export function SlashMenu({ editor, onImageInsert }: SlashMenuProps) {
  const menuRef = useRef<HTMLDivElement>(null);
  const stateRef = useRef<SlashMenuState>({ ...CLOSED });

  // Read plugin state reactively via forced re-render
  const pluginState = (slashMenuPluginKey.getState(editor.state) as SlashMenuState) ?? CLOSED;
  stateRef.current = pluginState;

  const filtered = slashCommands.filter(
    (cmd) =>
      cmd.title.toLowerCase().includes(pluginState.query.toLowerCase()) ||
      cmd.description.toLowerCase().includes(pluginState.query.toLowerCase()),
  );

  // Keep commandCount in sync so the plugin can wrap arrow keys
  useEffect(() => {
    if (pluginState.open && pluginState.commandCount !== filtered.length) {
      editor.view.dispatch(
        editor.state.tr.setMeta(slashMenuPluginKey, { commandCount: filtered.length }),
      );
    }
  }, [editor, pluginState.open, pluginState.commandCount, filtered.length]);

  // Force re-render on every transaction to pick up plugin state changes
  const [, forceUpdate] = React.useReducer((x: number) => x + 1, 0);
  useEffect(() => {
    editor.on('transaction', forceUpdate);
    return () => { editor.off('transaction', forceUpdate); };
  }, [editor]);

  const executeCommand = useCallback(
    (cmd: SlashCommand) => {
      const { from } = stateRef.current;
      // Delete the slash and query text
      editor
        .chain()
        .focus()
        .deleteRange({ from: from - 1, to: editor.state.selection.from })
        .run();

      if (cmd.title === 'Image' && onImageInsert) {
        onImageInsert();
      } else {
        cmd.action(editor);
      }

      // Close menu
      editor.view.dispatch(editor.state.tr.setMeta(slashMenuPluginKey, CLOSED));
    },
    [editor, onImageInsert],
  );

  // Handle executeSelected signal from ProseMirror plugin (Enter key)
  useEffect(() => {
    if ((pluginState as any).executeSelected && filtered[pluginState.selectedIndex]) {
      executeCommand(filtered[pluginState.selectedIndex]);
    }
  }, [(pluginState as any).executeSelected]);

  // Position the menu near the cursor
  useEffect(() => {
    if (!pluginState.open || !menuRef.current) return;
    const coords = editor.view.coordsAtPos(pluginState.from - 1);
    const wrapper = editor.view.dom.closest('.docs-editor-wrapper');
    if (!wrapper) return;
    const editorRect = wrapper.getBoundingClientRect();
    menuRef.current.style.top = `${coords.bottom - editorRect.top + 4}px`;
    menuRef.current.style.left = `${coords.left - editorRect.left}px`;
  }, [pluginState, editor]);

  // Close on click outside
  useEffect(() => {
    if (!pluginState.open) return;
    const handleClick = (e: MouseEvent) => {
      if (menuRef.current && !menuRef.current.contains(e.target as Node)) {
        editor.view.dispatch(editor.state.tr.setMeta(slashMenuPluginKey, CLOSED));
      }
    };
    document.addEventListener('mousedown', handleClick);
    return () => document.removeEventListener('mousedown', handleClick);
  }, [pluginState.open, editor]);

  if (!pluginState.open || filtered.length === 0) return null;

  return (
    <div
      ref={menuRef}
      className="absolute z-50 w-64 max-h-72 overflow-y-auto rounded-lg border bg-popover p-1 shadow-md"
    >
      {filtered.map((cmd, i) => (
        <button
          key={cmd.title}
          className={`flex w-full items-center gap-3 rounded-md px-2 py-1.5 text-left text-sm transition-colors ${
            i === pluginState.selectedIndex ? 'bg-accent text-accent-foreground' : 'hover:bg-accent/50'
          }`}
          onMouseEnter={() => {
            editor.view.dispatch(
              editor.state.tr.setMeta(slashMenuPluginKey, { selectedIndex: i }),
            );
          }}
          onMouseDown={(e) => {
            e.preventDefault(); // Don't steal focus from editor
            executeCommand(cmd);
          }}
        >
          <cmd.icon className="h-4 w-4 text-muted-foreground shrink-0" />
          <div>
            <p className="font-medium">{cmd.title}</p>
            <p className="text-xs text-muted-foreground">{cmd.description}</p>
          </div>
        </button>
      ))}
    </div>
  );
}
```


- [ ] **Step 2: Commit**

```bash
git add frontend/src/components/docs/SlashMenu.tsx
git commit -m "feat: add SlashMenu React component for docs editor command palette"
```

---

### Task 5: Integrate slash menu and table into DocsEditor

**Files:**
- Modify: `frontend/src/components/docs/DocsEditor.tsx`

This task registers the new extensions and renders the SlashMenu component.

- [ ] **Step 1: Add imports**

Add these imports to the top of DocsEditor.tsx:

```tsx
import Table from '@tiptap/extension-table';
import TableRow from '@tiptap/extension-table-row';
import TableHeader from '@tiptap/extension-table-header';
import TableCell from '@tiptap/extension-table-cell';
import { SlashMenuExtension } from './SlashMenuExtension';
import { SlashMenu } from './SlashMenu';
```

- [ ] **Step 2: Register extensions in useEditor**

In the `useEditor()` call, add the new extensions to the extensions array after `ResizableImageExtension`:

```tsx
Table.configure({ resizable: false }),
TableRow,
TableHeader,
TableCell,
SlashMenuExtension,
```

Note: StarterKit already includes Blockquote, CodeBlock, and HorizontalRule — no need to add those.

- [ ] **Step 3: Render SlashMenu component**

In the JSX, add the `SlashMenu` component inside the editor wrapper div (the one with `docs-editor-wrapper` class or the relative-positioned container). Place it after the editor content area:

```tsx
{editor && (
  <SlashMenu
    editor={editor}
    onImageInsert={() => {
      // Trigger the existing image file picker
      const input = document.createElement('input');
      input.type = 'file';
      input.accept = 'image/*';
      input.onchange = (e) => {
        const file = (e.target as HTMLInputElement).files?.[0];
        if (file && uploadConfig) {
          // Note: handleImageUpload requires (file, editorInstance)
          handleImageUpload(file, editor);
        }
      };
      input.click();
    }}
  />
)}
```

The `handleImageUpload` function in DocsEditor.tsx takes two arguments: `(file: File, editorInstance)`. Pass both `file` and `editor`.

- [ ] **Step 4: Add a hint for the slash menu**

Update the Placeholder text to hint about slash commands. In the `Placeholder.configure()` call, change the placeholder to include a slash hint:

```tsx
Placeholder.configure({
  placeholder: ({ node }) => {
    if (node.type.name === 'heading') return 'Heading';
    return "Type '/' for commands, or start writing...";
  },
}),
```

- [ ] **Step 5: Add `docs-editor-wrapper` class to the wrapper div**

The container wrapping the editor content needs `relative` positioning and the `docs-editor-wrapper` class for the absolutely-positioned SlashMenu. In DocsEditor.tsx, find the wrapper div around the editor content area (the one with `className="relative flex-1 ..."`). Add `docs-editor-wrapper` to its className:

```tsx
<div className={`relative flex-1 docs-editor-wrapper ${sourceView ? 'flex flex-col min-h-0' : 'overflow-y-auto'}`}>
```

This class is used by `SlashMenu.tsx` to calculate positioning via `editor.view.dom.closest('.docs-editor-wrapper')`.

- [ ] **Step 6: Verify it compiles**

Run: `cd frontend && pnpm build 2>&1 | grep -E "error TS|BUILD" | head -10`
Expected: No new TypeScript errors from our files

- [ ] **Step 7: Commit**

```bash
git add frontend/src/components/docs/DocsEditor.tsx
git commit -m "feat: integrate slash menu and table blocks into docs editor"
```

---

### Task 6: Add table toolbar controls

When the cursor is inside a table, show contextual controls for adding/removing rows and columns.

**Files:**
- Modify: `frontend/src/components/docs/DocsEditor.tsx` (floating toolbar section)

- [ ] **Step 1: Add table-aware toolbar buttons**

In the floating toolbar section of DocsEditor.tsx, add table controls that only appear when the cursor is inside a table. Add after the existing toolbar buttons:

```tsx
{editor.isActive('table') && (
  <>
    <div className="mx-1 h-4 w-px bg-border" />
    <ToolbarButton
      onClick={() => editor.chain().focus().addColumnAfter().run()}
      title="Add column"
    >
      <Plus className="h-3.5 w-3.5" />
      Col
    </ToolbarButton>
    <ToolbarButton
      onClick={() => editor.chain().focus().addRowAfter().run()}
      title="Add row"
    >
      <Plus className="h-3.5 w-3.5" />
      Row
    </ToolbarButton>
    <ToolbarButton
      onClick={() => editor.chain().focus().deleteColumn().run()}
      title="Remove column"
    >
      <Trash2 className="h-3.5 w-3.5" />
      Col
    </ToolbarButton>
    <ToolbarButton
      onClick={() => editor.chain().focus().deleteRow().run()}
      title="Remove row"
    >
      <Trash2 className="h-3.5 w-3.5" />
      Row
    </ToolbarButton>
    <ToolbarButton
      onClick={() => editor.chain().focus().deleteTable().run()}
      title="Remove table"
    >
      <Trash2 className="h-3.5 w-3.5" />
    </ToolbarButton>
  </>
)}
```

Ensure `Plus` and `Trash2` are imported from `lucide-react`.

- [ ] **Step 2: Add basic table editor styles**

Add table editing styles to the editor's Tailwind prose class or via a `<style>` block in the component. The editor needs basic table cell borders and padding:

Add a CSS block or Tailwind utility classes. The simplest approach — add to the existing editor styles or a scoped style in DocsEditor.tsx:

```css
.ProseMirror table {
  border-collapse: collapse;
  width: 100%;
  margin: 1em 0;
}
.ProseMirror th,
.ProseMirror td {
  border: 1px solid hsl(var(--border));
  padding: 0.5rem 0.75rem;
  min-width: 80px;
  vertical-align: top;
}
.ProseMirror th {
  background: hsl(var(--muted));
  font-weight: 600;
}
.ProseMirror .selectedCell {
  background: hsl(var(--accent));
}
```

Place this in the component's JSX as `<style>` or in the app's global CSS.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/components/docs/DocsEditor.tsx
git commit -m "feat: add table toolbar controls and editor styles"
```

---

### Task 7: Add Help Center hr styles

**Files:**
- Modify: `packages/widget-core/src/styles/widget.css`

The widget CSS has styles for `.helpin-article-content blockquote`, `code`, `pre`, and `table`, but NOT for `hr`. Add it.

- [ ] **Step 1: Add hr styles**

In `packages/widget-core/src/styles/widget.css`, find the `.helpin-article-content` rules section and add:

```css
.helpin-article-content hr {
  border: none;
  border-top: 1px solid rgba(15, 23, 42, 0.12);
  margin: 1.5em 0;
}
```

- [ ] **Step 2: Commit**

```bash
git add packages/widget-core/src/styles/widget.css
git commit -m "fix: add hr styles for Help Center article content"
```

---

### Task 8: Manual verification

- [ ] **Step 1:** Open a doc in the editor
- [ ] **Step 2:** Type `/` — slash menu should appear with all command options
- [ ] **Step 3:** Type `/code` — should filter to "Code Block", press Enter to insert
- [ ] **Step 4:** Type `/div` — should filter to "Divider", press Enter to insert horizontal rule
- [ ] **Step 5:** Type `/quote` — should filter to "Blockquote", press Enter to insert
- [ ] **Step 6:** Type `/table` — should insert a 2x2 table with header row
- [ ] **Step 7:** Click inside the table — toolbar should show add/remove row/column controls
- [ ] **Step 8:** Save the document, reload — all blocks should persist and re-render
- [ ] **Step 9:** View the document in the Help Center — blockquote, code, divider, table should render correctly (Go renderer already handles them)
- [ ] **Step 10:** Test `/` at start of line and after text with a space — both should trigger
- [ ] **Step 11:** Press Escape — slash menu should close
- [ ] **Step 12:** Arrow keys + Enter — should navigate and select slash menu items
