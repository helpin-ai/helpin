export interface DocumentOutlineItem {
  index: number
  level: number
  text: string
}

interface DocsOutlineSidebarProps {
  items: DocumentOutlineItem[]
  open: boolean
  onSelect: (index: number) => void
}

export function DocsOutlineSidebar({ items, open, onSelect }: DocsOutlineSidebarProps) {
  if (!open || items.length === 0) return null
  return (
    <aside className="hidden w-56 shrink-0 overflow-y-auto px-3 py-5 lg:block">
      <div className="space-y-0.5">
        {items.map((item) => (
          <button
            key={`${item.index}:${item.text}`}
            type="button"
            className="block w-full truncate rounded px-2 py-1 text-left text-sm text-muted-foreground hover:bg-muted/60 hover:text-foreground"
            style={{ paddingLeft: `${8 + Math.max(0, item.level - 2) * 10}px` }}
            onClick={() => onSelect(item.index)}
          >
            {item.text}
          </button>
        ))}
      </div>
    </aside>
  )
}
