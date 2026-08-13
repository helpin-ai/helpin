// @vitest-environment jsdom
import { describe, expect, it, afterEach } from 'vitest'
import { Editor } from '@tiptap/core'
import StarterKit from '@tiptap/starter-kit'

/**
 * The docs editor relies on TipTap's content check to refuse autosaving over a
 * document it could not parse. These tests pin that contract: without the
 * check, malformed stored content loads as an empty document and the next
 * autosave overwrites the real content with the loss.
 */
let editor: Editor | null = null

function createEditor(content: unknown, onContentError?: (error: unknown) => void) {
  const element = document.createElement('div')
  document.body.appendChild(element)
  editor = new Editor({
    element,
    extensions: [StarterKit],
    content: content as never,
    enableContentCheck: true,
    onContentError: ({ error }) => onContentError?.(error),
  })
  return editor
}

afterEach(() => {
  editor?.destroy()
  editor = null
  document.body.innerHTML = ''
})

describe('TipTap content check', () => {
  it('reports an error for a node whose content is a string', () => {
    const errors: unknown[] = []
    createEditor(
      { type: 'doc', content: [{ type: 'paragraph', content: 'Just text' }] },
      (error) => errors.push(error),
    )
    expect(errors).toHaveLength(1)
  })

  it('reports an error for an unknown node type', () => {
    const errors: unknown[] = []
    createEditor(
      { type: 'doc', content: [{ type: 'notARealNode', content: [{ type: 'text', text: 'Hi' }] }] },
      (error) => errors.push(error),
    )
    expect(errors).toHaveLength(1)
  })

  it('reports an error for duplicate marks on the same text node', () => {
    const errors: unknown[] = []
    createEditor(
      {
        type: 'doc',
        content: [{
          type: 'paragraph',
          content: [{
            type: 'text',
            text: 'Bold',
            marks: [{ type: 'bold' }, { type: 'bold' }],
          }],
        }],
      },
      (error) => errors.push(error),
    )
    expect(errors).toHaveLength(1)
  })

  it('does not report an error for a valid document', () => {
    const errors: unknown[] = []
    const ed = createEditor(
      { type: 'doc', content: [{ type: 'paragraph', content: [{ type: 'text', text: 'Hi' }] }] },
      (error) => errors.push(error),
    )
    expect(errors).toHaveLength(0)
    expect(ed.getText()).toBe('Hi')
  })

  it('leaves the editor empty when content fails the check, which is why saves must be blocked', () => {
    const ed = createEditor({ type: 'doc', content: [{ type: 'paragraph', content: 'Just text' }] })
    // This is the degraded document that must never reach onSave.
    expect(ed.getText()).toBe('')
  })
})
