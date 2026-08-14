import { useRef, useState } from 'react'
import { Braces, FileUp, Link2, Loader2, Plus, RefreshCw, Trash2 } from 'lucide-react'
import { toast } from 'sonner'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { ConfirmDialog } from '@/components/pm/ConfirmDialog'
import {
  useCreateDocsAPIReference,
  useDeleteDocsAPIReference,
  useDocsAPIReferences,
  usePublishDocsAPIReference,
  useSyncDocsAPIReference,
  useUnpublishDocsAPIReference,
  useUpdateDocsAPIReference,
} from '@/hooks/queries'
import type { DocsAPIReference, DocsAPIReferenceSourceType } from '@/lib/docsTypes'
import { cn } from '@/lib/utils'

interface APIReferenceSectionProps {
  wsId: string
  spaceId: string
  canEdit: boolean
}

interface APIReferenceDialogProps extends APIReferenceSectionProps {
  reference: DocsAPIReference | null
  open: boolean
  onOpenChange: (open: boolean) => void
}

function APIReferenceDialog({
  wsId,
  spaceId,
  canEdit,
  reference,
  open,
  onOpenChange,
}: APIReferenceDialogProps) {
  const [name, setName] = useState(reference?.name ?? '')
  const [sourceType, setSourceType] = useState<DocsAPIReferenceSourceType>(
    reference?.source_type ?? 'url',
  )
  const [sourceURL, setSourceURL] = useState(reference?.source_url ?? '')
  const [specificationText, setSpecificationText] = useState('')
  const [fileName, setFileName] = useState('')
  const fileInputRef = useRef<HTMLInputElement>(null)
  const createReference = useCreateDocsAPIReference(wsId, spaceId)
  const updateReference = useUpdateDocsAPIReference(wsId, spaceId)
  const syncReference = useSyncDocsAPIReference(wsId, spaceId)
  const isEditing = reference !== null

  const handleFile = async (file?: File) => {
    if (!file) return
    if (file.size > 5 * 1024 * 1024) {
      toast.error('OpenAPI files must be 5 MB or smaller.')
      return
    }
    setFileName(file.name)
    setSpecificationText(await file.text())
    if (!name.trim()) {
      setName(file.name.replace(/\.(json|ya?ml)$/i, '').replace(/[-_]+/g, ' '))
    }
  }

  const handleSubmit = async (event: React.FormEvent) => {
    event.preventDefault()
    if (!canEdit || !name.trim()) return
    try {
      if (reference) {
        const sourceChanged = reference.source_type === 'url' && sourceURL.trim() !== reference.source_url
        await updateReference.mutateAsync({
          id: reference.id,
          name: name.trim(),
          ...(reference.source_type === 'url' ? { source_url: sourceURL.trim() } : {}),
          ...(specificationText ? { specification_text: specificationText } : {}),
        })
        if (sourceChanged) {
          await syncReference.mutateAsync(reference.id)
        }
        toast.success('API reference updated')
      } else {
        await createReference.mutateAsync({
          name: name.trim(),
          source_type: sourceType,
          ...(sourceType === 'url'
            ? { source_url: sourceURL.trim() }
            : { specification_text: specificationText }),
        })
        toast.success('API reference imported as a draft')
      }
      onOpenChange(false)
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'Failed to save API reference')
    }
  }

  const effectiveSourceType = reference?.source_type ?? sourceType
  const isPending = createReference.isPending || updateReference.isPending || syncReference.isPending
  const sourceMissing = effectiveSourceType === 'url' ? !sourceURL.trim() : !isEditing && !specificationText

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-lg">
        <form onSubmit={handleSubmit}>
          <DialogHeader>
            <DialogTitle>{isEditing ? 'Edit API reference' : 'Add API reference'}</DialogTitle>
            <DialogDescription>
              Import an OpenAPI 3.0 or 3.1 document. Helpin validates and saves a managed draft before anything is published.
            </DialogDescription>
          </DialogHeader>

          <div className="grid gap-4 py-5">
            <div className="grid gap-2">
              <Label htmlFor="api-reference-name">Name</Label>
              <Input
                id="api-reference-name"
                value={name}
                onChange={(event) => setName(event.target.value)}
                placeholder="Product API"
                autoFocus
              />
            </div>

            {!isEditing && (
              <div className="grid grid-cols-2 gap-2 rounded-2xl bg-muted/40 p-1">
                <button
                  type="button"
                  className={cn(
                    'flex items-center justify-center gap-2 rounded-xl px-3 py-2 text-sm transition-colors',
                    sourceType === 'url' ? 'bg-background font-medium shadow-sm' : 'text-muted-foreground hover:text-foreground',
                  )}
                  onClick={() => setSourceType('url')}
                >
                  <Link2 className="size-4" />
                  OpenAPI URL
                </button>
                <button
                  type="button"
                  className={cn(
                    'flex items-center justify-center gap-2 rounded-xl px-3 py-2 text-sm transition-colors',
                    sourceType === 'upload' ? 'bg-background font-medium shadow-sm' : 'text-muted-foreground hover:text-foreground',
                  )}
                  onClick={() => setSourceType('upload')}
                >
                  <FileUp className="size-4" />
                  Upload file
                </button>
              </div>
            )}

            {effectiveSourceType === 'url' ? (
              <div className="grid gap-2">
                <Label htmlFor="api-reference-url">OpenAPI URL</Label>
                <Input
                  id="api-reference-url"
                  type="url"
                  value={sourceURL}
                  onChange={(event) => setSourceURL(event.target.value)}
                  placeholder="https://api.example.com/openapi.json"
                />
                <p className="text-xs text-muted-foreground">
                  Helpin securely fetches the HTTPS URL when you import or sync.
                </p>
              </div>
            ) : (
              <div className="grid gap-2">
                <Label>{isEditing ? 'Replace OpenAPI file' : 'OpenAPI file'}</Label>
                <input
                  ref={fileInputRef}
                  type="file"
                  accept=".json,.yaml,.yml,application/json,application/yaml,text/yaml"
                  className="hidden"
                  onChange={(event) => void handleFile(event.target.files?.[0])}
                />
                <button
                  type="button"
                  onClick={() => fileInputRef.current?.click()}
                  className="flex min-h-24 flex-col items-center justify-center gap-2 rounded-2xl border border-dashed border-border bg-muted/20 px-4 text-sm transition-colors hover:bg-muted/40"
                >
                  <FileUp className="size-5 text-muted-foreground" />
                  <span>{fileName || (isEditing ? 'Choose a replacement JSON or YAML file' : 'Choose a JSON or YAML file')}</span>
                  <span className="text-xs text-muted-foreground">Up to 5 MB</span>
                </button>
              </div>
            )}
          </div>

          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
              Cancel
            </Button>
            <Button type="submit" disabled={isPending || sourceMissing || !name.trim()}>
              {isPending && <Loader2 className="animate-spin" />}
              {isEditing ? 'Save changes' : 'Import draft'}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

export function APIReferenceSection({ wsId, spaceId, canEdit }: APIReferenceSectionProps) {
  const { data: references = [], isLoading } = useDocsAPIReferences(wsId, spaceId)
  const syncReference = useSyncDocsAPIReference(wsId, spaceId)
  const publishReference = usePublishDocsAPIReference(wsId, spaceId)
  const unpublishReference = useUnpublishDocsAPIReference(wsId, spaceId)
  const deleteReference = useDeleteDocsAPIReference(wsId, spaceId)
  const [dialogOpen, setDialogOpen] = useState(false)
  const [editing, setEditing] = useState<DocsAPIReference | null>(null)
  const [deleting, setDeleting] = useState<DocsAPIReference | null>(null)

  const run = async (action: () => Promise<unknown>, success: string) => {
    try {
      await action()
      toast.success(success)
    } catch (error) {
      toast.error(error instanceof Error ? error.message : 'API reference action failed')
    }
  }

  if (isLoading) {
    return <div className="h-24 animate-pulse rounded-2xl bg-muted/40" />
  }

  return (
    <section className="rounded-2xl border border-border/60 bg-card">
      <div className="flex items-center justify-between gap-3 px-4 py-3">
        <div>
          <h2 className="text-sm font-medium">API reference</h2>
          <p className="text-xs text-muted-foreground">Interactive OpenAPI documentation in this public space.</p>
        </div>
        {canEdit && (
          <Button
            size="sm"
            variant={references.length ? 'outline' : 'default'}
            onClick={() => {
              setEditing(null)
              setDialogOpen(true)
            }}
          >
            <Plus />
            Add reference
          </Button>
        )}
      </div>

      {references.length === 0 ? (
        <div className="border-t border-border/50 px-4 py-5 text-sm text-muted-foreground">
          Import an OpenAPI URL or file to add an interactive API reference beneath your help-center header.
        </div>
      ) : (
        <div className="divide-y divide-border/50 border-t border-border/50">
          {references.map((reference) => {
            const revision = reference.draft_revision
            const isPublished = !!reference.published_revision_id
            const hasUnpublishedChanges = isPublished && reference.draft_revision_id !== reference.published_revision_id
            return (
              <div key={reference.id} className="flex flex-col gap-3 px-4 py-3 lg:flex-row lg:items-center">
                <div className="flex min-w-0 flex-1 items-start gap-3">
                  <div className="mt-0.5 rounded-xl bg-primary/10 p-2 text-primary">
                    <Braces className="size-4" />
                  </div>
                  <div className="min-w-0">
                    <div className="flex flex-wrap items-center gap-2">
                      <span className="truncate text-sm font-medium">{reference.name}</span>
                      <Badge variant={isPublished ? 'secondary' : 'outline'}>
                        {isPublished ? 'Published' : 'Draft'}
                      </Badge>
                      {hasUnpublishedChanges && <Badge variant="outline">Changes pending</Badge>}
                      {reference.sync_status === 'failed' && <Badge variant="destructive">Sync failed</Badge>}
                    </div>
                    <p className="mt-1 truncate text-xs text-muted-foreground">
                      {reference.source_type === 'url' ? reference.source_url : 'Uploaded OpenAPI file'}
                      {revision && ` · OpenAPI ${revision.openapi_version} · ${revision.operation_count} operations`}
                    </p>
                    {reference.last_sync_error && (
                      <p className="mt-1 text-xs text-destructive">{reference.last_sync_error}</p>
                    )}
                  </div>
                </div>

                {canEdit && (
                  <div className="flex flex-wrap items-center gap-1.5">
                    {reference.source_type === 'url' && (
                      <Button
                        size="xs"
                        variant="ghost"
                        disabled={syncReference.isPending}
                        onClick={() => void run(() => syncReference.mutateAsync(reference.id), 'API reference synced')}
                      >
                        <RefreshCw className={cn(syncReference.isPending && 'animate-spin')} />
                        Sync
                      </Button>
                    )}
                    <Button
                      size="xs"
                      variant="ghost"
                      onClick={() => {
                        setEditing(reference)
                        setDialogOpen(true)
                      }}
                    >
                      Edit
                    </Button>
                    {reference.draft_revision_id !== reference.published_revision_id && (
                      <Button
                        size="xs"
                        disabled={publishReference.isPending}
                        onClick={() => void run(() => publishReference.mutateAsync(reference.id), 'API reference published')}
                      >
                        Publish
                      </Button>
                    )}
                    {isPublished && !hasUnpublishedChanges && (
                      <Button
                        size="xs"
                        variant="outline"
                        disabled={unpublishReference.isPending}
                        onClick={() => void run(() => unpublishReference.mutateAsync(reference.id), 'API reference unpublished')}
                      >
                        Unpublish
                      </Button>
                    )}
                    <Button size="icon-xs" variant="ghost" onClick={() => setDeleting(reference)} aria-label="Delete API reference">
                      <Trash2 />
                    </Button>
                  </div>
                )}
              </div>
            )
          })}
        </div>
      )}

      {dialogOpen && (
        <APIReferenceDialog
          wsId={wsId}
          spaceId={spaceId}
          canEdit={canEdit}
          reference={editing}
          open
          onOpenChange={(open) => {
            setDialogOpen(open)
            if (!open) setEditing(null)
          }}
        />
      )}
      <ConfirmDialog
        open={deleting !== null}
        onOpenChange={(open) => { if (!open) setDeleting(null) }}
        title="Delete API reference"
        description="This permanently removes the imported OpenAPI drafts and published snapshot."
        confirmLabel="Delete"
        onConfirm={() => {
          if (!deleting) return
          void run(() => deleteReference.mutateAsync(deleting.id), 'API reference deleted')
          setDeleting(null)
        }}
      />
    </section>
  )
}
