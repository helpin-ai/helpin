import { useState } from 'react'
import { Globe02Icon } from '@/lib/icons'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Label } from '@/components/ui/label'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import {
  getHelpcenterLocaleLabel,
  type DocsHelpcenterArticleTranslation,
  type UpsertDocsHelpcenterArticleTranslationRequest,
} from '@/lib/docsTypes'

interface EditArticleTranslationDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  locale: string
  sourceExcerpt?: string
  translation?: DocsHelpcenterArticleTranslation | null
  isSaving: boolean
  onSave: (data: UpsertDocsHelpcenterArticleTranslationRequest) => Promise<void> | void
}

export function EditArticleTranslationDialog({
  open,
  onOpenChange,
  locale,
  sourceExcerpt,
  translation,
  isSaving,
  onSave,
}: EditArticleTranslationDialogProps) {
  const [excerpt, setExcerpt] = useState(translation?.excerpt ?? sourceExcerpt ?? '')
  const [ogTitle, setOgTitle] = useState(translation?.og_title ?? '')
  const [ogDescription, setOgDescription] = useState(translation?.og_description ?? '')
  const [ogImageUrl, setOgImageUrl] = useState(translation?.og_image_url ?? '')
  const [ogImageAlt, setOgImageAlt] = useState(translation?.og_image_alt ?? '')

  const handleSave = async () => {
    const title = translation?.title?.trim() ?? ''
    await onSave({
      locale,
      title,
      excerpt: excerpt.trim() || undefined,
      content: translation?.content ?? {},
      seo_title: title || undefined,
      seo_description: excerpt.trim() || undefined,
      og_title: ogTitle,
      og_description: ogDescription,
      og_image_url: ogImageUrl,
      og_image_alt: ogImageAlt,
    })
    onOpenChange(false)
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="flex max-h-[92vh] max-w-lg flex-col overflow-hidden p-0">
        <DialogHeader className="border-b border-border/60 px-6 py-5">
          <DialogTitle className="flex items-center gap-2">
            <Globe02Icon className="h-4 w-4 text-primary" />
            {getHelpcenterLocaleLabel(locale)} translation details
          </DialogTitle>
          <DialogDescription>
            Adjust localized metadata while continuing to edit the translation directly on the article page.
          </DialogDescription>
        </DialogHeader>

        <div className="flex min-h-0 flex-col overflow-hidden">
          <div className="grid gap-4 border-b border-border/60 px-6 py-5">
            <div className="space-y-2">
              <Label htmlFor="article-translation-excerpt">Meta description</Label>
              <Textarea
                id="article-translation-excerpt"
                rows={3}
                value={excerpt}
                onChange={(event) => setExcerpt(event.target.value)}
                placeholder="Short summary used in collection, search, and meta description"
              />
              <p className="text-xs text-muted-foreground">
                Meta title uses the localized article title from the editor. The public slug is confirmed on first publish and can be edited later from the docs editor.
              </p>
            </div>
            <div className="grid gap-4 border-t border-border/60 pt-4">
              <div className="space-y-2">
                <Label htmlFor="article-translation-og-title">Social title</Label>
                <Input
                  id="article-translation-og-title"
                  value={ogTitle}
                  onChange={(event) => setOgTitle(event.target.value)}
                  placeholder="Falls back to the localized article title"
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="article-translation-og-desc">Social description</Label>
                <Textarea
                  id="article-translation-og-desc"
                  rows={2}
                  value={ogDescription}
                  onChange={(event) => setOgDescription(event.target.value)}
                  placeholder="Falls back to the meta description"
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="article-translation-og-image">Social image URL</Label>
                <Input
                  id="article-translation-og-image"
                  value={ogImageUrl}
                  onChange={(event) => setOgImageUrl(event.target.value)}
                  placeholder="https://..."
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="article-translation-og-image-alt">Image alt text</Label>
                <Input
                  id="article-translation-og-image-alt"
                  value={ogImageAlt}
                  onChange={(event) => setOgImageAlt(event.target.value)}
                  placeholder="Localized social preview image"
                />
              </div>
            </div>
          </div>
        </div>

        <DialogFooter className="border-t border-border/60 px-6 py-4">
          <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button type="button" onClick={() => void handleSave()} disabled={isSaving || !(translation?.title?.trim())}>
            {isSaving ? 'Saving…' : 'Save'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
