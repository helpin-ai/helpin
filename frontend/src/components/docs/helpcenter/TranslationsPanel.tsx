import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { getHelpcenterLocaleLabel, type DocsHelpcenterTranslationState } from '@/lib/docsTypes'
import { TranslationStatusBadge } from './TranslationStatusBadge'

export interface TranslationRow {
  locale: string
  state: DocsHelpcenterTranslationState
  updatedAtLabel?: string
  helperText?: string
  publishBlockedReason?: string
  isDefaultLocale?: boolean
  sourceMirrorLabel?: string
}

interface TranslationsPanelProps {
  title: string
  description: string
  locales: string[]
  rows: TranslationRow[]
  onAdd: (locale: string) => void
  onEdit: (locale: string) => void
  onPublish: (locale: string) => void
  onUnpublish: (locale: string) => void
  onMarkReviewed: (locale: string) => void
}

function compareLocales(locales: string[], a: string, b: string) {
  const order = new Map(locales.map((locale, index) => [locale, index]))
  return (order.get(a) ?? Number.MAX_SAFE_INTEGER) - (order.get(b) ?? Number.MAX_SAFE_INTEGER)
}

export function TranslationsPanel({
  title,
  description,
  locales,
  rows,
  onAdd,
  onEdit,
  onPublish,
  onUnpublish,
  onMarkReviewed,
}: TranslationsPanelProps) {
  const orderedRows = [...rows].sort((a, b) => compareLocales(locales, a.locale, b.locale))

  return (
    <Card className="border-border/60 shadow-none">
      <CardHeader className="pb-3">
        <CardTitle className="text-base">{title}</CardTitle>
        <CardDescription>{description}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-3">
        {orderedRows.map((row) => (
          <div
            key={row.locale}
            className="rounded-2xl border border-border/60 bg-background/80 px-4 py-3"
          >
            <div className="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
              <div className="min-w-0 space-y-2">
                <div className="flex flex-wrap items-center gap-2">
                  <div className="min-w-0">
                    <p className="text-sm font-medium text-foreground">{getHelpcenterLocaleLabel(row.locale)}</p>
                    <p className="text-xs uppercase tracking-[0.18em] text-muted-foreground">{row.locale}</p>
                  </div>
                  <TranslationStatusBadge state={row.state} />
                  {row.isDefaultLocale && (
                    <Badge variant="outline" className="h-6 rounded-full px-2.5 text-[11px] font-medium">
                      {row.sourceMirrorLabel ?? 'Source mirror'}
                    </Badge>
                  )}
                </div>
                {row.updatedAtLabel && (
                  <p className="text-xs text-muted-foreground">{row.updatedAtLabel}</p>
                )}
                {row.helperText && (
                  <p className="text-xs text-muted-foreground">{row.helperText}</p>
                )}
                {row.publishBlockedReason && (
                  <p className="text-xs text-amber-700 dark:text-amber-300">{row.publishBlockedReason}</p>
                )}
              </div>

              {!row.isDefaultLocale && (
                <div className="flex flex-wrap gap-2 lg:justify-end">
                  {row.state === 'missing' ? (
                    <Button size="sm" variant="outline" className="h-8" onClick={() => onAdd(row.locale)}>
                      Add translation
                    </Button>
                  ) : (
                    <>
                      <Button size="sm" variant="outline" className="h-8" onClick={() => onEdit(row.locale)}>
                        Edit
                      </Button>
                      {row.state === 'published' ? (
                        <Button size="sm" variant="outline" className="h-8" onClick={() => onUnpublish(row.locale)}>
                          Unpublish
                        </Button>
                      ) : (
                        <Button
                          size="sm"
                          className="h-8"
                          disabled={Boolean(row.publishBlockedReason)}
                          onClick={() => onPublish(row.locale)}
                        >
                          Publish
                        </Button>
                      )}
                      {row.state === 'needs_review' && (
                        <Button size="sm" variant="ghost" className="h-8" onClick={() => onMarkReviewed(row.locale)}>
                          Mark reviewed
                        </Button>
                      )}
                    </>
                  )}
                </div>
              )}
            </div>
          </div>
        ))}
      </CardContent>
    </Card>
  )
}
