import { Extension } from '@tiptap/core'
import { Plugin } from '@tiptap/pm/state'

const ADDRESSABLE_BLOCK_TYPES = new Set([
  'paragraph',
  'heading',
  'bulletList',
  'orderedList',
  'taskList',
  'blockquote',
  'callout',
  'codeBlock',
  'table',
  'resizableImage',
  'image',
  'videoEmbed',
  'htmlBlock',
  'aiSection',
  'citationBlock',
  'entityEmbed',
  'savedViewEmbed',
  'toggleSection',
  'fileAttachment',
  'tableOfContents',
  'richEmbed',
  'horizontalRule',
])

function newBlockId(): string {
  if (typeof crypto !== 'undefined' && 'randomUUID' in crypto) {
    return crypto.randomUUID()
  }
  const hex = '0123456789abcdef'
  const chars = Array.from({ length: 36 }, (_, index) => {
    if ([8, 13, 18, 23].includes(index)) return '-'
    if (index === 14) return '4'
    if (index === 19) return hex[8 + Math.floor(Math.random() * 4)]
    return hex[Math.floor(Math.random() * 16)]
  })
  return chars.join('')
}

export const BlockIdExtension = Extension.create({
  name: 'blockId',

  addGlobalAttributes() {
    return [
      {
        types: Array.from(ADDRESSABLE_BLOCK_TYPES),
        attributes: {
          blockId: {
            default: null,
            parseHTML: (element) => element.getAttribute('data-block-id'),
            renderHTML: (attributes) => {
              if (!attributes.blockId) return {}
              return { 'data-block-id': attributes.blockId }
            },
          },
          staleState: {
            default: null,
            parseHTML: (element) => element.getAttribute('data-docs-stale-state'),
            renderHTML: (attributes) => {
              if (!attributes.staleState) return {}
              return {
                'data-docs-stale': 'true',
                'data-docs-stale-state': attributes.staleState,
              }
            },
          },
          staleReason: {
            default: null,
            parseHTML: (element) => element.getAttribute('data-docs-stale-reason'),
            renderHTML: (attributes) => {
              if (!attributes.staleReason) return {}
              return { 'data-docs-stale-reason': attributes.staleReason }
            },
          },
          staleSource: {
            default: null,
            parseHTML: (element) => element.getAttribute('data-docs-stale-source'),
            renderHTML: (attributes) => {
              if (!attributes.staleSource) return {}
              return { 'data-docs-stale-source': attributes.staleSource }
            },
          },
          staleGapId: {
            default: null,
            parseHTML: (element) => element.getAttribute('data-docs-stale-gap-id'),
            renderHTML: (attributes) => {
              if (!attributes.staleGapId) return {}
              return { 'data-docs-stale-gap-id': attributes.staleGapId }
            },
          },
          staleMarkedAt: {
            default: null,
            parseHTML: (element) => element.getAttribute('data-docs-stale-marked-at'),
            renderHTML: (attributes) => {
              if (!attributes.staleMarkedAt) return {}
              return { 'data-docs-stale-marked-at': attributes.staleMarkedAt }
            },
          },
        },
      },
      {
        types: ['heading'],
        attributes: {
          id: {
            default: null,
            parseHTML: (element) => element.getAttribute('id'),
            renderHTML: (attributes) => {
              if (!attributes.id) return {}
              return { id: attributes.id }
            },
          },
          anchorAliases: {
            default: null,
            parseHTML: () => null,
            renderHTML: () => ({}),
          },
        },
      },
    ]
  },

  addProseMirrorPlugins() {
    return [
      new Plugin({
        appendTransaction: (_transactions, _oldState, newState) => {
          const tr = newState.tr
          const seen = new Set<string>()
          let changed = false

          newState.doc.forEach((node, offset) => {
            if (!ADDRESSABLE_BLOCK_TYPES.has(node.type.name)) return
            const current = typeof node.attrs.blockId === 'string' ? node.attrs.blockId.trim() : ''
            const duplicate = current !== '' && seen.has(current)
            const blockId = current === '' || duplicate ? newBlockId() : current
            seen.add(blockId)
            if (blockId !== current) {
              tr.setNodeMarkup(offset, undefined, { ...node.attrs, blockId })
              changed = true
            }
          })

          return changed ? tr : null
        },
      }),
    ]
  },
})
