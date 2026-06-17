import { useEffect, useState, useCallback, useMemo } from 'react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
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
  SupportCoverageClusterRebuildRun,
  SupportCoverageGapDetail,
  SupportCoverageGapListItem,
  SupportCoverageGapMergeSuggestion,
  SupportCoverageSummary,
} from '@/lib/supportCoverageTypes'
import { GAP_STATUS_LABELS, GAP_KIND_COLORS } from '@/lib/supportCoverageTypes'
import { cn, timeAgo } from '@/lib/utils'
import { useWorkspaceStore } from '@/stores/workspaceStore'

const STATUS_FILTERS = ['open', 'done', 'rejected'] as const
const GAP_PAGE_SIZE = 50
const KIND_TABS = [
  { value: 'all', label: 'All', dot: 'bg-muted-foreground/50', active: 'data-active:bg-muted/70' },
  { value: 'content', label: 'Content', dot: GAP_KIND_COLORS.content.dot, active: 'data-active:bg-blue-50 data-active:text-blue-800 dark:data-active:bg-blue-950/30 dark:data-active:text-blue-300' },
  { value: 'data', label: 'Data', dot: GAP_KIND_COLORS.data.dot, active: 'data-active:bg-amber-50 data-active:text-amber-800 dark:data-active:bg-amber-950/30 dark:data-active:text-amber-300' },
  { value: 'action', label: 'Action', dot: GAP_KIND_COLORS.action.dot, active: 'data-active:bg-purple-50 data-active:text-purple-800 dark:data-active:bg-purple-950/30 dark:data-active:text-purple-300' },
] as const
type KindTabValue = (typeof KIND_TABS)[number]['value']

export function SupportCoveragePage() {
  const workspace = useWorkspaceStore((s) => s.currentWorkspace)
  const wsId = workspace?.id ?? ''
  const wsSlug = workspace?.slug ?? ''

  const [summary, setSummary] = useState<SupportCoverageSummary | null>(null)
  const [gaps, setGaps] = useState<SupportCoverageGapListItem[]>([])
  const [total, setTotal] = useState(0)
  const [loadedPage, setLoadedPage] = useState(1)
  const [loading, setLoading] = useState(true)
  const [loadingMore, setLoadingMore] = useState(false)
  const [selectedGap, setSelectedGap] = useState<SupportCoverageGapDetail | null>(null)
  const [detailLoading, setDetailLoading] = useState(false)
  const [statusFilter, setStatusFilter] = useState<(typeof STATUS_FILTERS)[number]>('open')
  const [kindFilter, setKindFilter] = useState<KindTabValue>('all')
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
  const canRebuildClusters = has('settings.manage')
  const canReviewMergeSuggestions = has('support.edit')
  const [rebuildingClusters, setRebuildingClusters] = useState(false)
  const [latestClusterRun, setLatestClusterRun] = useState<SupportCoverageClusterRebuildRun | null>(null)
  const [mergeSuggestions, setMergeSuggestions] = useState<SupportCoverageGapMergeSuggestion[]>([])
  const [mergeActionSuggestionId, setMergeActionSuggestionId] = useState<string | null>(null)

  const listFilters = useCallback(
    (page: number) => ({
      status: statusFilter,
      page: String(page),
      per_page: String(GAP_PAGE_SIZE),
      ...(kindFilter !== 'all' && { gap_kind: kindFilter }),
    }),
    [statusFilter, kindFilter],
  )

  const handleRebuildClusters = useCallback(async () => {
    if (!wsId || rebuildingClusters) return
    setRebuildingClusters(true)
    try {
      const { data, error } = await supportCoverageService.rebuildClusters(wsId)
      if (error) {
        toast.error('Failed to rebuild gap clusters')
      } else {
        toast.success(
          `Cluster rebuild complete: ${data?.auto_merged ?? 0} merged, ${data?.suggestions_created ?? 0} for review.`,
        )
        const [summaryRes, gapsRes, latestRes] = await Promise.all([
          supportCoverageService.getSummary(wsId),
          supportCoverageService.listGaps(wsId, listFilters(1)),
          supportCoverageService.getLatestClusterRebuild(wsId),
        ])
        if (summaryRes.data) setSummary(summaryRes.data)
        if (gapsRes.data) {
          setGaps(gapsRes.data.items || [])
          setTotal(gapsRes.data.total || 0)
          setLoadedPage(1)
        }
        if (latestRes.data) setLatestClusterRun(latestRes.data)
      }
    } catch {
      toast.error('Failed to rebuild gap clusters')
    } finally {
      setRebuildingClusters(false)
    }
  }, [wsId, rebuildingClusters, listFilters])
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
      const [summaryRes, gapsRes, latestClusterRes] = await Promise.all([
        supportCoverageService.getSummary(wsId),
        supportCoverageService.listGaps(wsId, listFilters(1)),
        supportCoverageService.getLatestClusterRebuild(wsId),
      ])
      if (cancelled) return
      if (summaryRes.data) setSummary(summaryRes.data)
      setLatestClusterRun(latestClusterRes.data ?? null)
      if (gapsRes.data) {
        setGaps(gapsRes.data.items || [])
        setTotal(gapsRes.data.total || 0)
        setLoadedPage(1)
      }
      setLoading(false)
    }

    void loadCoverage()
    return () => {
      cancelled = true
    }
  }, [wsId, listFilters])

  const refreshGap = async (gapId: string) => {
    const [{ data }, suggestionsRes] = await Promise.all([
      supportCoverageService.getGap(wsId, gapId),
      supportCoverageService.listMergeSuggestions(wsId, gapId),
    ])
    if (data) setSelectedGap(data)
    if (suggestionsRes.data) setMergeSuggestions(suggestionsRes.data)
  }

  const refreshList = async () => {
    const { data } = await supportCoverageService.listGaps(wsId, listFilters(1))
    if (data) {
      setGaps(data.items || [])
      setTotal(data.total || 0)
      setLoadedPage(1)
    }
  }

  const handleLoadMore = async () => {
    if (loadingMore || gaps.length >= total) return
    const nextPage = loadedPage + 1
    setLoadingMore(true)
    const { data, error } = await supportCoverageService.listGaps(wsId, listFilters(nextPage))
    setLoadingMore(false)
    if (error) {
      toast.error(error || 'Failed to load more gaps')
      return
    }
    if (data) {
      setGaps((current) => [...current, ...(data.items || [])])
      setTotal(data.total || 0)
      setLoadedPage(nextPage)
    }
  }

  const openDetail = async (gapId: string) => {
    setDetailLoading(true)
    setGenerateError(null)
    setConfirmSuggestionId(null)
    setTargetSpaceId('')
    setTargetCollectionId('')
    setMergeSuggestions([])
    const [{ data }, suggestionsRes] = await Promise.all([
      supportCoverageService.getGap(wsId, gapId),
      supportCoverageService.listMergeSuggestions(wsId, gapId),
    ])
    if (data) setSelectedGap(data)
    if (suggestionsRes.data) setMergeSuggestions(suggestionsRes.data)
    setDetailLoading(false)
  }

  const handleStatusUpdate = async (gapId: string, status: string) => {
    await supportCoverageService.updateGapStatus(wsId, gapId, status)
    setSelectedGap(null)
    setMergeSuggestions([])
    await refreshList()
  }

  const handleApplyMergeSuggestion = async (suggestionId: string) => {
    if (!selectedGap || mergeActionSuggestionId) return
    setMergeActionSuggestionId(suggestionId)
    const { error } = await supportCoverageService.applyMergeSuggestion(wsId, suggestionId)
    setMergeActionSuggestionId(null)
    if (error) {
      toast.error(error || 'Failed to merge gaps')
      return
    }
    toast.success('Gaps merged')
    await refreshGap(selectedGap.id)
    await refreshList()
  }

  const handleDismissMergeSuggestion = async (suggestionId: string) => {
    if (!selectedGap || mergeActionSuggestionId) return
    setMergeActionSuggestionId(suggestionId)
    const { error } = await supportCoverageService.dismissMergeSuggestion(wsId, suggestionId)
    setMergeActionSuggestionId(null)
    if (error) {
      toast.error(error || 'Failed to dismiss suggestion')
      return
    }
    setMergeSuggestions((current) => current.filter((suggestion) => suggestion.id !== suggestionId))
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
          {canRebuildClusters && (
            <Button
              variant="outline"
              size="sm"
              disabled={rebuildingClusters}
              onClick={handleRebuildClusters}
            >
              {rebuildingClusters ? 'Rebuilding…' : 'Rebuild gap clusters'}
            </Button>
          )}
          {latestClusterRun?.completed_at && (
            <span className="text-[11px] text-muted-foreground/60">
              Last clustered {timeAgo(latestClusterRun.completed_at)} · {latestClusterRun.auto_merged} merged · {latestClusterRun.suggestions_created} review
            </span>
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
        <Tabs value={kindFilter} onValueChange={(value) => setKindFilter(value as KindTabValue)}>
          <TabsList variant="line" className="h-9 rounded-lg bg-transparent p-0.5">
            {KIND_TABS.map((tab) => (
              <TabsTrigger
                key={tab.value}
                value={tab.value}
                className={cn(
                  'h-8 rounded-md px-3 text-xs data-active:shadow-sm after:hidden',
                  tab.active,
                )}
              >
                <span className={cn('h-2 w-2 rounded-full', tab.dot)} />
                <span>{tab.label}</span>
              </TabsTrigger>
            ))}
          </TabsList>
        </Tabs>
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
          {gaps.length > 0 && (
            <div className="flex items-center justify-between px-1 pt-3">
              <span className="text-xs text-muted-foreground">
                Showing {gaps.length} of {total} gaps
              </span>
              {gaps.length < total && (
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  disabled={loadingMore}
                  onClick={handleLoadMore}
                >
                  {loadingMore ? 'Loading…' : 'Load more'}
                </Button>
              )}
            </div>
          )}
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
              mergeSuggestions={mergeSuggestions}
              canReviewMergeSuggestions={canReviewMergeSuggestions}
              mergeActionSuggestionId={mergeActionSuggestionId}
              onClose={() => setSelectedGap(null)}
              onTargetSpaceChange={setTargetSpaceId}
              onTargetCollectionChange={setTargetCollectionId}
              onSuggestImprovements={handleSuggestImprovements}
              onDraftNewArticle={handleDraftNewArticle}
              onRunDocumentationAgent={handleRunDocumentationAgent}
              onApplySuggestion={handleApplySuggestion}
              onDiscardSuggestion={handleDiscardSuggestion}
              onSetConfirmSuggestion={setConfirmSuggestionId}
              onApplyMergeSuggestion={handleApplyMergeSuggestion}
              onDismissMergeSuggestion={handleDismissMergeSuggestion}
              onStatusUpdate={handleStatusUpdate}
              onRegenerate={handleRegenerate}
            />
          </div>
        )}
      </div>
    </div>
  )
}
