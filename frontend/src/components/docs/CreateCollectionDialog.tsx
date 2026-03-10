import { useEffect, useState } from 'react'
import { X } from 'lucide-react'
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
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover'
import { useCreateDocsCollection, useDocsSpaces, useUpdateDocsCollection } from '@/hooks/queries'
import type { DocsCollection } from '@/lib/docsTypes'
import { toast } from 'sonner'

// ── Curated emoji grid for collections ──

const ICON_GROUPS: { label: string; icons: string[] }[] = [
  {
    label: 'Documents',
    icons: ['📄', '📝', '📋', '📑', '📃', '📜', '📓', '📔', '📒', '📕', '📗', '📘', '📙', '📚', '🗂️', '🗃️'],
  },
  {
    label: 'Categories',
    icons: ['📁', '📂', '🏷️', '🔖', '📌', '📎', '🔗', '📐', '📏', '✂️', '🧩', '🎯', '⭐', '💡', '🔑', '🏆'],
  },
  {
    label: 'Technical',
    icons: ['⚙️', '🔧', '🛠️', '💻', '🖥️', '📡', '🔬', '🧪', '🧮', '📊', '📈', '📉', '🗺️', '🌐', '☁️', '🔒'],
  },
  {
    label: 'People & Work',
    icons: ['👥', '👤', '🤝', '💼', '🏢', '📣', '📞', '✉️', '💬', '🎓', '🎧', '🎨', '✅', '❓', '💰', '🚀'],
  },
]

interface CreateCollectionDialogProps {
  wsId: string
  spaceId?: string
  open: boolean
  onOpenChange: (open: boolean) => void
  collection?: DocsCollection | null
}

export function CreateCollectionDialog({
  wsId,
  spaceId: defaultSpaceId,
  open,
  onOpenChange,
  collection,
}: CreateCollectionDialogProps) {
  const isEdit = !!collection
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [icon, setIcon] = useState('')
  const [iconPickerOpen, setIconPickerOpen] = useState(false)
  const [selectedSpaceId, setSelectedSpaceId] = useState(defaultSpaceId ?? '')

  const { data: spaces } = useDocsSpaces(wsId)

  useEffect(() => {
    if (!open) return

    if (collection) {
      setName(collection.name)
      setDescription(collection.description ?? '')
      setIcon(collection.icon ?? '')
      setSelectedSpaceId(collection.space_id)
      return
    }

    setName('')
    setDescription('')
    setIcon('')
    setSelectedSpaceId(defaultSpaceId ?? (spaces?.[0]?.id ?? ''))
  }, [open, collection, defaultSpaceId, spaces])

  useEffect(() => {
    if (!open || collection || selectedSpaceId || !spaces?.length) return
    setSelectedSpaceId(defaultSpaceId ?? spaces[0].id)
  }, [collection, defaultSpaceId, open, selectedSpaceId, spaces])

  const effectiveSpaceId = collection?.space_id ?? (selectedSpaceId || '')
  const currentSpace = spaces?.find((space) => space.id === effectiveSpaceId)
  const createCollection = useCreateDocsCollection(wsId, effectiveSpaceId)
  const updateCollection = useUpdateDocsCollection(wsId)

  const reset = () => {
    setName('')
    setDescription('')
    setIcon('')
    setSelectedSpaceId(defaultSpaceId ?? (spaces?.[0]?.id ?? ''))
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!name.trim() || !effectiveSpaceId) return

    try {
      if (isEdit && collection) {
        await updateCollection.mutateAsync({
          id: collection.id,
          spaceId: collection.space_id,
          name: name.trim(),
          description: description.trim(),
          icon: icon.trim(),
        })
        toast.success('Collection updated')
      } else {
        await createCollection.mutateAsync({
          name: name.trim(),
          description: description.trim() || undefined,
          icon: icon.trim() || undefined,
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
                <Popover open={iconPickerOpen} onOpenChange={setIconPickerOpen}>
                  <PopoverTrigger asChild>
                    <button
                      type="button"
                      className="flex h-9 w-9 items-center justify-center rounded-md border border-input bg-background text-lg transition-colors hover:bg-accent"
                    >
                      {icon || '📁'}
                    </button>
                  </PopoverTrigger>
                  <PopoverContent className="w-72 p-2" align="start">
                    <div className="space-y-2">
                      {ICON_GROUPS.map((group) => (
                        <div key={group.label}>
                          <p className="mb-1 px-1 text-[10px] font-medium uppercase tracking-wide text-muted-foreground">
                            {group.label}
                          </p>
                          <div className="grid grid-cols-8 gap-0.5">
                            {group.icons.map((emoji) => (
                              <button
                                key={emoji}
                                type="button"
                                onClick={() => {
                                  setIcon(emoji)
                                  setIconPickerOpen(false)
                                }}
                                className={`flex h-8 w-8 items-center justify-center rounded text-base transition-colors hover:bg-accent ${
                                  icon === emoji ? 'bg-accent ring-1 ring-primary/40' : ''
                                }`}
                              >
                                {emoji}
                              </button>
                            ))}
                          </div>
                        </div>
                      ))}
                    </div>
                  </PopoverContent>
                </Popover>
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
                  <X className="h-3.5 w-3.5" />
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
                  {currentSpace?.icon ? `${currentSpace.icon} ` : ''}
                  {currentSpace?.name ?? 'Current space'}
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
                        {s.icon ? `${s.icon} ` : ''}{s.name}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
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
