import { useMemo } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { timeAgo } from '@/lib/utils'
import { FileText, PenLine, User } from 'lucide-react'
import { useTitle } from '@/hooks/useTitle'
import { useWorkspaceStore } from '@/stores/workspaceStore'
import { useAuthStore } from '@/stores/authStore'
import { useDocsDocuments } from '@/hooks/queries'
import { Badge } from '@/components/ui/badge'
import type { DocsDocument } from '@/lib/docsTypes'
import { DOC_TYPE_LABELS, DOC_STATUS_LABELS } from '@/lib/docsTypes'

function statusVariant(status: string): 'default' | 'secondary' | 'outline' {
  switch (status) {
    case 'published':
      return 'default'
    case 'archived':
      return 'outline'
    default:
      return 'secondary'
  }
}

interface DocsDocumentListProps {
  title: string
  description: string
  filterMode: 'my' | 'drafts'
}

export function DocsDocumentList({ title, description, filterMode }: DocsDocumentListProps) {
  useTitle(title)
  const navigate = useNavigate()
  const workspace = useWorkspaceStore((s) => s.currentWorkspace)
  const user = useAuthStore((s) => s.user)
  const wsId = workspace?.id ?? ''
  const wsSlug = workspace?.slug ?? ''

  const filters = useMemo(() => {
    if (filterMode === 'drafts') return { status: 'draft' }
    if (filterMode === 'my' && user?.id) return { owner_id: user.id }
    return {}
  }, [filterMode, user?.id])

  const { data: documents, isLoading } = useDocsDocuments(wsId, filters)

  const emptyIcon = filterMode === 'my' ? User : PenLine
  const EmptyIcon = emptyIcon

  if (!workspace) {
    return <p className="text-sm text-muted-foreground">Workspace not found.</p>
  }

  return (
    <div className="space-y-4">
      <header>
        <h2 className="text-xl font-semibold">{title}</h2>
        <p className="text-sm text-muted-foreground">{description}</p>
      </header>

      {isLoading ? (
        <div className="space-y-2 py-4">
          {[1, 2, 3, 4].map((i) => (
            <div key={i} className="h-10 animate-pulse rounded-md bg-muted/60" />
          ))}
        </div>
      ) : !documents || documents.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-16 px-4">
          <div className="flex h-14 w-14 items-center justify-center rounded-full bg-muted/40 mb-5">
            <EmptyIcon className="h-7 w-7 text-muted-foreground/60" />
          </div>
          <h3 className="text-lg font-semibold mb-1.5">
            {filterMode === 'my' ? 'No documents yet' : 'No drafts'}
          </h3>
          <p className="text-sm text-muted-foreground text-center max-w-md">
            {filterMode === 'my'
              ? 'Documents you create or own will appear here.'
              : 'Draft documents will appear here until they are published.'}
          </p>
        </div>
      ) : (
        <div className="rounded-lg border border-border/60 bg-card divide-y divide-border/40">
          {/* Table header */}
          <div className="flex items-center gap-3 px-4 py-2 text-[11px] font-medium uppercase tracking-wide text-muted-foreground">
            <span className="min-w-0 flex-1">Title</span>
            <span className="w-28 shrink-0">Type</span>
            <span className="w-20 shrink-0">Status</span>
            <span className="w-20 shrink-0 text-right">Updated</span>
          </div>
          {documents.map((doc: DocsDocument) => (
            <button
              key={doc.id}
              type="button"
              onClick={() =>
                navigate({
                  to: '/w/$slug/docs/documents/$docId',
                  params: { slug: wsSlug, docId: doc.id },
                })
              }
              className="flex w-full items-center gap-3 px-4 py-2.5 text-left text-sm transition-colors hover:bg-muted/40"
            >
              <FileText className="h-4 w-4 shrink-0 text-muted-foreground" />
              <span className="min-w-0 flex-1 truncate font-medium">{doc.title}</span>
              <span className="w-28 shrink-0 text-xs text-muted-foreground">
                {DOC_TYPE_LABELS[doc.doc_type] ?? doc.doc_type}
              </span>
              <span className="w-20 shrink-0">
                <Badge
                  variant={statusVariant(doc.status)}
                  className="text-[10px] px-1.5 py-0"
                >
                  {DOC_STATUS_LABELS[doc.status] ?? doc.status}
                </Badge>
              </span>
              <span className="w-20 shrink-0 text-right text-xs text-muted-foreground">
                {timeAgo(doc.updated_at)}
              </span>
            </button>
          ))}
        </div>
      )}
    </div>
  )
}
