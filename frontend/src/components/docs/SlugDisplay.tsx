import { useRef, useState } from 'react'
import { Tick01Icon, Link01Icon, Cancel01Icon } from '@/lib/icons'
import { QuickTooltip } from '@/components/ui/quick-tooltip'
import { cn } from '@/lib/utils'

interface SlugDisplayProps {
  slug: string
  onSlugChange?: (slug: string) => Promise<void>
  readOnly?: boolean
  helperText?: string
  presentation?: 'editor' | 'header'
}

export function SlugDisplay({ slug, onSlugChange, readOnly, helperText, presentation = 'editor' }: SlugDisplayProps) {
  const [editing, setEditing] = useState(false)
  const [draft, setDraft] = useState(slug)
  const [saving, setSaving] = useState(false)
  const [showHelperText, setShowHelperText] = useState(false)
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
      setShowHelperText(!!helperText)
      setEditing(false)
    } catch {
      // error handled by caller
    } finally {
      setSaving(false)
    }
  }

  if (editing) {
    return (
      <div className={cn('flex max-w-full items-center gap-1.5', presentation === 'editor' && 'mb-3')}>
        <Link01Icon className="h-3.5 w-3.5 shrink-0 text-quiet-muted" />
        <span className="text-[11.5px] font-mono text-quiet-muted">/</span>
        <input
          ref={inputRef}
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter') void handleSave()
            if (e.key === 'Escape') setEditing(false)
          }}
          className="w-[min(20rem,60vw)] min-w-0 border-0 border-b border-quiet-field bg-transparent px-0.5 py-0.5 text-[11.5px] font-mono text-quiet-text-secondary outline-none hover:border-quiet-text-primary focus-visible:border-b-2 focus-visible:border-quiet-text-primary"
          disabled={saving}
        />
        <QuickTooltip label="Save slug">
          <button
            type="button"
            disabled={saving}
            onClick={() => void handleSave()}
            className="rounded-[6px] p-1 text-quiet-positive transition-colors hover:bg-quiet-hover disabled:opacity-50"
          >
            <Tick01Icon className="h-3.5 w-3.5" />
          </button>
        </QuickTooltip>
        <QuickTooltip label="Cancel">
          <button
            type="button"
            onClick={() => setEditing(false)}
            className="rounded-[6px] p-1 text-quiet-text-tertiary transition-colors hover:bg-quiet-hover hover:text-quiet-text-primary"
          >
            <Cancel01Icon className="h-3.5 w-3.5" />
          </button>
        </QuickTooltip>
      </div>
    )
  }

  return (
    <div className={cn('flex max-w-full flex-col gap-1', presentation === 'editor' && 'mb-3')}>
      <button
        type="button"
        onClick={canEdit ? handleStartEdit : undefined}
        className={cn(
          'flex max-w-full items-center gap-1.5 border-b border-transparent text-[11.5px] font-mono text-quiet-muted transition-colors focus-visible:border-b-2 focus-visible:border-quiet-text-primary focus-visible:outline-none',
          presentation === 'editor' && 'opacity-0 group-hover/title:opacity-100',
          canEdit && 'cursor-pointer hover:border-quiet-field hover:text-quiet-text-secondary',
        )}
      >
        <Link01Icon className="h-3.5 w-3.5" />
        <span className="truncate">/{slug}</span>
      </button>
      {helperText && showHelperText ? (
        <span className="text-[11px] text-quiet-muted">{helperText}</span>
      ) : null}
    </div>
  )
}
