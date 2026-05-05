import { Node, mergeAttributes } from '@tiptap/core'
import { ReactNodeViewRenderer } from '@tiptap/react'
import { ToggleSectionNodeView } from './ToggleSectionNodeView'

export interface ToggleSectionAttrs {
  title?: string | null
  open?: boolean
  icon?: string | null
  badgeText?: string | null
  sourceStyle?: string | null
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
      icon: {
        default: null,
        parseHTML: (el) => (el as HTMLElement).getAttribute('data-toggle-icon'),
        renderHTML: (attrs) => (attrs.icon ? { 'data-toggle-icon': attrs.icon } : {}),
      },
      badgeText: {
        default: null,
        parseHTML: (el) => (el as HTMLElement).getAttribute('data-toggle-badge'),
        renderHTML: (attrs) => (attrs.badgeText ? { 'data-toggle-badge': attrs.badgeText } : {}),
      },
      sourceStyle: {
        default: null,
        parseHTML: (el) => (el as HTMLElement).getAttribute('data-toggle-style'),
        renderHTML: (attrs) => (attrs.sourceStyle ? { 'data-toggle-style': attrs.sourceStyle } : {}),
      },
    }
  },

  parseHTML() {
    return [{ tag: 'details[data-toggle-section]' }]
  },

  renderHTML({ HTMLAttributes }) {
    const title = HTMLAttributes.title || HTMLAttributes['data-toggle-title'] || 'Details'
    const icon = HTMLAttributes.icon || HTMLAttributes['data-toggle-icon']
    const badgeText = HTMLAttributes.badgeText || HTMLAttributes['data-toggle-badge']
    const summary = icon || badgeText || HTMLAttributes.sourceStyle
      ? [
          'summary',
          ...(icon ? [['span', { class: 'docs-toggle-icon' }, icon]] : []),
          ['span', { class: 'docs-toggle-title' }, title],
          ...(badgeText ? [['span', { class: 'docs-toggle-badge' }, badgeText]] : []),
        ]
      : ['summary', title]
    return [
      'details',
      mergeAttributes(HTMLAttributes, { 'data-toggle-section': '' }),
      summary,
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
