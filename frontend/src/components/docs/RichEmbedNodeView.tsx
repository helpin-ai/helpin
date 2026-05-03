import { NodeViewWrapper, type NodeViewProps } from '@tiptap/react'
import { LinkSquare01Icon } from '@/lib/icons'

export function RichEmbedNodeView({ node, editor }: NodeViewProps) {
  const url = typeof node.attrs.url === 'string' ? node.attrs.url : ''
  const provider = typeof node.attrs.provider === 'string' && node.attrs.provider.trim() ? node.attrs.provider : 'Embed'
  const title = typeof node.attrs.title === 'string' && node.attrs.title.trim() ? node.attrs.title : url
  const description = typeof node.attrs.description === 'string' ? node.attrs.description : ''
  const imageUrl = typeof node.attrs.image_url === 'string' && node.attrs.image_url.trim() ? node.attrs.image_url : ''

  const card = (
    <div className="my-3 flex items-start gap-3 rounded-md border border-border bg-muted/20 px-3 py-2.5 text-sm transition-colors hover:bg-muted/30" data-drag-handle>
      <span className="mt-0.5 flex h-8 w-8 shrink-0 items-center justify-center rounded-md bg-background text-muted-foreground ring-1 ring-border/70">
        <LinkSquare01Icon className="h-4 w-4" />
      </span>
      <span className="min-w-0 flex-1">
        <span className="block text-[11px] font-medium uppercase text-muted-foreground">{provider}</span>
        <span className="mt-0.5 block truncate font-medium text-foreground">{title}</span>
        {description && <span className="mt-0.5 block truncate text-xs text-muted-foreground">{description}</span>}
      </span>
      {imageUrl && (
        <span
          className="h-12 w-20 shrink-0 rounded border border-border/70 bg-muted bg-cover bg-center"
          style={{ backgroundImage: `url("${imageUrl.replace(/"/g, '\\"')}")` }}
          aria-hidden
        />
      )}
    </div>
  )

  return (
    <NodeViewWrapper data-rich-embed-wrapper>
      {url ? (
        <a
          className="not-prose block"
          href={url}
          target="_blank"
          rel="noreferrer"
          contentEditable={false}
          onClick={(event) => {
            if (editor.isEditable) event.preventDefault()
          }}
        >
          {card}
        </a>
      ) : (
        <div className="not-prose" contentEditable={false}>{card}</div>
      )}
    </NodeViewWrapper>
  )
}
