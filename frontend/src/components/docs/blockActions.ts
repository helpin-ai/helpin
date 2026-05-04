import type { Editor } from '@tiptap/core'
import type { Node as ProseMirrorNode } from '@tiptap/pm/model'
import { slashMenuPluginKey } from './SlashMenuExtension'

export type TurnIntoKind =
  | 'paragraph'
  | 'heading2'
  | 'heading3'
  | 'heading4'
  | 'bulletList'
  | 'orderedList'
  | 'taskList'
  | 'toggle'
  | 'blockquote'
  | 'codeBlock'

export const TURN_INTO_OPTIONS: { kind: TurnIntoKind; label: string }[] = [
  { kind: 'paragraph', label: 'Text' },
  { kind: 'heading2', label: 'Heading 2' },
  { kind: 'heading3', label: 'Heading 3' },
  { kind: 'heading4', label: 'Heading 4' },
  { kind: 'bulletList', label: 'Bullet list' },
  { kind: 'orderedList', label: 'Numbered list' },
  { kind: 'taskList', label: 'To-do list' },
  { kind: 'toggle', label: 'Toggle' },
  { kind: 'blockquote', label: 'Quote' },
  { kind: 'codeBlock', label: 'Code block' },
]

export function turnInto(editor: Editor, pos: number, kind: TurnIntoKind): boolean {
  const node = editor.state.doc.nodeAt(pos)
  if (!node) return false
  // Select the block first so the toggle commands act on it.
  const chain = editor.chain().focus().setNodeSelection(pos)
  switch (kind) {
    case 'paragraph':
      return chain.clearNodes().run()
    case 'heading2':
      return chain.clearNodes().toggleHeading({ level: 2 }).run()
    case 'heading3':
      return chain.clearNodes().toggleHeading({ level: 3 }).run()
    case 'heading4':
      return chain.clearNodes().toggleHeading({ level: 4 }).run()
    case 'bulletList':
      return chain.clearNodes().toggleBulletList().run()
    case 'orderedList':
      return chain.clearNodes().toggleOrderedList().run()
    case 'taskList':
      return chain.clearNodes()
        .insertContent({ type: 'taskList', content: [{ type: 'taskItem', attrs: { checked: false }, content: [{ type: 'paragraph' }] }] })
        .run()
    case 'toggle':
      return (chain.clearNodes() as ReturnType<typeof chain.clearNodes> & { setToggleSection: (attrs: Record<string, unknown>) => typeof chain })
        .setToggleSection({ title: 'Details', open: true })
        .run()
    case 'blockquote':
      return chain.clearNodes().toggleBlockquote().run()
    case 'codeBlock':
      return chain.clearNodes().toggleCodeBlock().run()
    default:
      return false
  }
}

export function duplicateBlock(editor: Editor, pos: number): boolean {
  const node = editor.state.doc.nodeAt(pos)
  if (!node) return false
  const insertAt = pos + node.nodeSize
  return editor
    .chain()
    .focus()
    .insertContentAt(insertAt, node.toJSON())
    .run()
}

export function deleteBlock(editor: Editor, pos: number): boolean {
  const node = editor.state.doc.nodeAt(pos)
  if (!node) return false
  return editor
    .chain()
    .focus()
    .deleteRange({ from: pos, to: pos + node.nodeSize })
    .run()
}

export function getBlockId(node: ProseMirrorNode | null | undefined): string | null {
  if (!node) return null
  const id = (node.attrs as Record<string, unknown> | undefined)?.['blockId']
  return typeof id === 'string' && id ? id : null
}

export async function copyBlockLink(editor: Editor, pos: number): Promise<boolean> {
  const node = editor.state.doc.nodeAt(pos)
  const id = getBlockId(node)
  if (!id) return false
  const url = `${window.location.origin}${window.location.pathname}${window.location.search}#block-${id}`
  try {
    await navigator.clipboard.writeText(url)
    return true
  } catch {
    return false
  }
}

/**
 * Insert an empty paragraph after the block at `pos` and open the slash menu
 * inside it. Mirrors the user's "click + → new paragraph with slash menu" flow.
 */
export function insertParagraphAndOpenSlash(editor: Editor, pos: number): boolean {
  const node = editor.state.doc.nodeAt(pos)
  if (!node) return false
  const insertAt = pos + node.nodeSize
  const inserted = editor
    .chain()
    .focus()
    .insertContentAt(insertAt, { type: 'paragraph' })
    .setTextSelection(insertAt + 1)
    .run()
  if (!inserted) return false
  // Trigger the slash menu plugin directly (don't insert a literal '/')
  const view = editor.view
  const cursor = view.state.selection.from
  const tr = view.state.tr.setMeta(slashMenuPluginKey, {
    open: true,
    from: cursor,
    query: '',
    selectedIndex: 0,
  })
  view.dispatch(tr)
  return true
}
