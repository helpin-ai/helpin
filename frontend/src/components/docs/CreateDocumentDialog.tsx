import { useEffect, useState } from 'react'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { useCreateDocsDocument, useDocsSpaces, useDocsCollections } from '@/hooks/queries'
import { ICON_MAP } from '@/components/ui/icon-picker'
import { toast } from 'sonner'
import { CollectionTreePicker } from '@/components/docs/CollectionTreePicker'

interface CreateDocumentDialogProps {
  wsId: string
  open: boolean
  onOpenChange: (open: boolean) => void
  defaultSpaceId?: string
  defaultCollectionId?: string
  onCreated?: (docId: string) => void
}

export function CreateDocumentDialog({
  wsId,
  open,
  onOpenChange,
  defaultSpaceId,
  defaultCollectionId,
  onCreated,
}: CreateDocumentDialogProps) {
  const [title, setTitle] = useState('')
  const [spaceId, setSpaceId] = useState(defaultSpaceId ?? '')
  const [collectionId, setCollectionId] = useState(defaultCollectionId ?? '')
  const createDocument = useCreateDocsDocument(wsId)
  const { data: spaces } = useDocsSpaces(wsId)
  const { data: collections } = useDocsCollections(wsId, spaceId || '')
  const currentSpace = spaces?.find((s) => s.id === spaceId)

  useEffect(() => {
    if (!open) return
    setTitle('')
    setSpaceId(defaultSpaceId ?? (spaces?.[0]?.id ?? ''))
    setCollectionId(defaultCollectionId ?? '')
  }, [defaultCollectionId, defaultSpaceId, open, spaces])

  useEffect(() => {
    if (!open || spaceId || !spaces?.length) return
    setSpaceId(defaultSpaceId ?? spaces[0].id)
  }, [defaultSpaceId, open, spaceId, spaces])

  // Auto-select first collection when collections load for the current space
  // Only if no default was provided and we're not already set
  useEffect(() => {
    if (defaultCollectionId) return
    if (collections && collections.length > 0) {
      setCollectionId(collections[0].id)
    } else {
      setCollectionId('')
    }
  }, [collections, defaultCollectionId])

  const reset = () => {
    setTitle('')
    setSpaceId(defaultSpaceId ?? (spaces?.[0]?.id ?? ''))
    setCollectionId(defaultCollectionId ?? '')
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!title.trim() || !spaceId) return

    try {
      const doc = await createDocument.mutateAsync({
        title: title.trim(),
        space_id: spaceId,
        collection_id: collectionId || undefined,
      })
      toast.success('Document created')
      reset()
      onOpenChange(false)
      onCreated?.(doc.id)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to create document')
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <form onSubmit={handleSubmit}>
          <DialogHeader>
            <DialogTitle>Create Document</DialogTitle>
            <DialogDescription>
              Add a new document to your documentation.
            </DialogDescription>
          </DialogHeader>

          <div className="grid gap-4 py-4">
            <div className="grid gap-2">
              <Label htmlFor="doc-title">Title</Label>
              <Input
                id="doc-title"
                value={title}
                onChange={(e) => setTitle(e.target.value)}
                placeholder="e.g. Getting Started Guide"
                autoFocus
              />
            </div>

            <div className="grid gap-2">
              <Label>Space</Label>
              <Select value={spaceId} onValueChange={(v) => { setSpaceId(v); setCollectionId(''); }}>
                <SelectTrigger>
                  <SelectValue placeholder="Select a space" />
                </SelectTrigger>
                <SelectContent>
                  {(spaces ?? []).map((s) => {
                    const SpaceIcon = s.icon ? ICON_MAP[s.icon] : null;
                    return (
                      <SelectItem key={s.id} value={s.id}>
                        <span className="inline-flex items-center gap-1.5">
                          {SpaceIcon && <SpaceIcon className="h-4 w-4 shrink-0" />}
                          <span>{s.name}</span>
                        </span>
                      </SelectItem>
                    );
                  })}
                </SelectContent>
              </Select>
            </div>

            {spaceId && (
              <div className="grid gap-2">
                <Label>Collection</Label>
                <CollectionTreePicker
                  collections={collections ?? []}
                  spaceId={spaceId}
                  value={collectionId || null}
                  onChange={(next) => setCollectionId(next ?? '')}
                  allowNone={currentSpace?.type !== 'external_capable'}
                  noneLabel="Uncategorized"
                  placeholder="Select collection…"
                />
                {currentSpace?.type === 'external_capable' && !collectionId && (
                  <p className="text-[11px] text-amber-600 dark:text-amber-400">
                    A collection is required to publish to the help center.
                  </p>
                )}
              </div>
            )}

          </div>

          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
              Cancel
            </Button>
            <Button
              type="submit"
              disabled={!title.trim() || !spaceId || createDocument.isPending}
            >
              {createDocument.isPending ? 'Creating...' : 'Create Document'}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
