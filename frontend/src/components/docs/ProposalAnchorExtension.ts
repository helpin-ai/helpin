import { Extension } from '@tiptap/core'
import { Plugin, PluginKey } from '@tiptap/pm/state'
import { Decoration, DecorationSet } from '@tiptap/pm/view'
import type { EditorState, Transaction } from '@tiptap/pm/state'

/**
 * Anchors a pending change proposal to the block it targets. React owns the
 * card DOM (so it can render a portal into it); this plugin only places that
 * element in the document flow as a widget directly after the block, and
 * highlights the block itself.
 */
export interface DocsProposalAnchor {
  id: string
  blockId: string
  dom: HTMLElement
}

interface ProposalAnchorState {
  anchors: DocsProposalAnchor[]
}

type ProposalAnchorMeta =
  | { type: 'setAnchors'; anchors: DocsProposalAnchor[] }
  | { type: 'clearAnchors' }

export const proposalAnchorPluginKey = new PluginKey<ProposalAnchorState>('docsProposalAnchors')

function buildDecorations(state: EditorState, anchors: DocsProposalAnchor[]): DecorationSet {
  if (anchors.length === 0) return DecorationSet.empty
  const byBlockId = new Map<string, DocsProposalAnchor[]>()
  for (const anchor of anchors) {
    const blockId = anchor.blockId?.trim()
    if (!blockId) continue
    const existing = byBlockId.get(blockId)
    if (existing) existing.push(anchor)
    else byBlockId.set(blockId, [anchor])
  }
  if (byBlockId.size === 0) return DecorationSet.empty

  const decorations: Decoration[] = []
  state.doc.descendants((node, pos) => {
    const blockId = typeof node.attrs?.blockId === 'string' ? node.attrs.blockId.trim() : ''
    if (!blockId) return
    const matches = byBlockId.get(blockId)
    if (!matches) return
    decorations.push(
      Decoration.node(pos, pos + node.nodeSize, {
        class: 'docs-proposal-target',
        'data-docs-proposal-target': 'true',
      }),
    )
    matches.forEach((anchor) => {
      decorations.push(
        Decoration.widget(pos + node.nodeSize, () => anchor.dom, {
          key: `docs-proposal-${anchor.id}`,
          side: 1,
          // The card is React-owned UI, not document content: keep ProseMirror
          // from routing its events or placing a selection inside it.
          stopEvent: () => true,
          ignoreSelection: true,
        }),
      )
    })
    return false
  })

  return DecorationSet.create(state.doc, decorations)
}

export const ProposalAnchorExtension = Extension.create({
  name: 'docsProposalAnchors',

  addProseMirrorPlugins() {
    const plugin = new Plugin<ProposalAnchorState>({
      key: proposalAnchorPluginKey,
      state: {
        init(): ProposalAnchorState {
          return { anchors: [] }
        },
        apply(tr: Transaction, prev: ProposalAnchorState): ProposalAnchorState {
          const meta = tr.getMeta(proposalAnchorPluginKey) as ProposalAnchorMeta | undefined
          if (!meta) return prev
          if (meta.type === 'clearAnchors') return { anchors: [] }
          return { anchors: meta.anchors }
        },
      },
      props: {
        decorations(state): DecorationSet {
          const pluginState = plugin.getState(state)
          return buildDecorations(state, pluginState?.anchors ?? [])
        },
      },
    })

    return [plugin]
  },

  addCommands() {
    return {
      setProposalAnchors:
        (anchors: DocsProposalAnchor[]) =>
        ({ tr, dispatch }) => {
          if (dispatch) {
            tr.setMeta(proposalAnchorPluginKey, { type: 'setAnchors', anchors })
            dispatch(tr)
          }
          return true
        },
      clearProposalAnchors:
        () =>
        ({ tr, dispatch }) => {
          if (dispatch) {
            tr.setMeta(proposalAnchorPluginKey, { type: 'clearAnchors' })
            dispatch(tr)
          }
          return true
        },
    }
  },
})

declare module '@tiptap/core' {
  interface Commands<ReturnType> {
    docsProposalAnchors: {
      setProposalAnchors: (anchors: DocsProposalAnchor[]) => ReturnType
      clearProposalAnchors: () => ReturnType
    }
  }
}
