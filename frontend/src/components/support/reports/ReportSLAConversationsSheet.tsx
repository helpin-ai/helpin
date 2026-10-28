import { Link } from '@tanstack/react-router'
import { QuietEmptyState, QuietListRow, QuietMetaLine, QuietTextAction } from '@/components/design-system/quiet'
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from '@/components/ui/sheet'
import { Skeleton } from '@/components/ui/skeleton'
import { useSupportReportSLAConversations } from '@/hooks/queries/useSupportReports'
import type {
  SupportReportParams,
  SupportReportSLAConversation,
  SupportReportSLAMetric,
  SupportReportSLAOutcome,
} from '@/lib/supportReportTypes'
import { SUPPORT_SLA_METRIC_LABELS } from '@/lib/supportSLA'
import { timeAgo } from '@/lib/utils'

export interface ReportSLATarget {
  metric: SupportReportSLAMetric
  outcome: SupportReportSLAOutcome
}

function outcomeText(row: SupportReportSLAConversation, target: ReportSLATarget) {
  if (target.metric === 'next_response' && target.outcome === 'breached') {
    const late = row.next_response_breaches === 1 ? '1 late reply' : `${row.next_response_breaches} late replies`
    return row.outcome_at ? `${late} · last ${timeAgo(row.outcome_at)}` : late
  }
  if (!row.outcome_at) return null
  return `${target.outcome === 'breached' ? 'Missed' : 'Met'} ${timeAgo(row.outcome_at)}`
}

/** Lists the conversations behind one SLA count of the support report. */
export function ReportSLAConversationsSheet({
  workspaceId,
  workspaceSlug,
  params,
  target,
  onClose,
}: {
  workspaceId: string
  workspaceSlug: string
  params: SupportReportParams
  target: ReportSLATarget | null
  onClose: () => void
}) {
  const query = useSupportReportSLAConversations(workspaceId, params, target)
  const rows = query.data?.pages.flatMap((page) => page.data) ?? []
  const total = query.data?.pages[0]?.total ?? 0
  const title = target ? `${SUPPORT_SLA_METRIC_LABELS[target.metric]} ${target.outcome === 'breached' ? 'missed' : 'met'}` : ''

  return (
    <Sheet open={target !== null} onOpenChange={(open) => { if (!open) onClose() }}>
      <SheetContent className="flex flex-col gap-0 p-0 data-[side=right]:w-full data-[side=right]:sm:max-w-xl">
        <SheetHeader className="border-b border-quiet-divider-strong px-5 py-4 pr-14">
          <SheetTitle>{title}</SheetTitle>
          <SheetDescription>
            {query.data ? `${total.toLocaleString()} ${total === 1 ? 'conversation' : 'conversations'} created in this period.` : 'Conversations created in this period.'}
          </SheetDescription>
        </SheetHeader>
        <div className="min-h-0 flex-1 overflow-y-auto">
          {query.isLoading ? (
            <div className="space-y-3 p-5" aria-busy="true" aria-label="Loading conversations">
              <Skeleton className="h-12 w-full" />
              <Skeleton className="h-12 w-full" />
              <Skeleton className="h-12 w-full" />
            </div>
          ) : query.error ? (
            <QuietEmptyState
              className="border-t-0"
              title="Could not load these conversations"
              description={query.error instanceof Error ? query.error.message : 'Try again in a moment.'}
              action={<QuietTextAction onClick={() => void query.refetch()}>Try again</QuietTextAction>}
            />
          ) : rows.length === 0 ? (
            <QuietEmptyState
              className="border-t-0"
              title="No conversations"
              description="The report has changed since it loaded. Refresh it to see the latest counts."
            />
          ) : (
            <ul>
              {rows.map((row) => (
                <li key={row.id}>
                  <Link
                    to="/w/$slug/support/$conversationId"
                    params={{ slug: workspaceSlug, conversationId: row.id }}
                    className="block focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-quiet-text-primary"
                  >
                    <QuietListRow
                      state={target?.outcome === 'breached' ? 'blocker' : 'none'}
                      actor={[row.customer_name || row.customer_email, row.company_name].filter(Boolean).join(' · ') || 'Unknown customer'}
                      meta={target ? outcomeText(row, target) : null}
                      title={<span className="block truncate">#{row.display_id} {row.subject || '(no subject)'}</span>}
                      detail={
                        <QuietMetaLine
                          items={[
                            row.policy_name || null,
                            row.assignee_name ?? 'Unassigned',
                            <span key="priority" className="capitalize">{row.priority}</span>,
                          ]}
                        />
                      }
                    />
                  </Link>
                </li>
              ))}
            </ul>
          )}
          {query.hasNextPage ? (
            <div className="px-5 py-3">
              <QuietTextAction onClick={() => void query.fetchNextPage()} disabled={query.isFetchingNextPage}>
                {query.isFetchingNextPage ? 'Loading…' : `Show more (${(total - rows.length).toLocaleString()} left)`}
              </QuietTextAction>
            </div>
          ) : null}
        </div>
      </SheetContent>
    </Sheet>
  )
}
