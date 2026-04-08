import { useState, useMemo, useCallback, useEffect, useRef, type ReactNode } from 'react'
import { HugeiconsIcon } from '@hugeicons/react'
import type { IconComponent } from '@/lib/icons'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'

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
  return s.replace(/(\d+)/g, ' $1 ').replace(/([a-z0-9])([A-Z])/g, '$1 $2').replace(/Icon$/, '').trim()
}

type HugeIconData = Parameters<typeof HugeiconsIcon>[0]['icon'];

let _cachedIcons: IconEntry[] | null = null
let _cachedMap: Record<string, IconComponent> | null = null

async function loadAllIcons(): Promise<IconEntry[]> {
  if (_cachedIcons) return _cachedIcons
  const allIcons = await import('@hugeicons/core-free-icons')
  const entries: IconEntry[] = []
  for (const [name, iconData] of Object.entries(allIcons)) {
    if (!name.endsWith('Icon') || !Array.isArray(iconData)) continue
    const kebab = pascalToKebab(name.replace(/Icon$/, ''))
    const label = pascalToLabel(name)
    const data = iconData as HugeIconData
    const Component: IconComponent = ({ className, style }) => (
      <HugeiconsIcon icon={data} className={className} style={style} strokeWidth={2} />
    )
    entries.push({ value: kebab, label, Component })
  }
  entries.sort((a, b) => a.label.localeCompare(b.label))
  _cachedIcons = entries
  _cachedMap = Object.fromEntries(entries.map((e) => [e.value, e.Component]))
  return entries
}

function looksLikeIconKey(value: string): boolean {
  return /^[a-z0-9-]+$/.test(value)
}

/** Kebab-case -> icon component map. Populates async after chunk loads. */
export const ICON_MAP: Record<string, IconComponent> = {}

// Eagerly kick off the load so ICON_MAP is populated ASAP
loadAllIcons().then(() => {
  if (_cachedMap) Object.assign(ICON_MAP, _cachedMap)
})

interface StoredIconProps {
  name?: string | null
  className?: string
  textClassName?: string
  fallback?: ReactNode
}

export function StoredIcon({
  name,
  className,
  textClassName,
  fallback = null,
}: StoredIconProps) {
  const [, forceRender] = useState(0)

  useEffect(() => {
    if (!name || ICON_MAP[name] || !looksLikeIconKey(name)) {
      return
    }

    let cancelled = false
    loadAllIcons().then(() => {
      if (!cancelled) {
        forceRender((count) => count + 1)
      }
    })

    return () => {
      cancelled = true
    }
  }, [name])

  if (name) {
    const Icon = ICON_MAP[name]
    if (Icon) {
      return <Icon className={className} />
    }
    if (!looksLikeIconKey(name)) {
      return <span className={textClassName}>{name}</span>
    }
  }

  return <>{fallback}</>
}

interface IconPickerProps {
  value: string
  onChange: (value: string) => void
  placeholder?: string
}

export function IconPicker({ value, onChange, placeholder = 'Icon' }: IconPickerProps) {
  const [open, setOpen] = useState(false)
  const [search, setSearch] = useState('')
  const [icons, setIcons] = useState<IconEntry[]>(_cachedIcons ?? [])
  const loaded = useRef(false)

  useEffect(() => {
    if (loaded.current) return
    loaded.current = true
    loadAllIcons().then(setIcons)
  }, [])

  const filtered = useMemo(() => {
    if (!search) return icons.slice(0, 60)
    const q = search.toLowerCase()
    return icons.filter(
      (e) => e.label.toLowerCase().includes(q) || e.value.includes(q),
    ).slice(0, 60)
  }, [search, icons])

  const selected = useMemo(
    () => icons.find((e) => e.value === value),
    [value, icons],
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
