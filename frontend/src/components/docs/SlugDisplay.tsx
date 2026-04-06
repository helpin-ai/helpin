import { useRef, useState } from 'react'
import { Tick01Icon, Link01Icon, Cancel01Icon } from '@/lib/icons'
import { QuickTooltip } from '@/components/ui/quick-tooltip'

interface SlugDisplayProps {
  slug: string
  onSlugChange?: (slug: string) => Promise<void>
  readOnly?: boolean
  helperText?: string
}

export function SlugDisplay({ slug, onSlugChange, readOnly, helperText }: SlugDisplayProps) {
  const [editing, setEditing] = useState(false)
  const [draft, setDraft] = useState(slug)
  const [saving, setSaving] = useState(false)
  const inputRef = useRef<HTMLInputElement>(null)

  const canEdit = !!onSlugChange && !readOnly

  const handleStartEdit = () => {
    if (!canEdit) return
    setDraft(slug)
    setEditing(true)
    setTimeout(() => inputRef.current?.focus(), 0)
  }

  const handleSave = async () => {
    const cleaned = draft.trim().toLowerCase().replace(/[^a-z0-9-]/g, '-').replace(/^-|-$/g, '').replace(/-+/g, '-')
    if (!cleaned || cleaned === slug) {
      setEditing(false)
      return
    }
    setSaving(true)
    try {
      await onSlugChange!(cleaned)
      setEditing(false)
    } catch {
      // error handled by caller
    } finally {
      setSaving(false)
    }
  }

  if (editing) {
    return (
      <div className="mb-3 flex items-center gap-1.5">
        <Link01Icon className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
        <span className="text-sm font-mono text-muted-foreground">/</span>
        <input
          ref={inputRef}
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter') void handleSave()
            if (e.key === 'Escape') setEditing(false)
          }}
          className="min-w-[120px] rounded border border-border/60 bg-muted/40 px-1.5 py-0.5 text-sm font-mono text-foreground focus:outline-none focus:ring-1 focus:ring-primary/30"
          disabled={saving}
        />
        <QuickTooltip label="Save slug">
          <button
            type="button"
            disabled={saving}
            onClick={() => void handleSave()}
            className="rounded p-0.5 text-emerald-600 transition-colors hover:bg-emerald-500/10 disabled:opacity-50"
          >
            <Tick01Icon className="h-3.5 w-3.5" />
          </button>
        </QuickTooltip>
        <QuickTooltip label="Cancel">
          <button
            type="button"
            onClick={() => setEditing(false)}
            className="rounded p-0.5 text-muted-foreground transition-colors hover:bg-muted/60"
          >
            <Cancel01Icon className="h-3.5 w-3.5" />
          </button>
        </QuickTooltip>
      </div>
    )
  }

  return (
    <div className="mb-3 flex flex-col gap-1">
      <button
        type="button"
        onClick={canEdit ? handleStartEdit : undefined}
        className={`flex items-center gap-1.5 text-sm font-mono text-muted-foreground/60 opacity-0 transition-opacity group-hover/title:opacity-100 ${canEdit ? 'cursor-pointer hover:text-muted-foreground' : ''}`}
      >
        <Link01Icon className="h-3.5 w-3.5" />
        /{slug}
      </button>
      {helperText ? (
        <span className="text-[11px] text-muted-foreground/70">{helperText}</span>
      ) : null}
    </div>
  )
}
