import { useEffect, useState } from 'react'
import { useParams } from '@tanstack/react-router'
import { FileText } from 'lucide-react'
import { useTitle } from '@/hooks/useTitle'
import { docsService } from '@/lib/services/docsService'
import type { DocsDocument, DocsContent } from '@/lib/docsTypes'
import { DocsEditor } from '@/components/docs/DocsEditor'
import type { JSONContent } from '@tiptap/react'

export function SharedDocumentView() {
  const { shareToken } = useParams({ strict: false }) as { shareToken: string }

  const [doc, setDoc] = useState<DocsDocument | null>(null)
  const [content, setContent] = useState<DocsContent | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useTitle(doc?.title ?? 'Shared Document')

  useEffect(() => {
    if (!shareToken) return
    setLoading(true)
    setError(null)

    docsService.getSharedDoc(shareToken).then((res) => {
      if (res.error) {
        setError(res.error)
      } else if (res.data) {
        setDoc(res.data.document)
        setContent(res.data.content)
      } else {
        setError('Document not found')
      }
      setLoading(false)
    })
  }, [shareToken])

  if (loading) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-background">
        <div className="flex flex-col items-center gap-3">
          <div className="h-8 w-8 animate-spin rounded-full border-2 border-muted-foreground/30 border-t-foreground" />
          <p className="text-sm text-muted-foreground">Loading document...</p>
        </div>
      </div>
    )
  }

  if (error || !doc) {
    return (
      <div className="flex min-h-screen flex-col items-center justify-center bg-background p-4">
        <FileText className="h-16 w-16 text-muted-foreground/20 mb-4" />
        <h1 className="text-lg font-medium text-foreground mb-1">Document not available</h1>
        <p className="text-sm text-muted-foreground">
          This document may have been removed or sharing has been disabled.
        </p>
      </div>
    )
  }

  return (
    <div className="min-h-screen bg-background">
      {/* Clean header */}
      <header className="border-b border-border/40 bg-background/80 backdrop-blur-sm sticky top-0 z-10">
        <div className="mx-auto max-w-4xl px-6 py-3">
          <span className="text-xs text-muted-foreground">Shared document</span>
        </div>
      </header>

      {/* Document content */}
      <main className="mx-auto max-w-4xl px-6 py-8">
        <DocsEditor
          title={doc.title}
          initialContent={content?.content as JSONContent | null}
          readOnly
          onSave={async () => {}}
        />
      </main>
    </div>
  )
}
