import { NodeViewWrapper, type NodeViewProps } from '@tiptap/react'
import {
  BookOpen01Icon,
  LinkSquare01Icon,
  LockIcon,
  Message01Icon,
  SourceCodeIcon,
} from '@/lib/icons'
import { cn } from '@/lib/utils'
import type { CitationSourceRef } from './CitationBlockExtension'

function normalizeSources(value: unknown): CitationSourceRef[] {
  return Array.isArray(value)
    ? value.filter((source): source is CitationSourceRef => {
        if (!source || typeof source !== 'object') return false
        const ref = source as Record<string, unknown>
        return typeof ref.sourceType === 'string' && typeof ref.sourceId === 'string'
      })
    : []
}

function sourceLabel(sourceType: string) {
  switch (sourceType) {
    case 'support_conversation':
      return 'Support conversation'
    case 'docs_chunk':
      return 'Docs chunk'
    default:
      return 'Source'
  }
}

function sourceHref(source: CitationSourceRef, workspaceSlug?: string) {
  if (source.url) return source.url
  if (!workspaceSlug) return undefined
  if (source.sourceType === 'support_conversation') {
    const id = source.conversationId || source.sourceId
    return id ? `/w/${workspaceSlug}/support/${id}` : undefined
  }
  if (source.sourceType === 'docs_chunk' && source.documentId) {
    return `/w/${workspaceSlug}/docs/documents/${source.documentId}`
  }
  return undefined
}

function SourceIcon({ sourceType, className }: { sourceType: string; className?: string }) {
  if (sourceType === 'support_conversation') return <Message01Icon className={className} />
  if (sourceType === 'docs_chunk') return <BookOpen01Icon className={className} />
  return <SourceCodeIcon className={className} />
}

function formatConfidence(value: unknown) {
  if (typeof value !== 'number' || Number.isNaN(value)) return null
  const normalized = value > 1 ? value : value * 100
  return `${Math.round(Math.max(0, Math.min(100, normalized)))}%`
}

export function CitationBlockNodeView({ node, extension, editor }: NodeViewProps) {
  const title = typeof node.attrs.title === 'string' && node.attrs.title.trim()
    ? node.attrs.title.trim()
    : 'Sources'
  const sources = normalizeSources(node.attrs.sources)
  const workspaceSlug = extension.options.workspaceSlug as string | undefined
  const editable = editor.isEditable

  return (
    <NodeViewWrapper>
      <section
        className={cn(
          'my-4 rounded-md border border-border bg-muted/20 px-3 py-3 text-sm',
          editable && 'cursor-grab',
        )}
        data-citation-block=""
        contentEditable={false}
      >
        <div className="mb-2 flex items-center gap-2 text-xs font-medium uppercase tracking-normal text-muted-foreground">
          <SourceCodeIcon className="h-3.5 w-3.5" />
          <span>{title}</span>
          {sources.length > 0 && (
            <span className="rounded-full bg-background px-1.5 py-0.5 text-[10px] normal-case text-muted-foreground">
              {sources.length}
            </span>
          )}
        </div>
        {sources.length === 0 ? (
          <div className="rounded border border-dashed border-border bg-background/60 px-3 py-2 text-xs text-muted-foreground">
            No sources attached
          </div>
        ) : (
          <ol className="space-y-2">
            {sources.map((source, index) => {
              const access = source.access || 'unknown'
              const redacted = access === 'redacted'
              const href = redacted ? undefined : sourceHref(source, workspaceSlug)
              const titleText = redacted ? 'Restricted source' : (source.title || sourceLabel(source.sourceType))
              const confidence = formatConfidence(source.confidence)

              return (
                <li
                  key={`${source.sourceType}:${source.sourceId}:${index}`}
                  className="rounded border border-border/70 bg-background px-3 py-2"
                >
                  <div className="flex min-w-0 items-start gap-2">
                    <div className="mt-0.5 flex h-6 w-6 shrink-0 items-center justify-center rounded border border-border bg-muted/40 text-muted-foreground">
                      {redacted ? <LockIcon className="h-3.5 w-3.5" /> : <SourceIcon sourceType={source.sourceType} className="h-3.5 w-3.5" />}
                    </div>
                    <div className="min-w-0 flex-1">
                      <div className="flex min-w-0 flex-wrap items-center gap-1.5">
                        {href ? (
                          <a
                            href={href}
                            className="inline-flex min-w-0 items-center gap-1 font-medium text-foreground hover:underline"
                            onClick={(event) => {
                              if (editor.isEditable) event.preventDefault()
                            }}
                          >
                            <span className="truncate">{titleText}</span>
                            <LinkSquare01Icon className="h-3 w-3 shrink-0 text-muted-foreground" />
                          </a>
                        ) : (
                          <span className="min-w-0 truncate font-medium text-foreground">{titleText}</span>
                        )}
                        <span className="rounded bg-muted px-1.5 py-0.5 text-[10px] text-muted-foreground">
                          {sourceLabel(source.sourceType)}
                        </span>
                        {confidence && !redacted && (
                          <span className="rounded bg-emerald-50 px-1.5 py-0.5 text-[10px] text-emerald-700 dark:bg-emerald-950/30 dark:text-emerald-300">
                            {confidence}
                          </span>
                        )}
                      </div>
                      {redacted ? (
                        <p className="mt-1 text-xs text-muted-foreground">
                          Hidden because this viewer cannot access the underlying source.
                        </p>
                      ) : source.excerpt ? (
                        <p className="mt-1 line-clamp-2 text-xs leading-5 text-muted-foreground">
                          {source.excerpt}
                        </p>
                      ) : null}
                    </div>
                  </div>
                </li>
              )
            })}
          </ol>
        )}
      </section>
    </NodeViewWrapper>
  )
}
