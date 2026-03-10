import { Check, X, DollarSign, TrendingUp, AlertTriangle, Mail, Lightbulb } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import type { CRMSuggestion } from '@/lib/crmTypes'

const typeConfig: Record<string, { label: string; icon: typeof DollarSign; variant: 'default' | 'secondary' | 'destructive' | 'outline' }> = {
  deal_create: { label: 'New Deal', icon: DollarSign, variant: 'default' },
  deal_advance: { label: 'Stage Advance', icon: TrendingUp, variant: 'secondary' },
  follow_up: { label: 'Follow Up', icon: Mail, variant: 'outline' },
  risk_alert: { label: 'Risk Alert', icon: AlertTriangle, variant: 'destructive' },
  enrichment: { label: 'Enrichment', icon: Lightbulb, variant: 'outline' },
}

function confidenceColor(confidence: number): string {
  if (confidence >= 0.9) return 'text-green-600 dark:text-green-400'
  if (confidence >= 0.7) return 'text-yellow-600 dark:text-yellow-400'
  return 'text-red-600 dark:text-red-400'
}

function confidenceBg(confidence: number): string {
  if (confidence >= 0.9) return 'bg-green-500'
  if (confidence >= 0.7) return 'bg-yellow-500'
  return 'bg-red-500'
}

interface SuggestionCardProps {
  suggestion: CRMSuggestion
  onAccept: (id: string) => void
  onDismiss: (id: string) => void
  isAccepting?: boolean
  isDismissing?: boolean
}

export function SuggestionCard({ suggestion, onAccept, onDismiss, isAccepting, isDismissing }: SuggestionCardProps) {
  const config = typeConfig[suggestion.suggestion_type] ?? { label: suggestion.suggestion_type, icon: Lightbulb, variant: 'outline' as const }
  const Icon = config.icon
  const ctx = (suggestion.context || {}) as Record<string, unknown>

  return (
    <Card className="transition-colors hover:bg-muted/30">
      <CardContent className="p-4">
        <div className="flex items-start gap-3">
          <div className="mt-0.5 flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-muted">
            <Icon className="h-4 w-4 text-muted-foreground" />
          </div>

          <div className="min-w-0 flex-1">
            <div className="flex items-center gap-2 mb-1">
              <Badge variant={config.variant} className="text-[10px] px-1.5 py-0">
                {config.label}
              </Badge>
              <div className="flex items-center gap-1.5">
                <div className={`h-1.5 w-12 rounded-full bg-muted overflow-hidden`}>
                  <div
                    className={`h-full rounded-full ${confidenceBg(suggestion.confidence)}`}
                    style={{ width: `${Math.round(suggestion.confidence * 100)}%` }}
                  />
                </div>
                <span className={`text-[11px] font-medium ${confidenceColor(suggestion.confidence)}`}>
                  {Math.round(suggestion.confidence * 100)}%
                </span>
              </div>
            </div>

            <h4 className="text-sm font-medium leading-snug">{suggestion.title}</h4>

            {suggestion.description && (
              <p className="mt-1 text-xs text-muted-foreground line-clamp-2">
                {suggestion.description}
              </p>
            )}

            {/* Context details for deal_create */}
            {suggestion.suggestion_type === 'deal_create' && (
              <div className="mt-2 flex flex-wrap gap-x-4 gap-y-1 text-xs text-muted-foreground">
                {!!ctx.contact_name && (
                  <span>Contact: <span className="text-foreground">{String(ctx.contact_name)}</span></span>
                )}
                {!!ctx.deal_name && (
                  <span>Deal: <span className="text-foreground">{String(ctx.deal_name)}</span></span>
                )}
                {ctx.amount != null && (
                  <span>Est: <span className="text-foreground">${Number(ctx.amount).toLocaleString()}</span></span>
                )}
              </div>
            )}

            {/* Context details for deal_advance */}
            {suggestion.suggestion_type === 'deal_advance' && (
              <div className="mt-2 flex flex-wrap gap-x-4 gap-y-1 text-xs text-muted-foreground">
                {!!ctx.deal_name && (
                  <span>Deal: <span className="text-foreground">{String(ctx.deal_name)}</span></span>
                )}
                {!!ctx.current_stage && !!ctx.recommended_stage && (
                  <span>{String(ctx.current_stage)} → <span className="text-foreground">{String(ctx.recommended_stage)}</span></span>
                )}
              </div>
            )}

            <div className="mt-3 flex items-center gap-2">
              <Button
                size="sm"
                className="h-7 text-xs gap-1"
                onClick={() => onAccept(suggestion.id)}
                disabled={isAccepting || isDismissing}
              >
                <Check className="h-3 w-3" />
                Approve
              </Button>
              <Button
                size="sm"
                variant="ghost"
                className="h-7 text-xs gap-1 text-muted-foreground"
                onClick={() => onDismiss(suggestion.id)}
                disabled={isAccepting || isDismissing}
              >
                <X className="h-3 w-3" />
                Dismiss
              </Button>
            </div>
          </div>
        </div>
      </CardContent>
    </Card>
  )
}
