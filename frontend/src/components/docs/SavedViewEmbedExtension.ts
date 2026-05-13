import { Node, mergeAttributes } from '@tiptap/core'
import { ReactNodeViewRenderer } from '@tiptap/react'
import { SavedViewEmbedNodeView } from './SavedViewEmbedNodeView'
import { pickBlockNodeViewAttrs } from './nodeViewAttrs'

export interface SavedViewEmbedAttrs {
  module?: 'pm' | 'crm' | 'support'
  viewId?: string | null
  viewName?: string | null
}

export interface SavedViewEmbedOptions {
  workspaceId?: string
  workspaceSlug?: string
}

declare module '@tiptap/core' {
  interface Commands<ReturnType> {
    savedViewEmbed: {
      setSavedViewEmbed: (attrs?: SavedViewEmbedAttrs) => ReturnType
    }
  }
}

export const SavedViewEmbedExtension = Node.create<SavedViewEmbedOptions>({
  name: 'savedViewEmbed',
  group: 'block',
  atom: true,
  draggable: true,

  addOptions() {
    return {
      workspaceId: undefined,
      workspaceSlug: undefined,
    }
  },

  addAttributes() {
    return {
      module: {
        default: 'pm',
        parseHTML: (el) => (el as HTMLElement).getAttribute('data-saved-view-module') || 'pm',
        renderHTML: (attrs) => ({ 'data-saved-view-module': attrs.module || 'pm' }),
      },
      viewId: {
        default: null,
        parseHTML: (el) => (el as HTMLElement).getAttribute('data-saved-view-id'),
        renderHTML: (attrs) => (attrs.viewId ? { 'data-saved-view-id': attrs.viewId } : {}),
      },
      viewName: {
        default: null,
        parseHTML: (el) => (el as HTMLElement).getAttribute('data-saved-view-name'),
        renderHTML: (attrs) => (attrs.viewName ? { 'data-saved-view-name': attrs.viewName } : {}),
      },
    }
  },

  parseHTML() {
    return [{ tag: 'section[data-saved-view-embed]' }]
  },

  renderHTML({ HTMLAttributes }) {
    return ['section', mergeAttributes(HTMLAttributes, { 'data-saved-view-embed': '' }), HTMLAttributes.viewName || 'Saved view']
  },

  addNodeView() {
    return ReactNodeViewRenderer(SavedViewEmbedNodeView, { attrs: ({ HTMLAttributes }) => pickBlockNodeViewAttrs(HTMLAttributes) })
  },

  addCommands() {
    return {
      setSavedViewEmbed:
        (attrs) =>
        ({ commands }) =>
          commands.insertContent({
            type: this.name,
            attrs: {
              module: attrs?.module || 'pm',
              viewId: attrs?.viewId || null,
              viewName: attrs?.viewName || null,
            },
          }),
    }
  },

  addStorage() {
    return {
      markdown: {
        serialize(state: any, node: any) {
          const name = node.attrs?.viewName || node.attrs?.viewId || 'Saved view'
          state.write(`> Saved view: ${name}`)
          state.closeBlock(node)
        },
        parse: {},
      },
    }
  },
})
