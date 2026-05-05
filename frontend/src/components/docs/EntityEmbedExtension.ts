import { Node, mergeAttributes } from '@tiptap/core'
import { ReactNodeViewRenderer } from '@tiptap/react'
import { EntityEmbedNodeView } from './EntityEmbedNodeView'
import { pickBlockNodeViewAttrs } from './nodeViewAttrs'

export type DocsEntityEmbedType = 'reference' | 'task' | 'story' | 'epic' | 'support_conversation' | 'deal' | 'contact' | 'company'

export interface EntityEmbedAttrs {
  entityType: DocsEntityEmbedType
  entityId: string
  title?: string | null
  displayId?: string | number | null
  status?: string | null
  access?: string | null
}

export interface EntityEmbedOptions {
  workspaceId?: string
  workspaceSlug?: string
}

declare module '@tiptap/core' {
  interface Commands<ReturnType> {
    entityEmbed: {
      setEntityEmbed: (attrs: EntityEmbedAttrs) => ReturnType
    }
  }
}

export const EntityEmbedExtension = Node.create<EntityEmbedOptions>({
  name: 'entityEmbed',
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
      entityType: { default: 'task' },
      entityId: { default: null },
      title: { default: null },
      displayId: { default: null },
      status: { default: null },
      access: { default: null },
    }
  },

  parseHTML() {
    return [
      {
        tag: 'div[data-entity-embed]',
        getAttrs: (el) => {
          const dom = el as HTMLElement
          return {
            entityType: dom.getAttribute('data-entity-type') || 'task',
            entityId: dom.getAttribute('data-entity-id'),
            title: dom.getAttribute('data-entity-title'),
            displayId: dom.getAttribute('data-entity-display-id'),
            status: dom.getAttribute('data-entity-status'),
            access: dom.getAttribute('data-entity-access'),
          }
        },
      },
    ]
  },

  renderHTML({ HTMLAttributes }) {
    const title = HTMLAttributes.title || 'Linked entity'
    return [
      'div',
      mergeAttributes(HTMLAttributes, {
        'data-entity-embed': '',
        'data-entity-type': HTMLAttributes.entityType,
        'data-entity-id': HTMLAttributes.entityId,
        'data-entity-title': title,
        'data-entity-display-id': HTMLAttributes.displayId,
        'data-entity-status': HTMLAttributes.status,
        'data-entity-access': HTMLAttributes.access,
      }),
      title,
    ]
  },

  addNodeView() {
    return ReactNodeViewRenderer(EntityEmbedNodeView, { attrs: ({ HTMLAttributes }) => pickBlockNodeViewAttrs(HTMLAttributes) })
  },

  addStorage() {
    return {
      markdown: {
        serialize(
          this: { editor?: { storage?: { markdown?: { options?: { html?: boolean } } } } },
          state: any,
          node: any,
        ) {
          const attrs = node.attrs || {}
          const title = attrs.title || 'Linked entity'
          const entityType = attrs.entityType || 'task'
          const entityId = attrs.entityId || ''
          const displayId = attrs.displayId || ''
          const status = attrs.status || ''
          const access = attrs.access || ''
          if ((this as any).editor?.storage?.markdown?.options?.html) {
            state.write(
              `<div data-entity-embed data-entity-type="${entityType}" data-entity-id="${entityId}" data-entity-title="${title}" data-entity-display-id="${displayId}" data-entity-status="${status}" data-entity-access="${access}">${title}</div>`,
            )
            state.closeBlock(node)
          } else {
            state.write(`[${title}](helpin://${entityType}/${entityId})\n\n`)
          }
        },
        parse: {},
      },
    }
  },

  addCommands() {
    return {
      setEntityEmbed:
        (attrs) =>
        ({ commands }) =>
          commands.insertContent({ type: this.name, attrs }),
    }
  },
})
