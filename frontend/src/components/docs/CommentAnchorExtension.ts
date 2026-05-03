import { Extension } from '@tiptap/core'
import { Plugin, PluginKey } from '@tiptap/pm/state'
import { Decoration, DecorationSet } from '@tiptap/pm/view'
import type { EditorState, Transaction } from '@tiptap/pm/state'

export interface DocsCommentDecorationAnchor {
  id: string
  block_id?: string
  range?: Record<string, unknown>
  anchor_text?: string
}

interface CommentAnchorState {
  anchors: DocsCommentDecorationAnchor[]
}

interface CommentAnchorOptions {
  onOpenComment?: (commentId: string) => void
}

type CommentAnchorMeta =
  | { type: 'setAnchors'; anchors: DocsCommentDecorationAnchor[] }
  | { type: 'clearAnchors' }

export const commentAnchorPluginKey = new PluginKey<CommentAnchorState>('commentAnchors')

function rangeFromAnchor(anchor: DocsCommentDecorationAnchor) {
  const from = typeof anchor.range?.from === 'number' ? anchor.range.from : null
  const to = typeof anchor.range?.to === 'number' ? anchor.range.to : null
  if (from === null || to === null || to <= from) return null
  return { from, to }
}

function buildDecorations(state: EditorState, anchors: DocsCommentDecorationAnchor[]): DecorationSet {
  const decorations: Decoration[] = []
  const maxPos = state.doc.content.size

  for (const anchor of anchors) {
    const className = 'docs-comment-highlight'
    const attrs = {
      class: className,
      'data-doc-comment-id': anchor.id,
    }
    const range = rangeFromAnchor(anchor)
    if (range && range.from >= 0 && range.to <= maxPos) {
      decorations.push(Decoration.inline(range.from, range.to, attrs))
      continue
    }

    const anchorText = anchor.anchor_text?.trim()
    if (anchorText) {
      let found = false
      state.doc.descendants((node, pos) => {
        if (found || !node.isText || !node.text) return false
        const index = node.text.indexOf(anchorText)
        if (index === -1) return
        decorations.push(Decoration.inline(pos + index, pos + index + anchorText.length, attrs))
        found = true
        return false
      })
      if (found) continue
    }

    const blockId = anchor.block_id?.trim()
    if (!blockId) continue
    state.doc.descendants((node, pos) => {
      if (node.attrs?.blockId !== blockId) return
      decorations.push(
        Decoration.node(pos, pos + node.nodeSize, {
          class: 'docs-comment-block-highlight',
          'data-doc-comment-id': anchor.id,
        }),
      )
      return false
    })
  }

  return DecorationSet.create(state.doc, decorations)
}

export const CommentAnchorExtension = Extension.create<CommentAnchorOptions>({
  name: 'commentAnchors',

  addOptions() {
    return {
      onOpenComment: undefined,
    }
  },

  addProseMirrorPlugins() {
    const onOpenComment = this.options.onOpenComment

    const plugin = new Plugin<CommentAnchorState>({
      key: commentAnchorPluginKey,
      state: {
        init(): CommentAnchorState {
          return { anchors: [] }
        },
        apply(tr: Transaction, prev: CommentAnchorState): CommentAnchorState {
          const meta = tr.getMeta(commentAnchorPluginKey) as CommentAnchorMeta | undefined
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
        handleClick(_view, _pos, event) {
          const target = event.target instanceof HTMLElement ? event.target : null
          const anchor = target?.closest('[data-doc-comment-id]') as HTMLElement | null
          const commentId = anchor?.dataset.docCommentId
          if (!commentId) return false
          onOpenComment?.(commentId)
          return true
        },
      },
    })

    return [plugin]
  },

  addCommands() {
    return {
      setCommentAnchors:
        (anchors: DocsCommentDecorationAnchor[]) =>
        ({ tr, dispatch }) => {
          if (dispatch) {
            tr.setMeta(commentAnchorPluginKey, { type: 'setAnchors', anchors })
            dispatch(tr)
          }
          return true
        },
      clearCommentAnchors:
        () =>
        ({ tr, dispatch }) => {
          if (dispatch) {
            tr.setMeta(commentAnchorPluginKey, { type: 'clearAnchors' })
            dispatch(tr)
          }
          return true
        },
    }
  },
})

declare module '@tiptap/core' {
  interface Commands<ReturnType> {
    commentAnchors: {
      setCommentAnchors: (anchors: DocsCommentDecorationAnchor[]) => ReturnType
      clearCommentAnchors: () => ReturnType
    }
  }
}
