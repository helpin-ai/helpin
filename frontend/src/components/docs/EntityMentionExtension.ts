import { Node, mergeAttributes } from '@tiptap/core'
import { ReactNodeViewRenderer } from '@tiptap/react'
import { EntityMentionNodeView } from './EntityMentionNodeView'
import type { DocsEntitySearchType } from './entitySearch'

export interface EntityMentionAttrs {
  entityType: DocsEntitySearchType
  entityId: string
  label: string
  displayId?: string | number | null
  href?: string | null
  access?: string | null
}

export interface EntityMentionOptions {
  workspaceId?: string
  workspaceSlug?: string
}

declare module '@tiptap/core' {
  interface Commands<ReturnType> {
    entityMention: {
      setEntityMention: (attrs: EntityMentionAttrs) => ReturnType
    }
  }
}

export const EntityMentionExtension = Node.create<EntityMentionOptions>({
  name: 'entityMention',
  group: 'inline',
  inline: true,
  atom: true,
  selectable: false,

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
      label: { default: 'Linked entity' },
      displayId: { default: null },
      href: { default: null },
      access: { default: null },
    }
  },

  parseHTML() {
    return [
      {
        tag: 'span[data-entity-mention]',
        getAttrs: (el) => {
          const dom = el as HTMLElement
          return {
            entityType: dom.getAttribute('data-entity-type') || 'task',
            entityId: dom.getAttribute('data-entity-id'),
            label: dom.getAttribute('data-entity-label') || dom.textContent?.replace(/^@/, '') || 'Linked entity',
            displayId: dom.getAttribute('data-entity-display-id'),
            href: dom.getAttribute('data-entity-href'),
            access: dom.getAttribute('data-entity-access'),
          }
        },
      },
    ]
  },

  renderHTML({ HTMLAttributes }) {
    const label = HTMLAttributes.label || 'Linked entity'
    return [
      'span',
      mergeAttributes(HTMLAttributes, {
        'data-entity-mention': '',
        'data-entity-type': HTMLAttributes.entityType,
        'data-entity-id': HTMLAttributes.entityId,
        'data-entity-label': label,
        'data-entity-display-id': HTMLAttributes.displayId,
        'data-entity-href': HTMLAttributes.href,
        'data-entity-access': HTMLAttributes.access,
      }),
      `@${label}`,
    ]
  },

  addNodeView() {
    return ReactNodeViewRenderer(EntityMentionNodeView)
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
          const label = attrs.label || 'Linked entity'
          const entityType = attrs.entityType || 'task'
          const entityId = attrs.entityId || ''
          const href = attrs.href || `helpin://${entityType}/${entityId}`
          const access = attrs.access || ''
          if ((this as any).editor?.storage?.markdown?.options?.html) {
            state.write(
              `<span data-entity-mention data-entity-type="${entityType}" data-entity-id="${entityId}" data-entity-label="${label}" data-entity-href="${href}" data-entity-access="${access}">@${label}</span>`,
            )
          } else {
            state.write(`[@${label}](${href})`)
          }
        },
        parse: {},
      },
    }
  },

  addCommands() {
    return {
      setEntityMention:
        (attrs) =>
        ({ commands }) =>
          commands.insertContent({ type: this.name, attrs }),
    }
  },
})
