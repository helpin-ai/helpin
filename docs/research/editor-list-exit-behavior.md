# Editor List Exit Behavior Research

## 1. List Exit Behavior Across Editors

### Google Docs
- **Enter on empty list item**: Exits the list, converts the empty bullet into a normal paragraph.
- **Enter twice**: First Enter creates a new bullet, second Enter on the empty bullet exits the list.
- **Backspace on empty list item**: Removes the bullet formatting and exits the list.
- **Tab/Shift+Tab**: Tab indents (nests), Shift+Tab outdents (unnests).
- **Nested list exit**: Enter on an empty nested item outdents it one level first (back to parent list), then another Enter on the now-empty parent-level item exits the list entirely.
- **Summary**: Single Enter on empty item = exit. Progressive unnesting for nested lists.

### Notion
- **Enter on empty list item**: Un-indents (outdents) the bullet one level. At the top level, converts to a regular paragraph block.
- **Enter on empty nested item**: Moves it up one nesting level (equivalent to Shift+Tab). Does NOT immediately exit the list.
- **Backspace on empty list item**: Converts to a paragraph (exits list).
- **Tab/Shift+Tab**: Tab nests deeper, Shift+Tab un-nests.
- **Full exit sequence from nested**: Enter (unnest to parent) -> Enter (unnest to top-level) -> Enter (exit to paragraph).
- **Summary**: Enter on empty = unnest one level. At top level, Enter on empty = exit to paragraph. Progressive unwinding.

### Microsoft Word (Traditional Behavior)
- **Enter on empty list item**: First Enter creates a new bullet. Pressing Enter again on the empty bullet exits the list.
- **Backspace on empty list item**: Removes the bullet, stays at the same indentation level but as plain text. A second backspace removes the indent.
- **Tab/Shift+Tab**: Tab increases indent level, Shift+Tab decreases indent level.
- **Nested list exit**: Backspace or Shift+Tab un-nests. Enter on empty item at any level exits the list.
- **Note**: Recent Word 365 updates have reportedly broken this behavior for some users, where Enter and Backspace no longer properly exit lists. Alt+Shift+Left arrow is an alternative to outdent.
- **Summary**: Double-Enter exits (Enter to create empty bullet, Enter again to exit). Backspace removes bullet then indent.

### Apple Notes
- **Enter on empty list item**: Exits the list, returns to normal text formatting.
- **Enter twice**: First Enter adds a new bullet, second Enter on the empty bullet exits the list.
- **Auto-lists**: Typing `*`, `-`, or `1.` followed by a space auto-creates a list.
- **Summary**: Same as Google Docs - Enter on empty list item exits.

### Linear
- **Editor**: Linear uses a TipTap/ProseMirror-based editor.
- **List behavior**: Follows standard TipTap defaults (see TipTap section below). Enter on an empty list item exits the list. Their editor is relatively minimal with standard list behaviors.
- **Summary**: Standard TipTap/ProseMirror behavior.

### Confluence (Atlassian)
- **Enter on empty list item**: Exits the list.
- **Tab/Shift+Tab**: Tab indents list items, Shift+Tab outdents.
- **Nested exit**: Progressive unnesting like Notion - outdent first, then exit.
- **Summary**: Standard behavior - Enter on empty exits, Tab/Shift+Tab for nesting.

### Coda
- **Enter on empty list item**: Exits the list.
- **Tab/Shift+Tab**: Standard indent/outdent.
- **Summary**: Standard behavior following document editor conventions.

### Slite
- **Enter on empty list item**: Exits the list.
- **Summary**: Standard behavior, TipTap-based editor.

---

## 2. Specific Scenario Matrix

### Pressing Enter on an Empty List Item

| Editor | Top-Level Empty | Nested Empty |
|--------|----------------|--------------|
| Google Docs | Exit list -> paragraph | Unnest one level |
| Notion | Exit list -> paragraph | Unnest one level |
| Microsoft Word | Exit list -> paragraph | Exit list entirely |
| Apple Notes | Exit list -> paragraph | Exit list -> paragraph |
| Linear/TipTap | Exit list -> paragraph | Unnest one level |
| Confluence | Exit list -> paragraph | Unnest one level |

**Consensus**: Enter on an empty top-level list item exits to a paragraph. For nested items, the majority behavior is to unnest one level first (progressive exit).

### Pressing Enter Twice on Empty Items

| Editor | Behavior |
|--------|----------|
| Google Docs | 1st: creates empty bullet; 2nd: exits list |
| Notion | 1st: creates empty bullet; 2nd: exits list (at top level) or unnests (if nested) |
| Word | 1st: creates empty bullet; 2nd: exits list |
| Apple Notes | 1st: creates empty bullet; 2nd: exits list |

**Consensus**: Universally, Enter-Enter (creating then exiting an empty bullet) is the standard exit gesture.

### Pressing Backspace on an Empty List Item

| Editor | Behavior |
|--------|----------|
| Google Docs | Removes bullet, exits to paragraph |
| Notion | Converts to paragraph block |
| Word | Removes bullet (stays indented), 2nd backspace removes indent |
| TipTap (default) | Visually removes bullet but stays in list structure ("Bullet List Limbo") |
| TipTap (with ListKeymap) | Lifts content into the list item above |

**Consensus**: Backspace on empty list item should exit the list. TipTap's default behavior is a known pain point (issue #3128 "Bullet List Limbo").

### Tab/Shift+Tab for Nesting

| Editor | Tab | Shift+Tab |
|--------|-----|-----------|
| All editors | Indent / nest deeper | Outdent / unnest |

**Consensus**: Universal. Tab = nest, Shift+Tab = unnest.

### Nested List Exit Sequence (Nested -> Parent -> Exit)

The standard pattern across editors (Google Docs, Notion, Confluence, TipTap):
1. **Nested empty item** + Enter = unnest one level (move to parent list level)
2. **Parent-level empty item** + Enter = exit list entirely (become paragraph)

This is called "progressive exit" or "graduated unnesting." Word is the exception, where Enter on ANY empty list item exits the entire list immediately.

### Select All Text in List Item + Enter

| Editor | Behavior |
|--------|----------|
| Most editors | Replaces selected text with a new list item (splits the list item) |
| Google Docs | Selected text is deleted, cursor remains in the (now empty) list item |
| Notion | Selected text is deleted, cursor remains in the list item |

**Consensus**: This scenario does NOT trigger list exit. It's treated as a normal text replacement within the list item.

---

## 3. Other Common Editor Block Exit Behaviors

### Code Blocks

| Editor | Exit Method |
|--------|-------------|
| Google Docs | No native code block; uses add-ons with "Exit" buttons |
| Notion | Enter at end of last line creates new line; `/` command or click outside to exit |
| TipTap (default) | **Cmd/Ctrl+Enter** exits code block, creates a paragraph below |
| TipTap (`exitCode` command) | Creates a default block after the code block and moves cursor there |
| ProseMirror | Arrow down from last line or Cmd+Enter to exit |
| Word | No native code block |

**TipTap issue**: Cmd+Enter conflicts with form submission (issue #2195). Common workaround: also allow triple-Enter (3 empty lines) to exit, or arrow-down from last line.

### Blockquotes

| Editor | Exit Method |
|--------|-------------|
| Google Docs | Enter creates new line inside blockquote; must manually change formatting |
| Notion | Enter on empty line inside blockquote exits the blockquote |
| TipTap (default) | Enter always creates new paragraph inside blockquote; no automatic exit |
| TipTap (keyboard shortcut) | Cmd+Shift+B toggles blockquote off |
| Confluence | Enter on empty paragraph inside blockquote exits |

**Common pattern**: Enter on an empty paragraph at the end of a blockquote should exit the blockquote. TipTap does NOT do this by default -- it requires custom keyboard handling.

### Callouts/Admonitions

| Editor | Exit Method |
|--------|-------------|
| Notion | Enter on empty line at end exits the callout |
| Helpin (current) | **3 Enters to exit**: Two consecutive empty paragraphs at the end trigger exit (removes the empties and creates a paragraph after the callout) |
| Helpin (current) | Backspace on single empty paragraph inside callout = delete the entire callout |

**Helpin's CalloutExtension** already implements a "double-empty-paragraph" exit pattern:
- Tracks if the last two children are empty paragraphs
- If so, removes them and inserts a paragraph after the callout
- Backspace on a callout with a single empty paragraph replaces the callout with an empty paragraph

### Tables

| Editor | Exit Method |
|--------|-------------|
| Google Docs | Tab in last cell moves to new row; clicking outside exits |
| Notion | Enter in last cell creates new row; `/` outside to add block after |
| TipTap | Tab navigates between cells; arrow-down from last row can exit; clicking outside exits |
| Word | Tab in last cell creates new row; Enter below table creates paragraph after |

**Common pattern**: Tables are generally exited by clicking outside or pressing arrow-down from the last row. Some editors insert a trailing paragraph after the table to ensure there's always a place to click/navigate to.

---

## 4. TipTap Specific Details

### Default List Behavior (StarterKit)

TipTap's StarterKit includes `BulletList`, `OrderedList`, and `ListItem` by default. The default behavior:

- **Enter on non-empty list item**: Splits into two list items (`splitListItem`)
- **Enter on empty top-level list item**: Lifts the item out of the list (exits to paragraph)
- **Enter on empty nested list item**: Lifts the item one level up (unnests)
- **Backspace at start of list item**: Joins with previous list item (default ProseMirror behavior)
- **Backspace on empty list item (DEFAULT)**: Only visually removes the bullet - content remains in list structure. This is the "Bullet List Limbo" bug (issue #3128).

### ListKeymap Extension

**Purpose**: Fixes the backspace/delete behavior for lists.

**What it does**:
- Pressing backspace at the start of a list item lifts the content into the list item above
- Properly handles delete key in list contexts
- Configurable for custom list types (taskItem, etc.)

**Installation**:
```typescript
import { ListKeymap } from '@tiptap/extension-list'
// or via ListKit which includes it
import { ListKit } from '@tiptap/extension-list'
```

**Configuration**:
```typescript
ListKeymap.configure({
  listTypes: [
    { itemName: 'listItem', wrapperNames: ['bulletList', 'orderedList'] },
    { itemName: 'taskItem', wrapperNames: ['taskList'] },
  ],
})
```

**Note**: ListKeymap is part of `@tiptap/extension-list` (TipTap v3). In older versions, it was a separate `@tiptap/extension-list-keymap` package.

### Key TipTap List Commands

| Command | Description |
|---------|-------------|
| `liftListItem` | Lifts a list item up into a wrapping parent list. Can also lift items out of a list entirely. |
| `sinkListItem` | Sinks a list item down into a child list (increases nesting). |
| `splitListItem` | Splits one list item into two at the cursor position. |
| `toggleList` | Toggles between different list types (bullet <-> ordered). |
| `wrapInList` | Wraps content in a list. |

### How `liftListItem` Works

The `liftListItem` command:
1. Finds the list item around the current selection
2. Tries to lift it up into a wrapping parent list
3. If there's no parent list (the item is at the top level), it lifts the item OUT of the list entirely, converting it to a regular paragraph
4. Usage: `editor.commands.liftListItem('listItem')`

### Standard TipTap Key Bindings for Lists

| Key | Action |
|-----|--------|
| Enter | `splitListItem` (splits or exits if empty) |
| Tab | `sinkListItem` (nest deeper) |
| Shift+Tab | `liftListItem` (unnest) |
| Backspace | Default: join backward. With ListKeymap: lift into item above |

### `exitCode` Command

For code blocks specifically:
- `editor.commands.exitCode()` creates a default block after the code block and moves cursor there
- Default shortcut: Cmd/Ctrl+Enter
- Known conflict with form submission shortcuts (GitHub issue #2195)

### Common TipTap Customizations for Better List Behavior

1. **Add ListKeymap extension** to fix backspace behavior (prevents "Bullet List Limbo")
2. **Custom Enter handler** for empty list items that calls `liftListItem` to progressively unnest
3. **Custom Backspace handler** that properly exits the list when pressing backspace on an empty list item at the top level
4. **Triple-Enter exit** for code blocks (as an alternative to Cmd+Enter)
5. **Empty-paragraph exit** for blockquotes and callouts (like Helpin's CalloutExtension pattern)

### What Helpin Currently Has vs. What's Missing

**Currently implemented**:
- StarterKit with default list behavior (Enter exits empty top-level items, unnests nested items)
- TaskList/TaskItem with nested support
- CalloutExtension with custom Enter/Backspace exit handling (double-empty-paragraph pattern)
- CodeBlockExtension (lowlight, but no custom exit handling beyond TipTap defaults)
- No ListKeymap extension installed

**Potentially missing**:
- ListKeymap extension (would fix "Bullet List Limbo" backspace issue)
- Custom blockquote exit behavior (Enter on empty paragraph at end of blockquote to exit)
- Custom code block exit alternative to Cmd+Enter (e.g., triple-Enter or arrow-down-from-last-line)
- Table trailing paragraph (ensuring there's always a paragraph after a table to navigate to)

---

## 5. Recommended Standard Behavior (Industry Consensus)

### Lists
1. **Enter on non-empty item**: Split into two items
2. **Enter on empty top-level item**: Exit list, create paragraph
3. **Enter on empty nested item**: Unnest one level (progressive exit)
4. **Backspace on empty item**: Exit list / unnest (should NOT create "Bullet List Limbo")
5. **Tab**: Nest one level deeper
6. **Shift+Tab**: Unnest one level
7. **Select-all + Enter**: Replace text, stay in list item

### Code Blocks
1. **Cmd/Ctrl+Enter**: Exit code block, create paragraph below
2. **Arrow down from last line**: Exit to content below
3. Optional: Triple-Enter (3 empty lines) to exit

### Blockquotes
1. **Enter on empty paragraph at end**: Exit blockquote, create paragraph below
2. **Backspace on empty single-paragraph blockquote**: Delete blockquote, leave paragraph
3. **Cmd+Shift+B**: Toggle blockquote off

### Callouts
1. **Double-empty-paragraph at end + Enter**: Exit callout (Helpin already does this)
2. **Backspace on empty single-paragraph callout**: Delete callout

### Tables
1. **Tab in last cell**: Navigate or create new row (editor preference)
2. **Arrow down from last row**: Exit table, move to content below
3. Ensure trailing paragraph exists after table for navigation

---

## Sources

- [TipTap ListKeymap Extension Docs](https://tiptap.dev/docs/editor/extensions/functionality/listkeymap)
- [TipTap List Commands Docs](https://tiptap.dev/docs/editor/api/commands/lists)
- [TipTap liftListItem Command Docs](https://tiptap.dev/docs/editor/api/commands/lists/lift-list-item)
- [TipTap Keyboard Shortcuts Docs](https://tiptap.dev/docs/editor/core-concepts/keyboard-shortcuts)
- [TipTap exitCode Command Docs](https://tiptap.dev/docs/editor/api/commands/nodes-and-marks/exit-code)
- [TipTap Blockquote Extension Docs](https://tiptap.dev/docs/editor/api/nodes/blockquote)
- [TipTap Issue #3128 - "Bullet List Limbo"](https://github.com/ueberdosis/tiptap/issues/3128)
- [TipTap Issue #2195 - CodeBlock Exit Shortcut Conflict](https://github.com/ueberdosis/tiptap/issues/2195)
- [Notion Keyboard Shortcuts](https://www.notion.com/help/keyboard-shortcuts)
- [Notion Tweet - Enter on Empty Bullet](https://x.com/NotionHQ/status/1569728848966651906)
- [Microsoft Word List Issues](https://learn.microsoft.com/en-us/answers/questions/5104154/word-365-double-enter-press-not-ending-bullets-or)
- [Apple Notes Keyboard Shortcuts](https://support.apple.com/guide/notes/keyboard-shortcuts-and-gestures-apd46c25187e/mac)
- [ProseMirror Guide](https://prosemirror.net/docs/guide/)
- [Confluence Keyboard Shortcuts](https://support.atlassian.com/confluence-cloud/docs/keyboard-shortcuts-markdown-and-autocomplete/)
