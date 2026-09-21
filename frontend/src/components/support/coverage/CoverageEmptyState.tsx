import { Link } from '@tanstack/react-router'
import {
  QuietEmptyState,
  QuietTextAction,
} from '@/components/design-system/quiet'
import type { SupportCoverageGapStatus } from '@/lib/supportCoverageTypes'

interface CoverageEmptyStateProps {
  status: SupportCoverageGapStatus
  filtered: boolean
  mergeReview: boolean
  hasHistory: boolean
  wsSlug: string
  onClearFilters: () => void
  onShowOpen: () => void
  onViewAnalysis?: () => void
}

const coverageNeeds = [
  {
    title: 'Missing knowledge',
    description: 'Questions your articles do not answer clearly.',
  },
  {
    title: 'Missing context',
    description: 'Customer or account information the AI needs to help.',
  },
  {
    title: 'Missing actions',
    description:
      'Requests that need an action, workflow, or policy the AI cannot use.',
  },
]

export function CoverageEmptyState({
  status,
  filtered,
  mergeReview,
  hasHistory,
  wsSlug,
  onClearFilters,
  onShowOpen,
  onViewAnalysis,
}: CoverageEmptyStateProps) {
  if (mergeReview || filtered) {
    return (
      <QuietEmptyState
        title={
          mergeReview
            ? 'No merge suggestions to review'
            : 'No gaps match this filter'
        }
        description={
          mergeReview
            ? 'There are no pending duplicate suggestions in this view. Return to all gaps to continue reviewing coverage.'
            : 'Try another gap type or clear the filter to see all gaps in this status.'
        }
        action={
          <QuietTextAction
            className="underline underline-offset-4"
            onClick={onClearFilters}
          >
            Clear filters
          </QuietTextAction>
        }
      />
    )
  }

  if (status !== 'open') {
    return (
      <QuietEmptyState
        title={status === 'done' ? 'No completed gaps yet' : 'No rejected gaps'}
        description={
          status === 'done'
            ? 'Gaps you mark done will appear here, so you can revisit the evidence and the changes you made.'
            : 'Gaps you reject will appear here. You can reopen them if they need another look.'
        }
        action={
          <QuietTextAction
            className="underline underline-offset-4"
            onClick={onShowOpen}
          >
            View open gaps
          </QuietTextAction>
        }
      />
    )
  }

  return (
    <QuietEmptyState
      title={hasHistory ? 'No open coverage gaps' : 'No coverage gaps yet'}
      description="When analyzed support conversations reveal an unmet customer need, it will appear here with evidence and a recommended fix."
      action={
        <div className="flex flex-wrap items-center gap-x-6 gap-y-3">
          <QuietTextAction asChild className="underline underline-offset-4">
            <Link to="/w/$slug/support" params={{ slug: wsSlug }}>
              Go to support inbox
            </Link>
          </QuietTextAction>
          {onViewAnalysis && (
            <QuietTextAction
              className="underline underline-offset-4"
              onClick={onViewAnalysis}
            >
              View analysis status
            </QuietTextAction>
          )}
        </div>
      }
    >
      <dl className="max-w-[620px] divide-y divide-quiet-divider-light border-y border-quiet-divider-light">
        {coverageNeeds.map((need) => (
          <div
            key={need.title}
            className="grid gap-1 py-4 sm:grid-cols-[140px_minmax(0,1fr)] sm:gap-5"
          >
            <dt className="text-sm font-medium text-quiet-text-secondary">
              {need.title}
            </dt>
            <dd className="text-sm leading-relaxed text-quiet-text-tertiary">
              {need.description}
            </dd>
          </div>
        ))}
      </dl>
    </QuietEmptyState>
  )
}
