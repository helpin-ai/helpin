import { Badge } from '@/components/ui/badge'
import { cn } from '@/lib/utils'

type GapImpactTier = 'low' | 'medium' | 'high'

const IMPACT_STYLES: Record<GapImpactTier, string> = {
  high: 'border-red-200 bg-red-50 text-red-700',
  medium: 'border-amber-200 bg-amber-50 text-amber-700',
  low: 'border-border/60 bg-muted/50 text-muted-foreground',
}

const IMPACT_LABELS: Record<GapImpactTier, string> = {
  high: 'High impact',
  medium: 'Medium impact',
  low: 'Low impact',
}

export function GapImpactBadge({
  tier,
  className,
}: {
  tier: GapImpactTier
  className?: string
}) {
  return (
    <Badge
      variant="secondary"
      className={cn('shrink-0 border text-[11px] font-medium', IMPACT_STYLES[tier], className)}
    >
      {IMPACT_LABELS[tier]}
    </Badge>
  )
}
