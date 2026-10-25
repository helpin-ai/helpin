import type { Editor } from '@tiptap/core'
import { DOMParser, DOMSerializer, type Slice } from '@tiptap/pm/model'
import type { Transaction } from '@tiptap/pm/state'
import DOMPurify from 'dompurify'

export interface AISelection {
  from: number
  to: number
  slice: Slice
  html: string
  valid: boolean
}

export function sanitizeAIRewrite(html: string): string {
  return DOMPurify.sanitize(html, {
    ALLOWED_TAGS: ['p', 'br', 'h1', 'h2', 'h3', 'h4', 'h5', 'h6', 'strong', 'b', 'em', 'i', 'u', 's', 'del', 'sub', 'sup', 'a', 'ul', 'ol', 'li', 'blockquote', 'pre', 'code'],
    ALLOWED_ATTR: ['href', 'title', 'start'],
    ALLOW_DATA_ATTR: false,
    ALLOW_ARIA_ATTR: false,
  })
}

export function captureAISelection(editor: Editor): AISelection {
  const { from, to, empty } = editor.state.selection
  if (empty || !editor.state.doc.textBetween(from, to).trim()) throw new Error('Select some text to edit with AI.')
  let hasEmbed = false
  editor.state.doc.nodesBetween(from, to, node => {
    if (node.isAtom && !node.isText && node.type.name !== 'hardBreak') hasEmbed = true
  })
  if (hasEmbed) throw new Error('Select text without images or embedded blocks to edit with AI.')
  const slice = editor.state.doc.slice(from, to)
  const wrapper = document.createElement('div')
  wrapper.append(DOMSerializer.fromSchema(editor.schema).serializeFragment(slice.content))
  const html = sanitizeAIRewrite(wrapper.innerHTML)
  if (new TextEncoder().encode(html).length > 24000) throw new Error('Select a shorter passage to edit with AI.')
  return { from, to, slice, html, valid: true }
}

export function mapAISelection(selection: AISelection, transaction: Transaction): AISelection {
  if (!transaction.docChanged || !selection.valid) return selection
  const from = transaction.mapping.map(selection.from, 1)
  const to = transaction.mapping.map(selection.to, -1)
  const valid = from < to && transaction.doc.slice(from, to).eq(selection.slice)
  return { ...selection, from, to, valid }
}

export function replaceAISelection(editor: Editor, selection: AISelection, html: string) {
  if (!selection.valid || selection.to > editor.state.doc.content.size || !editor.state.doc.slice(selection.from, selection.to).eq(selection.slice)) {
    throw new Error('The selected text changed. Select it again to start a new rewrite.')
  }
  const wrapper = document.createElement('div')
  wrapper.innerHTML = sanitizeAIRewrite(html)
  if (!wrapper.textContent?.trim()) throw new Error('AI returned no text. Try again.')
  const slice = DOMParser.fromSchema(editor.schema).parseSlice(wrapper)
  editor.view.dispatch(editor.state.tr.replaceRange(selection.from, selection.to, slice).scrollIntoView())
  editor.commands.focus()
}
