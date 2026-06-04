import { useEffect, useState, useCallback, useMemo, useRef } from 'react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { GapDetailPane } from '@/components/support/coverage/GapDetailPane'
import { GapList } from '@/components/support/coverage/GapList'
import { useAgents } from '@/hooks/queries/useAgents'
import { useDocsCollections, useDocsSpaces } from '@/hooks/queries/useDocs'
import { usePermissions, useWorkspaceAccess } from '@/hooks/queries/useSession'
import { Loading01Icon } from '@/lib/icons'
import { isAgentAvailableForTarget } from '@/lib/agentAccess'
import { agentService } from '@/lib/services/agentService'
import { supportCoverageService } from '@/lib/services/supportCoverageService'
import type {
  SupportCoverageGapDetail,
  SupportCoverageGapListItem,
  SupportCoverageSummary,
} from '@/lib/supportCoverageTypes'
import { GAP_STATUS_LABELS, GAP_KIND_DESCRIPTIONS, GAP_KIND_COLORS } from '@/lib/supportCoverageTypes'
import { timeAgo } from '@/lib/utils'
import { useWorkspaceStore } from '@/stores/workspaceStore'

const STATUS_FILTERS = ['open', 'done', 'rejected'] as const

export function SupportCoveragePage() {
  const workspace = useWorkspaceStore((s) => s.currentWorkspace)
  const wsId = workspace?.id ?? ''
  const wsSlug = workspace?.slug ?? ''

  const [summary, setSummary] = useState<SupportCoverageSummary | null>(null)
  const [gaps, setGaps] = useState<SupportCoverageGapListItem[]>([])
  const [, setTotal] = useState(0)
  const [loading, setLoading] = useState(true)
  const [selectedGap, setSelectedGap] = useState<SupportCoverageGapDetail | null>(null)
  const [detailLoading, setDetailLoading] = useState(false)
  const [statusFilter, setStatusFilter] = useState<(typeof STATUS_FILTERS)[number]>('open')
  const [kindFilter, setKindFilter] = useState('all')
  const [generating, setGenerating] = useState(false)
  const [generateError, setGenerateError] = useState<string | null>(null)
  const [targetSpaceId, setTargetSpaceId] = useState('')
  const [targetCollectionId, setTargetCollectionId] = useState('')
  const [applying, setApplying] = useState(false)
  const [confirmSuggestionId, setConfirmSuggestionId] = useState<string | null>(null)
  const [startingDocsAgentGapId, setStartingDocsAgentGapId] = useState<string | null>(null)

  const { data: agents = [] } = useAgents(wsId)
  const { data: spaces } = useDocsSpaces(wsId)
  const { data: collections } = useDocsCollections(wsId, targetSpaceId)
  const { data: access } = useWorkspaceAccess(wsId)
  const { has, isAdmin } = usePermissions(access)
  const accessibleTeamIds = useMemo(
    () => new Set((access?.team_memberships ?? []).map((team) => team.team_id)),
    [access?.team_memberships],
  )
  const canGenerate = has('support.edit') && has('docs.edit')
  const canReanalyze = has('settings.manage')
  const [reanalyzing, setReanalyzing] = useState(false)
  const reanalyzeBaselineRef = useRef<string | null>(null)

  const handleReanalyze = useCallback(async () => {
    if (!wsId || reanalyzing) return
    setReanalyzing(true)
    reanalyzeBaselineRef.current = summary?.last_analyzed_at ?? null
    try {
      const { error } = await supportCoverageService.triggerReanalysis(wsId)
      if (error) {
        toast.error(error === 'reanalysis already in progress' ? 'Reanalysis is already running' : 'Failed to start reanalysis')
        setReanalyzing(false)
        reanalyzeBaselineRef.current = null
      } else {
        toast.success('Reanalysis started — the page will refresh automatically when complete.')
      }
    } catch {
      toast.error('Failed to start reanalysis')
      setReanalyzing(false)
      reanalyzeBaselineRef.current = null
    }
  }, [wsId, reanalyzing, summary?.last_analyzed_at])

  // Poll summary while reanalyzing to detect completion.
  useEffect(() => {
    if (!reanalyzing || !wsId) return
    const interval = setInterval(async () => {
      const { data } = await supportCoverageService.getSummary(wsId)
      if (data?.last_analyzed_at && data.last_analyzed_at !== reanalyzeBaselineRef.current) {
        setReanalyzing(false)
        reanalyzeBaselineRef.current = null
        setSummary(data)
        // Refresh gap list with new data.
        const gapsRes = await supportCoverageService.listGaps(wsId, { status: statusFilter })
        if (gapsRes.data) {
          setGaps(gapsRes.data.items || [])
          setTotal(gapsRes.data.total || 0)
        }
        toast.success('Reanalysis complete.')
      }
    }, 30_000)
    return () => clearInterval(interval)
  }, [reanalyzing, wsId, statusFilter])
  const externalSpaces = spaces?.filter((space) => space.type === 'external_capable') ?? []
  const documentationAgent = agents.find(
    (agent) =>
      agent.is_system &&
      agent.preset_key === 'documentation_agent' &&
      isAgentAvailableForTarget(agent, {
        targetType: 'support_coverage_gap',
        accessibleTeamIds,
        canSeeAllAgents: isAdmin,
      }),
  )

  useEffect(() => {
    if (!wsId) return
    let cancelled = false

    async function loadCoverage() {
      setLoading(true)
      setSelectedGap(null)
      const [summaryRes, gapsRes] = await Promise.all([
        supportCoverageService.getSummary(wsId),
        supportCoverageService.listGaps(wsId, { status: statusFilter, ...(kindFilter !== 'all' && { gap_kind: kindFilter }) }),
      ])
      if (cancelled) return
      if (summaryRes.data) setSummary(summaryRes.data)
      if (gapsRes.data) {
        setGaps(gapsRes.data.items || [])
        setTotal(gapsRes.data.total || 0)
      }
      setLoading(false)
    }

    void loadCoverage()
    return () => {
      cancelled = true
    }
  }, [wsId, statusFilter, kindFilter])

  const refreshGap = async (gapId: string) => {
    const { data } = await supportCoverageService.getGap(wsId, gapId)
    if (data) setSelectedGap(data)
  }

  const refreshList = async () => {
    const { data } = await supportCoverageService.listGaps(wsId, { status: statusFilter, ...(kindFilter !== 'all' && { gap_kind: kindFilter }) })
    if (data) {
      setGaps(data.items || [])
      setTotal(data.total || 0)
    }
  }

  const openDetail = async (gapId: string) => {
    setDetailLoading(true)
    setGenerateError(null)
    setConfirmSuggestionId(null)
    setTargetSpaceId('')
    setTargetCollectionId('')
    const { data } = await supportCoverageService.getGap(wsId, gapId)
    if (data) setSelectedGap(data)
    setDetailLoading(false)
  }

  const handleStatusUpdate = async (gapId: string, status: string) => {
    await supportCoverageService.updateGapStatus(wsId, gapId, status)
    setSelectedGap(null)
    await refreshList()
  }

  const handleSuggestImprovements = async () => {
    if (!selectedGap || selectedGap.related_articles.length === 0) return
    setGenerating(true)
    setGenerateError(null)
    const { error } = await supportCoverageService.createArticleUpdate(wsId, selectedGap.id, {
      target_document_id: selectedGap.related_articles[0].document_id,
    })
    setGenerating(false)
    if (error) {
      setGenerateError(error)
      return
    }
    await refreshGap(selectedGap.id)
  }

  const handleDraftNewArticle = async () => {
    if (!selectedGap || !targetSpaceId) return
    setGenerating(true)
    setGenerateError(null)
    const { error } = await supportCoverageService.createArticleDraft(wsId, selectedGap.id, {
      target_space_id: targetSpaceId,
      target_collection_id: targetCollectionId || undefined,
    })
    setGenerating(false)
    if (error) {
      setGenerateError(error)
      return
    }
    await refreshGap(selectedGap.id)
  }

  const handleApplySuggestion = async (
    suggestionId: string,
    override?: { route?: 'create_article' | 'update_article'; target_document_id?: string },
  ) => {
    setApplying(true)
    await supportCoverageService.applySuggestion(wsId, suggestionId, override)
    setApplying(false)
    setConfirmSuggestionId(null)
    if (selectedGap) await refreshGap(selectedGap.id)
    await refreshList()
  }

  const handleDiscardSuggestion = async (suggestionId: string) => {
    await supportCoverageService.discardSuggestion(wsId, suggestionId)
    if (selectedGap) await refreshGap(selectedGap.id)
  }

  const handleRegenerate = async (gapId: string) => {
    const { error } = await supportCoverageService.regenerate(wsId, gapId)
    if (error) {
      setGenerateError(error)
      return
    }
    await refreshGap(gapId)
  }

  const handleRunDocumentationAgent = async (gapId: string) => {
    if (!wsId || !documentationAgent || startingDocsAgentGapId) return
    setStartingDocsAgentGapId(gapId)
    const { error } = await agentService.startRun(wsId, {
      agent_id: documentationAgent.id,
      target_type: 'support_coverage_gap',
      target_id: gapId,
      additional_context: 'Convert this support coverage gap into the right documentation work. Draft or propose changes for review before publishing.',
    })
    setStartingDocsAgentGapId(null)
    if (error) {
      toast.error(error || 'Failed to start Quill')
      return
    }
    toast.success('Quill started')
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center py-20">
        <Loading01Icon className="h-6 w-6 animate-spin text-muted-foreground" />
      </div>
    )
  }

  return (
    <div className="mx-auto max-w-7xl space-y-6 p-6">
      <header className="flex items-start justify-between">
        <div>
          <h2 className="text-xl font-semibold">Coverage Gaps</h2>
          <p className="text-sm text-muted-foreground">
            Issues AI couldn't fully resolve — with recommended fixes to close each gap.
          </p>
        </div>
        <div className="flex flex-col items-end gap-1">
          {canReanalyze && (
            <Button
              variant="outline"
              size="sm"
              disabled={reanalyzing}
              onClick={handleReanalyze}
            >
              {reanalyzing ? 'Reanalyzing…' : 'Reanalyze'}
            </Button>
          )}
          {summary?.last_analyzed_at && (
            <span className="text-[11px] text-muted-foreground/60">
              Last analyzed {timeAgo(summary.last_analyzed_at)}
            </span>
          )}
        </div>
      </header>

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
              <CardTitle className="text-xs font-medium text-muted-foreground">Done This Week</CardTitle>
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

      <div className="flex flex-wrap items-center gap-3">
        <Select value={kindFilter} onValueChange={setKindFilter}>
          <SelectTrigger className="h-8 w-[150px] text-xs">
            <SelectValue>
              {kindFilter === 'all' ? (
                'All types'
              ) : (
                <span className="flex items-center gap-1.5">
                  <span className={`inline-block h-2.5 w-2.5 rounded-full ${GAP_KIND_COLORS[kindFilter]?.dot}`} />
                  {kindFilter.charAt(0).toUpperCase() + kindFilter.slice(1)} gaps
                </span>
              )}
            </SelectValue>
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">All types</SelectItem>
            {(['content', 'data', 'action'] as const).map((kind) => {
              const colors = GAP_KIND_COLORS[kind]
              return (
                <SelectItem key={kind} value={kind}>
                  <span className="flex items-center gap-1.5">
                    <span className={`inline-block h-2.5 w-2.5 rounded-full ${colors.dot}`} />
                    <span className="font-medium">{kind.charAt(0).toUpperCase() + kind.slice(1)} gaps</span>
                    <span className="text-muted-foreground">— {GAP_KIND_DESCRIPTIONS[kind]}</span>
                  </span>
                </SelectItem>
              )
            })}
          </SelectContent>
        </Select>
        <div className="ml-auto flex rounded-lg border border-border/60 bg-muted/30 p-0.5">
          {STATUS_FILTERS.map((status) => (
            <button
              key={status}
              type="button"
              onClick={() => setStatusFilter(status)}
              className={`rounded-md px-3 py-1.5 text-xs font-medium transition-colors ${
                statusFilter === status
                  ? 'bg-background text-foreground shadow-sm'
                  : 'text-muted-foreground hover:bg-muted'
              }`}
            >
              {GAP_STATUS_LABELS[status]}
            </button>
          ))}
        </div>
      </div>

      <div className="flex gap-4">
        <div className={`${selectedGap ? 'w-1/2' : 'w-full'} space-y-1`}>
          <GapList
            gaps={gaps}
            selectedGapId={selectedGap?.id}
            onSelect={openDetail}
            compact={!!selectedGap}
          />
        </div>

        {selectedGap && (
          <div className="w-1/2 overflow-hidden rounded-lg border border-border/60 bg-card">
            <GapDetailPane
              gap={selectedGap}
              wsSlug={wsSlug}
              loading={detailLoading}
              canGenerate={canGenerate}
              canRunDocumentationAgent={canGenerate && Boolean(documentationAgent)}
              startingDocumentationAgent={startingDocsAgentGapId === selectedGap.id}
              externalSpaces={externalSpaces}
              collections={collections}
              targetSpaceId={targetSpaceId}
              targetCollectionId={targetCollectionId}
              generating={generating}
              generateError={generateError}
              applying={applying}
              confirmSuggestionId={confirmSuggestionId}
              onClose={() => setSelectedGap(null)}
              onTargetSpaceChange={setTargetSpaceId}
              onTargetCollectionChange={setTargetCollectionId}
              onSuggestImprovements={handleSuggestImprovements}
              onDraftNewArticle={handleDraftNewArticle}
              onRunDocumentationAgent={handleRunDocumentationAgent}
              onApplySuggestion={handleApplySuggestion}
              onDiscardSuggestion={handleDiscardSuggestion}
              onSetConfirmSuggestion={setConfirmSuggestionId}
              onStatusUpdate={handleStatusUpdate}
              onRegenerate={handleRegenerate}
            />
          </div>
        )}
      </div>
    </div>
  )
}
