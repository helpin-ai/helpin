import { Extension } from '@tiptap/core'
import { Plugin, PluginKey } from '@tiptap/pm/state'
import { Decoration, DecorationSet } from '@tiptap/pm/view'
import type { EditorState, Transaction } from '@tiptap/pm/state'

export const searchReplacePluginKey = new PluginKey('searchReplace')

export interface SearchReplaceState {
  searchTerm: string
  replaceTerm: string
  results: { from: number; to: number }[]
  currentIndex: number
}

function findMatches(doc: EditorState['doc'], searchTerm: string): { from: number; to: number }[] {
  if (!searchTerm) return []

  const results: { from: number; to: number }[] = []
  const term = searchTerm.toLowerCase()

  doc.descendants((node, pos) => {
    if (!node.isText || !node.text) return
    const text = node.text.toLowerCase()
    let index = text.indexOf(term)
    while (index !== -1) {
      results.push({ from: pos + index, to: pos + index + searchTerm.length })
      index = text.indexOf(term, index + 1)
    }
  })

  return results
}

function buildDecorations(state: EditorState, pluginState: SearchReplaceState): DecorationSet {
  const { results, currentIndex } = pluginState
  if (results.length === 0) return DecorationSet.empty

  const decorations = results.map((result, i) =>
    Decoration.inline(result.from, result.to, {
      class: i === currentIndex ? 'search-match-current' : 'search-match',
    }),
  )

  return DecorationSet.create(state.doc, decorations)
}

type SearchReplaceMeta =
  | { type: 'setSearchTerm'; value: string }
  | { type: 'setReplaceTerm'; value: string }
  | { type: 'nextResult' }
  | { type: 'prevResult' }
  | { type: 'clearSearch' }

export const SearchReplaceExtension = Extension.create({
  name: 'searchReplace',

  addProseMirrorPlugins() {
    const plugin: Plugin<SearchReplaceState> = new Plugin<SearchReplaceState>({
      key: searchReplacePluginKey,

      state: {
        init(): SearchReplaceState {
          return { searchTerm: '', replaceTerm: '', results: [], currentIndex: 0 }
        },

        apply(tr: Transaction, prev: SearchReplaceState): SearchReplaceState {
          const meta = tr.getMeta(searchReplacePluginKey) as SearchReplaceMeta | undefined
          if (!meta) {
            // Doc changed — recompute matches if we have a search term.
            if (tr.docChanged && prev.searchTerm) {
              const results = findMatches(tr.doc, prev.searchTerm)
              const currentIndex = Math.min(prev.currentIndex, Math.max(results.length - 1, 0))
              return { ...prev, results, currentIndex }
            }
            return prev
          }

          switch (meta.type) {
            case 'setSearchTerm': {
              const searchTerm = meta.value
              const results = findMatches(tr.doc, searchTerm)
              return { ...prev, searchTerm, results, currentIndex: 0 }
            }
            case 'setReplaceTerm':
              return { ...prev, replaceTerm: meta.value }
            case 'nextResult': {
              if (prev.results.length === 0) return prev
              return { ...prev, currentIndex: (prev.currentIndex + 1) % prev.results.length }
            }
            case 'prevResult': {
              if (prev.results.length === 0) return prev
              return {
                ...prev,
                currentIndex: (prev.currentIndex - 1 + prev.results.length) % prev.results.length,
              }
            }
            case 'clearSearch':
              return { searchTerm: '', replaceTerm: '', results: [], currentIndex: 0 }
          }
        },
      },

      props: {
        decorations(state): DecorationSet {
          const pluginState = plugin.getState(state)
          if (!pluginState) return DecorationSet.empty
          return buildDecorations(state, pluginState)
        },
      },
    })

    return [plugin]
  },

  addCommands() {
    return {
      setSearchTerm:
        (value: string) =>
        ({ tr, dispatch }) => {
          if (dispatch) {
            tr.setMeta(searchReplacePluginKey, { type: 'setSearchTerm', value })
            dispatch(tr)
          }
          return true
        },

      setReplaceTerm:
        (value: string) =>
        ({ tr, dispatch }) => {
          if (dispatch) {
            tr.setMeta(searchReplacePluginKey, { type: 'setReplaceTerm', value })
            dispatch(tr)
          }
          return true
        },

      nextSearchResult:
        () =>
        ({ tr, dispatch, state }) => {
          if (dispatch) {
            tr.setMeta(searchReplacePluginKey, { type: 'nextResult' })
            dispatch(tr)
          }
          // Scroll to the next match after dispatching.
          const pluginState = searchReplacePluginKey.getState(state) as SearchReplaceState | undefined
          if (pluginState && pluginState.results.length > 0) {
            const nextIndex = (pluginState.currentIndex + 1) % pluginState.results.length
            const match = pluginState.results[nextIndex]
            if (match) {
              // Use requestAnimationFrame to scroll after state update.
              requestAnimationFrame(() => {
                this.editor?.commands.focus()
                this.editor?.commands.setTextSelection(match.from)
                this.editor?.view.dom
                  .querySelector('.search-match-current')
                  ?.scrollIntoView({ block: 'center', behavior: 'smooth' })
              })
            }
          }
          return true
        },

      prevSearchResult:
        () =>
        ({ tr, dispatch, state }) => {
          if (dispatch) {
            tr.setMeta(searchReplacePluginKey, { type: 'prevResult' })
            dispatch(tr)
          }
          const pluginState = searchReplacePluginKey.getState(state) as SearchReplaceState | undefined
          if (pluginState && pluginState.results.length > 0) {
            const prevIndex =
              (pluginState.currentIndex - 1 + pluginState.results.length) % pluginState.results.length
            const match = pluginState.results[prevIndex]
            if (match) {
              requestAnimationFrame(() => {
                this.editor?.commands.focus()
                this.editor?.commands.setTextSelection(match.from)
                this.editor?.view.dom
                  .querySelector('.search-match-current')
                  ?.scrollIntoView({ block: 'center', behavior: 'smooth' })
              })
            }
          }
          return true
        },

      replaceCurrent:
        () =>
        ({ state, dispatch, tr }) => {
          const pluginState = searchReplacePluginKey.getState(state) as SearchReplaceState | undefined
          if (!pluginState || pluginState.results.length === 0) return false
          const match = pluginState.results[pluginState.currentIndex]
          if (!match) return false

          if (dispatch) {
            tr.insertText(pluginState.replaceTerm, match.from, match.to)
            dispatch(tr)
          }
          return true
        },

      replaceAll:
        () =>
        ({ state, dispatch, tr }) => {
          const pluginState = searchReplacePluginKey.getState(state) as SearchReplaceState | undefined
          if (!pluginState || pluginState.results.length === 0) return false

          if (dispatch) {
            // Replace in reverse order to preserve positions.
            const sorted = [...pluginState.results].sort((a, b) => b.from - a.from)
            for (const match of sorted) {
              tr.insertText(pluginState.replaceTerm, match.from, match.to)
            }
            dispatch(tr)
          }
          return true
        },

      clearSearch:
        () =>
        ({ tr, dispatch }) => {
          if (dispatch) {
            tr.setMeta(searchReplacePluginKey, { type: 'clearSearch' })
            dispatch(tr)
          }
          return true
        },
    }
  },

  addKeyboardShortcuts() {
    return {
      'Mod-f': () => {
        // Handled by DocsEditor to toggle the search bar.
        return false
      },
      'Mod-h': () => {
        // Handled by DocsEditor to toggle the search bar with replace.
        return false
      },
    }
  },
})

// Augment the TipTap Commands interface so editor.commands.* is typed.
declare module '@tiptap/core' {
  interface Commands<ReturnType> {
    searchReplace: {
      setSearchTerm: (value: string) => ReturnType
      setReplaceTerm: (value: string) => ReturnType
      nextSearchResult: () => ReturnType
      prevSearchResult: () => ReturnType
      replaceCurrent: () => ReturnType
      replaceAll: () => ReturnType
      clearSearch: () => ReturnType
    }
  }
}
