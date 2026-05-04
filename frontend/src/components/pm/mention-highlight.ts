import { Extension } from '@tiptap/core'
import { Plugin, PluginKey } from '@tiptap/pm/state'
import { Decoration, DecorationSet } from '@tiptap/pm/view'
import type { Node as PMNode } from '@tiptap/pm/model'

export interface MentionHighlightOptions {
  validHandles: Set<string> | (() => Set<string>)
}

const mentionHighlightKey = new PluginKey('mentionHighlight')

/**
 * TipTap extension that highlights @mention patterns in blue
 * using ProseMirror decorations (purely visual, no schema change).
 *
 * Only handles present in `validHandles` are decorated, so things like
 * @tailwindcss or @import in code/CSS snippets stay plain text.
 */
export const MentionHighlight = Extension.create<MentionHighlightOptions>({
  name: 'mentionHighlight',

  addOptions() {
    return {
      validHandles: new Set<string>(),
    }
  },

  addProseMirrorPlugins() {
    const resolveValidHandles = (): Set<string> => {
      const opt = this.options.validHandles
      return typeof opt === 'function' ? opt() : opt
    }
    return [
      new Plugin({
        key: mentionHighlightKey,
        state: {
          init(_, { doc }) {
            return buildDecorations(doc, resolveValidHandles())
          },
          apply(tr, oldSet, _oldState, newState) {
            const refresh = tr.getMeta(mentionHighlightKey)
            if (tr.docChanged || refresh) {
              return buildDecorations(newState.doc, resolveValidHandles())
            }
            return oldSet
          },
        },
        props: {
          decorations(state) {
            return this.getState(state)
          },
        },
      }),
    ]
  },
})

export const mentionHighlightPluginKey = mentionHighlightKey

function buildDecorations(doc: PMNode, validHandles: Set<string>): DecorationSet {
  if (validHandles.size === 0) {
    return DecorationSet.empty
  }
  const decorations: Decoration[] = []
  const regex = /(?:^|\s)(@[a-z0-9._-]+)/gi

  doc.descendants((node, pos) => {
    if (!node.isText || !node.text) return

    let match: RegExpExecArray | null
    regex.lastIndex = 0

    while ((match = regex.exec(node.text)) !== null) {
      const handle = match[1].slice(1).toLowerCase()
      if (!validHandles.has(handle)) continue

      const mentionStart = match.index + match[0].indexOf('@')
      const from = pos + mentionStart
      const to = pos + match.index + match[0].length

      decorations.push(
        Decoration.inline(from, to, {
          class: 'mention-highlight',
        }),
      )
    }
  })

  return DecorationSet.create(doc, decorations)
}
