import { useEffect, useState } from 'react'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Cancel01Icon,
  CheckmarkCircle02Icon,
  FileSearchIcon,
  Loading01Icon,
  MagicWand01Icon,
  ArrowUpRight01Icon,
  HelpCircleIcon,
  AlertCircleIcon,
  ArrowReloadHorizontalIcon,
} from '@/lib/icons'
import { useWorkspaceStore } from '@/stores/workspaceStore'
import { useDocsSpaces, useDocsCollections } from '@/hooks/queries/useDocs'
import { useWorkspaceAccess, usePermissions } from '@/hooks/queries/useSession'
import { supportCoverageService } from '@/lib/services/supportCoverageService'
import type {
  SupportCoverageSummary,
  SupportCoverageGapListItem,
  SupportCoverageGapDetail,
  SupportGapSuggestion,
} from '@/lib/supportCoverageTypes'
import { V1_GAP_TYPE_LABELS, GAP_STATUS_LABELS } from '@/lib/supportCoverageTypes'
import { timeAgo } from '@/lib/utils'

// Gap types use neutral styling — color is reserved for status badges
// so users can scan the list by "what needs my attention next".
const GAP_TYPE_BADGE_CLASS =
  'bg-muted/60 text-muted-foreground border border-border/40'

const STATUS_COLORS: Record<string, string> = {
  open: 'bg-amber-100 text-amber-700',
  drafted: 'bg-blue-100 text-blue-700',
  fixed: 'bg-green-100 text-green-700',
  ignored: 'bg-muted text-muted-foreground/60',
  human_only: 'bg-purple-100 text-purple-700',
}

const EVIDENCE_TYPE_LABELS: Record<string, string> = {
  ai_handoff_triggered: 'AI Handoff',
  article_feedback_submitted: 'Article Feedback',
  widget_search_performed: 'Widget Search',
  docs_issue_feedback: 'Agent Feedback',
  human_reply_after_ai: 'Human Reply',
}

function confidenceLabel(confidence: number): { text: string; className: string } {
  if (confidence >= 0.7) return { text: 'High confidence', className: 'text-green-600' }
  if (confidence >= 0.4) return { text: 'Medium confidence', className: 'text-amber-600' }
  return { text: 'Low confidence', className: 'text-muted-foreground' }
}

function formatTopic(issueKey: string): string {
  if (!issueKey) return 'Unknown'
  return issueKey
    .replace(/[_-]/g, ' ')
    .replace(/\b\w/g, (c) => c.toUpperCase())
}

type ExtractedNode = { type?: string; text?: string; content?: ExtractedNode[] }

function extractTextFromTipTap(content: unknown): string {
  if (!content || typeof content !== 'object') return ''
  const node = content as ExtractedNode
  if (node.text) return node.text
  if (Array.isArray(node.content)) {
    return node.content.map((c) => extractTextFromTipTap(c)).join('')
  }
  return ''
}

function renderSuggestionPreview(suggestion: SupportGapSuggestion): string {
  if (!suggestion.content) return suggestion.evidence_summary || ''
  try {
    const node = suggestion.content as ExtractedNode
    if (!node.content) return ''
    const blocks: string[] = []
    for (const block of node.content) {
      const text = extractTextFromTipTap(block)
      if (!text) continue
      if (block.type === 'heading') {
        blocks.push(text.toUpperCase())
      } else {
        blocks.push(text)
      }
    }
    return blocks.join('\n\n')
  } catch {
    return ''
  }
}

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
  const [statusFilter, setStatusFilter] = useState('open')

  // Generate / apply state
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
  const externalSpaces = spaces?.filter((s) => s.type === 'external_capable') ?? []

  useEffect(() => {
    if (!wsId) return
    setLoading(true)
    // Close detail panel when filter changes — the selected gap may no longer match.
    setSelectedGap(null)
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

  const refreshGap = async (gapId: string) => {
    const { data } = await supportCoverageService.getGap(wsId, gapId)
    if (data) setSelectedGap(data)
  }

  const refreshList = async () => {
    const { data } = await supportCoverageService.listGaps(
      wsId,
      statusFilter ? { status: statusFilter } : undefined,
    )
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

  const handleStatusUpdate = async (gapId: string, status: string, issueResolved?: boolean) => {
    await supportCoverageService.updateGapStatus(wsId, gapId, status, issueResolved)
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

  const handleApplySuggestion = async (suggestionId: string) => {
    setApplying(true)
    await supportCoverageService.applySuggestion(wsId, suggestionId)
    setApplying(false)
    setConfirmSuggestionId(null)
    if (selectedGap) await refreshGap(selectedGap.id)
    await refreshList()
  }

  const handleDiscardSuggestion = async (suggestionId: string) => {
    await supportCoverageService.discardSuggestion(wsId, suggestionId)
    if (selectedGap) await refreshGap(selectedGap.id)
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center py-20">
        <Loading01Icon className="h-6 w-6 animate-spin text-muted-foreground" />
      </div>
    )
  }

  const draftSuggestion = selectedGap?.suggestions.find((s) => s.status === 'draft') ?? null
  const appliedSuggestion = selectedGap?.suggestions.find((s) => s.status === 'applied') ?? null
  const showLinkedArticle =
    selectedGap &&
    (selectedGap.v1_gap_type === 'weak_article' ||
      selectedGap.v1_gap_type === 'outdated_or_conflicting_article') &&
    selectedGap.related_articles.length > 0
  const canSuggestImprovements =
    selectedGap &&
    (selectedGap.v1_gap_type === 'weak_article' ||
      selectedGap.v1_gap_type === 'outdated_or_conflicting_article') &&
    selectedGap.related_articles.length > 0
  const canDraftNewArticle = selectedGap && selectedGap.v1_gap_type === 'missing_article'

  return (
    <div className="mx-auto max-w-7xl space-y-6 p-6">
      <header>
        <h2 className="text-xl font-semibold">Coverage Gaps</h2>
        <p className="text-sm text-muted-foreground">
          Where AI couldn't resolve a customer issue — and how to fix it.
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
        <span className="text-xs text-muted-foreground">
          {total} gap{total !== 1 ? 's' : ''}
        </span>
        <div className="ml-auto flex gap-1">
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

      {/* Gap inbox + detail */}
      <div className="flex gap-4">
        <div className={`${selectedGap ? 'w-1/2' : 'w-full'} space-y-1`}>
          {gaps.length === 0 ? (
            <div className="flex flex-col items-center py-12 text-muted-foreground">
              <FileSearchIcon className="mb-2 h-10 w-10 text-muted-foreground/30" />
              <p className="text-sm">No coverage gaps found.</p>
              <p className="mt-1 text-xs text-muted-foreground/60">
                Gaps appear when AI support encounters issues it cannot resolve.
              </p>
            </div>
          ) : (
            <div className="divide-y divide-border/40 rounded-lg border border-border/60 bg-card">
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
                    <p className="truncate font-medium">{gap.title}</p>
                    <p className="mt-0.5 text-xs text-muted-foreground">
                      {gap.evidence_count} conversation{gap.evidence_count !== 1 ? 's' : ''}
                      {gap.topic_title && <> &middot; {gap.topic_title}</>}
                    </p>
                  </div>
                  <Badge
                    variant="secondary"
                    className={`shrink-0 text-xs ${GAP_TYPE_BADGE_CLASS}`}
                  >
                    {V1_GAP_TYPE_LABELS[gap.v1_gap_type] ?? gap.v1_gap_type}
                  </Badge>
                  <Badge
                    variant="secondary"
                    className={`shrink-0 text-xs ${STATUS_COLORS[gap.status] ?? ''}`}
                  >
                    {GAP_STATUS_LABELS[gap.status] ?? gap.status}
                  </Badge>
                  <span className="shrink-0 text-xs text-muted-foreground">
                    {timeAgo(gap.last_seen_at)}
                  </span>
                </button>
              ))}
            </div>
          )}
        </div>

        {/* Detail panel */}
        {selectedGap && (
          <div className="w-1/2 overflow-hidden rounded-lg border border-border/60 bg-card">
            {detailLoading ? (
              <div className="flex justify-center py-12">
                <Loading01Icon className="h-5 w-5 animate-spin" />
              </div>
            ) : (
              <div className="divide-y divide-border/40">
                {/* Header */}
                <div className="p-4">
                  <div className="flex items-start justify-between gap-3">
                    <div className="min-w-0 flex-1">
                      <h3 className="font-semibold leading-tight">{selectedGap.title}</h3>
                      <div className="mt-1.5 flex flex-wrap items-center gap-2">
                        <Badge
                          variant="secondary"
                          className={`text-xs ${GAP_TYPE_BADGE_CLASS}`}
                        >
                          {V1_GAP_TYPE_LABELS[selectedGap.v1_gap_type] ?? selectedGap.v1_gap_type}
                        </Badge>
                        <span className={`text-xs ${confidenceLabel(selectedGap.confidence).className}`}>
                          {confidenceLabel(selectedGap.confidence).text}
                        </span>
                      </div>
                    </div>
                    <button
                      type="button"
                      onClick={() => setSelectedGap(null)}
                      className="rounded-md p-1 text-muted-foreground transition-colors hover:bg-muted/50 hover:text-foreground"
                    >
                      <Cancel01Icon className="h-4 w-4" />
                    </button>
                  </div>

                  {/* Quick metadata */}
                  <dl className="mt-3 grid grid-cols-3 gap-3 text-xs">
                    <div>
                      <dt className="text-muted-foreground">Topic</dt>
                      <dd className="mt-0.5 font-medium">{formatTopic(selectedGap.issue_key)}</dd>
                    </div>
                    <div>
                      <dt className="text-muted-foreground">First seen</dt>
                      <dd className="mt-0.5 font-medium">{timeAgo(selectedGap.first_seen_at)}</dd>
                    </div>
                    <div>
                      <dt className="text-muted-foreground">Evidence</dt>
                      <dd className="mt-0.5 font-medium">
                        {selectedGap.evidence_count} item{selectedGap.evidence_count !== 1 ? 's' : ''}
                      </dd>
                    </div>
                  </dl>

                  {/* Status change info */}
                  {selectedGap.status !== 'open' && selectedGap.status_changed_at && (
                    <p className="mt-3 text-xs text-muted-foreground">
                      Marked as{' '}
                      <span className="font-medium">
                        {GAP_STATUS_LABELS[selectedGap.status] ?? selectedGap.status}
                      </span>
                      {selectedGap.status_changed_by_name && (
                        <>
                          {' '}
                          by <span className="font-medium">{selectedGap.status_changed_by_name}</span>
                        </>
                      )}{' '}
                      {timeAgo(selectedGap.status_changed_at)}
                      {selectedGap.status === 'human_only' && selectedGap.issue_resolved != null && (
                        <span
                          className={
                            selectedGap.issue_resolved ? 'text-green-600' : 'text-amber-600'
                          }
                        >
                          {' '}
                          &middot; Customer issue{' '}
                          {selectedGap.issue_resolved ? 'resolved' : 'unresolved'}
                        </span>
                      )}
                    </p>
                  )}
                </div>

                {/* Linked Article */}
                {showLinkedArticle && (
                  <div className="flex items-center justify-between gap-3 px-4 py-3">
                    <div className="flex min-w-0 items-center gap-2 text-sm">
                      <FileSearchIcon className="h-4 w-4 shrink-0 text-muted-foreground" />
                      <span className="truncate font-medium">
                        {selectedGap.related_articles[0].article_title || 'Untitled article'}
                      </span>
                    </div>
                    <a
                      href={`/w/${wsSlug}/docs/${selectedGap.related_articles[0].document_id}`}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="inline-flex shrink-0 items-center gap-1 text-xs font-medium text-primary hover:underline"
                    >
                      Open in Editor
                      <ArrowUpRight01Icon className="h-3 w-3" />
                    </a>
                  </div>
                )}

                {/* Evidence */}
                {selectedGap.evidence.length > 0 && (
                  <div className="p-4">
                    <h4 className="mb-2 text-xs font-semibold uppercase tracking-wide text-muted-foreground">
                      Evidence
                    </h4>
                    <div className="space-y-2">
                      {selectedGap.evidence.map((ev) => (
                        <div
                          key={ev.id}
                          className="rounded-md border border-border/40 bg-muted/30 p-2.5 text-xs"
                        >
                          <div className="flex items-center justify-between gap-2">
                            <p className="font-medium text-muted-foreground">
                              {EVIDENCE_TYPE_LABELS[ev.evidence_type] ?? ev.evidence_type}
                              <span className="ml-2 font-normal">{timeAgo(ev.created_at)}</span>
                            </p>
                            <div className="flex gap-2">
                              {ev.conversation_id && (
                                <a
                                  href={`/w/${wsSlug}/support/${ev.conversation_id}`}
                                  target="_blank"
                                  rel="noopener noreferrer"
                                  className="inline-flex items-center gap-0.5 text-primary hover:underline"
                                >
                                  Conversation
                                  <ArrowUpRight01Icon className="h-2.5 w-2.5" />
                                </a>
                              )}
                              {ev.document_id && (
                                <a
                                  href={`/w/${wsSlug}/docs/${ev.document_id}`}
                                  target="_blank"
                                  rel="noopener noreferrer"
                                  className="inline-flex items-center gap-0.5 text-primary hover:underline"
                                >
                                  Article
                                  <ArrowUpRight01Icon className="h-2.5 w-2.5" />
                                </a>
                              )}
                            </div>
                          </div>
                          {ev.excerpt && <p className="mt-1.5 leading-relaxed">{ev.excerpt}</p>}
                        </div>
                      ))}
                    </div>
                  </div>
                )}

                {/* Applied suggestion success state */}
                {appliedSuggestion && (
                  <div className="m-4 rounded-md border border-green-200 bg-green-50 p-3">
                    <div className="flex items-center gap-2 text-sm text-green-800">
                      <CheckmarkCircle02Icon className="h-4 w-4 shrink-0" />
                      <span className="min-w-0 flex-1 truncate font-medium">
                        {appliedSuggestion.suggestion_type === 'update_article'
                          ? `Sections added to "${
                              selectedGap.related_articles[0]?.article_title ?? 'article'
                            }"`
                          : `Draft created: "${appliedSuggestion.title}"`}
                      </span>
                      {appliedSuggestion.result_document_id && (
                        <a
                          href={`/w/${wsSlug}/docs/${appliedSuggestion.result_document_id}`}
                          target="_blank"
                          rel="noopener noreferrer"
                          className="inline-flex shrink-0 items-center gap-1 text-xs font-medium hover:underline"
                        >
                          Open in Editor
                          <ArrowUpRight01Icon className="h-3 w-3" />
                        </a>
                      )}
                    </div>
                  </div>
                )}

                {/* Draft suggestion preview */}
                {draftSuggestion && (
                  <div className="m-4 rounded-md border border-primary/30 bg-primary/5 p-3">
                    <div className="mb-2 flex items-center gap-2">
                      <MagicWand01Icon className="h-4 w-4 text-primary" />
                      <h4 className="text-xs font-semibold uppercase tracking-wide text-primary">
                        {draftSuggestion.suggestion_type === 'update_article'
                          ? 'Suggested additions'
                          : `Draft: ${draftSuggestion.title}`}
                      </h4>
                    </div>
                    <div className="rounded bg-card p-3 text-xs leading-relaxed whitespace-pre-wrap text-foreground/90">
                      {renderSuggestionPreview(draftSuggestion) ||
                        draftSuggestion.evidence_summary ||
                        'No preview available.'}
                    </div>

                    {confirmSuggestionId === draftSuggestion.id ? (
                      <div className="mt-3 rounded-md border border-border/60 bg-card p-3 text-xs">
                        <div className="mb-2 flex items-start gap-2">
                          <AlertCircleIcon className="mt-0.5 h-4 w-4 shrink-0 text-amber-600" />
                          <p className="leading-relaxed">
                            {draftSuggestion.suggestion_type === 'update_article' ? (
                              <>
                                This will add new sections to{' '}
                                <span className="font-medium">
                                  "{selectedGap.related_articles[0]?.article_title ?? 'the article'}"
                                </span>
                                . The article will remain unpublished.
                              </>
                            ) : (
                              <>
                                This will create a new draft article{' '}
                                <span className="font-medium">"{draftSuggestion.title}"</span>.
                              </>
                            )}
                          </p>
                        </div>
                        <div className="flex justify-end gap-2">
                          <button
                            type="button"
                            onClick={() => setConfirmSuggestionId(null)}
                            disabled={applying}
                            className="rounded-md border border-border/60 px-2.5 py-1.5 text-xs font-medium text-muted-foreground hover:bg-muted disabled:opacity-50"
                          >
                            Cancel
                          </button>
                          <button
                            type="button"
                            onClick={() => handleApplySuggestion(draftSuggestion.id)}
                            disabled={applying}
                            className="inline-flex items-center gap-1 rounded-md bg-primary px-2.5 py-1.5 text-xs font-medium text-primary-foreground hover:bg-primary/90 disabled:opacity-50"
                          >
                            {applying && <Loading01Icon className="h-3 w-3 animate-spin" />}
                            {applying
                              ? 'Applying...'
                              : draftSuggestion.suggestion_type === 'update_article'
                              ? 'Apply Changes'
                              : 'Create Draft'}
                          </button>
                        </div>
                      </div>
                    ) : (
                      <div className="mt-3 flex justify-end gap-2">
                        <button
                          type="button"
                          onClick={() => handleDiscardSuggestion(draftSuggestion.id)}
                          className="rounded-md border border-border/60 px-2.5 py-1.5 text-xs font-medium text-muted-foreground hover:bg-muted"
                        >
                          Discard
                        </button>
                        <button
                          type="button"
                          onClick={() => setConfirmSuggestionId(draftSuggestion.id)}
                          className="rounded-md bg-primary px-2.5 py-1.5 text-xs font-medium text-primary-foreground hover:bg-primary/90"
                        >
                          {draftSuggestion.suggestion_type === 'update_article'
                            ? 'Apply to Article'
                            : 'Create as Draft in Docs'}
                        </button>
                      </div>
                    )}
                  </div>
                )}

                {/* Generate actions (when no suggestion yet) */}
                {!draftSuggestion && !appliedSuggestion && selectedGap.status === 'open' && canGenerate && (
                  <div className="p-4">
                    {generating ? (
                      <div className="flex items-center gap-2 rounded-md border border-border/40 bg-muted/30 px-3 py-3 text-xs text-muted-foreground">
                        <Loading01Icon className="h-4 w-4 animate-spin" />
                        Generating with AI...
                      </div>
                    ) : generateError ? (
                      <div className="rounded-md border border-red-200 bg-red-50 p-3">
                        <div className="mb-2 flex items-start gap-2">
                          <AlertCircleIcon className="mt-0.5 h-4 w-4 shrink-0 text-red-600" />
                          <p className="text-xs text-red-800 leading-relaxed">
                            Could not generate suggestions. This may be a temporary service issue.
                          </p>
                        </div>
                        <button
                          type="button"
                          onClick={() => {
                            setGenerateError(null)
                            if (canSuggestImprovements) handleSuggestImprovements()
                            else if (canDraftNewArticle && targetSpaceId) handleDraftNewArticle()
                          }}
                          className="inline-flex items-center gap-1 text-xs font-medium text-red-700 hover:underline"
                        >
                          <ArrowReloadHorizontalIcon className="h-3 w-3" />
                          Try Again
                        </button>
                      </div>
                    ) : canSuggestImprovements ? (
                      <button
                        type="button"
                        onClick={handleSuggestImprovements}
                        className="inline-flex w-full items-center justify-center gap-2 rounded-md border border-primary/30 bg-primary/5 px-3 py-2.5 text-sm font-medium text-primary transition-colors hover:bg-primary/10"
                      >
                        <MagicWand01Icon className="h-4 w-4" />
                        Suggest Improvements
                      </button>
                    ) : canDraftNewArticle ? (
                      <div className="space-y-2 rounded-md border border-border/40 bg-muted/20 p-3">
                        <p className="text-xs font-medium text-muted-foreground">
                          Draft a new article from this gap
                        </p>
                        {externalSpaces.length === 0 ? (
                          <p className="text-xs text-muted-foreground">
                            No external docs space available. Create one in Docs settings first.
                          </p>
                        ) : (
                          <>
                            <div className="flex gap-2">
                              <Select
                                value={targetSpaceId}
                                onValueChange={(v) => {
                                  setTargetSpaceId(v)
                                  setTargetCollectionId('')
                                }}
                              >
                                <SelectTrigger size="sm" className="flex-1">
                                  <SelectValue placeholder="Select space" />
                                </SelectTrigger>
                                <SelectContent>
                                  {externalSpaces.map((s) => (
                                    <SelectItem key={s.id} value={s.id}>
                                      {s.name}
                                    </SelectItem>
                                  ))}
                                </SelectContent>
                              </Select>
                              <Select
                                value={targetCollectionId}
                                onValueChange={setTargetCollectionId}
                                disabled={!targetSpaceId || (collections?.length ?? 0) === 0}
                              >
                                <SelectTrigger size="sm" className="flex-1">
                                  <SelectValue placeholder="Collection (optional)" />
                                </SelectTrigger>
                                <SelectContent>
                                  {collections?.map((c) => (
                                    <SelectItem key={c.id} value={c.id}>
                                      {c.name}
                                    </SelectItem>
                                  ))}
                                </SelectContent>
                              </Select>
                            </div>
                            <button
                              type="button"
                              onClick={handleDraftNewArticle}
                              disabled={!targetSpaceId}
                              className="inline-flex w-full items-center justify-center gap-2 rounded-md bg-primary px-3 py-2 text-xs font-medium text-primary-foreground transition-colors hover:bg-primary/90 disabled:opacity-50"
                            >
                              <MagicWand01Icon className="h-3.5 w-3.5" />
                              Draft New Article
                            </button>
                          </>
                        )}
                      </div>
                    ) : selectedGap.v1_gap_type === 'needs_review' ? (
                      <div className="flex items-start gap-2 rounded-md border border-border/40 bg-muted/30 px-3 py-3 text-xs text-muted-foreground">
                        <HelpCircleIcon className="mt-0.5 h-4 w-4 shrink-0" />
                        <p className="leading-relaxed">
                          Triage this gap to unlock AI suggestions. Reclassify it as a missing or
                          weak article if you know the resolution path.
                        </p>
                      </div>
                    ) : null}
                  </div>
                )}

                {/* Status actions bar */}
                <div className="flex flex-wrap items-center gap-2 bg-muted/20 p-4">
                  <Badge
                    variant="secondary"
                    className={`text-xs ${STATUS_COLORS[selectedGap.status] ?? ''}`}
                  >
                    {GAP_STATUS_LABELS[selectedGap.status] ?? selectedGap.status}
                  </Badge>
                  <div className="flex-1" />
                  {selectedGap.status === 'open' && (
                    <>
                      <button
                        type="button"
                        onClick={() => handleStatusUpdate(selectedGap.id, 'ignored')}
                        className="rounded-md border border-border/60 px-2.5 py-1.5 text-xs font-medium text-muted-foreground hover:bg-muted"
                      >
                        Ignore
                      </button>
                      <TooltipProvider>
                        <Tooltip>
                          <TooltipTrigger asChild>
                            <button
                              type="button"
                              onClick={() => handleStatusUpdate(selectedGap.id, 'human_only', false)}
                              className="rounded-md border border-border/60 px-2.5 py-1.5 text-xs font-medium text-purple-600 hover:bg-purple-50"
                            >
                              Requires Human
                            </button>
                          </TooltipTrigger>
                          <TooltipContent side="bottom">
                            <p>This topic can't be resolved by AI and will always need a human agent.</p>
                          </TooltipContent>
                        </Tooltip>
                      </TooltipProvider>
                      <button
                        type="button"
                        onClick={() => handleStatusUpdate(selectedGap.id, 'fixed')}
                        className="inline-flex items-center gap-1 rounded-md bg-green-600 px-2.5 py-1.5 text-xs font-medium text-white hover:bg-green-700"
                      >
                        <CheckmarkCircle02Icon className="h-3 w-3" />
                        Mark Fixed
                      </button>
                    </>
                  )}
                  {(selectedGap.status === 'ignored' ||
                    selectedGap.status === 'human_only' ||
                    selectedGap.status === 'fixed' ||
                    selectedGap.status === 'drafted') && (
                    <button
                      type="button"
                      onClick={() => handleStatusUpdate(selectedGap.id, 'open')}
                      className="rounded-md border border-border/60 px-2.5 py-1.5 text-xs font-medium text-muted-foreground hover:bg-muted"
                    >
                      Reopen
                    </button>
                  )}
                </div>
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  )
}
