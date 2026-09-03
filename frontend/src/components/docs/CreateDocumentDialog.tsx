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
import { StoredIcon } from '@/components/ui/icon-picker'
import { toast } from 'sonner'
import { CollectionTreePicker } from '@/components/docs/CollectionTreePicker'
import { UpgradeRequiredDialog } from '@/components/billing/UpgradeRequiredDialog'
import { getUpgradeRequiredReason, type UpgradeRequiredReason } from '@/lib/upgradeRequired'

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
  const [upgradeDialogReason, setUpgradeDialogReason] = useState<UpgradeRequiredReason | null>(null)

  return (
    <>
      <Dialog open={open} onOpenChange={onOpenChange}>
        <DialogContent>
          {/* Mounted only while open so every open starts with fresh form state. */}
          {open ? (
            <CreateDocumentForm
              wsId={wsId}
              defaultSpaceId={defaultSpaceId}
              defaultCollectionId={defaultCollectionId}
              onOpenChange={onOpenChange}
              onCreated={onCreated}
              onUpgradeRequired={setUpgradeDialogReason}
            />
          ) : null}
        </DialogContent>
      </Dialog>
      <UpgradeRequiredDialog
        open={upgradeDialogReason !== null}
        onOpenChange={(nextOpen) => {
          if (!nextOpen) setUpgradeDialogReason(null)
        }}
        onUpgrade={() => onOpenChange(false)}
        reason={upgradeDialogReason}
      />
    </>
  )
}

interface CreateDocumentFormProps {
  wsId: string
  defaultSpaceId?: string
  defaultCollectionId?: string
  onOpenChange: (open: boolean) => void
  onCreated?: (docId: string) => void
  onUpgradeRequired: (reason: UpgradeRequiredReason) => void
}

function CreateDocumentForm({
  wsId,
  defaultSpaceId,
  defaultCollectionId,
  onOpenChange,
  onCreated,
  onUpgradeRequired,
}: CreateDocumentFormProps) {
  const [title, setTitle] = useState('')
  // `undefined` means the user has not picked yet, so the default / first
  // option applies. Once a choice is made it is never overwritten by data
  // refetches.
  const [spaceChoice, setSpaceChoice] = useState<string | undefined>(undefined)
  // `null` is an explicit "Uncategorized" pick; `undefined` means auto.
  const [collectionChoice, setCollectionChoice] = useState<string | null | undefined>(defaultCollectionId)
  const createDocument = useCreateDocsDocument(wsId)
  const { data: spaces } = useDocsSpaces(wsId)

  const spaceId = spaceChoice ?? defaultSpaceId ?? spaces?.[0]?.id ?? ''
  const { data: collections } = useDocsCollections(wsId, spaceId)
  const currentSpace = spaces?.find((s) => s.id === spaceId)

  // Auto-select the first collection of the current space, but only when no
  // default collection was provided (matches the previous behavior).
  const autoSelectCollection = !defaultCollectionId
  const collectionId = collectionChoice === undefined
    ? (autoSelectCollection ? (collections?.[0]?.id ?? '') : '')
    : (collectionChoice ?? '')

  const handleSpaceChange = (next: string) => {
    setSpaceChoice(next)
    setCollectionChoice(undefined)
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
      onOpenChange(false)
      onCreated?.(doc.id)
    } catch (err) {
      const reason = getUpgradeRequiredReason(err)
      if (reason) {
        onUpgradeRequired(reason)
        return
      }
      toast.error(err instanceof Error ? err.message : 'Failed to create document')
    }
  }

  return (
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
          <Select value={spaceId} onValueChange={handleSpaceChange}>
            <SelectTrigger>
              <SelectValue placeholder="Select a space" />
            </SelectTrigger>
            <SelectContent>
              {(spaces ?? []).map((s) => {
                return (
                  <SelectItem key={s.id} value={s.id}>
                    <span className="inline-flex items-center gap-1.5">
                      {s.icon ? <StoredIcon name={s.icon} className="h-4 w-4 shrink-0" /> : null}
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
              onChange={(next) => setCollectionChoice(next)}
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
  )
}
