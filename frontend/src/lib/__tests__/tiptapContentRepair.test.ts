// @vitest-environment jsdom
import { Editor, Node } from '@tiptap/core'
import StarterKit from '@tiptap/starter-kit'
import { afterEach, describe, expect, it } from 'vitest'
import { repairTiptapDocument } from '../tiptapContentRepair'

const ResizableImage = Node.create({
  name: 'resizableImage',
  group: 'block',
  atom: true,
  renderHTML: ({ HTMLAttributes }) => ['img', HTMLAttributes],
})

let editor: Editor | null = null

function expectSchemaValid(content: unknown) {
  const errors: unknown[] = []
  editor = new Editor({
    element: document.createElement('div'),
    extensions: [StarterKit, ResizableImage],
    content: content as never,
    enableContentCheck: true,
    onContentError: ({ error }) => errors.push(error),
  })
  expect(errors).toEqual([])
  editor.schema.nodeFromJSON(content).check()
}

afterEach(() => {
  editor?.destroy()
  editor = null
})

describe('repairTiptapDocument', () => {
  it('renames standard image nodes recursively without changing attributes or the original', () => {
    const image = { type: 'image', attrs: { src: 'https://example.test/a.png', alt: 'Original', width: 320, metadata: { type: 'image' } } }
    const original = { type: 'doc', content: [image, { type: 'bulletList', content: [{ type: 'listItem', content: [{ type: 'paragraph' }, image] }] }] }
    const repaired = repairTiptapDocument(original)
    expect(repaired.content?.[0]).toEqual({ ...image, type: 'resizableImage' })
    expect(repaired.content?.[1]?.content?.[0]?.content?.[1]).toEqual({ ...image, type: 'resizableImage' })
    expect(original.content[0].type).toBe('image')
    expect(repairTiptapDocument(repaired)).toEqual(repaired)
    expectSchemaValid(repaired)
  })

  it('wraps a paragraph stored directly in an ordered list', () => {
    const repaired = repairTiptapDocument({
      type: 'doc',
      content: [{
        type: 'orderedList',
        attrs: { start: 1 },
        content: [{ type: 'paragraph', content: [{ type: 'text', text: 'Make.com step' }] }],
      }],
    })

    expect(repaired.content?.[0]?.content?.[0]).toMatchObject({
      type: 'listItem',
      content: [{ type: 'paragraph', content: [{ type: 'text', text: 'Make.com step' }] }],
    })
    expectSchemaValid(repaired)
  })

  it('converts a leading heading in a list item into its required paragraph', () => {
    const repaired = repairTiptapDocument({
      type: 'doc',
      content: [{
        type: 'bulletList',
        content: [{
          type: 'listItem',
          content: [
            { type: 'heading', attrs: { level: 3 }, content: [{ type: 'text', text: 'First item' }] },
            { type: 'paragraph', content: [{ type: 'text', text: 'Details' }] },
          ],
        }],
      }],
    })

    const itemContent = repaired.content?.[0]?.content?.[0]?.content
    expect(itemContent?.[0]).toEqual({
      type: 'paragraph',
      content: [{ type: 'text', text: 'First item' }],
    })
    expect(itemContent?.[1]?.type).toBe('paragraph')
    expectSchemaValid(repaired)
  })

  it('moves block images out of headings while preserving surrounding text', () => {
    const repaired = repairTiptapDocument({
      type: 'doc',
      content: [{
        type: 'heading',
        attrs: { level: 2 },
        content: [
          { type: 'text', text: 'Before' },
          { type: 'resizableImage', attrs: { src: 'https://cdn.example.com/image.png' } },
          { type: 'text', text: 'After' },
        ],
      }],
    })

    expect(repaired.content?.map((node) => node.type)).toEqual([
      'heading',
      'resizableImage',
      'heading',
    ])
    expect(repaired.content?.[0]?.content?.[0]?.text).toBe('Before')
    expect(repaired.content?.[2]?.content?.[0]?.text).toBe('After')
    expectSchemaValid(repaired)
  })

  it('recovers string content and normalizes incompatible duplicate marks', () => {
    const repaired = repairTiptapDocument({
      type: 'doc',
      content: [
        { type: 'paragraph', content: 'Stored as the wrong JSON type' },
        {
          type: 'paragraph',
          content: [{
            type: 'text',
            text: 'code',
            marks: [{ type: 'bold' }, { type: 'code' }, { type: 'code' }],
          }],
        },
      ],
    })

    expect(repaired.content?.[0]?.content?.[0]?.text).toBe('Stored as the wrong JSON type')
    expect(repaired.content?.[1]?.content?.[0]?.marks).toEqual([{ type: 'code' }])
    expectSchemaValid(repaired)
  })
})
