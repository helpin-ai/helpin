import { useEffect, useState } from 'react'
import {
  QuietDropdownContent,
  QuietDropdownEmpty,
  QuietDropdownItem,
  QuietDropdownOptions,
  QuietDropdownRoot,
  QuietDropdownTrigger,
} from '@/components/design-system/quiet-dropdown'
import { Cancel01Icon, PlusSignIcon } from '@/lib/icons'
import { crmSearchService } from '@/lib/services/crmService'
import type { CRMSearchResult } from '@/lib/crmTypes'

interface SLACompany {
  id: string
  name: string
}

/** Chooses the companies an SLA policy names, searching CRM companies. */
export function SLACompanyPicker({
  workspaceId,
  value,
  onChange,
  limit,
}: {
  workspaceId: string
  value: SLACompany[]
  onChange: (companies: SLACompany[]) => void
  limit: number
}) {
  const [open, setOpen] = useState(false)
  const [query, setQuery] = useState('')
  // Results are kept with the search they answer, so a stale answer never shows.
  const [found, setFound] = useState<{ term: string; items: CRMSearchResult[] }>({ term: '', items: [] })
  const term = query.trim()

  useEffect(() => {
    if (!open || term.length < 2) return
    let cancelled = false
    const timer = window.setTimeout(async () => {
      const response = await crmSearchService.search(workspaceId, term)
      if (!cancelled) setFound({ term, items: (response.data ?? []).filter((item) => item.type === 'company') })
    }, 250)
    return () => {
      cancelled = true
      window.clearTimeout(timer)
    }
  }, [open, term, workspaceId])

  const searching = term.length >= 2 && found.term !== term
  const results = term.length >= 2 && found.term === term ? found.items : []
  const chosen = new Set(value.map((company) => company.id))
  const options = results.filter((result) => !chosen.has(result.id))

  return (
    <div className="flex flex-wrap items-center gap-1.5">
      {value.map((company) => (
        <span key={company.id} className="inline-flex max-w-full items-center gap-1 rounded-full border border-border px-2.5 py-0.5 text-xs">
          <span className="truncate">{company.name}</span>
          <button
            type="button"
            className="rounded-full text-muted-foreground hover:text-foreground focus-visible:outline-2 focus-visible:outline-quiet-text-primary"
            aria-label={`Remove ${company.name}`}
            onClick={() => onChange(value.filter((item) => item.id !== company.id))}
          >
            <Cancel01Icon className="h-3 w-3" />
          </button>
        </span>
      ))}
      {value.length < limit ? (
        <QuietDropdownRoot open={open} onOpenChange={(next) => { setOpen(next); if (!next) setQuery('') }}>
          <QuietDropdownTrigger asChild>
            <button type="button" className="inline-flex items-center gap-1 rounded-full border border-dashed border-border px-2.5 py-0.5 text-xs text-muted-foreground hover:text-foreground">
              <PlusSignIcon className="h-3 w-3" />
              Add company
            </button>
          </QuietDropdownTrigger>
          <QuietDropdownContent align="start" className="w-64" aria-label="Add a company">
            <QuietDropdownOptions shouldFilter={false} searchMode="always" searchPlaceholder="Search companies" query={query} onQueryChange={setQuery}>
              {options.map((result) => (
                <QuietDropdownItem
                  key={result.id}
                  value={result.id}
                  fullText={result.name}
                  onSelect={() => {
                    onChange([...value, { id: result.id, name: result.name }])
                    setOpen(false)
                    setQuery('')
                  }}
                >
                  <span className="min-w-0 truncate">{result.name}</span>
                  {result.detail ? <span className="min-w-0 shrink truncate text-xs text-muted-foreground">{result.detail}</span> : null}
                </QuietDropdownItem>
              ))}
              {options.length === 0 ? (
                <QuietDropdownEmpty className="px-3 py-3 text-center text-muted-foreground">
                  {searching ? 'Searching…' : term.length < 2 ? 'Type at least 2 characters' : 'No companies found'}
                </QuietDropdownEmpty>
              ) : null}
            </QuietDropdownOptions>
          </QuietDropdownContent>
        </QuietDropdownRoot>
      ) : null}
    </div>
  )
}
