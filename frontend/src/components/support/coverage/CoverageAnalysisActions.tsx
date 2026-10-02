import { Button } from '@/components/ui/button'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import type { CoveragePipelineHealthV2 } from '@/lib/supportCoverageTypes'
import { coverageAnalysisControl } from './coverageHealth'

interface Props {
  health: CoveragePipelineHealthV2 | null
  canReanalyze: boolean
  submitting: boolean
  loading: boolean
  onReanalyze: () => void
}

export function CoverageAnalysisActions({ health, canReanalyze, submitting, loading, onReanalyze }: Props) {
  const state = coverageAnalysisControl(health, submitting)
  const disabled = loading || submitting || state.disabled
  const tooltip = 'Recheck eligible conversations from the last 30 days against current knowledge to identify new or changed coverage gaps.'
  return (
    <div className="flex flex-wrap items-center justify-end gap-3">
      {!loading && state.pill && (
        <Tooltip>
          <TooltipTrigger asChild>
            <span tabIndex={0} role="status" aria-live="polite" className="rounded-full bg-quiet-hover px-2.5 py-1 text-xs font-medium text-quiet-text-secondary">
              {state.pill}
            </span>
          </TooltipTrigger>
          <TooltipContent>{state.description}</TooltipContent>
        </Tooltip>
      )}
      {canReanalyze && (
        <Tooltip>
          <TooltipTrigger asChild>
            <span tabIndex={disabled ? 0 : undefined}>
              <Button size="sm" variant="outline" disabled={disabled} onClick={onReanalyze}>Re-analyze</Button>
            </span>
          </TooltipTrigger>
          <TooltipContent>{disabled && state.description ? state.description : tooltip}</TooltipContent>
        </Tooltip>
      )}
    </div>
  )
}
