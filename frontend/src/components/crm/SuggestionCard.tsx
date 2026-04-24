import {
  Alert01Icon,
  BulbIcon,
  Cancel01Icon,
  ChartIncreaseIcon,
  Clock03Icon,
  DollarCircleIcon,
  Mail01Icon,
  Tick01Icon,
} from '@/lib/icons'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { cn, timeAgo } from '@/lib/utils'
import type { CRMSuggestion } from '@/lib/crmTypes'

type SuggestionTone = 'neutral' | 'good' | 'warn' | 'danger' | 'accent'

const typeConfig: Record<CRMSuggestion['suggestion_type'], { label: string; icon: typeof DollarCircleIcon; tone: SuggestionTone }> = {
  deal_create: { label: 'New Deal', icon: DollarCircleIcon, tone: 'good' },
  deal_advance: { label: 'Stage Advance', icon: ChartIncreaseIcon, tone: 'accent' },
  follow_up: { label: 'Follow Up', icon: Mail01Icon, tone: 'neutral' },
  risk_alert: { label: 'Risk Alert', icon: Alert01Icon, tone: 'danger' },
  enrichment: { label: 'Enrichment', icon: BulbIcon, tone: 'warn' },
}

function toneClasses(tone: SuggestionTone) {
  switch (tone) {
    case 'good':
      return 'border-emerald-500/20 bg-emerald-500/10 text-emerald-700 dark:text-emerald-300'
    case 'warn':
      return 'border-amber-500/20 bg-amber-500/10 text-amber-700 dark:text-amber-300'
    case 'danger':
      return 'border-rose-500/20 bg-rose-500/10 text-rose-700 dark:text-rose-300'
    case 'accent':
      return 'border-sky-500/20 bg-sky-500/10 text-sky-700 dark:text-sky-300'
    default:
      return 'border-border bg-muted text-muted-foreground'
  }
}

function confidenceTone(confidence: number): SuggestionTone {
  if (confidence >= 0.9) return 'good'
  if (confidence >= 0.7) return 'warn'
  return 'danger'
}

function progressClass(tone: SuggestionTone) {
  switch (tone) {
    case 'good':
      return 'bg-emerald-500'
    case 'warn':
      return 'bg-amber-500'
    case 'danger':
      return 'bg-rose-500'
    case 'accent':
      return 'bg-sky-500'
    default:
      return 'bg-foreground/60'
  }
}

function contextValue(value: unknown) {
  if (typeof value === 'string' && value.trim()) return value
  if (typeof value === 'number' && Number.isFinite(value)) return value.toLocaleString()
  return null
}

function contextRows(suggestion: CRMSuggestion) {
  const ctx = (suggestion.context || {}) as Record<string, unknown>
  const rows: Array<{ label: string; value: string }> = []

  const contactName = contextValue(ctx.contact_name)
  const dealName = contextValue(ctx.deal_name)
  const companyName = contextValue(ctx.company_name)
  const amount = typeof ctx.amount === 'number' && Number.isFinite(ctx.amount)
    ? `$${ctx.amount.toLocaleString()}`
    : contextValue(ctx.amount)
  const currentStage = contextValue(ctx.current_stage)
  const recommendedStage = contextValue(ctx.recommended_stage)
  const source = contextValue(ctx.source)

  if (contactName) rows.push({ label: 'Contact', value: contactName })
  if (companyName) rows.push({ label: 'Company', value: companyName })
  if (dealName) rows.push({ label: 'Deal', value: dealName })
  if (amount) rows.push({ label: 'Est.', value: amount })
  if (currentStage && recommendedStage) rows.push({ label: 'Move', value: `${currentStage} -> ${recommendedStage}` })
  if (source) rows.push({ label: 'Source', value: source })

  return rows.slice(0, 5)
}

interface SuggestionCardProps {
  suggestion: CRMSuggestion
  onAccept: (id: string) => void
  onDismiss: (id: string) => void
  isAccepting?: boolean
  isDismissing?: boolean
  compact?: boolean
}

export function SuggestionCard({ suggestion, onAccept, onDismiss, isAccepting, isDismissing, compact }: SuggestionCardProps) {
  const config = typeConfig[suggestion.suggestion_type] ?? {
    label: suggestion.suggestion_type.replace(/_/g, ' '),
    icon: BulbIcon,
    tone: 'neutral' as const,
  }
  const Icon = config.icon
  const confidence = Math.round(suggestion.confidence * 100)
  const confidenceLevel = confidenceTone(suggestion.confidence)
  const rows = contextRows(suggestion)

  return (
    <Card className="rounded-lg transition-colors hover:bg-muted/25">
      <CardContent className={cn('p-4', compact && 'p-3')}>
        <div className="flex items-start gap-3">
          <div className={cn('mt-0.5 flex h-9 w-9 shrink-0 items-center justify-center rounded-lg border', toneClasses(config.tone))}>
            <Icon className="h-4 w-4" />
          </div>

          <div className="min-w-0 flex-1">
            <div className="flex flex-wrap items-center gap-2">
              <Badge variant="outline" className={cn('text-[11px]', toneClasses(config.tone))}>
                {config.label}
              </Badge>
              <span className="inline-flex items-center gap-1 text-xs text-muted-foreground">
                <Clock03Icon className="h-3.5 w-3.5" />
                {timeAgo(suggestion.created_at)}
              </span>
            </div>

            <div className="mt-2 flex items-start justify-between gap-3">
              <div className="min-w-0">
                <h4 className="text-sm font-medium leading-5">{suggestion.title}</h4>
                {suggestion.description ? (
                  <p className={cn('mt-1 text-xs leading-5 text-muted-foreground', compact ? 'line-clamp-2' : 'line-clamp-3')}>
                    {suggestion.description}
                  </p>
                ) : null}
              </div>
              <div className="w-20 shrink-0 text-right">
                <p className={cn('text-xs font-semibold tabular-nums', toneClasses(confidenceLevel).split(' ').filter((part) => part.startsWith('text-')).join(' '))}>
                  {confidence}%
                </p>
                <div className="mt-2 h-1.5 overflow-hidden rounded-full bg-muted">
                  <div className={cn('h-full rounded-full', progressClass(confidenceLevel))} style={{ width: `${confidence}%` }} />
                </div>
              </div>
            </div>

            {rows.length > 0 ? (
              <div className="mt-3 grid gap-2 sm:grid-cols-2">
                {rows.map((row) => (
                  <div key={`${row.label}:${row.value}`} className="min-w-0 rounded-md border bg-muted/20 px-2.5 py-1.5">
                    <p className="text-[10px] font-medium uppercase tracking-wide text-muted-foreground">{row.label}</p>
                    <p className="truncate text-xs text-foreground">{row.value}</p>
                  </div>
                ))}
              </div>
            ) : null}

            <div className="mt-3 flex flex-wrap items-center gap-2">
              <Button
                size="sm"
                className="h-8 gap-1"
                onClick={() => onAccept(suggestion.id)}
                disabled={isAccepting || isDismissing}
              >
                <Tick01Icon className="h-3.5 w-3.5" />
                Approve
              </Button>
              <Button
                size="sm"
                variant="ghost"
                className="h-8 gap-1 text-muted-foreground"
                onClick={() => onDismiss(suggestion.id)}
                disabled={isAccepting || isDismissing}
              >
                <Cancel01Icon className="h-3.5 w-3.5" />
                Dismiss
              </Button>
            </div>
          </div>
        </div>
      </CardContent>
    </Card>
  )
}
