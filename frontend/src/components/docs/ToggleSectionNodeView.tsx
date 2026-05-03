import { useState } from 'react'
import { NodeViewContent, NodeViewWrapper, type NodeViewProps } from '@tiptap/react'
import { ArrowRight01Icon } from '@/lib/icons'
import { cn } from '@/lib/utils'

export function ToggleSectionNodeView({ node, updateAttributes, editor }: NodeViewProps) {
  const editable = editor.isEditable
  const [open, setOpen] = useState(Boolean(node.attrs.open))
  const title = typeof node.attrs.title === 'string' && node.attrs.title.trim() ? node.attrs.title : 'Details'

  const setNextOpen = (next: boolean) => {
    setOpen(next)
    updateAttributes({ open: next })
  }

  return (
    <NodeViewWrapper>
      <section className="not-prose my-4 rounded-md border border-border bg-background" data-toggle-section-wrapper>
        <button
          type="button"
          className="flex w-full items-center gap-2 px-3 py-2 text-left text-sm font-medium"
          contentEditable={false}
          onClick={() => setNextOpen(!open)}
        >
          <ArrowRight01Icon className={cn('h-4 w-4 shrink-0 text-muted-foreground transition-transform', open && 'rotate-90')} />
          {editable ? (
            <input
              value={title}
              onClick={(event) => event.stopPropagation()}
              onChange={(event) => updateAttributes({ title: event.target.value })}
              className="min-w-0 flex-1 bg-transparent outline-none"
            />
          ) : (
            <span className="min-w-0 flex-1 truncate">{title}</span>
          )}
        </button>
        {open && (
          <div className="border-t border-border px-3 py-3 text-sm">
            <NodeViewContent className="docs-toggle-content" />
          </div>
        )}
      </section>
    </NodeViewWrapper>
  )
}
