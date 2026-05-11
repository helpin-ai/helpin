import { Node, mergeAttributes } from '@tiptap/core'
import { ReactNodeViewRenderer } from '@tiptap/react'
import { TableOfContentsNodeView } from './TableOfContentsNodeView'
import { pickBlockNodeViewAttrs } from './nodeViewAttrs'

declare module '@tiptap/core' {
  interface Commands<ReturnType> {
    tableOfContents: {
      setTableOfContents: () => ReturnType
    }
  }
}

export const TableOfContentsExtension = Node.create({
  name: 'tableOfContents',
  group: 'block',
  atom: true,
  draggable: true,

  parseHTML() {
    return [{ tag: 'nav[data-docs-toc]' }]
  },

  renderHTML({ HTMLAttributes }) {
    return ['nav', mergeAttributes(HTMLAttributes, { 'data-docs-toc': '' }), 'Table of contents']
  },

  addNodeView() {
    return ReactNodeViewRenderer(TableOfContentsNodeView, { attrs: ({ HTMLAttributes }) => pickBlockNodeViewAttrs(HTMLAttributes) })
  },

  addCommands() {
    return {
      setTableOfContents:
        () =>
        ({ commands }) =>
          commands.insertContent({ type: this.name }),
    }
  },
})
