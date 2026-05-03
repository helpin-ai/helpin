import { Node, mergeAttributes } from '@tiptap/core'
import { ReactNodeViewRenderer } from '@tiptap/react'
import { RichEmbedNodeView } from './RichEmbedNodeView'

export interface RichEmbedAttrs {
  url: string
  provider?: string | null
  title?: string | null
  description?: string | null
  image_url?: string | null
}

declare module '@tiptap/core' {
  interface Commands<ReturnType> {
    richEmbed: {
      setRichEmbed: (attrs: RichEmbedAttrs) => ReturnType
    }
  }
}

export const RichEmbedExtension = Node.create({
  name: 'richEmbed',
  group: 'block',
  atom: true,
  draggable: true,

  addAttributes() {
    return {
      url: { default: null, parseHTML: (el) => (el as HTMLElement).getAttribute('data-embed-url'), renderHTML: (attrs) => (attrs.url ? { 'data-embed-url': attrs.url } : {}) },
      provider: { default: null, parseHTML: (el) => (el as HTMLElement).getAttribute('data-embed-provider'), renderHTML: (attrs) => (attrs.provider ? { 'data-embed-provider': attrs.provider } : {}) },
      title: { default: null, parseHTML: (el) => (el as HTMLElement).getAttribute('data-embed-title'), renderHTML: (attrs) => (attrs.title ? { 'data-embed-title': attrs.title } : {}) },
      description: { default: null, parseHTML: (el) => (el as HTMLElement).getAttribute('data-embed-description'), renderHTML: (attrs) => (attrs.description ? { 'data-embed-description': attrs.description } : {}) },
      image_url: { default: null, parseHTML: (el) => (el as HTMLElement).getAttribute('data-embed-image-url'), renderHTML: (attrs) => (attrs.image_url ? { 'data-embed-image-url': attrs.image_url } : {}) },
    }
  },

  parseHTML() {
    return [{ tag: 'a[data-rich-embed]' }]
  },

  renderHTML({ HTMLAttributes }) {
    return ['a', mergeAttributes(HTMLAttributes, { 'data-rich-embed': '', href: HTMLAttributes.url || HTMLAttributes['data-embed-url'] || '#' }), HTMLAttributes.title || HTMLAttributes.url || 'Embed']
  },

  addNodeView() {
    return ReactNodeViewRenderer(RichEmbedNodeView)
  },

  addCommands() {
    return {
      setRichEmbed:
        (attrs) =>
        ({ commands }) =>
          commands.insertContent({ type: this.name, attrs }),
    }
  },
})
