import { toast } from 'sonner'
import { docsService } from '@/lib/services/docsService'
import { getHelpcenterLocaleLabel } from '@/lib/docsTypes'

const SETTINGS_HINT = 'You can manage translations later from Settings > Help Center > Languages & Translation.'

/**
 * translateEntityToLocales fires per-locale AI-generate calls for a
 * single space or collection in parallel. It shows a result toast
 * on completion and a fallback-to-settings toast on failure.
 *
 * Designed to be called from the "Translate now" action button on
 * the post-create toast. Each locale hits the existing single-cell
 * generate endpoint so no new backend work is needed.
 */
export async function translateEntityToLocales(
  wsId: string,
  kind: 'space' | 'collection',
  entityId: string,
  locales: string[],
): Promise<void> {
  if (locales.length === 0) return

  const results = await Promise.allSettled(
    locales.map((locale) =>
      kind === 'space'
        ? docsService.generateSpaceTranslation(wsId, entityId, locale)
        : docsService.generateCollectionTranslation(wsId, entityId, locale),
    ),
  )

  const succeeded = results.filter((r) => r.status === 'fulfilled').length
  const failed = locales.length - succeeded

  if (failed === 0) {
    toast.success(`Translated to ${formatLocaleList(locales)}`)
  } else if (succeeded > 0) {
    toast.warning(
      `Translated ${succeeded} of ${locales.length} locales. ${SETTINGS_HINT}`,
    )
  } else {
    toast.error(`Translation failed. ${SETTINGS_HINT}`)
  }
}

/**
 * showAutoTranslateToast displays a post-create success toast with
 * a "Translate now" action button. Call this instead of a plain
 * toast.success after creating a space or collection when the
 * workspace has non-default locales enabled and the entity is in
 * (or is) an external-capable space.
 *
 * If `nonDefaultLocales` is empty, falls back to a plain success
 * toast with no action button.
 */
export function showAutoTranslateToast(
  message: string,
  wsId: string,
  kind: 'space' | 'collection',
  entityId: string,
  nonDefaultLocales: string[],
): void {
  if (nonDefaultLocales.length === 0) {
    toast.success(message)
    return
  }

  toast.success(message, {
    description: `Missing translations for ${formatLocaleList(nonDefaultLocales)}.`,
    action: {
      label: 'Translate now',
      onClick: () => void translateEntityToLocales(wsId, kind, entityId, nonDefaultLocales),
    },
    duration: 10_000, // longer than default so user has time to decide
  })
}

function formatLocaleList(locales: string[]): string {
  return locales.map((l) => getHelpcenterLocaleLabel(l)).join(', ')
}
