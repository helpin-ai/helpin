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
import { useNavigate } from '@tanstack/react-router'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { cn, timeAgo } from '@/lib/utils'
import type { CRMSignal, CRMSignalDismissalReason, CRMSuggestion } from '@/lib/crmTypes'
import { useConfirm } from '@/components/ui/confirm-dialog'
import { useWorkspaceStore } from '@/stores/workspaceStore'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu'

const suggestionDismissalReasons: Array<{ value: CRMSignalDismissalReason; label: string }> = [
  { value: 'incorrect_evidence', label: 'Evidence is incorrect' },
  { value: 'wrong_entity', label: 'Wrong person or account' },
  { value: 'duplicate', label: 'Duplicate recommendation' },
  { value: 'irrelevant', label: 'Not relevant' },
  { value: 'handled', label: 'Already handled' },
  { value: 'bad_timing', label: 'Bad timing' },
]

type SuggestionTone = 'neutral' | 'good' | 'warn' | 'danger' | 'accent'

const typeConfig: Record<CRMSuggestion['suggestion_type'], { label: string; icon: typeof DollarCircleIcon; tone: SuggestionTone }> = {
  playbook_action: { label: 'Playbook action', icon: BulbIcon, tone: 'neutral' },
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

function approvalCopy(suggestion: CRMSuggestion) {
  const ctx = (suggestion.context || {}) as Record<string, unknown>
  if (suggestion.suggestion_type === 'deal_create') {
    return {
      label: 'Create deal',
      title: `Create ${contextValue(ctx.deal_name) || 'this deal'}?`,
      description: 'This immediately creates the deal with the pipeline, stage, amount, and contact shown in this recommendation.',
    }
  }
  if (suggestion.suggestion_type === 'deal_advance') {
    return {
      label: 'Change stage',
      title: `Move ${contextValue(ctx.deal_name) || 'this deal'} to ${contextValue(ctx.recommended_stage) || 'the recommended stage'}?`,
      description: 'This immediately changes the current deal stage. Review the supporting evidence before continuing.',
    }
  }
  return {
    label: 'Approve',
    title: 'Approve this recommendation?',
    description: 'Approval records your decision. Follow-up drafts are not sent automatically.',
  }
}

interface SuggestionCardProps {
  suggestion: CRMSuggestion
  onAccept: (id: string) => void
  onDismiss: (id: string, reason: CRMSignalDismissalReason) => void
  isAccepting?: boolean
  isDismissing?: boolean
  compact?: boolean
}

export function SuggestionCard({ suggestion, onAccept, onDismiss, isAccepting, isDismissing, compact }: SuggestionCardProps) {
  const confirm = useConfirm()
  const navigate = useNavigate()
  const workspaceSlug = useWorkspaceStore((state) => state.currentWorkspace?.slug ?? '')
  const config = typeConfig[suggestion.suggestion_type] ?? {
    label: suggestion.suggestion_type.replace(/_/g, ' '),
    icon: BulbIcon,
    tone: 'neutral' as const,
  }
  const Icon = config.icon
  const confidence = Math.round(suggestion.confidence * 100)
  const confidenceLevel = confidenceTone(suggestion.confidence)
  const rows = contextRows(suggestion)
  const approval = approvalCopy(suggestion)
  const evidence = suggestion.signals ?? []
  const suggestionContext = (suggestion.context || {}) as Record<string, unknown>
  const fallbackTarget = [
    { type: 'deal', id: contextValue(suggestionContext.deal_id) },
    { type: 'contact', id: contextValue(suggestionContext.contact_id) },
    { type: 'company', id: contextValue(suggestionContext.company_id) },
  ].find((target) => !!target.id)
  const targetType = suggestion.object_id && ['contact', 'company', 'deal', 'meeting'].includes(suggestion.object_type ?? '') ? suggestion.object_type : fallbackTarget?.type
  const targetID = suggestion.object_id && targetType === suggestion.object_type ? suggestion.object_id : fallbackTarget?.id
  const canOpenRecord = !!workspaceSlug && !!targetID && !!targetType
  const openRecord = () => {
    if (!canOpenRecord || !targetID) return
    if (targetType === 'contact') void navigate({ to: '/w/$slug/crm/contacts/$contactId', params: { slug: workspaceSlug, contactId: targetID } } as never)
    if (targetType === 'company') void navigate({ to: '/w/$slug/crm/companies/$companyId', params: { slug: workspaceSlug, companyId: targetID } } as never)
    if (targetType === 'deal') void navigate({ to: '/w/$slug/crm/deals/$dealId', params: { slug: workspaceSlug, dealId: targetID } } as never)
    if (targetType === 'meeting') void navigate({ to: '/w/$slug/crm/meetings/$meetingId', params: { slug: workspaceSlug, meetingId: targetID } } as never)
  }
  const approve = async () => {
    if (suggestion.suggestion_type === 'playbook_action') {
      await navigate({ to: '/w/$slug/crm/insights', params: { slug: workspaceSlug }, search: { scope: 'all', signal: contextValue(suggestion.context?.situation_id) || undefined } } as never)
      return
    }
    const accepted = await confirm({ title: approval.title, description: approval.description, confirmText: approval.label })
    if (accepted) onAccept(suggestion.id)
  }
  const canOpenEvidence = (signal: CRMSignal) => {
    if (!workspaceSlug) return false
    if (signal.source_type === 'email') return !!signal.source_thread_id && (!!signal.contact_id || !!signal.company_id)
    if (signal.source_type === 'meeting') return !!signal.source_id
    if (signal.source_type === 'support') return !!signal.source_thread_id
    return false
  }
  const openEvidence = (signal: CRMSignal) => {
    if (!canOpenEvidence(signal)) return
    if (signal.source_type === 'meeting' && signal.source_id) void navigate({ to: '/w/$slug/crm/meetings/$meetingId', params: { slug: workspaceSlug, meetingId: signal.source_id } } as never)
    else if (signal.source_type === 'support' && signal.source_thread_id) void navigate({ to: '/w/$slug/support/$conversationId', params: { slug: workspaceSlug, conversationId: signal.source_thread_id } } as never)
    else if (signal.contact_id) void navigate({ to: '/w/$slug/crm/contacts/$contactId', params: { slug: workspaceSlug, contactId: signal.contact_id }, search: { tab: 'emails', thread: signal.source_thread_id } } as never)
    else if (signal.company_id) void navigate({ to: '/w/$slug/crm/companies/$companyId', params: { slug: workspaceSlug, companyId: signal.company_id }, search: { tab: 'emails', thread: signal.source_thread_id } } as never)
  }

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
              <div className="w-24 shrink-0 text-right">
                <p className={cn('text-xs font-semibold tabular-nums', toneClasses(confidenceLevel).split(' ').filter((part) => part.startsWith('text-')).join(' '))}>
                  {confidence}%
                </p>
                <p className="mt-0.5 text-[9px] uppercase tracking-wide text-muted-foreground">recommendation</p>
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

            {evidence.length > 0 ? (
              <div className="mt-3 rounded-md border border-border/70 bg-muted/15 px-3 py-2.5">
                <p className="text-[10px] font-semibold uppercase tracking-wide text-muted-foreground">Why Helpin recommends this</p>
                <div className="mt-2 space-y-2">
                  {evidence.slice(0, compact ? 1 : 3).map((signal) => (
                    <div key={signal.id} className="text-xs leading-5">
                      <p className="font-medium text-foreground/85">{signal.summary}</p>
                      {signal.evidence_excerpt ? <p className="line-clamp-2 text-muted-foreground">“{signal.evidence_excerpt}”</p> : null}
                      <p className="text-[10px] uppercase tracking-wide text-muted-foreground/75">{signal.source_type.replace(/_/g, ' ')} · {signal.evidence_identity_trust || 'unknown'} identity</p>
                      {canOpenEvidence(signal) ? <button type="button" className="mt-1 text-[11px] font-medium text-orange-700 hover:text-orange-800 dark:text-orange-400" onClick={() => openEvidence(signal)}>Open exact source</button> : null}
                    </div>
                  ))}
                </div>
              </div>
            ) : (
              <p className="mt-3 text-[11px] text-amber-700 dark:text-amber-300">No linked signal evidence is available for this legacy recommendation.</p>
            )}

            <div className="mt-3 flex flex-wrap items-center gap-2">
              <Button
                size="sm"
                className="h-8 gap-1"
                onClick={() => void approve()}
                disabled={isAccepting || isDismissing}
              >
                <Tick01Icon className="h-3.5 w-3.5" />
                {suggestion.suggestion_type === 'playbook_action' ? 'Review action' : approval.label}
              </Button>
              {canOpenRecord ? <Button size="sm" variant="outline" className="h-8" onClick={openRecord}>Open record</Button> : null}
              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <Button size="sm" variant="ghost" className="h-8 gap-1 text-muted-foreground" disabled={isAccepting || isDismissing}>
                    <Cancel01Icon className="h-3.5 w-3.5" />
                    Dismiss
                  </Button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="start">
                  {suggestionDismissalReasons.map((reason) => <DropdownMenuItem key={reason.value} onSelect={() => onDismiss(suggestion.id, reason.value)}>{reason.label}</DropdownMenuItem>)}
                </DropdownMenuContent>
              </DropdownMenu>
            </div>
          </div>
        </div>
      </CardContent>
    </Card>
  )
}
