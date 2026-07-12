import { useMemo, useState } from 'react'
import { Search, Zap } from 'lucide-react'
import type { SupportCannedResponse } from '@helpin-ai/support-core'
import { filterShortcuts, stripShortcutContent } from '@/components/support/shortcutFiltering'
import { Sheet } from '@mobile/ui/sheet'
import { Pressable } from '@mobile/ui/pressable'
import { Spinner } from '@mobile/ui/spinner'

export interface CannedResponsesSheetProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  responses: SupportCannedResponse[]
  loading?: boolean
  onSelect: (response: SupportCannedResponse) => void
}

export function CannedResponsesSheet({ open, onOpenChange, responses, loading, onSelect }: CannedResponsesSheetProps) {
  const [query, setQuery] = useState('')
  const filtered = useMemo(() => filterShortcuts(responses, query, 50), [responses, query])

  return (
    <Sheet open={open} onOpenChange={onOpenChange} title="Canned responses" className="h-[70vh]">
      <div className="flex min-h-0 flex-1 flex-col px-4">
        <h2 className="px-1 pb-2 text-headline font-semibold">Canned responses</h2>
        <div className="flex items-center gap-2 rounded-xl border border-input bg-background px-3 py-2">
          <Search className="h-4 w-4 shrink-0 text-muted-foreground" />
          <input
            autoFocus
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            placeholder="Search shortcuts…"
            className="min-w-0 flex-1 bg-transparent text-body text-foreground outline-none placeholder:text-muted-foreground"
          />
        </div>

        <div className="mt-2 min-h-0 flex-1 overflow-y-auto">
          {loading ? (
            <div className="flex justify-center py-8">
              <Spinner />
            </div>
          ) : filtered.length === 0 ? (
            <p className="px-1 py-8 text-center text-footnote text-muted-foreground">
              {responses.length === 0 ? 'No canned responses yet.' : 'No matches.'}
            </p>
          ) : (
            filtered.map((response) => (
              <Pressable
                key={response.id}
                haptic="selection"
                onPress={() => onSelect(response)}
                aria-label={response.short_code}
                className="flex w-full flex-col gap-0.5 rounded-xl px-2 py-2.5 text-left active:bg-muted"
              >
                <span className="flex items-center gap-1.5 text-footnote font-semibold text-primary">
                  <Zap className="h-3.5 w-3.5" />
                  {response.short_code}
                </span>
                <span className="line-clamp-2 text-footnote text-muted-foreground">
                  {stripShortcutContent(response.content)}
                </span>
              </Pressable>
            ))
          )}
        </div>
      </div>
    </Sheet>
  )
}
