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
import { CollectionTreePicker } from './CollectionTreePicker'

interface MoveDocumentDialogProps {
  wsId: string
  open: boolean
  onOpenChange: (open: boolean) => void
  docId: string
  docTitle: string
  currentSpaceId: string
  currentCollectionId?: string | null
}

export function MoveDocumentDialog({
  wsId,
  open,
  onOpenChange,
  docId,
  docTitle,
  currentSpaceId,
  currentCollectionId,
}: MoveDocumentDialogProps) {
  const [spaceId, setSpaceId] = useState(currentSpaceId)
  const [collectionId, setCollectionId] = useState<string | null>(currentCollectionId ?? null)

  const { data: spaces } = useDocsSpaces(wsId)
  const { data: collections } = useDocsCollections(wsId, spaceId)
  const moveDoc = useMoveDocsDocument(wsId)

  useEffect(() => {
    if (open) {
      setSpaceId(currentSpaceId)
      setCollectionId(currentCollectionId ?? null)
    }
  }, [open, currentSpaceId, currentCollectionId])

  // Reset collection when space changes.
  useEffect(() => {
    if (spaceId !== currentSpaceId) {
      setCollectionId(null)
    }
  }, [spaceId, currentSpaceId])

  const hasChanged = spaceId !== currentSpaceId || collectionId !== (currentCollectionId ?? null)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
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
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <form onSubmit={handleSubmit}>
          <DialogHeader>
            <DialogTitle>Move document</DialogTitle>
            <DialogDescription>
              Move "{docTitle}" to a different space or collection.
            </DialogDescription>
          </DialogHeader>

          <div className="grid gap-4 py-4">
            <div className="grid gap-2">
              <Label>Space</Label>
              <Select value={spaceId} onValueChange={setSpaceId}>
                <SelectTrigger>
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
                collections={collections ?? []}
                spaceId={spaceId}
                value={collectionId}
                onChange={setCollectionId}
                noneLabel="None (Uncategorized)"
              />
            </div>
          </div>

          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
              Cancel
            </Button>
            <Button type="submit" disabled={!hasChanged || moveDoc.isPending}>
              {moveDoc.isPending ? 'Moving...' : 'Move'}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
