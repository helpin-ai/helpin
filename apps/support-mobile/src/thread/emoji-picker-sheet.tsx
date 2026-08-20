import { useEffect, useMemo, useState } from 'react'
import { Search, X } from 'lucide-react'
import { loadEmojiCatalog } from '@helpin-ai/widget-core/emoji-loader'
import type { EmojiCatalog } from '@helpin-ai/widget-core/emoji-catalog'

import { Pressable } from '@mobile/ui/pressable'
import { Sheet } from '@mobile/ui/sheet'
import { Spinner } from '@mobile/ui/spinner'

const CATEGORY_ICONS: Record<string, string> = {
  smileys: '😀',
  animals: '🐾',
  food: '🍔',
  activities: '⚽',
  travel: '✈️',
  objects: '💡',
  symbols: '❤️',
  flags: '🏳️',
}

export interface EmojiPickerSheetProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  onSelect: (emoji: string) => void
}

export function EmojiPickerSheet({ open, onOpenChange, onSelect }: EmojiPickerSheetProps) {
  const [catalog, setCatalog] = useState<EmojiCatalog | null>(null)
  const [activeCategory, setActiveCategory] = useState('smileys')
  const [query, setQuery] = useState('')
  const [loading, setLoading] = useState(false)
  const [loadError, setLoadError] = useState(false)

  useEffect(() => {
    if (!open || catalog || loadError) return
    let cancelled = false
    setLoading(true)
    void loadEmojiCatalog()
      .then((nextCatalog) => {
        if (!cancelled) setCatalog(nextCatalog)
      })
      .catch(() => {
        if (!cancelled) setLoadError(true)
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => { cancelled = true }
  }, [catalog, loadError, open])

  useEffect(() => {
    if (!open) setQuery('')
  }, [open])

  const emojis = useMemo(() => {
    if (!catalog) return []
    if (query.trim()) return catalog.search(query.trim())
    return catalog.categories.find((category) => category.id === activeCategory)?.emojis ?? []
  }, [activeCategory, catalog, query])

  return (
    <Sheet open={open} onOpenChange={onOpenChange} title="Emoji picker" className="h-[70vh]">
      <div className="flex min-h-0 flex-1 flex-col px-4">
        <h2 className="px-1 pb-2 text-headline font-semibold">Emoji</h2>
        <div className="flex items-center gap-2 rounded-xl border border-input bg-background px-3 py-2">
          <Search className="h-4 w-4 shrink-0 text-muted-foreground" />
          <input
            autoFocus
            aria-label="Search emojis"
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            placeholder="Search emojis…"
            className="min-w-0 flex-1 bg-transparent text-body text-foreground outline-none placeholder:text-muted-foreground"
          />
          {query && (
            <button type="button" aria-label="Clear emoji search" onClick={() => setQuery('')} className="text-muted-foreground">
              <X className="h-4 w-4" />
            </button>
          )}
        </div>

        {catalog && !query.trim() && (
          <div className="mt-2 flex gap-1 overflow-x-auto pb-1">
            {catalog.categories.map((category) => (
              <Pressable
                key={category.id}
                aria-label={category.label}
                aria-pressed={activeCategory === category.id}
                onPress={() => setActiveCategory(category.id)}
                className="flex h-10 w-10 shrink-0 items-center justify-center rounded-full text-lg aria-pressed:bg-primary/10"
              >
                {CATEGORY_ICONS[category.id] ?? category.emojis[0] ?? '•'}
              </Pressable>
            ))}
          </div>
        )}

        <div className="mt-2 min-h-0 flex-1 overflow-y-auto">
          {loading ? (
            <div className="flex justify-center py-10"><Spinner /></div>
          ) : loadError ? (
            <div className="space-y-3 py-10 text-center">
              <p className="text-footnote text-muted-foreground">Emojis could not be loaded.</p>
              <Pressable
                onPress={() => setLoadError(false)}
                className="mx-auto h-auto min-h-9 w-auto min-w-0 rounded-full px-4 text-footnote font-medium text-primary"
              >
                Retry
              </Pressable>
            </div>
          ) : emojis.length === 0 ? (
            <p className="py-10 text-center text-footnote text-muted-foreground">No emojis found.</p>
          ) : (
            <div className="grid grid-cols-7 gap-1 pb-4">
              {emojis.map((emoji, index) => (
                <Pressable
                  key={`${emoji}-${index}`}
                  aria-label={`Insert ${emoji}`}
                  haptic="selection"
                  onPress={() => {
                    onSelect(emoji)
                    onOpenChange(false)
                  }}
                  className="flex aspect-square h-auto min-h-10 w-full min-w-0 items-center justify-center rounded-xl text-2xl active:bg-muted"
                >
                  {emoji}
                </Pressable>
              ))}
            </div>
          )}
        </div>
      </div>
    </Sheet>
  )
}
