import { useState } from 'react'
import { NodeViewContent, NodeViewWrapper, type NodeViewProps } from '@tiptap/react'

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
        className="docs-toggle-section not-prose"
        data-toggle-section-wrapper
        data-toggle-style={helpScoutCard ? 'helpScoutCard' : undefined}
        data-open={open ? '' : undefined}
      >
        <button
          type="button"
          className="docs-toggle-summary"
          contentEditable={false}
          aria-expanded={open}
          onClick={() => setNextOpen(!open)}
        >
          {helpScoutCard && icon ? (
            <span className="docs-toggle-icon">{icon}</span>
          ) : null}
          {editable ? (
            <input
              value={title}
              onClick={(event) => event.stopPropagation()}
              onChange={(event) => updateAttributes({ title: event.target.value })}
              className="docs-toggle-title docs-toggle-title-input"
            />
          ) : (
            <span className="docs-toggle-title">{title}</span>
          )}
          {helpScoutCard && badgeText ? (
            <span className="docs-toggle-badge">{badgeText}</span>
          ) : null}
          <span className="docs-toggle-chevron" aria-hidden="true" />
        </button>
        {open && (
          <div className="docs-toggle-content">
            <NodeViewContent className="docs-toggle-content-inner" />
          </div>
        )}
      </section>
    </NodeViewWrapper>
  )
}
