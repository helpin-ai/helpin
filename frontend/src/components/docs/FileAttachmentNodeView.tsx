import { NodeViewWrapper, type NodeViewProps } from '@tiptap/react'
import { Download04Icon, File01Icon } from '@/lib/icons'
import { cn } from '@/lib/utils'

function formatFileSize(value: unknown) {
  const bytes = typeof value === 'number' ? value : Number(value || 0)
  if (!Number.isFinite(bytes) || bytes <= 0) return ''
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${Math.round(bytes / 1024)} KB`
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}

export function FileAttachmentNodeView({ node, editor }: NodeViewProps) {
  const fileName = typeof node.attrs.fileName === 'string' && node.attrs.fileName.trim() ? node.attrs.fileName : 'Attachment'
  const contentType = typeof node.attrs.contentType === 'string' ? node.attrs.contentType : ''
  const url = typeof node.attrs.url === 'string' ? node.attrs.url : ''
  const size = formatFileSize(node.attrs.fileSize)

  const card = (
    <div
      className={cn(
        'my-3 flex items-center gap-3 rounded-md border border-border bg-muted/20 px-3 py-2.5 text-sm',
        editor.isEditable && 'cursor-grab hover:bg-muted/30',
      )}
      data-drag-handle
    >
      <span className="flex h-8 w-8 shrink-0 items-center justify-center rounded-md bg-background text-muted-foreground ring-1 ring-border/70">
        <File01Icon className="h-4 w-4" />
      </span>
      <span className="min-w-0 flex-1">
        <span className="block truncate font-medium text-foreground">{fileName}</span>
        <span className="mt-0.5 block truncate text-xs text-muted-foreground">
          {[contentType, size].filter(Boolean).join(' · ') || 'File'}
        </span>
      </span>
      {url && <Download04Icon className="h-4 w-4 shrink-0 text-muted-foreground" />}
    </div>
  )

  return (
    <NodeViewWrapper data-file-attachment-wrapper>
      {url ? (
        <a className="not-prose block" href={url} target="_blank" rel="noreferrer" contentEditable={false}>
          {card}
        </a>
      ) : (
        <div className="not-prose" contentEditable={false}>
          {card}
        </div>
      )}
    </NodeViewWrapper>
  )
}
