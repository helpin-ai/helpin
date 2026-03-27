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
import { sanitizeDocsSlugInput } from '@/lib/docsSlugs'

interface PublishSlugDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  title: string
  description: string
  slug: string
  onSlugChange: (slug: string) => void
  onConfirm: () => void
  isPublishing?: boolean
  confirmLabel?: string
}

export function PublishSlugDialog({
  open,
  onOpenChange,
  title,
  description,
  slug,
  onSlugChange,
  onConfirm,
  isPublishing = false,
  confirmLabel = 'Publish',
}: PublishSlugDialogProps) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          <DialogDescription>{description}</DialogDescription>
        </DialogHeader>
        <div className="space-y-2">
          <Label htmlFor="publish-slug">Slug</Label>
          <div className="flex items-center">
            <span className="inline-flex h-9 items-center rounded-l-md border border-r-0 border-input bg-muted px-3 text-sm text-muted-foreground">/</span>
            <Input
              id="publish-slug"
              value={slug}
              onChange={(event) => onSlugChange(sanitizeDocsSlugInput(event.target.value))}
              placeholder="article-slug"
              className="rounded-l-none"
            />
          </div>
        </div>
        <DialogFooter>
          <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button type="button" onClick={onConfirm} disabled={!slug || isPublishing}>
            {confirmLabel}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
