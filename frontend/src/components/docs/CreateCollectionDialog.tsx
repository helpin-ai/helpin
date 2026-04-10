import { useEffect, useState } from 'react'
import { Cancel01Icon } from '@/lib/icons'
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
import { Textarea } from '@/components/ui/textarea'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { IconPicker } from '@/components/ui/icon-picker'
import { StoredIcon } from '@/components/ui/icon-picker'
import { CollectionTreePicker } from '@/components/docs/CollectionTreePicker'
import {
  useCreateDocsCollection,
  useDocsCollections,
  useDocsSpaces,
  useUpdateDocsCollection,
} from '@/hooks/queries'
import type { DocsCollection } from '@/lib/docsTypes'
import { toast } from 'sonner'

interface CreateCollectionDialogProps {
  wsId: string
  spaceId?: string
  open: boolean
  onOpenChange: (open: boolean) => void
  collection?: DocsCollection | null
  /**
   * When provided, the dialog opens with this collection preselected
   * as the parent so the new row lands as a sub-collection. Callers
   * like the Arrange tree's "Add sub-collection" button pass the
   * parent collection's id here.
   */
  defaultParentCollectionId?: string | null
}

export function CreateCollectionDialog({
  wsId,
  spaceId: defaultSpaceId,
  open,
  onOpenChange,
  collection,
  defaultParentCollectionId,
}: CreateCollectionDialogProps) {
  const isEdit = !!collection
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [icon, setIcon] = useState('')
  const [selectedSpaceId, setSelectedSpaceId] = useState(defaultSpaceId ?? '')
  const [parentCollectionId, setParentCollectionId] = useState<string | null>(null)

  const { data: spaces } = useDocsSpaces(wsId)

  useEffect(() => {
    if (!open) return

    if (collection) {
      setName(collection.name)
      setDescription(collection.description ?? '')
      setIcon(collection.icon ?? '')
      setSelectedSpaceId(collection.space_id)
      setParentCollectionId(collection.parent_collection_id ?? null)
      return
    }

    setName('')
    setDescription('')
    setIcon('folder')
    setSelectedSpaceId(defaultSpaceId ?? (spaces?.[0]?.id ?? ''))
    setParentCollectionId(defaultParentCollectionId ?? null)
  }, [open, collection, defaultSpaceId, defaultParentCollectionId, spaces])

  useEffect(() => {
    if (!open || collection || selectedSpaceId || !spaces?.length) return
    setSelectedSpaceId(defaultSpaceId ?? spaces[0].id)
  }, [collection, defaultSpaceId, open, selectedSpaceId, spaces])

  const effectiveSpaceId = collection?.space_id ?? (selectedSpaceId || '')
  const currentSpace = spaces?.find((space) => space.id === effectiveSpaceId)
  const createCollection = useCreateDocsCollection(wsId, effectiveSpaceId)
  const updateCollection = useUpdateDocsCollection(wsId)

  // Load the collection list for the selected space so the parent
  // picker can show the tree the user is adding to. The hook is safe
  // to call with an empty spaceId (it just returns nothing).
  const { data: spaceCollections } = useDocsCollections(wsId, effectiveSpaceId)
  // Remove the collection being edited from the parent choices so a
  // user cannot accidentally reparent it under itself from the edit
  // dialog. The service layer also rejects this case.
  const parentCandidates = (spaceCollections ?? []).filter(
    (c) => !collection || c.id !== collection.id,
  )

  const reset = () => {
    setName('')
    setDescription('')
    setIcon('folder')
    setSelectedSpaceId(defaultSpaceId ?? (spaces?.[0]?.id ?? ''))
    setParentCollectionId(null)
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!name.trim() || !effectiveSpaceId) return

    try {
      if (isEdit && collection) {
        // Only send parent_collection_id when it actually changed.
        // The empty-string sentinel means "move to top-level"; nil
        // means leave the parent alone. Matching values short-circuit.
        let parentUpdate: string | null | undefined = undefined
        if (parentCollectionId !== (collection.parent_collection_id ?? null)) {
          parentUpdate = parentCollectionId ?? ''
        }
        await updateCollection.mutateAsync({
          id: collection.id,
          spaceId: collection.space_id,
          name: name.trim(),
          description: description.trim(),
          icon: icon.trim(),
          ...(parentUpdate !== undefined ? { parent_collection_id: parentUpdate } : {}),
        })
        toast.success('Collection updated')
      } else {
        await createCollection.mutateAsync({
          name: name.trim(),
          description: description.trim() || undefined,
          icon: icon.trim() || undefined,
          parent_collection_id: parentCollectionId ?? undefined,
        })
        toast.success('Collection created')
      }
      reset()
      onOpenChange(false)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : `Failed to ${isEdit ? 'update' : 'create'} collection`)
    }
  }

  const isPending = isEdit ? updateCollection.isPending : createCollection.isPending

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <form onSubmit={handleSubmit}>
          <DialogHeader>
            <DialogTitle>{isEdit ? 'Edit Collection' : 'Create Collection'}</DialogTitle>
            <DialogDescription>
              {isEdit
                ? 'Update the collection name, icon, and description.'
                : 'Collections group related documents within a space.'}
            </DialogDescription>
          </DialogHeader>

          <div className="grid gap-4 py-4">
            <div className="flex items-end gap-2">
              <div className="grid gap-2">
                <Label>Icon</Label>
                <IconPicker value={icon} onChange={setIcon} />
              </div>
              <div className="grid flex-1 gap-2">
                <Label htmlFor="collection-name">Name</Label>
                <Input
                  id="collection-name"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  placeholder="e.g. Getting Started"
                  autoFocus
                />
              </div>
              {icon && (
                <button
                  type="button"
                  onClick={() => setIcon('')}
                  className="mb-0.5 flex h-6 w-6 shrink-0 items-center justify-center rounded-full text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
                  title="Remove icon"
                >
                  <Cancel01Icon className="h-3.5 w-3.5" />
                </button>
              )}
            </div>
            <div className="grid gap-2">
              <Label htmlFor="collection-desc">Description</Label>
              <Textarea
                id="collection-desc"
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                placeholder="Optional description"
                rows={2}
              />
            </div>
            {isEdit ? (
              <div className="grid gap-2">
                <Label>Space</Label>
                <div className="rounded-md border border-border/60 bg-muted/30 px-3 py-2 text-sm text-muted-foreground">
                  <span className="inline-flex items-center gap-1">
                    <StoredIcon name={currentSpace?.icon} className="h-4 w-4 shrink-0" textClassName="" />
                    <span>{currentSpace?.name ?? 'Current space'}</span>
                  </span>
                </div>
              </div>
            ) : (
              <div className="grid gap-2">
                <Label>Space</Label>
                <Select value={selectedSpaceId} onValueChange={setSelectedSpaceId}>
                  <SelectTrigger>
                    <SelectValue placeholder="Select a space" />
                  </SelectTrigger>
                  <SelectContent>
                    {(spaces ?? []).map((s) => (
                      <SelectItem key={s.id} value={s.id}>
                        <span className="inline-flex items-center gap-1">
                          <StoredIcon name={s.icon} className="h-4 w-4 shrink-0" textClassName="" />
                          <span>{s.name}</span>
                        </span>
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
            )}

            {effectiveSpaceId && (
              <div className="grid gap-2">
                <Label>Parent collection</Label>
                <CollectionTreePicker
                  collections={parentCandidates}
                  spaceId={effectiveSpaceId}
                  value={parentCollectionId}
                  onChange={setParentCollectionId}
                  noneLabel="None (top-level)"
                />
                <p className="text-[11px] text-muted-foreground">
                  Pick an existing collection to make this a sub-collection. Max 3 levels.
                </p>
              </div>
            )}
          </div>

          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
              Cancel
            </Button>
            <Button type="submit" disabled={!name.trim() || !effectiveSpaceId || isPending}>
              {isPending ? (isEdit ? 'Saving...' : 'Creating...') : isEdit ? 'Save Changes' : 'Create Collection'}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
