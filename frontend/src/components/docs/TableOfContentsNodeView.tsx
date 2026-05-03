import { useEffect, useState } from 'react'
import { NodeViewWrapper, type NodeViewProps } from '@tiptap/react'
import { LeftToRightListBulletIcon } from '@/lib/icons'

interface TocItem {
  pos: number
  level: number
  text: string
}

export function tableOfContentsIndent(level: number): string {
  return `${Math.max(0, level - 2) * 12}px`
}

function collectHeadings(editor: NodeViewProps['editor']): TocItem[] {
  const items: TocItem[] = []
  editor.state.doc.descendants((node, pos) => {
    if (node.type.name !== 'heading') return
    const text = node.textContent.trim()
    if (!text) return
    items.push({ pos, level: Number(node.attrs.level || 2), text })
  })
  return items
}

export function TableOfContentsNodeView({ editor }: NodeViewProps) {
  const [items, setItems] = useState<TocItem[]>(() => collectHeadings(editor))

  useEffect(() => {
    const update = () => setItems(collectHeadings(editor))
    editor.on('update', update)
    editor.on('transaction', update)
    return () => {
      editor.off('update', update)
      editor.off('transaction', update)
    }
  }, [editor])

  const navigateToHeading = (item: TocItem) => {
    const target = editor.view.nodeDOM(item.pos)
    if (target instanceof HTMLElement && typeof target.scrollIntoView === 'function') {
      target.scrollIntoView({ behavior: 'smooth', block: 'start' })
    }

    if (editor.isEditable) {
      editor.chain().focus().setTextSelection(item.pos + 1).run()
    }
  }

  return (
    <NodeViewWrapper>
      <nav className="not-prose my-4 text-sm" data-docs-toc contentEditable={false}>
        <div className="mb-2 flex items-center gap-2 text-xs font-medium uppercase text-muted-foreground">
          <LeftToRightListBulletIcon className="h-3.5 w-3.5" />
          <span>Contents</span>
        </div>
        {items.length === 0 ? (
          <div className="text-xs text-muted-foreground">
            Add headings to populate this block
          </div>
        ) : (
          <div className="space-y-1" role="list" aria-label="Document headings">
            {items.map((item) => (
              <div key={`${item.pos}:${item.text}`} role="listitem" style={{ paddingLeft: tableOfContentsIndent(item.level) }}>
                <button
                  type="button"
                  className="block max-w-full truncate text-left text-muted-foreground hover:text-foreground hover:underline"
                  onClick={() => navigateToHeading(item)}
                >
                  {item.text}
                </button>
              </div>
            ))}
          </div>
        )}
      </nav>
    </NodeViewWrapper>
  )
}
