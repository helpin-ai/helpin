import { useEffect, useState } from 'react'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { CheckmarkCircle02Icon, FileSearchIcon, Loading01Icon } from '@/lib/icons'
import { useWorkspaceStore } from '@/stores/workspaceStore'
import { supportCoverageService } from '@/lib/services/supportCoverageService'
import type {
  SupportCoverageSummary,
  SupportCoverageGapListItem,
  SupportCoverageGapDetail,
} from '@/lib/supportCoverageTypes'
import { V1_GAP_TYPE_LABELS, GAP_STATUS_LABELS } from '@/lib/supportCoverageTypes'
import { timeAgo } from '@/lib/utils'

const GAP_TYPE_COLORS: Record<string, string> = {
  missing_article: 'bg-red-100 text-red-700',
  weak_article: 'bg-amber-100 text-amber-700',
  outdated_or_conflicting_article: 'bg-orange-100 text-orange-700',
  needs_review: 'bg-blue-100 text-blue-700',
}

const STATUS_COLORS: Record<string, string> = {
  open: 'bg-muted text-muted-foreground',
  drafted: 'bg-blue-100 text-blue-700',
  fixed: 'bg-green-100 text-green-700',
  ignored: 'bg-muted text-muted-foreground/60',
  human_only: 'bg-purple-100 text-purple-700',
}

export function SupportCoveragePage() {
  const workspace = useWorkspaceStore((s) => s.currentWorkspace)
  const wsId = workspace?.id ?? ''

  const [summary, setSummary] = useState<SupportCoverageSummary | null>(null)
  const [gaps, setGaps] = useState<SupportCoverageGapListItem[]>([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(true)
  const [selectedGap, setSelectedGap] = useState<SupportCoverageGapDetail | null>(null)
  const [detailLoading, setDetailLoading] = useState(false)
  const [statusFilter, setStatusFilter] = useState('')

  useEffect(() => {
    if (!wsId) return
    setLoading(true)
    Promise.all([
      supportCoverageService.getSummary(wsId),
      supportCoverageService.listGaps(wsId, statusFilter ? { status: statusFilter } : undefined),
    ]).then(([summaryRes, gapsRes]) => {
      if (summaryRes.data) setSummary(summaryRes.data)
      if (gapsRes.data) {
        setGaps(gapsRes.data.items || [])
        setTotal(gapsRes.data.total || 0)
      }
      setLoading(false)
    })
  }, [wsId, statusFilter])

  const openDetail = async (gapId: string) => {
    setDetailLoading(true)
    const { data } = await supportCoverageService.getGap(wsId, gapId)
    if (data) setSelectedGap(data)
    setDetailLoading(false)
  }

  const handleStatusUpdate = async (gapId: string, status: string) => {
    await supportCoverageService.updateGapStatus(wsId, gapId, status)
    // Refresh.
    setSelectedGap(null)
    const { data } = await supportCoverageService.listGaps(wsId, statusFilter ? { status: statusFilter } : undefined)
    if (data) {
      setGaps(data.items || [])
      setTotal(data.total || 0)
    }
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center py-20">
        <Loading01Icon className="h-6 w-6 animate-spin text-muted-foreground" />
      </div>
    )
  }

  return (
    <div className="mx-auto max-w-5xl space-y-6 p-6">
      <header>
        <h2 className="text-xl font-semibold">Docs Coverage</h2>
        <p className="text-sm text-muted-foreground">
          Gaps preventing AI from resolving customer issues autonomously.
        </p>
      </header>

      {/* Summary cards */}
      {summary && (
        <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
          <Card>
            <CardHeader className="p-3 pb-1">
              <CardTitle className="text-xs font-medium text-muted-foreground">New This Week</CardTitle>
            </CardHeader>
            <CardContent className="p-3 pt-0">
              <p className="text-2xl font-bold">{summary.new_gaps_this_week}</p>
            </CardContent>
          </Card>
          <Card>
            <CardHeader className="p-3 pb-1">
              <CardTitle className="text-xs font-medium text-muted-foreground">Open Gaps</CardTitle>
            </CardHeader>
            <CardContent className="p-3 pt-0">
              <p className="text-2xl font-bold">{summary.total_open_gaps}</p>
            </CardContent>
          </Card>
          <Card>
            <CardHeader className="p-3 pb-1">
              <CardTitle className="text-xs font-medium text-muted-foreground">Fixed This Week</CardTitle>
            </CardHeader>
            <CardContent className="p-3 pt-0">
              <p className="text-2xl font-bold text-green-600">{summary.gaps_fixed_this_week}</p>
            </CardContent>
          </Card>
          <Card>
            <CardHeader className="p-3 pb-1">
              <CardTitle className="text-xs font-medium text-muted-foreground">Evidence</CardTitle>
            </CardHeader>
            <CardContent className="p-3 pt-0">
              <p className="text-2xl font-bold">{summary.total_evidence_count}</p>
            </CardContent>
          </Card>
        </div>
      )}

      {/* Filter bar */}
      <div className="flex items-center gap-2">
        <span className="text-xs text-muted-foreground">{total} gap{total !== 1 ? 's' : ''}</span>
        <div className="flex gap-1 ml-auto">
          {['', 'open', 'drafted', 'fixed', 'ignored'].map((s) => (
            <button
              key={s}
              type="button"
              onClick={() => setStatusFilter(s)}
              className={`rounded-md px-2.5 py-1 text-xs font-medium transition-colors ${
                statusFilter === s
                  ? 'bg-primary text-primary-foreground'
                  : 'text-muted-foreground hover:bg-muted'
              }`}
            >
              {s === '' ? 'All' : GAP_STATUS_LABELS[s as keyof typeof GAP_STATUS_LABELS] ?? s}
            </button>
          ))}
        </div>
      </div>

      {/* Gap inbox */}
      <div className="flex gap-4">
        <div className={`${selectedGap ? 'w-1/2' : 'w-full'} space-y-1`}>
          {gaps.length === 0 ? (
            <div className="flex flex-col items-center py-12 text-muted-foreground">
              <FileSearchIcon className="h-10 w-10 text-muted-foreground/30 mb-2" />
              <p className="text-sm">No coverage gaps found.</p>
              <p className="text-xs text-muted-foreground/60 mt-1">Gaps appear when AI support encounters issues it cannot resolve.</p>
            </div>
          ) : (
            <div className="rounded-lg border border-border/60 bg-card divide-y divide-border/40">
              {gaps.map((gap) => (
                <button
                  key={gap.id}
                  type="button"
                  onClick={() => openDetail(gap.id)}
                  className={`flex w-full items-center gap-3 px-4 py-3 text-left text-sm transition-colors hover:bg-muted/40 ${
                    selectedGap?.id === gap.id ? 'bg-muted/60' : ''
                  }`}
                >
                  <div className="min-w-0 flex-1">
                    <p className="font-medium truncate">{gap.title}</p>
                    <p className="text-xs text-muted-foreground mt-0.5">
                      {gap.evidence_count} conversation{gap.evidence_count !== 1 ? 's' : ''}
                      {gap.topic_title && <> &middot; {gap.topic_title}</>}
                    </p>
                  </div>
                  <Badge variant="secondary" className={`text-xs shrink-0 ${GAP_TYPE_COLORS[gap.v1_gap_type] ?? ''}`}>
                    {V1_GAP_TYPE_LABELS[gap.v1_gap_type] ?? gap.v1_gap_type}
                  </Badge>
                  <span className="text-xs text-muted-foreground shrink-0">{timeAgo(gap.last_seen_at)}</span>
                </button>
              ))}
            </div>
          )}
        </div>

        {/* Detail panel */}
        {selectedGap && (
          <div className="w-1/2 rounded-lg border border-border/60 bg-card p-4 space-y-4">
            {detailLoading ? (
              <div className="flex justify-center py-8">
                <Loading01Icon className="h-5 w-5 animate-spin" />
              </div>
            ) : (
              <>
                <div className="flex items-start justify-between">
                  <div>
                    <h3 className="font-semibold">{selectedGap.title}</h3>
                    <div className="flex items-center gap-2 mt-1">
                      <Badge variant="secondary" className={`text-xs ${GAP_TYPE_COLORS[selectedGap.v1_gap_type] ?? ''}`}>
                        {V1_GAP_TYPE_LABELS[selectedGap.v1_gap_type] ?? selectedGap.v1_gap_type}
                      </Badge>
                      <Badge variant="secondary" className={`text-xs ${STATUS_COLORS[selectedGap.status] ?? ''}`}>
                        {GAP_STATUS_LABELS[selectedGap.status] ?? selectedGap.status}
                      </Badge>
                    </div>
                  </div>
                  <button
                    type="button"
                    onClick={() => setSelectedGap(null)}
                    className="text-muted-foreground hover:text-foreground text-xs"
                  >
                    Close
                  </button>
                </div>

                <div className="text-xs text-muted-foreground space-y-1">
                  <p>Issue: {selectedGap.issue_key || 'Unknown'}</p>
                  <p>First seen: {timeAgo(selectedGap.first_seen_at)}</p>
                  <p>Evidence: {selectedGap.evidence_count} item{selectedGap.evidence_count !== 1 ? 's' : ''}</p>
                </div>

                {/* Evidence */}
                {selectedGap.evidence.length > 0 && (
                  <div>
                    <h4 className="text-xs font-medium mb-2">Evidence</h4>
                    <div className="space-y-2 max-h-48 overflow-y-auto">
                      {selectedGap.evidence.map((ev) => (
                        <div key={ev.id} className="rounded-md bg-muted/50 p-2 text-xs">
                          <p className="text-muted-foreground">{ev.evidence_type} &middot; {timeAgo(ev.created_at)}</p>
                          {ev.excerpt && <p className="mt-1">{ev.excerpt}</p>}
                        </div>
                      ))}
                    </div>
                  </div>
                )}

                {/* Actions */}
                <div className="flex flex-wrap gap-2 pt-2 border-t border-border/40">
                  {selectedGap.status === 'open' && (
                    <>
                      <button
                        type="button"
                        onClick={() => handleStatusUpdate(selectedGap.id, 'ignored')}
                        className="rounded-md border border-border/60 px-2.5 py-1.5 text-xs font-medium text-muted-foreground hover:bg-muted"
                      >
                        Ignore
                      </button>
                      <button
                        type="button"
                        onClick={() => handleStatusUpdate(selectedGap.id, 'fixed')}
                        className="rounded-md bg-green-600 px-2.5 py-1.5 text-xs font-medium text-white hover:bg-green-700"
                      >
                        <CheckmarkCircle02Icon className="inline h-3 w-3 mr-1" />
                        Mark Fixed
                      </button>
                      <button
                        type="button"
                        onClick={() => handleStatusUpdate(selectedGap.id, 'human_only')}
                        className="rounded-md border border-border/60 px-2.5 py-1.5 text-xs font-medium text-muted-foreground hover:bg-muted"
                      >
                        Human Only
                      </button>
                    </>
                  )}
                  {selectedGap.status === 'ignored' && (
                    <button
                      type="button"
                      onClick={() => handleStatusUpdate(selectedGap.id, 'open')}
                      className="rounded-md border border-border/60 px-2.5 py-1.5 text-xs font-medium text-muted-foreground hover:bg-muted"
                    >
                      Reopen
                    </button>
                  )}
                </div>
              </>
            )}
          </div>
        )}
      </div>
    </div>
  )
}
