import { useState, useMemo, useCallback } from 'react'
import { icons } from 'lucide-react';
import type { IconComponent } from '@/lib/icons'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'

// Build a searchable list from all Lucide exports.
interface IconEntry {
  /** kebab-case key stored in the DB, e.g. "rocket" */
  value: string
  /** Human-readable label, e.g. "Rocket" */
  label: string
  /** The React component */
  Component: IconComponent
}

function pascalToKebab(s: string): string {
  return s.replace(/([a-z0-9])([A-Z])/g, '$1-$2').toLowerCase()
}

function pascalToLabel(s: string): string {
  return s.replace(/([a-z0-9])([A-Z])/g, '$1 $2')
}

const ALL_ICONS: IconEntry[] = (() => {
  const entries: IconEntry[] = []
  for (const [name, component] of Object.entries(icons)) {
    if (typeof component !== 'object' && typeof component !== 'function') continue
    if (component === null) continue
    const kebab = pascalToKebab(name)
    entries.push({
      value: kebab,
      label: pascalToLabel(name),
      Component: component as IconComponent,
    })
  }
  entries.sort((a, b) => a.label.localeCompare(b.label))
  return entries
})()

/** Kebab-case -> Lucide component map for rendering icons by stored name */
export const ICON_MAP: Record<string, IconComponent> = (() => {
  const map: Record<string, IconComponent> = {}
  for (const entry of ALL_ICONS) {
    map[entry.value] = entry.Component
  }
  return map
})()

interface IconPickerProps {
  value: string
  onChange: (value: string) => void
  placeholder?: string
}

export function IconPicker({ value, onChange, placeholder = 'Icon' }: IconPickerProps) {
  const [open, setOpen] = useState(false)
  const [search, setSearch] = useState('')

  const filtered = useMemo(() => {
    if (!search) return ALL_ICONS.slice(0, 60)
    const q = search.toLowerCase()
    return ALL_ICONS.filter(
      (e) => e.label.toLowerCase().includes(q) || e.value.includes(q),
    ).slice(0, 60)
  }, [search])

  const selected = useMemo(
    () => ALL_ICONS.find((e) => e.value === value),
    [value],
  )

  const handleSelect = useCallback(
    (v: string) => {
      onChange(v)
      setOpen(false)
      setSearch('')
    },
    [onChange],
  )

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <Button
          type="button"
          variant="outline"
          role="combobox"
          aria-expanded={open}
          className="justify-center w-10 h-9 px-0 font-normal"
        >
          {selected ? (
            <selected.Component className="h-4 w-4" />
          ) : (
            <span className="text-muted-foreground">{placeholder}</span>
          )}
        </Button>
      </PopoverTrigger>
      <PopoverContent className="w-[280px] p-0" align="start">
        <div className="p-2 border-b">
          <Input
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Search icons..."
            className="h-8 text-sm"
            autoFocus
          />
        </div>
        <div className="grid grid-cols-6 gap-0.5 p-2 max-h-[240px] overflow-y-auto">
          {filtered.map((entry) => (
            <button
              key={entry.value}
              type="button"
              title={entry.label}
              onClick={() => handleSelect(entry.value)}
              className={`flex items-center justify-center h-9 w-9 rounded-md transition-colors ${
                value === entry.value
                  ? 'bg-primary text-primary-foreground'
                  : 'hover:bg-muted text-foreground'
              }`}
            >
              <entry.Component className="h-[18px] w-[18px]" />
            </button>
          ))}
          {filtered.length === 0 && (
            <p className="col-span-6 py-4 text-center text-xs text-muted-foreground">
              No icons found
            </p>
          )}
        </div>
      </PopoverContent>
    </Popover>
  )
}
