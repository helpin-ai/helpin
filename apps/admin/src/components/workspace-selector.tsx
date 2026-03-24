import { useEffect, useRef, useState } from 'react'
import { Check, ChevronsUpDown } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { cn } from '@/lib/utils'
import type { Workspace } from '@/lib/types'

interface WorkspaceSelectorProps {
  workspaces: Workspace[]
  value: string
  onChange: (id: string) => void
  loading?: boolean
}

export function WorkspaceSelector({ workspaces, value, onChange, loading }: WorkspaceSelectorProps) {
  const [open, setOpen] = useState(false)
  const [search, setSearch] = useState('')
  const ref = useRef<HTMLDivElement>(null)

  const selected = workspaces.find((w) => w.id === value)
  const filtered = workspaces.filter(
    (w) =>
      w.name.toLowerCase().includes(search.toLowerCase()) ||
      w.slug.toLowerCase().includes(search.toLowerCase()),
  )

  useEffect(() => {
    function handleClickOutside(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) {
        setOpen(false)
      }
    }
    document.addEventListener('mousedown', handleClickOutside)
    return () => document.removeEventListener('mousedown', handleClickOutside)
  }, [])

  return (
    <div ref={ref} className="relative">
      <Button
        type="button"
        variant="outline"
        role="combobox"
        aria-expanded={open}
        className="w-full justify-between font-normal"
        onClick={() => setOpen(!open)}
        disabled={loading}
      >
        <span className="truncate">
          {loading ? 'Loading workspaces...' : (selected?.name ?? 'Select workspace')}
        </span>
        <ChevronsUpDown className="ml-2 h-4 w-4 shrink-0 opacity-50" />
      </Button>
      {open && (
        <div className="absolute left-0 right-0 top-full z-50 mt-1 rounded-md border bg-popover shadow-md">
          <div className="p-2">
            <Input
              placeholder="Search workspace..."
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              className="h-8"
              autoFocus
            />
          </div>
          <div className="max-h-60 overflow-auto p-1">
            {filtered.length === 0 ? (
              <p className="py-4 text-center text-sm text-muted-foreground">No workspace found.</p>
            ) : (
              filtered.map((ws) => (
                <button
                  key={ws.id}
                  type="button"
                  className={cn(
                    'flex w-full items-center gap-2 rounded-sm px-2 py-1.5 text-left text-sm hover:bg-accent',
                    ws.id === value && 'bg-accent',
                  )}
                  onClick={() => {
                    onChange(ws.id)
                    setOpen(false)
                    setSearch('')
                  }}
                >
                  <Check
                    className={cn(
                      'h-3.5 w-3.5 shrink-0',
                      ws.id === value ? 'opacity-100' : 'opacity-0',
                    )}
                  />
                  <span className="truncate">{ws.name}</span>
                  <span className="ml-auto truncate text-xs text-muted-foreground">
                    /{ws.slug}
                  </span>
                </button>
              ))
            )}
          </div>
        </div>
      )}
    </div>
  )
}
