import { Node, mergeAttributes } from '@tiptap/core'
import { ReactNodeViewRenderer } from '@tiptap/react'
import { ToggleSectionNodeView } from './ToggleSectionNodeView'

export interface ToggleSectionAttrs {
  title?: string | null
  open?: boolean
}

declare module '@tiptap/core' {
  interface Commands<ReturnType> {
    toggleSection: {
      setToggleSection: (attrs?: ToggleSectionAttrs) => ReturnType
    }
  }
}

export const ToggleSectionExtension = Node.create({
  name: 'toggleSection',
  group: 'block',
  content: 'block+',
  defining: true,

  addAttributes() {
    return {
      title: {
        default: 'Details',
        parseHTML: (el) => (el as HTMLElement).getAttribute('data-toggle-title') || 'Details',
        renderHTML: (attrs) => ({ 'data-toggle-title': attrs.title || 'Details' }),
      },
      open: {
        default: false,
        parseHTML: (el) => (el as HTMLElement).hasAttribute('open'),
        renderHTML: (attrs) => (attrs.open ? { open: '' } : {}),
      },
    }
  },

  parseHTML() {
    return [{ tag: 'details[data-toggle-section]' }]
  },

  renderHTML({ HTMLAttributes }) {
    const title = HTMLAttributes.title || HTMLAttributes['data-toggle-title'] || 'Details'
    return [
      'details',
      mergeAttributes(HTMLAttributes, { 'data-toggle-section': '' }),
      ['summary', title],
      ['div', { 'data-toggle-content': '' }, 0],
    ]
  },

  addNodeView() {
    return ReactNodeViewRenderer(ToggleSectionNodeView)
  },

  addCommands() {
    return {
      setToggleSection:
        (attrs) =>
        ({ commands }) =>
          commands.insertContent({
            type: this.name,
            attrs: { title: attrs?.title || 'Details', open: attrs?.open ?? true },
            content: [{ type: 'paragraph' }],
          }),
    }
  },
})
