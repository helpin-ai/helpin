import { useState } from 'react'
import { Globe, GlobeLock } from 'lucide-react'
import { toast } from 'sonner'
import {
  usePublishDocsExternally,
  useUnpublishDocsExternally,
} from '@/hooks/queries'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetDescription,
} from '@/components/ui/sheet'
import type { DocsDocument } from '@/lib/docsTypes'

interface ExternalPublishPanelProps {
  wsId: string
  doc: DocsDocument
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function ExternalPublishPanel({
  wsId,
  doc,
  open,
  onOpenChange,
}: ExternalPublishPanelProps) {
  const publishExternally = usePublishDocsExternally(wsId)
  const unpublishExternally = useUnpublishDocsExternally(wsId)

  const [slug, setSlug] = useState('')

  const isExternallyPublished = doc.doc_type === 'help_center_article' && doc.status === 'published'
  // Note: We don't have public_published_at on the frontend DocsDocument type currently,
  // but we can check doc_type and status as proxies. The backend enforces eligibility.

  const handlePublish = async () => {
    try {
      await publishExternally.mutateAsync({
        docId: doc.id,
        slug: slug.trim() || undefined,
      })
      toast.success('Published externally')
      setSlug('')
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to publish externally')
    }
  }

  const handleUnpublish = async () => {
    if (!window.confirm('Remove this article from the public help center?')) return
    try {
      await unpublishExternally.mutateAsync(doc.id)
      toast.success('Unpublished from help center')
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to unpublish')
    }
  }

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className="w-80 sm:w-96">
        <SheetHeader>
          <SheetTitle className="flex items-center gap-2">
            <Globe className="h-4 w-4" />
            External Publishing
          </SheetTitle>
          <SheetDescription>
            Publish this document to your public help center.
          </SheetDescription>
        </SheetHeader>

        <div className="mt-4 space-y-4">
          {doc.doc_type !== 'help_center_article' ? (
            <div className="rounded-md border border-border/60 bg-muted/30 p-4 text-center">
              <GlobeLock className="mx-auto h-8 w-8 text-muted-foreground/40 mb-2" />
              <p className="text-sm text-muted-foreground">
                Only help center articles can be published externally.
              </p>
              <p className="mt-1 text-xs text-muted-foreground">
                Change the document type to "Help Center Article" first.
              </p>
            </div>
          ) : doc.status !== 'published' ? (
            <div className="rounded-md border border-border/60 bg-muted/30 p-4 text-center">
              <GlobeLock className="mx-auto h-8 w-8 text-muted-foreground/40 mb-2" />
              <p className="text-sm text-muted-foreground">
                This document must be published internally before it can be made public.
              </p>
            </div>
          ) : (
            <>
              <div className="grid gap-2">
                <Label className="text-xs">Custom URL Slug (optional)</Label>
                <Input
                  className="h-8 text-xs"
                  placeholder="e.g. getting-started"
                  value={slug}
                  onChange={(e) => setSlug(e.target.value)}
                />
                <p className="text-[10px] text-muted-foreground">
                  Leave blank to auto-generate from the document title.
                </p>
              </div>

              <div className="flex gap-2">
                <Button
                  size="sm"
                  className="flex-1 gap-1.5"
                  onClick={handlePublish}
                  disabled={publishExternally.isPending}
                >
                  <Globe className="h-3.5 w-3.5" />
                  {publishExternally.isPending ? 'Publishing...' : 'Publish Externally'}
                </Button>
              </div>

              {isExternallyPublished && (
                <Button
                  variant="outline"
                  size="sm"
                  className="w-full text-destructive hover:text-destructive"
                  onClick={handleUnpublish}
                  disabled={unpublishExternally.isPending}
                >
                  Unpublish from Help Center
                </Button>
              )}
            </>
          )}
        </div>
      </SheetContent>
    </Sheet>
  )
}
