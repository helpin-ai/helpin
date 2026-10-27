import { useEffect, useMemo, useState } from 'react'
import { format } from 'date-fns'
import { toast } from 'sonner'
import {
  QuietEmptyState,
  QuietFilterDropdown,
  QuietMetricBlock,
  QuietMetricGrid,
  QuietPageHeader,
  QuietSection,
} from '@/components/design-system/quiet'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { useInboxScopes } from '@/hooks/queries/useSupport'
import { useSupportReportOverview } from '@/hooks/queries/useSupportReports'
import { Download04Icon, Loading01Icon } from '@/lib/icons'
import { supportReportsService } from '@/lib/services/supportReportsService'
import {
  SUPPORT_REPORT_RANGE_OPTIONS,
  describeReportTrend,
  formatReportDuration,
  formatReportPercent,
  formatReportRating,
  reportChannelLabel,
  supportReportParams,
  type ReportTrend,
  type SupportReportRangePreset,
} from '@/lib/supportReportFormat'
import type { SupportReportOverview } from '@/lib/supportReportTypes'
import { getInitials } from '@/lib/initials'
import { cn, timeAgo } from '@/lib/utils'
import { useWorkspaceStore } from '@/stores/workspaceStore'
import { ReportPortalIntake } from './ReportPortalIntake'
import { ReportDailyChart } from './ReportDailyChart'
import { ReportTopicsTable } from './ReportTopicsTable'

const ALL_INBOXES = 'all'
const COLLAPSED_COMMENTS = 6
const REPORT_REFRESH_MS = 60_000

function TrendText({ trend }: { trend: ReportTrend }) {
  if (!trend) return <span>No previous data</span>
  return (
    <span className={cn(trend.tone === 'positive' && 'text-emerald-600 dark:text-emerald-400', trend.tone === 'negative' && 'text-red-600 dark:text-red-400')}>
      {trend.label} vs previous period
    </span>
  )
}

const RATING_LABELS = ['Very unsatisfied', 'Unsatisfied', 'Neutral', 'Satisfied', 'Very satisfied']

function CsatDistribution({ distribution }: { distribution: SupportReportOverview['csat']['distribution'] }) {
  const total = distribution.reduce((sum, count) => sum + count, 0)
  return (
    <div className="space-y-1.5">
      {[...distribution].map((count, index) => ({ count, rating: index + 1 })).reverse().map(({ count, rating }) => (
        <div key={rating} className="flex items-center gap-3 text-[13px]">
          <span className="w-36 shrink-0 whitespace-nowrap text-quiet-text-tertiary">{rating} · {RATING_LABELS[rating - 1]}</span>
          <div className="h-2 min-w-0 flex-1 overflow-hidden rounded-full bg-quiet-divider-light">
            <div
              className={cn('h-full rounded-full', rating >= 4 ? 'bg-emerald-500/80' : rating === 3 ? 'bg-amber-400/80' : 'bg-red-500/70')}
              style={{ width: total ? `${(count / total) * 100}%` : 0 }}
            />
          </div>
          <span className="w-8 shrink-0 text-right tabular-nums text-quiet-text-secondary">{count}</span>
        </div>
      ))}
    </div>
  )
}

function ReportSkeleton() {
  return (
    <div className="space-y-4 px-4 py-5 sm:px-6 lg:px-8" aria-busy="true" aria-label="Loading report">
      <Skeleton className="h-24 w-full" />
      <Skeleton className="h-40 w-full" />
      <Skeleton className="h-40 w-full" />
    </div>
  )
}

export function SupportReportsPage() {
  const workspace = useWorkspaceStore((s) => s.currentWorkspace)
  const workspaceId = workspace?.id ?? ''
  const workspaceSlug = workspace?.slug ?? ''
  const [preset, setPreset] = useState<SupportReportRangePreset>('30d')
  const [inbox, setInbox] = useState<string>(ALL_INBOXES)
  const [exporting, setExporting] = useState(false)
  const [showAllComments, setShowAllComments] = useState(false)
  const { data: scopes } = useInboxScopes(workspaceId)

  // The period ends "now"; advance it every minute and when the tab regains
  // focus, so ratings and replies made meanwhile show up.
  const [now, setNow] = useState(() => new Date())
  useEffect(() => {
    const advance = () => setNow(new Date())
    const onVisible = () => {
      if (document.visibilityState === 'visible') advance()
    }
    const timer = window.setInterval(advance, REPORT_REFRESH_MS)
    window.addEventListener('focus', advance)
    document.addEventListener('visibilitychange', onVisible)
    return () => {
      window.clearInterval(timer)
      window.removeEventListener('focus', advance)
      document.removeEventListener('visibilitychange', onVisible)
    }
  }, [])
  const params = useMemo(
    () => supportReportParams(preset, inbox === ALL_INBOXES ? [] : [inbox], now),
    [preset, inbox, now],
  )
  const { data: report, isLoading, isFetching, error, refetch } = useSupportReportOverview(workspaceId, params)

  const inboxOptions = useMemo(() => {
    const options = [{ value: ALL_INBOXES, label: 'All inboxes' }]
    if (scopes?.shared_inbox) options.push({ value: 'shared', label: scopes.shared_inbox.name || 'Shared inbox' })
    for (const mailbox of scopes?.mailboxes ?? []) options.push({ value: mailbox.id, label: mailbox.name })
    return options
  }, [scopes])

  const handleExport = async () => {
    setExporting(true)
    try {
      const { blob, filename } = await supportReportsService.exportConversations(workspaceId, params)
      const url = URL.createObjectURL(blob)
      const link = document.createElement('a')
      link.href = url
      link.download = filename
      document.body.appendChild(link)
      link.click()
      link.remove()
      URL.revokeObjectURL(url)
    } catch (exportError) {
      toast.error(exportError instanceof Error ? exportError.message : 'Could not export conversations.')
    } finally {
      setExporting(false)
    }
  }

  const summary = report?.summary
  const previous = report?.previous
  // The period always ends now, so a start date is enough to match it.
  const topicSearchFilters = useMemo(
    () => ({ created_from: format(new Date(params.from), 'yyyy-MM-dd'), mailbox_ids: inbox === ALL_INBOXES ? undefined : inbox }),
    [params.from, inbox],
  )

  return (
    <div className="min-h-full pb-16">
      <QuietPageHeader
        variant="shell"
        title="Reports"
        description="How quickly your team and AI agent answer, how much gets resolved, and how customers rate it."
        actions={
          <>
            {isFetching && !isLoading ? <Loading01Icon className="h-4 w-4 animate-spin text-quiet-muted" aria-label="Refreshing" /> : null}
            <QuietFilterDropdown
              label="Period"
              value={preset}
              options={SUPPORT_REPORT_RANGE_OPTIONS}
              onChange={(value) => setPreset(value as SupportReportRangePreset)}
              searchable={false}
              bordered
            />
            <QuietFilterDropdown label="Inbox" value={inbox} options={inboxOptions} onChange={setInbox} bordered />
            <Button variant="outline" size="sm" onClick={handleExport} disabled={exporting || !workspaceId}>
              {exporting ? <Loading01Icon className="h-4 w-4 animate-spin" /> : <Download04Icon className="h-4 w-4" />}
              Export CSV
            </Button>
          </>
        }
      />

      {error ? (
        <div className="px-4 py-10 sm:px-6 lg:px-8">
          <QuietEmptyState
            title="The report could not load"
            description={error instanceof Error ? error.message : 'Try again in a moment.'}
            action={<Button variant="outline" size="sm" onClick={() => void refetch()}>Try again</Button>}
          />
        </div>
      ) : isLoading || !report || !summary || !previous ? (
        <ReportSkeleton />
      ) : (
        <>
          <QuietSection title="Volume and speed" bodyClassName="space-y-0">
            <QuietMetricGrid>
              <QuietMetricBlock
                className="items-start"
                label="New conversations"
                value={summary.conversations_created.toLocaleString()}
                description={<TrendText trend={describeReportTrend(summary.conversations_created, previous.conversations_created)} />}
              />
              <QuietMetricBlock
                className="items-start"
                label="Resolved"
                value={summary.conversations_resolved.toLocaleString()}
                description={<TrendText trend={describeReportTrend(summary.conversations_resolved, previous.conversations_resolved, { better: 'up' })} />}
              />
              <QuietMetricBlock
                className="items-start"
                label="First response (team)"
                value={formatReportDuration(summary.first_human_response.median_seconds)}
                description={
                  <>
                    Median · 90th percentile {formatReportDuration(summary.first_human_response.p90_seconds)}
                    <br />
                    <TrendText trend={describeReportTrend(summary.first_human_response.median_seconds, previous.first_human_response.median_seconds, { better: 'down', kind: 'duration' })} />
                  </>
                }
              />
              <QuietMetricBlock
                className="items-start"
                label="Time to resolution"
                value={formatReportDuration(summary.resolution.median_seconds)}
                description={
                  <>
                    Median · 90th percentile {formatReportDuration(summary.resolution.p90_seconds)}
                    <br />
                    <TrendText trend={describeReportTrend(summary.resolution.median_seconds, previous.resolution.median_seconds, { better: 'down', kind: 'duration' })} />
                  </>
                }
              />
            </QuietMetricGrid>
          </QuietSection>

          <QuietSection title="AI agent">
            <QuietMetricGrid>
              <QuietMetricBlock
                className="items-start"
                label="AI resolution rate"
                value={formatReportPercent(summary.ai_resolution_rate)}
                description={
                  <>
                    {summary.ai_resolved} of {summary.ai_involved} conversations the AI joined
                    {summary.ai_resolved ? ` · ${summary.ai_resolved_confirmed} confirmed, ${summary.ai_resolved_assumed} assumed` : ''}
                    <br />
                    <TrendText trend={describeReportTrend(summary.ai_resolution_rate, previous.ai_resolution_rate, { better: 'up', kind: 'rate' })} />
                  </>
                }
              />
              <QuietMetricBlock
                className="items-start"
                label="AI involvement"
                value={formatReportPercent(summary.ai_involvement_rate)}
                description={`${summary.ai_involved} of ${summary.conversations_created} new conversations`}
              />
              <QuietMetricBlock
                className="items-start"
                label="Handed to the team"
                value={formatReportPercent(summary.ai_handoff_rate)}
                description={`${summary.ai_handoffs} handoffs`}
              />
              <QuietMetricBlock
                className="items-start"
                label="First response (any)"
                value={formatReportDuration(summary.first_response.median_seconds)}
                description="Median, including AI replies"
              />
            </QuietMetricGrid>
          </QuietSection>

          <QuietSection title="Conversations per day">
            <ReportDailyChart days={report.daily} />
          </QuietSection>

          <QuietSection title="Topics" count="by conversation tag">
            <ReportTopicsTable
              topics={report.topics}
              totalCreated={summary.conversations_created}
              workspaceSlug={workspaceSlug}
              searchFilters={topicSearchFilters}
            />
          </QuietSection>

          {report.portal_intake ? <ReportPortalIntake intake={report.portal_intake} workspaceSlug={workspaceSlug} /> : null}

          <QuietSection title="Customer satisfaction" count={summary.csat_responses ? `${summary.csat_responses} ratings` : undefined}>
            {summary.csat_responses === 0 ? (
              <p className="py-2 text-sm text-quiet-text-tertiary">
                No ratings in this period. Customers are asked to rate resolved conversations when CSAT surveys are turned on in Chat widget settings.
              </p>
            ) : (
              <div className="grid gap-6 lg:grid-cols-[minmax(0,1fr)_minmax(0,1fr)]">
                <div className="space-y-4">
                  <div className="flex flex-wrap items-end gap-x-8 gap-y-2">
                    <div>
                      <div className="text-[28px] font-semibold tracking-[-0.02em] text-quiet-text-primary">{formatReportRating(summary.csat_average)}<span className="text-base font-normal text-quiet-text-tertiary"> / 5</span></div>
                      <div className="text-[12px] text-quiet-text-tertiary">
                        {formatReportPercent(summary.csat_positive_rate)} rated 4 or 5 ·{' '}
                        <TrendText trend={describeReportTrend(summary.csat_average, previous.csat_average, { better: 'up', kind: 'rating' })} />
                      </div>
                    </div>
                    <div className="text-[13px] text-quiet-text-tertiary">
                      <div>Resolved by AI <strong className="font-semibold text-quiet-text-primary">{formatReportRating(report.csat.ai_average)}</strong> ({report.csat.ai_responses})</div>
                      <div>Resolved by team <strong className="font-semibold text-quiet-text-primary">{formatReportRating(report.csat.human_average)}</strong> ({report.csat.human_responses})</div>
                    </div>
                  </div>
                  <CsatDistribution distribution={report.csat.distribution} />
                </div>
                <div className="min-w-0">
                  <div className="mb-2 text-[12px] font-semibold uppercase tracking-[0.06em] text-quiet-muted">Recent comments</div>
                  {report.csat.recent_comments.length === 0 ? (
                    <p className="text-sm text-quiet-text-tertiary">No comments yet.</p>
                  ) : (
                    <ul className="divide-y divide-quiet-divider-light">
                      {(showAllComments ? report.csat.recent_comments : report.csat.recent_comments.slice(0, COLLAPSED_COMMENTS)).map((comment) => (
                        <li key={comment.conversation_id} className="py-2.5">
                          <div className="flex items-center gap-2 text-[12px] text-quiet-text-tertiary">
                            <span className={cn('font-semibold tabular-nums', comment.rating >= 4 ? 'text-emerald-600 dark:text-emerald-400' : comment.rating <= 2 ? 'text-red-600 dark:text-red-400' : 'text-amber-600')}>{comment.rating}/5</span>
                            <a className="min-w-0 truncate hover:text-quiet-text-primary hover:underline" href={`/w/${workspaceSlug}/support/${comment.conversation_id}`}>
                              #{comment.display_id} {comment.subject}
                            </a>
                            <span className="ml-auto shrink-0">{timeAgo(comment.submitted_at)}</span>
                          </div>
                          <p className="mt-1 text-[13px] leading-relaxed text-quiet-text-secondary [overflow-wrap:anywhere]">{comment.comment}</p>
                        </li>
                      ))}
                    </ul>
                  )}
                  {report.csat.recent_comments.length > COLLAPSED_COMMENTS ? (
                    <Button variant="link" size="sm" className="h-auto px-0" onClick={() => setShowAllComments((value) => !value)}>
                      {showAllComments ? 'Show fewer' : `Show all ${report.csat.recent_comments.length} comments`}
                    </Button>
                  ) : null}
                </div>
              </div>
            )}
          </QuietSection>

          <QuietSection title="SLA targets" count={report.sla.conversations ? `${report.sla.conversations} conversations` : undefined}>
            {report.sla.conversations === 0 ? (
              <p className="py-2 text-sm text-quiet-text-tertiary">
                No conversations in this period had SLA targets. Set them up in <a className="underline hover:text-quiet-text-primary" href={`/w/${workspaceSlug}/settings/support-sla`}>SLA policies</a>.
              </p>
            ) : (
              <QuietMetricGrid className="xl:grid-cols-3">
                {([
                  ['First response', report.sla.first_response],
                  ['Next response', report.sla.next_response],
                  ['Resolution', report.sla.resolution],
                ] as const).map(([label, target]) => (
                  <QuietMetricBlock
                    key={label}
                    className="items-start"
                    label={label}
                    value={formatReportPercent(target.hit_rate)}
                    tone={target.hit_rate !== null && target.hit_rate < 0.8 ? 'warning' : 'neutral'}
                    description={`${target.met} met · ${target.missed} missed`}
                  />
                ))}
              </QuietMetricGrid>
            )}
          </QuietSection>

          <QuietSection title="Channels">
            {report.channels.length === 0 ? (
              <p className="text-sm text-quiet-text-tertiary">No conversations in this period.</p>
            ) : (
              <div className="overflow-x-auto">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>Channel</TableHead>
                      <TableHead className="text-right">New</TableHead>
                      <TableHead className="text-right">Resolved</TableHead>
                      <TableHead className="text-right">First response (team)</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {report.channels.map((channel) => (
                      <TableRow key={channel.channel}>
                        <TableCell>{reportChannelLabel(channel.channel)}</TableCell>
                        <TableCell className="text-right tabular-nums">{channel.conversations_created}</TableCell>
                        <TableCell className="text-right tabular-nums">{channel.conversations_resolved}</TableCell>
                        <TableCell className="text-right tabular-nums">{formatReportDuration(channel.first_human_response.median_seconds)}</TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </div>
            )}
          </QuietSection>

          <QuietSection title="Teammates">
            {report.teammates.length === 0 ? (
              <p className="text-sm text-quiet-text-tertiary">No teammate replies or resolutions in this period.</p>
            ) : (
              <div className="overflow-x-auto">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>Teammate</TableHead>
                      <TableHead className="text-right">Resolved</TableHead>
                      <TableHead className="text-right">Replies sent</TableHead>
                      <TableHead className="text-right">First response</TableHead>
                      <TableHead className="text-right">CSAT</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {report.teammates.map((teammate) => (
                      <TableRow key={teammate.user_id}>
                        <TableCell>
                          <span className="flex min-w-0 items-center gap-2">
                            {teammate.avatar_url ? (
                              <img src={teammate.avatar_url} alt="" className="h-6 w-6 shrink-0 rounded-full object-cover" />
                            ) : (
                              <span className="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-muted text-[10px] font-semibold">{getInitials(teammate.name)}</span>
                            )}
                            <span className="truncate">{teammate.name}</span>
                          </span>
                        </TableCell>
                        <TableCell className="text-right tabular-nums">{teammate.resolved}</TableCell>
                        <TableCell className="text-right tabular-nums">{teammate.replies_sent}</TableCell>
                        <TableCell className="text-right tabular-nums">{formatReportDuration(teammate.first_human_response.median_seconds)}</TableCell>
                        <TableCell className="text-right tabular-nums">
                          {teammate.csat_responses ? `${formatReportRating(teammate.csat_average)} (${teammate.csat_responses})` : '—'}
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </div>
            )}
          </QuietSection>
        </>
      )}
    </div>
  )
}
