import { useMemo, useState } from 'react'
import { Check, ChevronDown, Languages } from 'lucide-react'
import { cn } from '@/lib/utils'

export interface LocaleSwitcherOption {
  code: string
  label: string
  href: string
  active: boolean
}

interface LocaleSwitcherProps {
  currentLocale: string
  options: LocaleSwitcherOption[]
  onSelect?: (href: string) => void
}

function normalizeLocaleLabel(code: string, label?: string) {
  if (label) return label
  try {
    return new Intl.DisplayNames(['en'], { type: 'language' }).of(code) ?? code.toUpperCase()
  } catch {
    return code.toUpperCase()
  }
}

export function LocaleSwitcher({
  currentLocale,
  options,
  onSelect,
}: LocaleSwitcherProps) {
  const [open, setOpen] = useState(false)

  const current = useMemo(
    () => options.find((option) => option.active) ?? options[0],
    [options],
  )

  if (!current || options.length <= 1) {
    return null
  }

  return (
    <div className="relative">
      <button
        type="button"
        onClick={() => setOpen((value) => !value)}
        className="inline-flex items-center gap-2 rounded-full border border-border/80 bg-background px-3 py-1.5 text-[12px] font-medium text-foreground shadow-sm transition-colors hover:border-border hover:bg-muted/40"
        aria-expanded={open}
        aria-haspopup="menu"
        aria-label={normalizeLocaleLabel(currentLocale, current.label)}
      >
        <Languages size={14} className="text-muted-foreground" />
        <span>{normalizeLocaleLabel(currentLocale, current.label)}</span>
        <ChevronDown
          size={14}
          className={cn(
            'text-muted-foreground transition-transform duration-150',
            open && 'rotate-180',
          )}
        />
      </button>

      {open && (
        <div
          className="absolute right-0 top-[calc(100%+0.5rem)] z-40 min-w-[180px] rounded-2xl border border-border/70 bg-background/98 p-1.5 shadow-xl backdrop-blur"
          role="menu"
        >
          {options.map((option) => (
            <button
              key={option.code}
              type="button"
              onClick={() => {
                setOpen(false)
                if (option.active) return
                if (onSelect) {
                  onSelect(option.href)
                  return
                }
                window.location.assign(option.href)
              }}
              className={cn(
                'flex w-full items-center justify-between gap-3 rounded-xl px-3 py-2 text-left text-[13px] transition-colors',
                option.active
                  ? 'bg-primary/[0.08] text-foreground'
                  : 'text-muted-foreground hover:bg-muted/60 hover:text-foreground',
              )}
              role="menuitem"
              aria-label={normalizeLocaleLabel(option.code, option.label)}
            >
              <div className="min-w-0">
                <div className="font-medium">
                  {normalizeLocaleLabel(option.code, option.label)}
                </div>
                <div className="text-[11px] uppercase tracking-[0.18em] text-muted-foreground/70">
                  {option.code}
                </div>
              </div>
              {option.active && <Check size={14} className="text-primary" />}
            </button>
          ))}
        </div>
      )}
    </div>
  )
}
