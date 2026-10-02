import { useState } from 'react'
import type { DocsDocument } from '@/lib/docsTypes'

export function useDocsSelection(scope: string, documents: DocsDocument[]) {
  const visible = documents.filter(doc => !doc.is_locked).map(doc => doc.id)
  const visibleKey = [...visible].sort().join(',')
  const [state, setState] = useState({ scope, visibleKey, ids: [] as string[] })
  const [busy, setBusy] = useState(false)
  // Reset immediately on navigation/filter changes and prune disappeared/locked rows.
  if (state.scope !== scope || state.visibleKey !== visibleKey) {
    setState({ scope, visibleKey, ids: state.scope === scope ? state.ids.filter(id => visible.includes(id)) : [] })
  }
  const ids = state.scope === scope ? state.ids.filter(id => visible.includes(id)) : []
  const update = (next: string[] | ((ids: string[]) => string[])) => {
    setState(previous => previous.scope !== scope ? previous : {
      ...previous,
      ids: typeof next === 'function' ? next(previous.ids) : next,
    })
  }
  return {
    scope, busy, setBusy, ids,
    selected: documents.filter(doc => ids.includes(doc.id)),
    allChecked: visible.length > 0 && ids.length === visible.length,
    hasSelectable: visible.length > 0,
    toggle: (id: string) => { if (!busy) update(previous => previous.includes(id) ? previous.filter(value => value !== id) : [...previous, id]) },
    toggleAll: () => { if (!busy) update(ids.length === visible.length ? [] : visible) },
    clear: () => { if (!busy) update([]) },
    remove: (completed: string[]) => update(previous => previous.filter(id => !completed.includes(id))),
  }
}
export type DocsSelection = ReturnType<typeof useDocsSelection>
