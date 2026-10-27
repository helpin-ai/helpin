import { useState } from 'react'
import { Link } from '@tanstack/react-router'
import { Button } from '@/components/ui/button'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { describeReportTrend, formatReportDuration, formatReportPercent, formatReportRating } from '@/lib/supportReportFormat'
import type { SupportReportTopic } from '@/lib/supportReportTypes'

const COLLAPSED_TOPICS = 8

interface ReportTopicsTableProps {
  topics: SupportReportTopic[]
  /** New conversations in the period, the base of each topic's share. */
  totalCreated: number
  workspaceSlug: string
  /** Support search filters matching the report's period and inbox. */
  searchFilters: { created_from: string; mailbox_ids?: string }
}

/**
 * One row per tag, busiest first, with conversations without a tag last.
 * Each tag opens Support search narrowed to that tag and the report period.
 */
export function ReportTopicsTable({ topics, totalCreated, workspaceSlug, searchFilters }: ReportTopicsTableProps) {
  const [showAll, setShowAll] = useState(false)
  const tagged = topics.filter((topic) => topic.tag_id)
  const untagged = topics.find((topic) => !topic.tag_id)
  if (tagged.length === 0) {
    return (
      <p className="py-2 text-sm text-quiet-text-tertiary">
        {untagged ? 'None of the conversations in this period have tags.' : 'No conversations in this period.'}
      </p>
    )
  }
  const visible = showAll ? tagged : tagged.slice(0, COLLAPSED_TOPICS)
  const rows = untagged ? [...visible, untagged] : visible
  const multiTagged = topics.reduce((sum, topic) => sum + topic.conversations_created, 0) > totalCreated

  return (
    <div className="space-y-2">
      <div className="overflow-x-auto">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Topic</TableHead>
              <TableHead className="text-right">New</TableHead>
              <TableHead className="text-right">Share</TableHead>
              <TableHead className="text-right">AI resolved</TableHead>
              <TableHead className="text-right">Handed off</TableHead>
              <TableHead className="text-right">First response (team)</TableHead>
              <TableHead className="text-right">Resolution</TableHead>
              <TableHead className="text-right">CSAT</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {rows.map((topic) => {
              const trend = describeReportTrend(topic.conversations_created, topic.previous_conversations_created)
              return (
                <TableRow key={topic.tag_id || 'untagged'}>
                  <TableCell>
                    {topic.tag_id ? (
                      <Link
                        to="/w/$slug/support/search"
                        params={{ slug: workspaceSlug }}
                        search={{ tag_ids: topic.tag_id, ...searchFilters }}
                        className="inline-flex max-w-[240px] items-center gap-2 hover:underline"
                      >
                        <span className="h-2 w-2 shrink-0 rounded-full" style={{ backgroundColor: topic.color || 'var(--muted-foreground)' }} />
                        <span className="truncate">{topic.name}</span>
                      </Link>
                    ) : (
                      <span className="text-quiet-text-tertiary">No tag</span>
                    )}
                  </TableCell>
                  <TableCell className="text-right tabular-nums">
                    {topic.conversations_created}
                    {trend && trend.label !== 'No change' ? (
                      <span className="ml-1.5 text-[12px] text-quiet-text-tertiary">
                        {trend.label}<span className="sr-only"> vs previous period</span>
                      </span>
                    ) : null}
                  </TableCell>
                  <TableCell className="text-right tabular-nums">
                    {formatReportPercent(totalCreated ? topic.conversations_created / totalCreated : null)}
                  </TableCell>
                  <TableCell className="text-right tabular-nums">{formatReportPercent(topic.ai_resolution_rate)}</TableCell>
                  <TableCell className="text-right tabular-nums">{formatReportPercent(topic.ai_handoff_rate)}</TableCell>
                  <TableCell className="text-right tabular-nums">{formatReportDuration(topic.first_human_response.median_seconds)}</TableCell>
                  <TableCell className="text-right tabular-nums">{formatReportDuration(topic.resolution.median_seconds)}</TableCell>
                  <TableCell className="text-right tabular-nums">
                    {topic.csat_responses ? `${formatReportRating(topic.csat_average)} (${topic.csat_responses})` : '—'}
                  </TableCell>
                </TableRow>
              )
            })}
          </TableBody>
        </Table>
      </div>
      {tagged.length > COLLAPSED_TOPICS || multiTagged ? (
        <div className="flex flex-wrap items-center gap-x-4 gap-y-1 text-[12px] text-quiet-text-tertiary">
          {tagged.length > COLLAPSED_TOPICS ? (
            <Button variant="link" size="sm" className="h-auto px-0" onClick={() => setShowAll((value) => !value)}>
              {showAll ? 'Show fewer' : `Show all ${tagged.length} topics`}
            </Button>
          ) : null}
          {multiTagged ? <span>Conversations with several tags count under each tag.</span> : null}
        </div>
      ) : null}
    </div>
  )
}
