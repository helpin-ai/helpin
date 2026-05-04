import { Node, mergeAttributes } from '@tiptap/core'
import { ReactNodeViewRenderer } from '@tiptap/react'
import { CitationBlockNodeView } from './CitationBlockNodeView'

export type CitationSourceType = 'docs_chunk' | 'support_conversation'
export type CitationAccess = 'granted' | 'redacted' | 'unknown'

export interface CitationSourceRef {
  sourceType: CitationSourceType
  sourceId: string
  title?: string | null
  excerpt?: string | null
  url?: string | null
  confidence?: number | null
  access?: CitationAccess
  documentId?: string | null
  blockId?: string | null
  chunkIndex?: number | null
  conversationId?: string | null
}

export interface CitationBlockAttrs {
  title?: string | null
  sources?: CitationSourceRef[]
}

export interface CitationBlockOptions {
  workspaceSlug?: string
}

declare module '@tiptap/core' {
  interface Commands<ReturnType> {
    citationBlock: {
      setCitationBlock: (attrs?: CitationBlockAttrs) => ReturnType
    }
  }
}

export const CitationBlockExtension = Node.create<CitationBlockOptions>({
  name: 'citationBlock',
  group: 'block',
  atom: true,
  draggable: true,

  addOptions() {
    return {
      workspaceSlug: undefined,
    }
  },

  addAttributes() {
    return {
      title: {
        default: 'Sources',
        parseHTML: (el) => (el as HTMLElement).getAttribute('data-citation-title') || 'Sources',
        renderHTML: (attrs) => ({ 'data-citation-title': attrs.title || 'Sources' }),
      },
      sources: {
        default: [],
        parseHTML: (el) => {
          const raw = (el as HTMLElement).getAttribute('data-citation-sources')
          if (!raw) return []
          try {
            const parsed = JSON.parse(raw)
            return Array.isArray(parsed) ? parsed : []
          } catch {
            return []
          }
        },
        renderHTML: (attrs) => ({
          'data-citation-sources': JSON.stringify(Array.isArray(attrs.sources) ? attrs.sources : []),
        }),
      },
    }
  },

  parseHTML() {
    return [{ tag: 'section[data-citation-block]' }]
  },

  renderHTML({ HTMLAttributes }) {
    const title = HTMLAttributes.title || 'Sources'
    return [
      'section',
      mergeAttributes(HTMLAttributes, { 'data-citation-block': '' }),
      title,
    ]
  },

  addNodeView() {
    return ReactNodeViewRenderer(CitationBlockNodeView)
  },

  addCommands() {
    return {
      setCitationBlock:
        (attrs) =>
        ({ commands }) =>
          commands.insertContent({
            type: this.name,
            attrs: {
              title: attrs?.title || 'Sources',
              sources: Array.isArray(attrs?.sources) ? attrs?.sources : [],
            },
          }),
    }
  },

  addStorage() {
    return {
      markdown: {
        serialize(state: any, node: any) {
          const title = node.attrs?.title || 'Sources'
          const sources = Array.isArray(node.attrs?.sources) ? node.attrs.sources : []
          state.write(`### ${title}`)
          state.ensureNewLine()
          if (sources.length === 0) {
            state.write('- No sources attached')
            state.ensureNewLine()
            state.closeBlock(node)
            return
          }
          for (const source of sources) {
            const label = source?.title || source?.sourceId || 'Source'
            const url = source?.url || ''
            state.write(url ? `- [${label}](${url})` : `- ${label}`)
            if (source?.excerpt) state.write(`: ${source.excerpt}`)
            state.ensureNewLine()
          }
          state.closeBlock(node)
        },
        parse: {},
      },
    }
  },
})
