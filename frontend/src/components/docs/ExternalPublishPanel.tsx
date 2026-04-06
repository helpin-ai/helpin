import { useState } from 'react'
import { GlobeIcon } from '@/lib/icons'
import { toast } from 'sonner'
import { useConfirm } from '@/components/ui/confirm-dialog'
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
  isExternalCapable: boolean
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function ExternalPublishPanel({
  wsId,
  doc,
  isExternalCapable,
  open,
  onOpenChange,
}: ExternalPublishPanelProps) {
  const publishExternally = usePublishDocsExternally(wsId)
  const unpublishExternally = useUnpublishDocsExternally(wsId)

  const [slug, setSlug] = useState('')

  const isExternallyPublished = isExternalCapable && doc.status === 'published'

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

  const confirm = useConfirm()

  const handleUnpublish = async () => {
    const ok = await confirm({
      title: 'Unpublish article?',
      description: 'This will remove the article from the public help center.',
      confirmText: 'Unpublish',
      variant: 'destructive',
    })
    if (!ok) return
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
            <GlobeIcon className="h-4 w-4" />
            External Publishing
          </SheetTitle>
          <SheetDescription>
            Publish this document to your public help center.
          </SheetDescription>
        </SheetHeader>

        <div className="mt-4 space-y-4">
          {!isExternalCapable ? (
            <div className="rounded-md border border-border/60 bg-muted/30 p-4 text-center">
              <GlobeIcon className="mx-auto h-8 w-8 text-muted-foreground/40 mb-2" />
              <p className="text-sm text-muted-foreground">
                Only documents in external-capable spaces can be published.
              </p>
              <p className="mt-1 text-xs text-muted-foreground">
                Move this document to a help center space first.
              </p>
            </div>
          ) : doc.status !== 'published' ? (
            <div className="rounded-md border border-border/60 bg-muted/30 p-4 text-center">
              <GlobeIcon className="mx-auto h-8 w-8 text-muted-foreground/40 mb-2" />
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
                  <GlobeIcon className="h-3.5 w-3.5" />
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
