import { useMemo, useState } from 'react'
import type { JSONContent } from '@tiptap/react'
import { FileText, Globe2 } from 'lucide-react'
import { DocsEditor } from '@/components/docs/DocsEditor'
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
  type DocsHelpcenterArticleTranslation,
  type UpsertDocsHelpcenterArticleTranslationRequest,
} from '@/lib/docsTypes'

interface EditArticleTranslationDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  locale: string
  sourceTitle: string
  sourceSlug?: string
  sourceExcerpt?: string
  sourceContent?: JSONContent | null
  translation?: DocsHelpcenterArticleTranslation | null
  isSaving: boolean
  mode?: 'full' | 'details_only'
  onSave: (data: UpsertDocsHelpcenterArticleTranslationRequest) => Promise<void> | void
}

export function EditArticleTranslationDialog({
  open,
  onOpenChange,
  locale,
  sourceTitle,
  sourceSlug,
  sourceExcerpt,
  sourceContent,
  translation,
  isSaving,
  mode = 'full',
  onSave,
}: EditArticleTranslationDialogProps) {
  const [title, setTitle] = useState(translation?.title ?? sourceTitle)
  const [slug, setSlug] = useState(translation?.slug ?? sourceSlug ?? '')
  const [excerpt, setExcerpt] = useState(translation?.excerpt ?? sourceExcerpt ?? '')
  const [seoTitle, setSeoTitle] = useState(translation?.seo_title ?? '')
  const [seoDescription, setSeoDescription] = useState(translation?.seo_description ?? '')
  const [contentDraft, setContentDraft] = useState<JSONContent | null>(
    (translation?.content as JSONContent | null | undefined) ?? sourceContent ?? null,
  )

  const editorKey = useMemo(
    () => `${locale}-${translation?.updated_at ?? 'new'}-${open ? 'open' : 'closed'}`,
    [locale, translation?.updated_at, open],
  )

  const handleSave = async () => {
    await onSave({
      locale,
      title: title.trim(),
      slug: slug.trim(),
      excerpt: excerpt.trim() || undefined,
      content: contentDraft ?? {},
      seo_title: seoTitle.trim() || undefined,
      seo_description: seoDescription.trim() || undefined,
    })
    onOpenChange(false)
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className={`flex max-h-[92vh] flex-col overflow-hidden p-0 ${mode === 'details_only' ? 'max-w-lg' : 'max-w-6xl'}`}>
        <DialogHeader className="border-b border-border/60 px-6 py-5">
          <DialogTitle className="flex items-center gap-2">
            <Globe2 className="h-4 w-4 text-primary" />
            {mode === 'details_only' ? `${getHelpcenterLocaleLabel(locale)} translation details` : `${getHelpcenterLocaleLabel(locale)} article translation`}
          </DialogTitle>
          <DialogDescription>
            {mode === 'details_only'
              ? 'Adjust localized metadata while continuing to edit the translation directly on the article page.'
              : 'Translate the public article title, slug, summary, and body while keeping the source article as the default-locale mirror.'}
          </DialogDescription>
        </DialogHeader>

        <div className={`grid min-h-0 flex-1 gap-0 ${mode === 'details_only' ? '' : 'lg:grid-cols-[280px_minmax(0,1fr)]'}`}>
          {mode !== 'details_only' && (
            <aside className="border-b border-border/60 bg-muted/20 px-6 py-5 lg:border-r lg:border-b-0">
              <div className="rounded-2xl border border-border/60 bg-background/70 p-4">
                <p className="text-xs font-medium uppercase tracking-[0.18em] text-muted-foreground">Source article</p>
                <div className="mt-3 space-y-3">
                  <div className="flex items-start gap-2">
                    <FileText className="mt-0.5 h-4 w-4 text-muted-foreground" />
                    <div>
                      <p className="text-sm font-medium">{sourceTitle}</p>
                      {sourceSlug && <p className="text-xs text-muted-foreground">/{sourceSlug}</p>}
                    </div>
                  </div>
                  {sourceExcerpt && <p className="text-xs text-muted-foreground">{sourceExcerpt}</p>}
                  <p className="text-xs text-muted-foreground">
                    Keep terminology and product names aligned with the source article, then adapt examples and phrasing for the locale.
                  </p>
                </div>
              </div>
            </aside>
          )}

          <div className="flex min-h-0 flex-col overflow-hidden">
            <div className="grid gap-4 border-b border-border/60 px-6 py-5 lg:grid-cols-2">
              <div className="space-y-2 lg:col-span-2">
                <Label htmlFor="article-translation-title">Localized title</Label>
                <Input id="article-translation-title" value={title} onChange={(event) => setTitle(event.target.value)} />
              </div>
              <div className="space-y-2">
                <Label htmlFor="article-translation-slug">Localized slug</Label>
                <Input id="article-translation-slug" value={slug} onChange={(event) => setSlug(event.target.value)} />
              </div>
              <div className="space-y-2">
                <Label htmlFor="article-translation-seo-title">SEO title</Label>
                <Input id="article-translation-seo-title" value={seoTitle} onChange={(event) => setSeoTitle(event.target.value)} />
              </div>
              <div className="space-y-2 lg:col-span-2">
                <Label htmlFor="article-translation-excerpt">Summary</Label>
                <Textarea
                  id="article-translation-excerpt"
                  rows={3}
                  value={excerpt}
                  onChange={(event) => setExcerpt(event.target.value)}
                  placeholder="Short summary shown in collection and search views"
                />
              </div>
              <div className="space-y-2 lg:col-span-2">
                <Label htmlFor="article-translation-seo-description">SEO description</Label>
                <Textarea
                  id="article-translation-seo-description"
                  rows={2}
                  value={seoDescription}
                  onChange={(event) => setSeoDescription(event.target.value)}
                  placeholder="Optional meta description for this locale"
                />
              </div>
            </div>

            {mode === 'full' && (
              <div className="min-h-0 flex-1 overflow-hidden">
                <DocsEditor
                  key={editorKey}
                  initialContent={contentDraft}
                  onSave={async (json) => {
                    setContentDraft(json)
                  }}
                  autoSaveMs={700}
                />
              </div>
            )}
          </div>
        </div>

        <DialogFooter className="border-t border-border/60 px-6 py-4">
          <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button type="button" onClick={() => void handleSave()} disabled={isSaving || !title.trim() || !slug.trim()}>
            {isSaving ? 'Saving…' : 'Save translation'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
