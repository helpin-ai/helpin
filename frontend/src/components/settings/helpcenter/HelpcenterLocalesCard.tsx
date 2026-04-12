import { useMemo, useState } from 'react'
import { Card, CardContent } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { ConfirmDialog } from '@/components/pm/ConfirmDialog'
import {
  HELP_CENTER_LOCALE_OPTIONS,
  getHelpcenterLocaleLabel,
  type DocsHelpcenterLocalesConfig,
  type UpdateDocsHelpcenterLocalesRequest,
} from '@/lib/docsTypes'

interface HelpcenterLocalesCardProps {
  config: DocsHelpcenterLocalesConfig
  isSaving: boolean
  onSave: (data: UpdateDocsHelpcenterLocalesRequest) => void | Promise<void>
}

function withDefaultFirst(locales: string[], defaultLocale: string) {
  const unique = Array.from(new Set(locales))
  const withoutDefault = unique.filter((locale) => locale !== defaultLocale)
  return [defaultLocale, ...withoutDefault]
}

export function HelpcenterLocalesCard({ config, isSaving, onSave }: HelpcenterLocalesCardProps) {
  const [draft, setDraft] = useState<UpdateDocsHelpcenterLocalesRequest>({
    default_locale: config.default_locale,
    enabled_locales: config.enabled_locales,
    show_language_switcher: config.show_language_switcher,
    fallback_to_default_locale: config.fallback_to_default_locale,
  })
  const [pendingLocaleChange, setPendingLocaleChange] = useState<string | null>(null)

  const localeOptions = useMemo(() => {
    const byValue = new Map(HELP_CENTER_LOCALE_OPTIONS.map((option) => [option.value, option]))
    for (const locale of draft.enabled_locales) {
      if (!byValue.has(locale)) {
        byValue.set(locale, { value: locale, label: getHelpcenterLocaleLabel(locale) })
      }
    }
    return Array.from(byValue.values())
  }, [draft.enabled_locales])

  const toggleLocale = (locale: string, checked: boolean) => {
    setDraft((current) => {
      const next = new Set(current.enabled_locales)
      if (checked) {
        next.add(locale)
      } else if (locale !== current.default_locale) {
        next.delete(locale)
      }
      return {
        ...current,
        enabled_locales: withDefaultFirst(Array.from(next), current.default_locale),
      }
    })
  }

  return (
    <Card className="border-0 shadow-none p-0">
      <CardContent className="space-y-5 px-0 pb-0 pt-1">
        <div className="grid gap-6 lg:grid-cols-[2fr_3fr]">
          <div className="space-y-2">
            <Label htmlFor="helpcenter-default-locale">Default locale</Label>
            <Select
              value={draft.default_locale}
              onValueChange={(value) => {
                if (value === draft.default_locale) return
                setPendingLocaleChange(value)
              }}
            >
              <SelectTrigger id="helpcenter-default-locale" className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {localeOptions.map((option) => (
                  <SelectItem key={option.value} value={option.value}>
                    {option.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <p className="text-xs text-muted-foreground">
              Published fallback pages resolve from this locale when a requested translation is missing.
            </p>
          </div>

          <div className="space-y-3">
            <div className="flex items-start justify-between gap-3 rounded-2xl border border-border/60 bg-muted/20 px-4 py-3">
              <div className="space-y-1">
                <Label htmlFor="show-language-switcher" className="text-sm font-medium">
                  Show language switcher
                </Label>
                <p className="text-xs text-muted-foreground">
                  Let visitors jump between translated help-center pages directly from public navigation.
                </p>
              </div>
              <Switch
                id="show-language-switcher"
                checked={draft.show_language_switcher}
                onCheckedChange={(checked) => setDraft((current) => ({ ...current, show_language_switcher: checked }))}
              />
            </div>

            <div className="flex items-start justify-between gap-3 rounded-2xl border border-border/60 bg-muted/20 px-4 py-3">
              <div className="space-y-1">
                <Label htmlFor="fallback-to-default" className="text-sm font-medium">
                  Fallback to default locale
                </Label>
                <p className="text-xs text-muted-foreground">
                  Serve the default-language page when a requested translation has not been created or published yet.
                </p>
              </div>
              <Switch
                id="fallback-to-default"
                checked={draft.fallback_to_default_locale}
                onCheckedChange={(checked) => setDraft((current) => ({ ...current, fallback_to_default_locale: checked }))}
              />
            </div>
          </div>
        </div>

        <div className="space-y-3">
          <div>
            <h3 className="text-sm font-medium">Enabled locales</h3>
            <p className="text-xs text-muted-foreground">
              Select which languages your help center will be available in.
            </p>
          </div>

          <div className="flex flex-wrap gap-2">
            {localeOptions.map((option) => {
              const checked = draft.enabled_locales.includes(option.value)
              const isDefault = option.value === draft.default_locale
              return (
                <label
                  key={option.value}
                  htmlFor={`locale-${option.value}`}
                  className={`inline-flex items-center gap-1.5 rounded-full border px-3 py-1 text-xs cursor-pointer transition-colors ${
                    checked ? 'border-primary/25 bg-primary/5 text-foreground' : 'border-border/60 bg-background text-muted-foreground hover:text-foreground'
                  }`}
                >
                  <Checkbox
                    id={`locale-${option.value}`}
                    checked={checked}
                    disabled={isDefault}
                    onCheckedChange={(value) => toggleLocale(option.value, Boolean(value))}
                    className="h-3.5 w-3.5"
                  />
                  <span>{option.label}</span>
                  {isDefault && (
                    <span className="text-[9px] text-muted-foreground">(default)</span>
                  )}
                </label>
              )
            })}
          </div>
        </div>

        <div className="flex justify-end border-t border-border/60 pt-4">
          <Button type="button" onClick={() => onSave(draft)} disabled={isSaving}>
            {isSaving ? 'Saving locales…' : 'Save locales'}
          </Button>
        </div>
      </CardContent>

      <ConfirmDialog
        open={pendingLocaleChange !== null}
        onOpenChange={(open) => { if (!open) setPendingLocaleChange(null) }}
        title="Change default locale?"
        description={`Changing the default locale to ${pendingLocaleChange ? getHelpcenterLocaleLabel(pendingLocaleChange) : ''} will update how fallback pages resolve and re-mirror source content for all spaces and collections. Already-published translations are not affected, but untranslated pages will now fall back to the new default language.`}
        confirmLabel="Change default locale"
        variant="destructive"
        onConfirm={() => {
          if (!pendingLocaleChange) return
          setDraft((current) => ({
            ...current,
            default_locale: pendingLocaleChange,
            enabled_locales: withDefaultFirst([...current.enabled_locales, pendingLocaleChange], pendingLocaleChange),
          }))
          setPendingLocaleChange(null)
        }}
      />
    </Card>
  )
}
