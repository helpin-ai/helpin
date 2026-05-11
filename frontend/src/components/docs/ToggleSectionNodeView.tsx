import { useState } from 'react'
import { NodeViewContent, NodeViewWrapper, type NodeViewProps } from '@tiptap/react'
import { ArrowRight01Icon } from '@/lib/icons'
import { cn } from '@/lib/utils'

export function ToggleSectionNodeView({ node, updateAttributes, editor }: NodeViewProps) {
  const editable = editor.isEditable
  const [open, setOpen] = useState(Boolean(node.attrs.open))
  const title = typeof node.attrs.title === 'string' && node.attrs.title.trim() ? node.attrs.title : 'Details'
  const icon = typeof node.attrs.icon === 'string' ? node.attrs.icon : ''
  const badgeText = typeof node.attrs.badgeText === 'string' ? node.attrs.badgeText : ''
  const helpScoutCard = node.attrs.sourceStyle === 'helpScoutCard' || Boolean(icon || badgeText)

  const setNextOpen = (next: boolean) => {
    setOpen(next)
    updateAttributes({ open: next })
  }

  return (
    <NodeViewWrapper>
      <section
        className={cn(
          'not-prose my-4 overflow-hidden border bg-background',
          helpScoutCard ? 'rounded-[10px] border-[#e5e7eb]' : 'rounded-md border-border',
        )}
        data-toggle-section-wrapper
      >
        <button
          type="button"
          className={cn(
            'flex w-full items-center text-left text-sm',
            helpScoutCard ? 'gap-3 bg-background px-4 py-3 font-normal' : 'gap-2 px-3 py-2 font-medium',
          )}
          contentEditable={false}
          onClick={() => setNextOpen(!open)}
        >
          {helpScoutCard && icon ? (
            <span className="flex h-8 w-8 shrink-0 items-center justify-center rounded-md bg-blue-50 text-base leading-none text-blue-600">
              {icon}
            </span>
          ) : (
            <ArrowRight01Icon className={cn('h-4 w-4 shrink-0 text-muted-foreground transition-transform', open && 'rotate-90')} />
          )}
          {editable ? (
            <input
              value={title}
              onClick={(event) => event.stopPropagation()}
              onChange={(event) => updateAttributes({ title: event.target.value })}
              className={cn('min-w-0 flex-1 bg-transparent outline-none', helpScoutCard && 'font-normal text-foreground')}
            />
          ) : (
            <span className={cn('min-w-0 flex-1 truncate', helpScoutCard && 'text-foreground')}>{title}</span>
          )}
          {helpScoutCard && badgeText ? (
            <span className="shrink-0 rounded-full bg-blue-50 px-2.5 py-0.5 text-[11px] font-normal leading-5 text-blue-600">
              {badgeText}
            </span>
          ) : null}
          {helpScoutCard ? (
            <ArrowRight01Icon className={cn('h-4 w-4 shrink-0 text-muted-foreground transition-transform', open && 'rotate-90')} />
          ) : null}
        </button>
        {open && (
          <div className={cn('border-t text-sm', helpScoutCard ? 'border-[#f3f4f6] bg-muted/30 px-4 py-2.5' : 'border-border px-3 py-3')}>
            <NodeViewContent
              className={cn(
                'docs-toggle-content',
                helpScoutCard && '[&_p]:my-0 [&_p]:border-b [&_p]:border-[#f3f4f6] [&_p]:py-2 last:[&_p]:border-b-0',
              )}
            />
          </div>
        )}
      </section>
    </NodeViewWrapper>
  )
}
