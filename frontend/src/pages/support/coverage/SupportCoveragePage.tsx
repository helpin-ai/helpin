import { useEffect, useState, useCallback } from 'react'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { GapDetailPane } from '@/components/support/coverage/GapDetailPane'
import { GapList } from '@/components/support/coverage/GapList'
import { useDocsCollections, useDocsSpaces } from '@/hooks/queries/useDocs'
import { usePermissions, useWorkspaceAccess } from '@/hooks/queries/useSession'
import { Loading01Icon } from '@/lib/icons'
import { supportCoverageService } from '@/lib/services/supportCoverageService'
import type {
  SupportCoverageGapDetail,
  SupportCoverageGapListItem,
  SupportCoverageSummary,
} from '@/lib/supportCoverageTypes'
import { GAP_STATUS_LABELS } from '@/lib/supportCoverageTypes'
import { useWorkspaceStore } from '@/stores/workspaceStore'

const STATUS_FILTERS = ['open', 'done', 'rejected'] as const

export function SupportCoveragePage() {
  const workspace = useWorkspaceStore((s) => s.currentWorkspace)
  const wsId = workspace?.id ?? ''
  const wsSlug = workspace?.slug ?? ''

  const [summary, setSummary] = useState<SupportCoverageSummary | null>(null)
  const [gaps, setGaps] = useState<SupportCoverageGapListItem[]>([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(true)
  const [selectedGap, setSelectedGap] = useState<SupportCoverageGapDetail | null>(null)
  const [detailLoading, setDetailLoading] = useState(false)
  const [statusFilter, setStatusFilter] = useState<(typeof STATUS_FILTERS)[number]>('open')
  const [generating, setGenerating] = useState(false)
  const [generateError, setGenerateError] = useState<string | null>(null)
  const [targetSpaceId, setTargetSpaceId] = useState('')
  const [targetCollectionId, setTargetCollectionId] = useState('')
  const [applying, setApplying] = useState(false)
  const [confirmSuggestionId, setConfirmSuggestionId] = useState<string | null>(null)

  const { data: spaces } = useDocsSpaces(wsId)
  const { data: collections } = useDocsCollections(wsId, targetSpaceId)
  const { data: access } = useWorkspaceAccess(wsId)
  const { has } = usePermissions(access)
  const canGenerate = has('support.edit') && has('docs.edit')
  const canReanalyze = has('settings.manage')
  const [reanalyzing, setReanalyzing] = useState(false)

  const handleReanalyze = useCallback(async () => {
    if (!wsId || reanalyzing) return
    setReanalyzing(true)
    await supportCoverageService.triggerReanalysis(wsId)
    setReanalyzing(false)
  }, [wsId, reanalyzing])
  const externalSpaces = spaces?.filter((space) => space.type === 'external_capable') ?? []

  useEffect(() => {
    if (!wsId) return
    let cancelled = false

    async function loadCoverage() {
      setLoading(true)
      setSelectedGap(null)
      const [summaryRes, gapsRes] = await Promise.all([
        supportCoverageService.getSummary(wsId),
        supportCoverageService.listGaps(wsId, { status: statusFilter }),
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
  }, [wsId, statusFilter])

  const refreshGap = async (gapId: string) => {
    const { data } = await supportCoverageService.getGap(wsId, gapId)
    if (data) setSelectedGap(data)
  }

  const refreshList = async () => {
    const { data } = await supportCoverageService.listGaps(wsId, { status: statusFilter })
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

  if (loading) {
    return (
      <div className="flex items-center justify-center py-20">
        <Loading01Icon className="h-6 w-6 animate-spin text-muted-foreground" />
      </div>
    )
  }

  return (
    <div className="mx-auto max-w-7xl space-y-6 p-6">
      <header>
        <h2 className="text-xl font-semibold">Coverage Gaps</h2>
        <p className="text-sm text-muted-foreground">
          Where AI could not resolve a customer issue, and what docs work closes the gap.
        </p>
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
        <div className="flex items-center gap-1.5">
          <button
            type="button"
            className="rounded-full border border-primary/25 bg-primary/10 px-3 py-1 text-xs font-semibold text-primary"
          >
            Content gaps
          </button>
          {['Data gaps', 'Action gaps'].map((label) => (
            <button
              key={label}
              type="button"
              disabled
              className="cursor-not-allowed rounded-full border border-border/60 px-3 py-1 text-xs font-medium text-muted-foreground/45"
            >
              {label} <span className="text-muted-foreground/35">(soon)</span>
            </button>
          ))}
        </div>
        <span className="text-xs text-muted-foreground">
          {total} gap{total !== 1 ? 's' : ''}
        </span>
        <div className="ml-auto flex items-center gap-2">
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
        <div className="flex rounded-lg border border-border/60 bg-muted/30 p-0.5">
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
