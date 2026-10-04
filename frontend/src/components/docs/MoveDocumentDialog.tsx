import { useState } from 'react'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { useDocsSpaces, useDocsCollections, useMoveDocsDocument } from '@/hooks/queries'
import { toast } from 'sonner'
import type { MoveDocsDocumentRequest } from '@/lib/docsTypes'
import { CollectionTreePicker } from './CollectionTreePicker'

interface MoveDocumentDialogProps {
  wsId: string
  open: boolean
  onOpenChange: (open: boolean) => void
  docId?: string
  docTitle: string
  currentSpaceId: string
  currentCollectionId?: string | null
  onMove?: (destination: MoveDocsDocumentRequest) => Promise<void>
  documentCount?: number
  pending?: boolean
}

export function MoveDocumentDialog(props: MoveDocumentDialogProps) {
  return props.open ? <MoveDocumentDialogContent
    key={`${props.docId ?? 'bulk'}:${props.currentSpaceId}:${props.currentCollectionId ?? ''}`}
    {...props}
  /> : null
}

function MoveDocumentDialogContent({
  wsId,
  open,
  onOpenChange,
  docId,
  docTitle,
  currentSpaceId,
  currentCollectionId,
  onMove,
  documentCount,
  pending = false,
}: MoveDocumentDialogProps) {
  const [spaceId, setSpaceId] = useState(currentSpaceId)
  const [collectionId, setCollectionId] = useState<string | null>(currentCollectionId ?? null)

  const { data: spaces } = useDocsSpaces(wsId)
  const { data: collections } = useDocsCollections(wsId, spaceId)
  const moveDoc = useMoveDocsDocument(wsId)

  const isPending = pending || moveDoc.isPending
  const hasChanged = spaceId !== currentSpaceId || collectionId !== (currentCollectionId ?? null)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (isPending || !spaceId) return
    const destination = { space_id: spaceId, collection_id: collectionId ?? undefined }
    if (onMove) {
      await onMove(destination)
      onOpenChange(false)
      return
    }
    if (!docId) return
    try {
      await moveDoc.mutateAsync({
        id: docId,
        space_id: spaceId,
        collection_id: collectionId ?? undefined,
      })
      toast.success('Document moved')
      onOpenChange(false)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : 'Failed to move document')
    }
  }

  return (
    <Dialog open={open} onOpenChange={(next) => { if (!isPending) onOpenChange(next) }}>
      <DialogContent>
        <form onSubmit={handleSubmit}>
          <DialogHeader>
            <DialogTitle>{documentCount ? `Move ${documentCount} document${documentCount === 1 ? '' : 's'}` : 'Move document'}</DialogTitle>
            <DialogDescription>
              {documentCount ? 'Choose a destination for the selected documents.' : `Move "${docTitle}" to a different space or collection.`}
            </DialogDescription>
          </DialogHeader>

          <fieldset disabled={isPending} className="grid gap-4 py-4">
            <div className="grid gap-2">
              <Label>Space</Label>
              <Select disabled={isPending} value={spaceId} onValueChange={(value) => { setSpaceId(value); setCollectionId(null) }}>
                <SelectTrigger aria-label="Destination space">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {(spaces ?? []).map((s) => (
                    <SelectItem key={s.id} value={s.id}>
                      {s.icon ?? '📁'} {s.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className="grid gap-2">
              <Label>Collection</Label>
              <CollectionTreePicker
                disabled={isPending}
                collections={collections ?? []}
                spaceId={spaceId}
                value={collectionId}
                onChange={setCollectionId}
                noneLabel="None (Uncategorized)"
              />
            </div>
          </fieldset>

          <DialogFooter>
            <Button type="button" variant="outline" disabled={isPending} onClick={() => onOpenChange(false)}>
              Cancel
            </Button>
            <Button type="submit" disabled={!spaceId || (!onMove && !hasChanged) || isPending}>
              {isPending ? 'Moving...' : 'Move'}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
