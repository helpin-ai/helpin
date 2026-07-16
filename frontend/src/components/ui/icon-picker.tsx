import { useCallback, useMemo, useState, type ReactNode } from 'react'

import {
  ICON_ALIASES,
  NORMALIZED_ICON_ALIASES,
  TOKEN_ICON_ALIASES,
} from '@/generated/iconAliases'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'

const ICON_CATALOG_VERSION = '4.1.1'
const ICON_ID_PATTERN = /^[a-z0-9-]+$/
const baseURL = import.meta.env.BASE_URL.replace(/\/+$/, '')
const iconAssetRoot = `${baseURL}/assets/helpin-icons/hugeicons/${ICON_CATALOG_VERSION}`
const iconManifestURL = `${baseURL}/assets/helpin-icons/catalog.json`

interface IconEntry {
  value: string
  label: string
}

interface IconManifest {
  version: string
  icons: Array<{ id: string; label: string }>
}

let iconManifestPromise: Promise<IconEntry[]> | null = null

function loadIconManifest(): Promise<IconEntry[]> {
  if (iconManifestPromise) return iconManifestPromise

  iconManifestPromise = fetch(iconManifestURL, { cache: 'force-cache' })
    .then(async (response) => {
      if (!response.ok) throw new Error(`icon catalog request failed: ${response.status}`)
      const manifest = (await response.json()) as IconManifest
      if (manifest.version !== ICON_CATALOG_VERSION || !Array.isArray(manifest.icons)) {
        throw new Error('icon catalog version mismatch')
      }
      return manifest.icons.map(({ id, label }) => ({ value: id, label }))
    })
    .catch((error) => {
      iconManifestPromise = null
      throw error
    })

  return iconManifestPromise
}

function normalizeLegacyName(value: string): string {
  return value
    .replace(/Icon$/i, '')
    .replace(/([a-z0-9])([A-Z])/g, '$1-$2')
    .toLowerCase()
    .replace(/\d+/g, '')
    .replace(/[^a-z-]+/g, '-')
    .replace(/-+/g, '-')
    .replace(/^-|-$/g, '')
}

function resolveStoredIconID(value: string): string | null {
  const trimmed = value.trim()
  const lower = trimmed.toLowerCase()
  const directAlias = ICON_ALIASES[lower]
  if (directAlias) return directAlias
  if (ICON_ID_PATTERN.test(trimmed)) return trimmed
  if (/^[A-Z][A-Za-z0-9-]*$/.test(trimmed) && !/^[A-Z0-9-]+$/.test(trimmed)) {
    const caseVariant = lower
    if (ICON_ID_PATTERN.test(caseVariant)) return caseVariant
  }
  if (!/Icon$/i.test(trimmed)) return null

  const normalized = normalizeLegacyName(trimmed)
  if (!normalized) return null
  const normalizedAlias = ICON_ALIASES[normalized] ?? NORMALIZED_ICON_ALIASES[normalized]
  if (normalizedAlias) return normalizedAlias
  for (const token of normalized.split('-')) {
    const tokenAlias = TOKEN_ICON_ALIASES[token]
    if (tokenAlias) return tokenAlias
  }
  return ICON_ID_PATTERN.test(normalized) ? normalized : null
}

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
  if (name) {
    const iconID = resolveStoredIconID(name)
    if (iconID) {
      const assetURL = `${iconAssetRoot}/${encodeURIComponent(iconID)}.svg`
      return (
        <span
          aria-hidden="true"
          className={className}
          style={{
            backgroundColor: 'currentColor',
            display: 'inline-block',
            flexShrink: 0,
            maskImage: `url("${assetURL}")`,
            maskPosition: 'center',
            maskRepeat: 'no-repeat',
            maskSize: 'contain',
            WebkitMaskImage: `url("${assetURL}")`,
            WebkitMaskPosition: 'center',
            WebkitMaskRepeat: 'no-repeat',
            WebkitMaskSize: 'contain',
          }}
        />
      )
    }
    return <span className={textClassName}>{name}</span>
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
  const [icons, setIcons] = useState<IconEntry[]>([])
  const [loadFailed, setLoadFailed] = useState(false)

  const ensureManifest = useCallback(() => {
    if (icons.length > 0) return
    void loadIconManifest()
      .then((entries) => {
        setIcons(entries)
        setLoadFailed(false)
      })
      .catch(() => setLoadFailed(true))
  }, [icons.length])

  const filtered = useMemo(() => {
    if (!search) return icons.slice(0, 60)
    const query = search.toLowerCase()
    return icons
      .filter(
        (entry) =>
          entry.label.toLowerCase().includes(query) || entry.value.includes(query),
      )
      .slice(0, 60)
  }, [search, icons])

  const handleSelect = useCallback(
    (nextValue: string) => {
      onChange(nextValue)
      setOpen(false)
      setSearch('')
    },
    [onChange],
  )

  const handleOpenChange = useCallback(
    (nextOpen: boolean) => {
      setOpen(nextOpen)
      if (nextOpen) ensureManifest()
    },
    [ensureManifest],
  )

  const placeholderNode = (
    <span className="text-[10px] text-muted-foreground">{placeholder}</span>
  )

  return (
    <Popover open={open} onOpenChange={handleOpenChange}>
      <PopoverTrigger asChild>
        <Button
          type="button"
          variant="outline"
          role="combobox"
          aria-expanded={open}
          className="h-9 w-10 justify-center px-0 font-normal"
          onPointerEnter={ensureManifest}
          onFocus={ensureManifest}
        >
          <StoredIcon name={value} className="h-4 w-4" fallback={placeholderNode} />
        </Button>
      </PopoverTrigger>
      <PopoverContent className="w-[280px] p-0" align="start">
        <div className="border-b p-2">
          <Input
            value={search}
            onChange={(event) => setSearch(event.target.value)}
            placeholder="Search icons..."
            className="h-8 text-sm"
            autoFocus
          />
        </div>
        <div
          className="grid max-h-[240px] grid-cols-6 gap-0.5 overflow-y-auto overscroll-contain p-2"
          onWheel={(event) => event.stopPropagation()}
          onTouchMove={(event) => event.stopPropagation()}
        >
          {filtered.map((entry) => (
            <button
              key={entry.value}
              type="button"
              title={entry.label}
              onClick={() => handleSelect(entry.value)}
              className={`flex h-9 w-9 items-center justify-center rounded-md transition-colors ${
                value === entry.value
                  ? 'bg-primary text-primary-foreground'
                  : 'text-foreground hover:bg-muted'
              }`}
            >
              <StoredIcon name={entry.value} className="h-[18px] w-[18px]" />
            </button>
          ))}
          {loadFailed ? (
            <p className="col-span-6 py-4 text-center text-xs text-muted-foreground">
              Icons could not be loaded. Reopen to retry.
            </p>
          ) : null}
          {!loadFailed && filtered.length === 0 ? (
            <p className="col-span-6 py-4 text-center text-xs text-muted-foreground">
              {icons.length === 0 ? 'Loading icons…' : 'No icons found'}
            </p>
          ) : null}
        </div>
      </PopoverContent>
    </Popover>
  )
}
