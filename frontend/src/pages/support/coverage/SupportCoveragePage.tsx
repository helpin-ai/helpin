import { useEffect, useState, useCallback, useRef } from 'react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import {
  QuietPageHeader,
  QuietPageViewport,
} from '@/components/design-system/quiet'
import { Tabs, TabsList, TabsTrigger, TabsContent } from '@/components/ui/tabs'
import { CoverageAnalysisActions } from '@/components/support/coverage/CoverageAnalysisActions'
import { coverageAnalysisControl } from '@/components/support/coverage/coverageHealth'
import { CoverageInsights } from '@/components/support/coverage/CoverageInsights'
import { UpgradeRequiredDialog } from '@edition'
import {
  getUpgradeRequiredReason,
  type UpgradeRequiredReason,
} from '@edition/errors'
import {
  Sheet,
  SheetClose,
  SheetContent,
  SheetDescription,
  SheetTitle,
} from '@/components/ui/sheet'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/design-system/quiet-dropdown-select'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { GapDetailPane } from '@/components/support/coverage/GapDetailPane'
import { PMFilterBar } from '@/components/pm/PMFilterControls'
import { GapList } from '@/components/support/coverage/GapList'
import { CoverageEmptyState } from '@/components/support/coverage/CoverageEmptyState'
import { useCoverageInsightPages } from '@/hooks/queries/useCoverageInsightPages'
import { useDocsCollections, useDocsSpaces } from '@/hooks/queries/useDocs'
import { usePermissions, useWorkspaceAccess } from '@/hooks/queries/useSession'
import { Loading01Icon } from '@/lib/icons'
import { supportCoverageService } from '@/lib/services/supportCoverageService'
import {
  formatClusterRebuildSuccess,
  isClusterRebuildResult,
} from '@/lib/supportCoverageClusterRebuild'
import type {
  CoverageSuggestionReview,
  SupportCoverageClusterRebuildRun,
  SupportCoverageGapDetail,
  SupportCoverageGapListItem,
  SupportCoverageGapMergeSuggestion,
  SupportCoverageSummary,
  CoveragePipelineHealthV2,
  CoverageTopicDetailV2,
} from '@/lib/supportCoverageTypes'
import { GAP_STATUS_LABELS, GAP_KIND_COLORS } from '@/lib/supportCoverageTypes'
import { cn, timeAgo } from '@/lib/utils'
import { useWorkspaceStore } from '@/stores/workspaceStore'

const STATUS_FILTERS = ['open', 'done', 'rejected'] as const
const GAP_PAGE_SIZE = 50
const KIND_OPTIONS = [
  { value: 'all', label: 'All gaps', dot: 'bg-muted-foreground/50' },
  { value: 'content', label: 'Content gaps', dot: GAP_KIND_COLORS.content.dot },
  { value: 'data', label: 'Data gaps', dot: GAP_KIND_COLORS.data.dot },
  { value: 'action', label: 'Action gaps', dot: GAP_KIND_COLORS.action.dot },
] as const
type KindFilterValue = (typeof KIND_OPTIONS)[number]['value']

export function SupportCoveragePage() {
  const workspace = useWorkspaceStore((s) => s.currentWorkspace)
  const wsId = workspace?.id ?? ''
  const wsSlug = workspace?.slug ?? ''

  const [summary, setSummary] = useState<SupportCoverageSummary | null>(null)
  const [gaps, setGaps] = useState<SupportCoverageGapListItem[]>([])
  const [total, setTotal] = useState(0)
  const [loadedPage, setLoadedPage] = useState(1)
  const [loading, setLoading] = useState(true)
  const [loadError, setLoadError] = useState<string | null>(null)
  const [reloadKey, setReloadKey] = useState(0)
  const [detailError, setDetailError] = useState<string | null>(null)
  const detailRequest = useRef(0)
  const detailRefreshRequest = useRef(0)
  const activeGapId = useRef<string | null>(null)
  const listRequest = useRef(0)
  const [upgradeReason, setUpgradeReason] =
    useState<UpgradeRequiredReason | null>(null)
  const [statusUpdating, setStatusUpdating] = useState(false)
  const [loadingMore, setLoadingMore] = useState(false)
  const [selectedGapId, setSelectedGapId] = useState<string | null>(null)
  const [selectedGap, setSelectedGap] =
    useState<SupportCoverageGapDetail | null>(null)
  const [detailLoading, setDetailLoading] = useState(false)
  const [statusFilter, setStatusFilter] =
    useState<(typeof STATUS_FILTERS)[number]>('open')
  const [reviewEarlierDetections, setReviewEarlierDetections] = useState(false)
  const [conversationFilter, setConversationFilter] = useState<{ workspaceId: string; id: string } | null>(null)
  const conversationId = conversationFilter?.workspaceId === wsId ? conversationFilter.id : ''
  const [kindFilter, setKindFilter] = useState<KindFilterValue>('all')
  const [showMergeSuggestionsOnly, setShowMergeSuggestionsOnly] =
    useState(false)
  const [mergeSuggestionCount, setMergeSuggestionCount] = useState(0)
  const [generating, setGenerating] = useState(false)
  const [generateError, setGenerateError] = useState<string | null>(null)
  const [targetSpaceId, setTargetSpaceId] = useState('')
  const [targetCollectionId, setTargetCollectionId] = useState('')
  const [applying, setApplying] = useState(false)
  const [coverageSurface, setCoverageSurface] = useState('gaps')
  const [healthV2, setHealthV2] = useState<CoveragePipelineHealthV2 | null>(
    null,
  )
  const topicsQuery = useCoverageInsightPages(wsId, 'topics', Boolean(healthV2?.rollout.read_v2_enabled), supportCoverageService.listTopicsV2)
  const signalsQuery = useCoverageInsightPages(wsId, 'signals', Boolean(healthV2?.rollout.read_v2_enabled), supportCoverageService.listSignalsV2)
  const refreshTopics = topicsQuery.refresh
  const refreshSignals = signalsQuery.refresh
  const topicsV2 = topicsQuery.items
  const signalsV2 = signalsQuery.items
  const [selectedTopicV2, setSelectedTopicV2] =
    useState<CoverageTopicDetailV2 | null>(null)
  const [topicLoading, setTopicLoading] = useState(false)
  const [insightErrors, setInsightErrors] = useState<Record<string, string>>({})
  const [pendingSignalId, setPendingSignalId] = useState<string | null>(null)
  const [submittingAnalysis, setSubmittingAnalysis] = useState(false)
  const analysisSubmission = useRef(false)
  const analysisRequest = useRef(0)
  const healthSnapshot = useRef<CoveragePipelineHealthV2 | null>(null)
  const analysisWorkspace = useRef(wsId)
  useEffect(() => {
    analysisRequest.current += 1
    analysisSubmission.current = false
    healthSnapshot.current = null
  }, [wsId])
  const topicRequest = useRef(0)
  const [signalTopicSelections, setSignalTopicSelections] = useState<
    Record<string, string>
  >({})

  const { data: spaces } = useDocsSpaces(wsId)
  const { data: collections } = useDocsCollections(wsId, targetSpaceId)
  const { data: access } = useWorkspaceAccess(wsId)
  const { has } = usePermissions(access)
  const canGenerate = has('support.edit') && has('docs.edit')
  const canRebuildClusters = has('settings.manage')
  const canReviewMergeSuggestions = has('support.edit')
  const [rebuildingClusters, setRebuildingClusters] = useState(false)
  const [latestClusterRun, setLatestClusterRun] =
    useState<SupportCoverageClusterRebuildRun | null>(null)
  const [mergeSuggestions, setMergeSuggestions] = useState<
    SupportCoverageGapMergeSuggestion[]
  >([])
  const [mergeActionSuggestionId, setMergeActionSuggestionId] = useState<
    string | null
  >(null)

  const listFilters = useCallback(
    (page: number) => ({
      status: reviewEarlierDetections ? 'open' : statusFilter,
      ...(reviewEarlierDetections && { review_only: 'true' }),
      ...(conversationId && { conversation_id: conversationId }),
      page: String(page),
      per_page: String(GAP_PAGE_SIZE),
      ...(kindFilter !== 'all' && { gap_kind: kindFilter }),
      ...(showMergeSuggestionsOnly && { has_merge_suggestions: 'true' }),
    }),
    [statusFilter, kindFilter, showMergeSuggestionsOnly, reviewEarlierDetections, conversationId],
  )

  const refreshMergeSuggestionCount = useCallback(async () => {
    if (!wsId) return
    const { data } = await supportCoverageService.listGaps(wsId, {
      status: 'open',
      page: '1',
      per_page: '1',
      has_merge_suggestions: 'true',
    })
    setMergeSuggestionCount(data?.total ?? 0)
  }, [wsId])

  const handleRebuildClusters = useCallback(async () => {
    if (!wsId || rebuildingClusters) return
    setRebuildingClusters(true)
    try {
      const { data, error } = await supportCoverageService.rebuildClusters(wsId)
      if (error) {
        toast.error('Failed to rebuild gap clusters')
      } else if (!isClusterRebuildResult(data)) {
        toast.error('Failed to rebuild gap clusters')
      } else if (
        data.status === 'failed' ||
        data.embedding_status === 'failed'
      ) {
        toast.error(formatClusterRebuildSuccess(data))
      } else {
        toast.success(formatClusterRebuildSuccess(data))
        const [summaryRes, gapsRes, latestRes, mergeSuggestionRes] =
          await Promise.all([
            supportCoverageService.getSummary(wsId),
            supportCoverageService.listGaps(wsId, listFilters(1)),
            supportCoverageService.getLatestClusterRebuild(wsId),
            supportCoverageService.listGaps(wsId, {
              status: 'open',
              page: '1',
              per_page: '1',
              has_merge_suggestions: 'true',
            }),
          ])
        if (summaryRes.data) setSummary(summaryRes.data)
        if (gapsRes.data) {
          setGaps(gapsRes.data.items || [])
          setTotal(gapsRes.data.total || 0)
          setLoadedPage(1)
        }
        if (latestRes.data) setLatestClusterRun(latestRes.data)
        setMergeSuggestionCount(mergeSuggestionRes.data?.total ?? 0)
      }
    } catch {
      toast.error('Failed to rebuild gap clusters')
    } finally {
      setRebuildingClusters(false)
    }
  }, [wsId, rebuildingClusters, listFilters])
  const externalSpaces =
    spaces?.filter((space) => space.type === 'external_capable') ?? []

  useEffect(() => {
    if (!wsId) return
    let cancelled = false

    async function loadCoverage() {
      if (analysisWorkspace.current !== wsId) {
        analysisWorkspace.current = wsId
        setHealthV2(null)
        setSubmittingAnalysis(false)
      }
      listRequest.current += 1
      setLoadingMore(false)
      setLoading(true)
      setLoadError(null)
      detailRequest.current += 1
      topicRequest.current += 1
      setSelectedTopicV2(null)
      setTopicLoading(false)
      activeGapId.current = null
      setSelectedGapId(null)
      setSelectedGap(null)
      const [
        summaryRes,
        gapsRes,
        latestClusterRes,
        mergeSuggestionRes,
        healthV2Res,
      ] = await Promise.all([
        supportCoverageService.getSummary(wsId),
        supportCoverageService.listGaps(wsId, listFilters(1)),
        supportCoverageService.getLatestClusterRebuild(wsId),
        supportCoverageService.listGaps(wsId, {
          status: 'open',
          page: '1',
          per_page: '1',
          has_merge_suggestions: 'true',
        }),
        supportCoverageService.getPipelineHealthV2(wsId),
      ])
      if (cancelled) return
      const errors: Record<string, string> = {}
      if (healthV2Res.error) errors.health = 'Could not load analysis status.'
      setSummary(summaryRes.data ?? null)
      setGaps(gapsRes.data?.items ?? [])
      setTotal(gapsRes.data?.total ?? 0)
      setLoadError(
        gapsRes.error ? 'Could not load coverage gaps. Try again.' : null,
      )
      setInsightErrors(errors)
      setLatestClusterRun(latestClusterRes.data ?? null)
      setLoadedPage(1)
      setMergeSuggestionCount(mergeSuggestionRes.data?.total ?? 0)
      healthSnapshot.current = healthV2Res.data ?? null
      setHealthV2(healthV2Res.data ?? null)
      if (!healthV2Res.data?.rollout.read_v2_enabled) {
        setCoverageSurface((current) =>
          current === 'topics' || current === 'signals' ? 'gaps' : current,
        )
      }
      setLoading(false)
    }

    void loadCoverage()
    return () => {
      cancelled = true
    }
  }, [wsId, listFilters, reloadKey])

  useEffect(() => {
    if (!wsId) return
    let cancelled = false
    const timer = window.setInterval(async () => {
      if (document.visibilityState !== 'visible') return
      const { data, error } = await supportCoverageService.getPipelineHealthV2(wsId)
      if (cancelled) return
      if (data) {
        const previous = healthSnapshot.current
        const wasActive = previous?.reanalysis_status === 'queued' || previous?.reanalysis_status === 'running' || previous?.latest_batch?.status === 'running'
        const nowActive = data.reanalysis_status === 'queued' || data.reanalysis_status === 'running' || data.latest_batch?.status === 'running'
        if (wasActive && !nowActive) {
          setReloadKey(key => key + 1)
          if (data.rollout.read_v2_enabled) {
            void refreshTopics()
            void refreshSignals()
          }
        }
        healthSnapshot.current = data
        setHealthV2(data)
        if (!data.rollout.read_v2_enabled) {
          setCoverageSurface(current => current === 'topics' || current === 'signals' ? 'gaps' : current)
        }
      }
      setInsightErrors(current => {
        const next = { ...current }
        if (error) next.health = 'Could not load analysis status.'
        else delete next.health
        return next
      })
    }, 30_000)
    return () => {
      cancelled = true
      window.clearInterval(timer)
    }
  }, [wsId, refreshTopics, refreshSignals])

  const openTopicV2 = async (topicId: string) => {
    const request = ++topicRequest.current
    setSelectedTopicV2(null)
    setTopicLoading(true)
    const { data, error } = await supportCoverageService.getTopicV2(
      wsId,
      topicId,
    )
    if (request !== topicRequest.current) return
    setTopicLoading(false)
    if (error) {
      toast.error('Could not load this topic. Try again.')
      return
    }
    setSelectedTopicV2(data ?? null)
  }

  const reviewSignalV2 = async (signalId: string) => {
    const topicId = signalTopicSelections[signalId]
    if (!topicId || pendingSignalId) return
    setPendingSignalId(signalId)
    const { error } = await supportCoverageService.reviewSignalV2(
      wsId,
      signalId,
      topicId,
    )
    if (error) {
      setPendingSignalId(null)
      toast.error('Failed to attach signal')
      return
    }
    await Promise.all([signalsQuery.refresh(), topicsQuery.refresh()])
    setPendingSignalId(null)
    toast.success('Signal attached to topic')
  }

  const dismissSignalV2 = async (signalId: string) => {
    if (pendingSignalId) return
    setPendingSignalId(signalId)
    const { error } = await supportCoverageService.dismissSignalV2(
      wsId,
      signalId,
    )
    if (error) {
      setPendingSignalId(null)
      toast.error('Failed to dismiss signal')
      return
    }
    await signalsQuery.refresh()
    setPendingSignalId(null)
  }

  const reanalyze = async () => {
    if (analysisSubmission.current || !has('settings.manage') || coverageAnalysisControl(insightErrors.health ? null : healthV2).disabled) return
    analysisSubmission.current = true
    setSubmittingAnalysis(true)
    const request = ++analysisRequest.current
    try {
      const { error } = await supportCoverageService.triggerReanalysis(wsId)
      if (request !== analysisRequest.current) return
      if (error) {
        toast.error(error)
      } else {
        toast.success('Re-analysis queued')
        setHealthV2(current => current ? { ...current, reanalysis_status: 'queued' } : current)
        setReloadKey(key => key + 1)
        if (healthV2?.rollout.read_v2_enabled) {
          void refreshTopics()
          void refreshSignals()
        }
      }
      const { data, error: healthError } = await supportCoverageService.getPipelineHealthV2(wsId)
      if (request !== analysisRequest.current) return
      if (data) {
        healthSnapshot.current = data
        setHealthV2(data)
      }
      setInsightErrors(current => {
        const next = { ...current }
        if (healthError) next.health = 'Could not load analysis status.'
        else delete next.health
        return next
      })
    } catch {
      if (request === analysisRequest.current) toast.error('Could not queue re-analysis. Reload the page and try again.')
    } finally {
      if (request === analysisRequest.current) {
        analysisSubmission.current = false
        setSubmittingAnalysis(false)
      }
    }
  }

  const showAIError = (error: string) => {
    const reason = getUpgradeRequiredReason(error)
    if (reason) setUpgradeReason(reason)
    else setGenerateError(error)
  }

  const refreshGap = async (gapId: string) => {
    if (activeGapId.current !== gapId) return
    const request = detailRequest.current
    const refreshRequest = ++detailRefreshRequest.current
    const [{ data }, suggestionsRes] = await Promise.all([
      supportCoverageService.getGap(wsId, gapId),
      supportCoverageService.listMergeSuggestions(wsId, gapId),
    ])
    if (request !== detailRequest.current || refreshRequest !== detailRefreshRequest.current) return
    if (data) setSelectedGap(data)
    else toast.error('Could not refresh the gap. Close it and try again.')
    if (suggestionsRes.data) setMergeSuggestions(suggestionsRes.data)
    return data
  }

  const refreshList = async () => {
    const request = ++listRequest.current
    setLoadingMore(false)
    const [{ data }, , summaryRes] = await Promise.all([
      supportCoverageService.listGaps(wsId, listFilters(1)),
      refreshMergeSuggestionCount(),
      supportCoverageService.getSummary(wsId),
    ])
    if (request !== listRequest.current) return
    if (!data)
      toast.error('Could not refresh the list. Use Refresh to try again.')
    if (summaryRes.data) setSummary(summaryRes.data)
    if (data) {
      setGaps(data.items || [])
      setTotal(data.total || 0)
      setLoadedPage(1)
    }
  }

  const handleLoadMore = async () => {
    if (loadingMore || gaps.length >= total) return
    const request = listRequest.current
    const nextPage = loadedPage + 1
    setLoadingMore(true)
    const { data, error } = await supportCoverageService.listGaps(
      wsId,
      listFilters(nextPage),
    )
    if (request !== listRequest.current) return
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
    activeGapId.current = gapId
    const request = ++detailRequest.current
    setDetailError(null)
    setSelectedGapId(gapId)
    setSelectedGap(null)
    setDetailLoading(true)
    setGenerateError(null)
    setTargetSpaceId('')
    setTargetCollectionId('')
    setMergeSuggestions([])
    const [{ data }, suggestionsRes] = await Promise.all([
      supportCoverageService.getGap(wsId, gapId),
      supportCoverageService.listMergeSuggestions(wsId, gapId),
    ])
    if (request !== detailRequest.current) return
    if (data) {
      setSelectedGap(data)
      const proposal = data.suggestions.find(suggestion => suggestion.status === 'draft' && suggestion.is_active !== false && !suggestion.superseded_at)
      setTargetSpaceId(proposal?.target_space_id ?? '')
      setTargetCollectionId(proposal?.target_collection_id ?? '')
    } else setDetailError('Could not load this gap. Try again.')
    if (suggestionsRes.data) setMergeSuggestions(suggestionsRes.data)
    setDetailLoading(false)
  }

  const closeDetail = () => {
    activeGapId.current = null
    detailRequest.current += 1
    setDetailError(null)
    setSelectedGapId(null)
    setSelectedGap(null)
    setDetailLoading(false)
    setGenerateError(null)
    setMergeSuggestions([])
    setMergeActionSuggestionId(null)
  }

  const handleStatusUpdate = async (gapId: string, status: string) => {
    if (statusUpdating) return
    setStatusUpdating(true)
    const { error } = await supportCoverageService.updateGapStatus(
      wsId,
      gapId,
      status,
    )
    setStatusUpdating(false)
    if (error) {
      toast.error('Could not update this gap. Your changes were not saved.')
      return
    }
    toast.success(
      status === 'done'
        ? 'Gap marked done'
        : status === 'open'
          ? 'Gap reopened'
          : 'Gap rejected',
    )
    if (activeGapId.current === gapId) closeDetail()
    await refreshList()
  }

  const handleReclassify = async (gapId: string, gapType: string) => {
    if (statusUpdating) return
    setStatusUpdating(true)
    const { error } = await supportCoverageService.reclassifyGap(wsId, gapId, gapType)
    setStatusUpdating(false)
    if (error) {
      toast.error('Could not classify this detection. Try again.')
      return
    }
    if (activeGapId.current === gapId) closeDetail()
    await refreshList()
    toast.success('Detection classified')
  }

  const handleApplyMergeSuggestion = async (suggestionId: string) => {
    if (!selectedGap || mergeActionSuggestionId) return
    setMergeActionSuggestionId(suggestionId)
    const { error } = await supportCoverageService.applyMergeSuggestion(
      wsId,
      suggestionId,
    )
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
    const { error } = await supportCoverageService.dismissMergeSuggestion(
      wsId,
      suggestionId,
    )
    setMergeActionSuggestionId(null)
    if (error) {
      toast.error(error || 'Failed to dismiss suggestion')
      return
    }
    setMergeSuggestions((current) =>
      current.filter((suggestion) => suggestion.id !== suggestionId),
    )
    await refreshList()
  }

  const handleSuggestImprovements = async () => {
    if (!selectedGap || selectedGap.related_articles.length === 0) return
    setGenerating(true)
    setGenerateError(null)
    const { error } = await supportCoverageService.createArticleUpdate(
      wsId,
      selectedGap.id,
      {
        target_document_id: selectedGap.related_articles[0].document_id,
      },
    )
    setGenerating(false)
    if (error) {
      showAIError(error)
      return
    }
    await refreshGap(selectedGap.id)
  }

  const handleDraftNewArticle = async () => {
    if (!selectedGap || !targetSpaceId) return
    setGenerating(true)
    setGenerateError(null)
    const { error } = await supportCoverageService.createArticleDraft(
      wsId,
      selectedGap.id,
      {
        target_space_id: targetSpaceId,
        target_collection_id: targetCollectionId || undefined,
      },
    )
    setGenerating(false)
    if (error) {
      showAIError(error)
      return
    }
    await refreshGap(selectedGap.id)
  }

  const handleApplySuggestion = async (
    suggestionId: string,
    override?: CoverageSuggestionReview,
  ) => {
    if (applying || !selectedGap) return null
    const gapId = selectedGap.id
    setApplying(true)
    const { error } = await supportCoverageService.applySuggestion(
      wsId,
      suggestionId,
      override,
    )
    if (error) {
      setApplying(false)
      showAIError(error)
      return null
    }
    toast.success('Draft saved for review. Gap remains open.')
    const refreshed = await refreshGap(gapId)
    await refreshList()
    setApplying(false)
    return refreshed?.suggestions.find(suggestion => suggestion.id === suggestionId)?.result_document_id ?? null
  }

  const handleDiscardSuggestion = async (suggestionId: string) => {
    const { error } = await supportCoverageService.discardSuggestion(
      wsId,
      suggestionId,
    )
    if (error) {
      toast.error('Could not discard this suggestion')
      return
    }
    if (selectedGap) await refreshGap(selectedGap.id)
  }

  const handleRegenerate = async (gapId: string) => {
    const { error } = await supportCoverageService.regenerate(wsId, gapId)
    if (error) {
      showAIError(error)
      return
    }
    await refreshGap(gapId)
  }


  const refreshCoverage = () => {
    setReloadKey(key => key + 1)
    if (healthV2?.rollout.read_v2_enabled) {
      void refreshTopics()
      void refreshSignals()
    }
  }

  return (
    <QuietPageViewport contentClassName="space-y-5">
      <QuietPageHeader
        className="[&>div]:flex-col sm:[&>div]:flex-row [&>div>div:last-child]:self-start sm:[&>div>div:last-child]:self-auto"
        title="Support coverage"
        description="Find recurring customer needs, review the evidence, and close support gaps. Analysis runs automatically every 3 hours."
        actions={
          <CoverageAnalysisActions
            health={insightErrors.health ? null : healthV2}
            canReanalyze={has('settings.manage')}
            submitting={submittingAnalysis}
            loading={loading}
            onReanalyze={() => { void reanalyze() }}
          />
        }
      />
      {summary &&
        (summary.total_open_gaps > 0 ||
          summary.new_gaps_this_week > 0 ||
          summary.gaps_fixed_this_week > 0 ||
          summary.total_evidence_count > 0 ||
          summary.last_analyzed_at) && (
          <div
            className="flex flex-wrap gap-x-6 gap-y-2 text-sm text-quiet-text-tertiary"
            aria-label="Coverage summary"
          >
            <span>
              <strong className="font-semibold text-quiet-text-primary">
                {summary.total_open_gaps}
              </strong>{' '}
              open gaps
            </span>
            <span>
              <strong className="font-semibold text-quiet-text-primary">
                {summary.new_gaps_this_week}
              </strong>{' '}
              new this week
            </span>
            <span>
              <strong className="font-semibold text-quiet-positive">
                {summary.gaps_fixed_this_week}
              </strong>{' '}
              closed this week
            </span>
            <span>
              <strong className="font-semibold text-quiet-text-primary">
                {summary.total_evidence_count}
              </strong>{' '}
              evidence records
            </span>
            {summary.last_analyzed_at && (
              <span>Last analyzed {timeAgo(summary.last_analyzed_at)}</span>
            )}
          </div>
        )}
      <Tabs
        value={coverageSurface}
        onValueChange={setCoverageSurface}
        className="gap-5"
      >
        <div className="overflow-x-auto">
          <TabsList
            variant="quiet"
            aria-label="Coverage views"
            className="min-w-full"
          >
            <TabsTrigger value="gaps">Gaps to resolve</TabsTrigger>
            {healthV2?.rollout.read_v2_enabled && (
              <>
                <TabsTrigger value="topics">Customer topics</TabsTrigger>
                <TabsTrigger value="signals">
                  Needs review{' '}
                  <span className="text-quiet-muted">{signalsQuery.total ?? (signalsQuery.hasMore ? `${signalsV2.length}+` : signalsV2.length)}</span>
                </TabsTrigger>
              </>
            )}
          </TabsList>
        </div>
        <TabsContent value="gaps" className="space-y-4">
          <div className="flex flex-wrap items-center gap-3">
            <Select
              size="sm"
              value={kindFilter}
              onValueChange={(value) => setKindFilter(value as KindFilterValue)}
            >
              <SelectTrigger
                size="sm"
                className="w-[150px]"
                aria-label="Gap type"
              >
                <SelectValue />
              </SelectTrigger>
              <SelectContent align="start">
                {KIND_OPTIONS.map((option) => (
                  <SelectItem key={option.value} value={option.value}>
                    <span className="flex items-center gap-2">
                      <span
                        className={cn('h-2 w-2 rounded-full', option.dot)}
                      />
                      <span>{option.label}</span>
                    </span>
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            {mergeSuggestionCount > 0 && !showMergeSuggestionsOnly && (
              <button
                type="button"
                onClick={() => {
                  setShowMergeSuggestionsOnly(true)
                  setReviewEarlierDetections(false)
                  setConversationFilter(null)
                  setStatusFilter('open')
                }}
                className="ml-auto text-xs font-medium text-amber-700 underline-offset-4 transition-colors hover:text-amber-800 hover:underline dark:text-amber-300 dark:hover:text-amber-200"
              >
                Show {mergeSuggestionCount} merge{' '}
                {mergeSuggestionCount === 1 ? 'suggestion' : 'suggestions'}
              </button>
            )}
            <div
              className={cn(
                'flex items-center gap-1',
                mergeSuggestionCount === 0 || showMergeSuggestionsOnly
                  ? 'ml-auto'
                  : '',
              )}
            >
              {STATUS_FILTERS.map((status) => (
                <button
                  key={status}
                  aria-pressed={statusFilter === status}
                  type="button"
                  disabled={(showMergeSuggestionsOnly || reviewEarlierDetections) && status !== 'open'}
                  onClick={() => {
                    setStatusFilter(status)
                    if (status !== 'open') setShowMergeSuggestionsOnly(false)
                  }}
                  className={cn(
                    'border-b-2 border-transparent px-3 py-2 text-sm transition-colors focus-visible:outline-2 focus-visible:outline-ring disabled:pointer-events-none disabled:opacity-40',
                    statusFilter === status
                      ? 'border-quiet-text-primary font-semibold text-quiet-text-primary'
                      : 'text-muted-foreground hover:bg-muted',
                  )}
                >
                  {GAP_STATUS_LABELS[status]}
                </button>
              ))}
            </div>
          </div>

          <PMFilterBar
            definitions={[
              {
                key: 'kind',
                label: 'Gap type',
                singleSelect: true,
                options: KIND_OPTIONS.filter(
                  (option) => option.value !== 'all',
                ).map((option) => ({
                  value: option.value,
                  label: option.label,
                })),
              },
            ]}
            values={{ kind: kindFilter === 'all' ? [] : [kindFilter] }}
            visibleKeys={new Set(kindFilter === 'all' ? [] : ['kind'])}
            onToggle={(_, value) =>
              setKindFilter((current) =>
                current === value ? 'all' : (value as KindFilterValue),
              )
            }
            onRemove={() => setKindFilter('all')}
            onClearAll={() => setKindFilter('all')}
          />
          {summary && ((canRebuildClusters && summary.total_open_gaps > 0) || (canReviewMergeSuggestions && (summary.unreviewed_detection_count ?? 0) > 0)) && (
            <div className="flex flex-wrap items-center justify-end gap-3">
              {canReviewMergeSuggestions && (summary.unreviewed_detection_count ?? 0) > 0 && !reviewEarlierDetections && (
                <Button variant="ghost" size="sm" onClick={() => {
                  setConversationFilter(null)
                  setShowMergeSuggestionsOnly(false)
                  setStatusFilter('open')
                  setKindFilter('all')
                  setReviewEarlierDetections(true)
                }}>
                  Review earlier detections ({summary.unreviewed_detection_count})
                </Button>
              )}
              {canRebuildClusters && latestClusterRun?.completed_at && (
                <span className="text-xs text-quiet-text-tertiary">
                  Last checked {timeAgo(latestClusterRun.completed_at)} ·{' '}
                  {latestClusterRun.auto_merged} merged ·{' '}
                  {latestClusterRun.suggestions_created} for review
                </span>
              )}
              {canRebuildClusters && summary.total_open_gaps > 0 && <Tooltip>
                <TooltipTrigger asChild>
                  <Button
                    variant="ghost"
                    size="sm"
                    disabled={rebuildingClusters}
                    onClick={handleRebuildClusters}
                  >
                    {rebuildingClusters
                      ? 'Checking duplicates…'
                      : 'Check for duplicate gaps'}
                  </Button>
                </TooltipTrigger>
                <TooltipContent>
                  Merge high-confidence duplicates and suggest uncertain matches
                  for review.
                </TooltipContent>
              </Tooltip>}
            </div>
          )}
          <div className="space-y-1">
            {(reviewEarlierDetections || conversationId) && (
              <div className="mb-3 flex flex-wrap items-center justify-between gap-2 rounded-lg bg-muted px-3 py-2 text-xs">
                <span>{reviewEarlierDetections ? 'Review earlier detections. Classify relevant gaps or dismiss irrelevant ones.' : 'Open gaps from the selected conversation.'}</span>
                <button type="button" className="font-medium underline-offset-4 hover:underline" onClick={() => {
                  setReviewEarlierDetections(false)
                  setConversationFilter(null)
                }}>Back to all gaps</button>
              </div>
            )}
            {showMergeSuggestionsOnly && (
              <div className="mb-3 flex flex-wrap items-center justify-between gap-2 rounded-lg bg-amber-50 px-3 py-2 text-xs text-amber-900 dark:bg-amber-950/30 dark:text-amber-200">
                <span>
                  Viewing merge suggestions for open gaps. Review similar gaps
                  and merge or keep separate.
                </span>
                <button
                  type="button"
                  onClick={() => setShowMergeSuggestionsOnly(false)}
                  className="font-medium underline-offset-4 hover:underline"
                >
                  Back to all gaps
                </button>
              </div>
            )}
            <div>
              {loading ? (
                <p
                  role="status"
                  className="py-10 text-sm text-quiet-text-tertiary"
                >
                  Loading coverage…
                </p>
              ) : loadError ? (
                <div role="alert" className="py-8 space-y-3">
                  <p>{loadError}</p>
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => setReloadKey((key) => key + 1)}
                  >
                    Try again
                  </Button>
                </div>
              ) : gaps.length === 0 && (reviewEarlierDetections || conversationId) ? (
                <p className="py-10 text-sm text-quiet-text-tertiary">
                  {reviewEarlierDetections ? 'No earlier detections waiting for review.' : 'No open gaps linked to this conversation.'}
                </p>
              ) : gaps.length === 0 ? (
                <CoverageEmptyState
                  status={statusFilter}
                  filtered={kindFilter !== 'all'}
                  mergeReview={showMergeSuggestionsOnly}
                  hasHistory={Boolean(
                    summary?.last_analyzed_at || summary?.total_evidence_count,
                  )}
                  wsSlug={wsSlug}
                  onClearFilters={() => {
                    setKindFilter('all')
                    setShowMergeSuggestionsOnly(false)
                  }}
                  onShowOpen={() => setStatusFilter('open')}
                />
              ) : (
                <GapList
                  gaps={gaps}
                  selectedGapId={selectedGapId ?? undefined}
                  onSelect={openDetail}
                />
              )}
              {!loading && !loadError && gaps.length > 0 && (
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
          </div>
        </TabsContent>
        <CoverageInsights
          topics={topicsV2}
          signals={signalsV2}
          selectedTopic={selectedTopicV2}
          topicLoading={topicLoading}
          wsSlug={wsSlug}
          errors={{ ...insightErrors, ...(topicsQuery.error && { topics: 'Could not load customer topics.' }), ...(signalsQuery.error && { signals: 'Could not load review signals.' }) }}
          topicsLoading={topicsQuery.loading}
          signalsLoading={signalsQuery.loading}
          topicPagination={topicsQuery}
          signalPagination={signalsQuery}
          onOpenConversationGaps={(id) => {
            topicRequest.current += 1
            setSelectedTopicV2(null)
            setTopicLoading(false)
            setConversationFilter({ workspaceId: wsId, id })
            setReviewEarlierDetections(false)
            setShowMergeSuggestionsOnly(false)
            setStatusFilter('open')
            setKindFilter('all')
            setCoverageSurface('gaps')
          }}
          signalTopicSelections={signalTopicSelections}
          onSelectSignalTopic={(signalId, topicId) =>
            setSignalTopicSelections((current) => ({
              ...current,
              [signalId]: topicId,
            }))
          }
          onOpenTopic={openTopicV2}
          onCloseTopic={() => {
            topicRequest.current += 1
            setSelectedTopicV2(null)
            setTopicLoading(false)
          }}
          onAttachSignal={reviewSignalV2}
          onDismissSignal={dismissSignalV2}
          canEdit={canReviewMergeSuggestions}
          pendingSignalId={pendingSignalId}
          onRefresh={refreshCoverage}
        />
      </Tabs>
      <Sheet
        open={Boolean(selectedGapId)}
        onOpenChange={(open) => !open && closeDetail()}
      >
        <SheetContent
          side="right"
          showCloseButton={false}
          overlayClassName="bg-black/20"
          className="block overflow-y-auto border-l border-border/50 bg-card p-0 shadow-2xl data-[side=right]:w-full data-[side=right]:sm:w-[min(780px,calc(100vw-24px))] data-[side=right]:sm:max-w-none"
        >
          <SheetTitle className="sr-only">Coverage gap details</SheetTitle>
          <SheetDescription className="sr-only">
            Review evidence, recommendations, merge suggestions, and actions for
            the selected coverage gap.
          </SheetDescription>
          {selectedGap ? (
            <GapDetailPane
              key={selectedGap.id}
              gap={selectedGap}
              wsSlug={wsSlug}
              onRefresh={() => { void refreshGap(selectedGap.id); void refreshList() }}
              loading={detailLoading}
              canGenerate={canGenerate}
              canEdit={canReviewMergeSuggestions}
              statusUpdating={statusUpdating}
              externalSpaces={externalSpaces}
              collections={collections}
              targetSpaceId={targetSpaceId}
              targetCollectionId={targetCollectionId}
              generating={generating}
              generateError={generateError}
              applying={applying}
              mergeSuggestions={mergeSuggestions}
              canReviewMergeSuggestions={canReviewMergeSuggestions}
              mergeActionSuggestionId={mergeActionSuggestionId}
              onClose={closeDetail}
              onTargetSpaceChange={(spaceId) => {
                setTargetSpaceId(spaceId)
                setTargetCollectionId('')
              }}
              onTargetCollectionChange={setTargetCollectionId}
              onSuggestImprovements={handleSuggestImprovements}
              onDraftNewArticle={handleDraftNewArticle}
              onApplySuggestion={handleApplySuggestion}
              onDiscardSuggestion={handleDiscardSuggestion}
              onApplyMergeSuggestion={handleApplyMergeSuggestion}
              onDismissMergeSuggestion={handleDismissMergeSuggestion}
              onStatusUpdate={handleStatusUpdate}
              onReclassify={handleReclassify}
              onRegenerate={handleRegenerate}
            />
          ) : (
            <div className="relative flex min-h-svh items-center justify-center">
              <SheetClose asChild>
                <Button
                  type="button"
                  variant="ghost"
                  size="icon-sm"
                  className="absolute right-4 top-4 text-muted-foreground"
                >
                  x<span className="sr-only">Close</span>
                </Button>
              </SheetClose>
              {detailError ? (
                <div role="alert" className="space-y-3 p-6">
                  <p>{detailError}</p>
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() =>
                      selectedGapId && void openDetail(selectedGapId)
                    }
                  >
                    Try again
                  </Button>
                </div>
              ) : (
                <Loading01Icon
                  aria-label="Loading gap"
                  className="h-5 w-5 animate-spin text-muted-foreground"
                />
              )}
            </div>
          )}
        </SheetContent>
      </Sheet>
      <UpgradeRequiredDialog
        open={upgradeReason !== null}
        onOpenChange={(open) => {
          if (!open) setUpgradeReason(null)
        }}
        reason={upgradeReason}
      />
    </QuietPageViewport>
  )
}
