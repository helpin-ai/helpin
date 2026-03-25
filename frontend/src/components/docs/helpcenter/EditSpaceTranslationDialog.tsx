import { useState } from 'react'
import { Globe2 } from 'lucide-react'
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
  type DocsHelpcenterSpaceTranslation,
  type UpsertDocsHelpcenterSpaceTranslationRequest,
} from '@/lib/docsTypes'

interface EditSpaceTranslationDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  locale: string
  sourceName: string
  sourceSlug: string
  translation?: DocsHelpcenterSpaceTranslation | null
  isSaving: boolean
  onSave: (data: UpsertDocsHelpcenterSpaceTranslationRequest) => Promise<void> | void
}

export function EditSpaceTranslationDialog({
  open,
  onOpenChange,
  locale,
  sourceName,
  sourceSlug,
  translation,
  isSaving,
  onSave,
}: EditSpaceTranslationDialogProps) {
  const [name, setName] = useState(translation?.name ?? sourceName)
  const [slug, setSlug] = useState(translation?.slug ?? sourceSlug)
  const [description, setDescription] = useState(translation?.description ?? '')

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
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <Globe2 className="h-4 w-4 text-primary" />
            {getHelpcenterLocaleLabel(locale)} space translation
          </DialogTitle>
          <DialogDescription>
            Localize the public-facing space label and slug while keeping the source space untouched.
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
              <p className="text-xs text-muted-foreground">
                Default locale is mirrored from this source record. This translation only affects the public locale path.
              </p>
            </div>
          </div>

          <div className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="space-translation-name">Localized name</Label>
              <Input id="space-translation-name" value={name} onChange={(event) => setName(event.target.value)} />
            </div>
            <div className="space-y-2">
              <Label htmlFor="space-translation-slug">Localized slug</Label>
              <Input id="space-translation-slug" value={slug} onChange={(event) => setSlug(event.target.value)} />
            </div>
            <div className="space-y-2">
              <Label htmlFor="space-translation-description">Description</Label>
              <Textarea
                id="space-translation-description"
                rows={4}
                value={description}
                onChange={(event) => setDescription(event.target.value)}
                placeholder="Optional locale-specific space description"
              />
            </div>
          </div>
        </div>

        <DialogFooter>
          <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button type="button" onClick={() => void handleSave()} disabled={isSaving || !name.trim() || !slug.trim()}>
            {isSaving ? 'Saving…' : 'Save translation'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
