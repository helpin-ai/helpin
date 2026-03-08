import { Extension } from '@tiptap/core'
import { Plugin, PluginKey } from '@tiptap/pm/state'
import { Decoration, DecorationSet } from '@tiptap/pm/view'
import type { Node as PMNode } from '@tiptap/pm/model'

const mentionHighlightKey = new PluginKey('mentionHighlight')

/**
 * TipTap extension that highlights @mention patterns in blue
 * using ProseMirror decorations (purely visual, no schema change).
 */
export const MentionHighlight = Extension.create({
  name: 'mentionHighlight',

  addProseMirrorPlugins() {
    return [
      new Plugin({
        key: mentionHighlightKey,
        state: {
          init(_, { doc }) {
            return buildDecorations(doc)
          },
          apply(tr, oldSet) {
            if (tr.docChanged) {
              return buildDecorations(tr.doc)
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

function buildDecorations(doc: PMNode): DecorationSet {
  const decorations: Decoration[] = []
  const regex = /(?:^|\s)(@[a-z0-9._-]+)/gi

  doc.descendants((node, pos) => {
    if (!node.isText || !node.text) return

    let match: RegExpExecArray | null
    regex.lastIndex = 0

    while ((match = regex.exec(node.text)) !== null) {
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
