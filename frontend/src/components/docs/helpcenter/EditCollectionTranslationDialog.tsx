import { useState } from 'react'
import { FolderOpenIcon } from '@/lib/icons'
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
import { Textarea } from '@/components/ui/textarea'
import {
  getHelpcenterLocaleLabel,
  type DocsHelpcenterCollectionTranslation,
  type UpsertDocsHelpcenterCollectionTranslationRequest,
} from '@/lib/docsTypes'

interface EditCollectionTranslationDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  locale: string
  sourceName: string
  sourceSlug: string
  sourceDescription?: string
  translation?: DocsHelpcenterCollectionTranslation | null
  isSaving: boolean
  onSave: (data: UpsertDocsHelpcenterCollectionTranslationRequest) => Promise<void> | void
}

export function EditCollectionTranslationDialog({
  open,
  onOpenChange,
  locale,
  sourceName,
  sourceSlug,
  sourceDescription,
  translation,
  isSaving,
  onSave,
}: EditCollectionTranslationDialogProps) {
  const [name, setName] = useState(translation?.name ?? sourceName)
  const [slug, setSlug] = useState(translation?.slug ?? '')
  const [description, setDescription] = useState(translation?.description ?? sourceDescription ?? '')

  const handleSave = async () => {
    await onSave({
      locale,
      name: name.trim(),
      slug: slug.trim(),
      description: description.trim() || undefined,
    })
    onOpenChange(false)
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-xl">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <FolderOpenIcon className="h-4 w-4 text-primary" />
            {getHelpcenterLocaleLabel(locale)} collection translation
          </DialogTitle>
          <DialogDescription>
            Localize the collection label and description used in the public help center navigation.
          </DialogDescription>
        </DialogHeader>

        <div className="grid gap-4 lg:grid-cols-[220px_minmax(0,1fr)]">
          <div className="rounded-2xl border border-border/60 bg-muted/20 p-4">
            <p className="text-xs font-medium uppercase tracking-[0.18em] text-muted-foreground">Source</p>
            <div className="mt-3 space-y-3">
              <div>
                <p className="text-sm font-medium">{sourceName}</p>
                <p className="text-xs text-muted-foreground">/{sourceSlug}</p>
              </div>
              {sourceDescription && (
                <p className="text-xs text-muted-foreground">{sourceDescription}</p>
              )}
            </div>
          </div>

          <div className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="collection-translation-name">Localized name</Label>
              <Input id="collection-translation-name" value={name} onChange={(event) => setName(event.target.value)} />
            </div>
            <div className="space-y-2">
              <Label htmlFor="collection-translation-slug">Localized slug</Label>
              <Input id="collection-translation-slug" value={slug} onChange={(event) => setSlug(event.target.value)} />
              <p className="text-xs text-muted-foreground">
                Leave blank to derive the public slug from this localized name on first publish.
              </p>
            </div>
            <div className="space-y-2">
              <Label htmlFor="collection-translation-description">Description</Label>
              <Textarea
                id="collection-translation-description"
                rows={5}
                value={description}
                onChange={(event) => setDescription(event.target.value)}
                placeholder="Optional locale-specific collection description"
              />
            </div>
          </div>
        </div>

        <DialogFooter>
          <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button type="button" onClick={() => void handleSave()} disabled={isSaving || !name.trim()}>
            {isSaving ? 'Saving…' : 'Save translation'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
