import { useEffect, useRef, useState } from 'react'
import { toast } from 'sonner'
import {
  AlertCircleIcon,
  CheckmarkCircle02Icon,
  Loading01Icon,
  MagicWand01Icon,
} from '@/lib/icons'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { docsService } from '@/lib/services/docsService'
import { getHelpcenterLocaleLabel } from '@/lib/docsTypes'
import type {
  AutoTranslateFailedItem,
  DocsHelpcenterCollectionTranslation,
  DocsHelpcenterSpaceTranslation,
} from '@/lib/docsTypes'

type LocaleState =
  | { status: 'pending' }
  | { status: 'running' }
  | { status: 'success'; created: number }
  | { status: 'partial'; created: number; failed: AutoTranslateFailedItem[] }
  | { status: 'failed'; error: string }

export interface AutoTranslateMissingDialogProps {
  wsId: string
  /** Non-default locales to fill. The dialog fires one request per locale in parallel. */
  locales: string[]
  open: boolean
  onOpenChange: (open: boolean) => void
  /** Called once per created space translation row so the table can splice local state. */
  onSpaceTranslationCreated: (row: DocsHelpcenterSpaceTranslation) => void
  /** Called once per created collection translation row so the table can splice local state. */
  onCollectionTranslationCreated: (row: DocsHelpcenterCollectionTranslation) => void
}

/**
 * AutoTranslateMissingDialog is a progress + results modal for the
 * bulk auto-translate action. On open it immediately fires one
 * request per non-default locale in parallel and streams per-locale
 * status as each response lands:
 *
 *   pending → running → success | partial | failed
 *
 * Existing translation rows are never overwritten server-side
 * (Option C, 2026-04-11), so the dialog is safe to reopen — each
 * run is idempotent in the sense that it only fills whatever is
 * currently missing.
 *
 * Closing the dialog mid-flight does NOT cancel in-flight requests;
 * their responses still splice into the parent's translation maps
 * via the onCreated callbacks. Reopening triggers a fresh run.
 */
export function AutoTranslateMissingDialog({
  wsId,
  locales,
  open,
  onOpenChange,
  onSpaceTranslationCreated,
  onCollectionTranslationCreated,
}: AutoTranslateMissingDialogProps) {
  const [results, setResults] = useState<Record<string, LocaleState>>({})
  const [expandedFailures, setExpandedFailures] = useState<Record<string, boolean>>({})
  // Track the run id so late-arriving responses from a closed
  // previous run don't overwrite state of a fresh run that was
  // kicked off from a subsequent open.
  const runIdRef = useRef(0)

  useEffect(() => {
    if (!open) return
    if (locales.length === 0) {
      setResults({})
      return
    }

    runIdRef.current += 1
    const thisRun = runIdRef.current

    // Reset state for this run.
    const initial: Record<string, LocaleState> = {}
    for (const locale of locales) {
      initial[locale] = { status: 'pending' }
    }
    setResults(initial)
    setExpandedFailures({})

    // Fire one request per locale concurrently. Promise.allSettled
    // so one locale failure doesn't abort the others.
    void Promise.allSettled(
      locales.map(async (locale) => {
        if (runIdRef.current !== thisRun) return
        setResults((prev) => ({ ...prev, [locale]: { status: 'running' } }))

        try {
          const res = await docsService.autoTranslateMissing(wsId, locale)
          if (runIdRef.current !== thisRun) return

          if (res.error || !res.data) {
            setResults((prev) => ({
              ...prev,
              [locale]: { status: 'failed', error: res.error ?? 'Unknown error' },
            }))
            return
          }

          const data = res.data
          // Splice every created row into the parent's local state so
          // the settings table updates live without a reload.
          for (const row of data.spaces ?? []) {
            onSpaceTranslationCreated(row)
          }
          for (const row of data.collections ?? []) {
            onCollectionTranslationCreated(row)
          }

          const created = (data.spaces?.length ?? 0) + (data.collections?.length ?? 0)
          const failed = data.failed ?? []
          if (failed.length > 0) {
            setResults((prev) => ({
              ...prev,
              [locale]: { status: 'partial', created, failed },
            }))
          } else {
            setResults((prev) => ({
              ...prev,
              [locale]: { status: 'success', created },
            }))
          }
        } catch (err) {
          if (runIdRef.current !== thisRun) return
          setResults((prev) => ({
            ...prev,
            [locale]: {
              status: 'failed',
              error: err instanceof Error ? err.message : String(err),
            },
          }))
        }
      }),
    ).then(() => {
      if (runIdRef.current !== thisRun) return
      // Fire a summary toast once the whole run is done, so users
      // who close the dialog mid-flight still get feedback.
      const final = Object.values(resultsRef.current)
      const createdTotal = final.reduce(
        (sum, s) => sum + (s.status === 'success' || s.status === 'partial' ? s.created : 0),
        0,
      )
      const failedLocales = final.filter((s) => s.status === 'failed').length
      const partialLocales = final.filter((s) => s.status === 'partial').length
      if (failedLocales === 0 && partialLocales === 0) {
        if (createdTotal > 0) {
          toast.success(`Auto-translated ${createdTotal} rows.`)
        }
      } else if (failedLocales === locales.length) {
        toast.error('Auto-translate failed for every locale.')
      } else {
        toast.warning(
          `Auto-translated ${createdTotal} rows. ${failedLocales + partialLocales} locale${failedLocales + partialLocales === 1 ? '' : 's'} had issues.`,
        )
      }
    })
    // Intentionally omit other deps — we want this to run on every
    // open→true transition, not on prop changes during an open run.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open])

  // Keep a ref of the latest results so the finalizer toast sees
  // the post-resolution snapshot rather than a stale closure.
  const resultsRef = useRef(results)
  useEffect(() => {
    resultsRef.current = results
  }, [results])

  const summary = computeSummary(results, locales)

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <MagicWand01Icon className="h-4 w-4 text-primary" />
            Auto-translate missing
          </DialogTitle>
          <DialogDescription>
            Filling missing translation rows for each enabled locale via AI. Existing rows stay as-is — this run never overwrites manual edits.
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-2 py-2">
          {locales.length === 0 ? (
            <p className="text-xs text-muted-foreground">
              No non-default locales enabled. Enable at least one locale in Settings → Languages first.
            </p>
          ) : (
            locales.map((locale) => {
              const state = results[locale] ?? { status: 'pending' }
              const isExpanded = expandedFailures[locale] ?? false
              return (
                <LocaleRow
                  key={locale}
                  locale={locale}
                  state={state}
                  expanded={isExpanded}
                  onToggleExpand={() =>
                    setExpandedFailures((prev) => ({ ...prev, [locale]: !isExpanded }))
                  }
                />
              )
            })
          )}
        </div>

        {locales.length > 0 && (
          <p className="border-t border-border/40 pt-3 text-xs text-muted-foreground">
            {summary}
          </p>
        )}

        <DialogFooter>
          <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
            Close
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

function computeSummary(results: Record<string, LocaleState>, locales: string[]): string {
  const states = locales.map((l) => results[l] ?? { status: 'pending' } as LocaleState)
  const running = states.filter((s) => s.status === 'running' || s.status === 'pending').length
  if (running > 0) {
    return `Translating ${running} of ${locales.length} locale${locales.length === 1 ? '' : 's'}…`
  }
  const created = states.reduce(
    (sum, s) => sum + (s.status === 'success' || s.status === 'partial' ? s.created : 0),
    0,
  )
  const failed = states.reduce(
    (sum, s) =>
      sum +
      (s.status === 'failed' ? 1 : s.status === 'partial' ? 1 : 0),
    0,
  )
  if (failed === 0) {
    return `Done — ${created} row${created === 1 ? '' : 's'} translated across ${locales.length} locale${locales.length === 1 ? '' : 's'}.`
  }
  return `Done — ${created} row${created === 1 ? '' : 's'} translated, ${failed} locale${failed === 1 ? '' : 's'} had issues.`
}

interface LocaleRowProps {
  locale: string
  state: LocaleState
  expanded: boolean
  onToggleExpand: () => void
}

function LocaleRow({ locale, state, expanded, onToggleExpand }: LocaleRowProps) {
  const label = getHelpcenterLocaleLabel(locale)
  const canExpand = state.status === 'partial' || state.status === 'failed'

  return (
    <div className="rounded-md border border-border/60 bg-card/40 px-3 py-2">
      <button
        type="button"
        onClick={canExpand ? onToggleExpand : undefined}
        disabled={!canExpand}
        className="flex w-full items-center justify-between gap-3 text-left disabled:cursor-default"
      >
        <span className="flex items-center gap-2 text-sm font-medium">
          <StatusIcon state={state} />
          <span>{label}</span>
          <span className="text-[11px] uppercase tracking-wide text-muted-foreground/60">
            {locale}
          </span>
        </span>
        <span className="text-xs text-muted-foreground">
          {renderRightLabel(state)}
        </span>
      </button>
      {canExpand && expanded && (
        <div className="mt-2 rounded border border-border/40 bg-muted/40 px-2 py-1.5 text-[11px] text-muted-foreground">
          {state.status === 'partial' ? (
            <>
              <p className="mb-1 font-medium text-foreground">
                {state.failed.length} item{state.failed.length === 1 ? '' : 's'} skipped:
              </p>
              <ul className="space-y-0.5">
                {state.failed.map((item, i) => (
                  <li key={`${item.kind}-${item.id}-${i}`} className="truncate">
                    <span className="text-muted-foreground/60">{item.kind}</span>{' '}
                    <span className="font-mono">{item.id.slice(0, 8)}</span>
                    {' — '}
                    {item.reason}
                  </li>
                ))}
              </ul>
            </>
          ) : state.status === 'failed' ? (
            <p className="text-destructive">{state.error}</p>
          ) : null}
        </div>
      )}
    </div>
  )
}

function StatusIcon({ state }: { state: LocaleState }) {
  switch (state.status) {
    case 'pending':
      return <span className="h-3.5 w-3.5 rounded-full border border-muted-foreground/40" aria-label="Pending" />
    case 'running':
      return <Loading01Icon className="h-3.5 w-3.5 animate-spin text-primary" />
    case 'success':
      return <CheckmarkCircle02Icon className="h-3.5 w-3.5 text-emerald-600 dark:text-emerald-400" />
    case 'partial':
      return <AlertCircleIcon className="h-3.5 w-3.5 text-amber-600 dark:text-amber-400" />
    case 'failed':
      return <AlertCircleIcon className="h-3.5 w-3.5 text-destructive" />
  }
}

function renderRightLabel(state: LocaleState): string {
  switch (state.status) {
    case 'pending':
      return 'Queued'
    case 'running':
      return 'Translating…'
    case 'success':
      return state.created === 0
        ? 'Already filled'
        : `${state.created} row${state.created === 1 ? '' : 's'}`
    case 'partial':
      return `${state.created} of ${state.created + state.failed.length}`
    case 'failed':
      return 'Failed'
  }
}
