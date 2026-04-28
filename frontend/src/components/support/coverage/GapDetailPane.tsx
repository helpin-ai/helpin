import { useEffect, useState } from 'react'
import { Badge } from '@/components/ui/badge'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import type { DocsCollection, DocsSpace } from '@/lib/docsTypes'
import {
  AlertCircleIcon,
  ArrowReloadHorizontalIcon,
  ArrowUpRight01Icon,
  Cancel01Icon,
  CheckmarkCircle02Icon,
  FileSearchIcon,
  HelpCircleIcon,
  Loading01Icon,
  MagicWand01Icon,
} from '@/lib/icons'
import type { SupportCoverageGapDetail } from '@/lib/supportCoverageTypes'
import { GAP_STATUS_LABELS, V1_GAP_TYPE_LABELS } from '@/lib/supportCoverageTypes'
import { timeAgo } from '@/lib/utils'
import { GapAddSplitButton, type GapAddRoute } from './GapAddSplitButton'
import {
  coverageConfidenceLabel,
  coverageSuggestionPreview,
  coverageTopicLabel,
} from './coverageUi'

const GAP_TYPE_BADGE_CLASS = 'bg-muted/60 text-muted-foreground border border-border/40'

const STATUS_COLORS: Record<string, string> = {
  open: 'bg-amber-100 text-amber-700',
  done: 'bg-green-100 text-green-700',
  rejected: 'bg-muted text-muted-foreground/60',
}

const EVIDENCE_TYPE_LABELS: Record<string, string> = {
  ai_handoff_triggered: 'AI Handoff',
  article_feedback_submitted: 'Article Feedback',
  widget_search_performed: 'Widget Search',
  docs_issue_feedback: 'Agent Feedback',
  human_reply_after_ai: 'Human Reply',
}

export function GapDetailPane({
  gap,
  wsSlug,
  loading,
  canGenerate,
  externalSpaces,
  collections,
  targetSpaceId,
  targetCollectionId,
  generating,
  generateError,
  applying,
  confirmSuggestionId,
  onClose,
  onTargetSpaceChange,
  onTargetCollectionChange,
  onSuggestImprovements,
  onDraftNewArticle,
  onApplySuggestion,
  onDiscardSuggestion,
  onSetConfirmSuggestion,
  onStatusUpdate,
  onRegenerate,
}: {
  gap: SupportCoverageGapDetail
  wsSlug: string
  loading: boolean
  canGenerate: boolean
  externalSpaces: DocsSpace[]
  collections: DocsCollection[] | undefined
  targetSpaceId: string
  targetCollectionId: string
  generating: boolean
  generateError: string | null
  applying: boolean
  confirmSuggestionId: string | null
  onClose: () => void
  onTargetSpaceChange: (spaceId: string) => void
  onTargetCollectionChange: (collectionId: string) => void
  onSuggestImprovements: () => void
  onDraftNewArticle: () => void
  onApplySuggestion: (suggestionId: string, override?: { route?: GapAddRoute; target_document_id?: string }) => void
  onDiscardSuggestion: (suggestionId: string) => void
  onSetConfirmSuggestion: (suggestionId: string | null) => void
  onStatusUpdate: (gapId: string, status: string) => void
  onRegenerate: (gapId: string) => void
}) {
  const [regenerateLockedUntil, setRegenerateLockedUntil] = useState<number | null>(null)
  const [now, setNow] = useState(Date.now())

  useEffect(() => {
    if (!regenerateLockedUntil) return
    const timer = window.setInterval(() => setNow(Date.now()), 1000)
    return () => window.clearInterval(timer)
  }, [regenerateLockedUntil])

  const draftSuggestion = gap.suggestions.find((s) => s.status === 'draft') ?? null
  const appliedSuggestion = gap.suggestions.find((s) => s.status === 'applied') ?? null
  const confidence = coverageConfidenceLabel(gap.confidence)
  const showLinkedArticle =
    (gap.v1_gap_type === 'weak_article' || gap.v1_gap_type === 'outdated_or_conflicting_article') &&
    gap.related_articles.length > 0
  const canSuggestImprovements =
    (gap.v1_gap_type === 'weak_article' || gap.v1_gap_type === 'outdated_or_conflicting_article') &&
    gap.related_articles.length > 0
  const canDraftNewArticle = gap.v1_gap_type === 'missing_article'
  const regenerateDisabled = Boolean(regenerateLockedUntil && regenerateLockedUntil > now)
  const regenerateSeconds = regenerateLockedUntil
    ? Math.max(0, Math.ceil((regenerateLockedUntil - now) / 1000))
    : 0

  const handleRegenerate = () => {
    setRegenerateLockedUntil(Date.now() + 30_000)
    onRegenerate(gap.id)
  }

  const handleOverride = (route: GapAddRoute, targetDocumentId?: string) => {
    if (!draftSuggestion) return
    onApplySuggestion(draftSuggestion.id, { route, target_document_id: targetDocumentId })
  }

  if (loading) {
    return (
      <div className="flex justify-center py-12">
        <Loading01Icon className="h-5 w-5 animate-spin" />
      </div>
    )
  }

  return (
    <div className="divide-y divide-border/40">
      <div className="p-4">
        <div className="flex items-start justify-between gap-3">
          <div className="min-w-0 flex-1">
            <h3 className="font-semibold leading-tight">{gap.title}</h3>
            <div className="mt-1.5 flex flex-wrap items-center gap-2">
              <Badge variant="secondary" className={`text-xs ${GAP_TYPE_BADGE_CLASS}`}>
                {V1_GAP_TYPE_LABELS[gap.v1_gap_type] ?? gap.v1_gap_type}
              </Badge>
              <span className={`text-xs ${confidence.className}`}>{confidence.text}</span>
            </div>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="rounded-md p-1 text-muted-foreground transition-colors hover:bg-muted/50 hover:text-foreground"
          >
            <Cancel01Icon className="h-4 w-4" />
          </button>
        </div>

        <dl className="mt-3 grid grid-cols-3 gap-3 text-xs">
          <div>
            <dt className="text-muted-foreground">Topic</dt>
            <dd className="mt-0.5 font-medium">{coverageTopicLabel(gap.issue_key)}</dd>
          </div>
          <div>
            <dt className="text-muted-foreground">First seen</dt>
            <dd className="mt-0.5 font-medium">{timeAgo(gap.first_seen_at)}</dd>
          </div>
          <div>
            <dt className="text-muted-foreground">Evidence</dt>
            <dd className="mt-0.5 font-medium">
              {gap.evidence_count} item{gap.evidence_count !== 1 ? 's' : ''}
            </dd>
          </div>
        </dl>

        {gap.status !== 'open' && gap.status_changed_at && (
          <p className="mt-3 text-xs text-muted-foreground">
            Marked as <span className="font-medium">{GAP_STATUS_LABELS[gap.status] ?? gap.status}</span>
            {gap.status_changed_by_name && (
              <>
                {' '}by <span className="font-medium">{gap.status_changed_by_name}</span>
              </>
            )}{' '}
            {timeAgo(gap.status_changed_at)}
          </p>
        )}
      </div>

      {showLinkedArticle && (
        <div className="flex items-center justify-between gap-3 px-4 py-3">
          <div className="flex min-w-0 items-center gap-2 text-sm">
            <FileSearchIcon className="h-4 w-4 shrink-0 text-muted-foreground" />
            <span className="truncate font-medium">
              {gap.related_articles[0].article_title || 'Untitled article'}
            </span>
          </div>
          <a
            href={`/w/${wsSlug}/docs/${gap.related_articles[0].document_id}`}
            target="_blank"
            rel="noopener noreferrer"
            className="inline-flex shrink-0 items-center gap-1 text-xs font-medium text-primary hover:underline"
          >
            Open in Editor
            <ArrowUpRight01Icon className="h-3 w-3" />
          </a>
        </div>
      )}

      {gap.evidence.length > 0 && (
        <div className="p-4">
          <h4 className="mb-2 text-xs font-semibold uppercase tracking-wide text-muted-foreground">
            Evidence
          </h4>
          <div className="max-h-56 space-y-2 overflow-y-auto">
            {gap.evidence.map((ev) => (
              <div key={ev.id} className="rounded-md border border-border/40 bg-muted/30 p-2.5 text-xs">
                <div className="flex items-center justify-between gap-2">
                  <p className="font-medium text-muted-foreground">
                    {EVIDENCE_TYPE_LABELS[ev.evidence_type] ?? ev.evidence_type}
                    <span className="ml-2 font-normal">{timeAgo(ev.created_at)}</span>
                  </p>
                  <div className="flex gap-2">
                    {ev.conversation_id && (
                      <a
                        href={`/w/${wsSlug}/support/inbox?conversation=${ev.conversation_id}`}
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

      {appliedSuggestion && (
        <div className="m-4 rounded-md border border-green-200 bg-green-50 p-3">
          <div className="flex items-center gap-2 text-sm text-green-800">
            <CheckmarkCircle02Icon className="h-4 w-4 shrink-0" />
            <span className="min-w-0 flex-1 truncate font-medium">
              {appliedSuggestion.suggestion_type === 'update_article'
                ? `Sections added to "${gap.related_articles[0]?.article_title ?? 'article'}"`
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
          <div className="max-h-56 overflow-y-auto whitespace-pre-wrap rounded bg-card p-3 text-xs leading-relaxed text-foreground/90">
            {coverageSuggestionPreview(draftSuggestion) ||
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
                        "{gap.related_articles[0]?.article_title ?? 'the article'}"
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
                  onClick={() => onSetConfirmSuggestion(null)}
                  disabled={applying}
                  className="rounded-md border border-border/60 px-2.5 py-1.5 text-xs font-medium text-muted-foreground hover:bg-muted disabled:opacity-50"
                >
                  Cancel
                </button>
                <GapAddSplitButton
                  gap={gap}
                  suggestion={draftSuggestion}
                  disabled={applying}
                  onPrimary={() => onApplySuggestion(draftSuggestion.id)}
                  onOverride={handleOverride}
                />
              </div>
            </div>
          ) : (
            <div className="mt-3 flex justify-end gap-2">
              <button
                type="button"
                onClick={() => onDiscardSuggestion(draftSuggestion.id)}
                className="rounded-md border border-border/60 px-2.5 py-1.5 text-xs font-medium text-muted-foreground hover:bg-muted"
              >
                Discard
              </button>
              <button
                type="button"
                onClick={() => onSetConfirmSuggestion(draftSuggestion.id)}
                className="rounded-md bg-primary px-2.5 py-1.5 text-xs font-medium text-primary-foreground hover:bg-primary/90"
              >
                Review Add
              </button>
            </div>
          )}
        </div>
      )}

      {!draftSuggestion && !appliedSuggestion && gap.status === 'open' && canGenerate && (
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
                <p className="text-xs leading-relaxed text-red-800">
                  Could not generate suggestions. This may be a temporary service issue.
                </p>
              </div>
              <button
                type="button"
                onClick={canSuggestImprovements ? onSuggestImprovements : onDraftNewArticle}
                className="inline-flex items-center gap-1 text-xs font-medium text-red-700 hover:underline"
              >
                <ArrowReloadHorizontalIcon className="h-3 w-3" />
                Try Again
              </button>
            </div>
          ) : canSuggestImprovements ? (
            <button
              type="button"
              onClick={onSuggestImprovements}
              className="inline-flex w-full items-center justify-center gap-2 rounded-md border border-primary/30 bg-primary/5 px-3 py-2.5 text-sm font-medium text-primary transition-colors hover:bg-primary/10"
            >
              <MagicWand01Icon className="h-4 w-4" />
              Suggest Improvements
            </button>
          ) : canDraftNewArticle ? (
            <div className="space-y-2 rounded-md border border-border/40 bg-muted/20 p-3">
              <p className="text-xs font-medium text-muted-foreground">Draft a new article from this gap</p>
              {externalSpaces.length === 0 ? (
                <p className="text-xs text-muted-foreground">
                  No external docs space available. Create one in Docs settings first.
                </p>
              ) : (
                <>
                  <div className="flex gap-2">
                    <Select
                      value={targetSpaceId}
                      onValueChange={(spaceId) => {
                        onTargetSpaceChange(spaceId)
                        onTargetCollectionChange('')
                      }}
                    >
                      <SelectTrigger size="sm" className="flex-1">
                        <SelectValue placeholder="Select space" />
                      </SelectTrigger>
                      <SelectContent>
                        {externalSpaces.map((space) => (
                          <SelectItem key={space.id} value={space.id}>
                            {space.name}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                    <Select
                      value={targetCollectionId}
                      onValueChange={onTargetCollectionChange}
                      disabled={!targetSpaceId || (collections?.length ?? 0) === 0}
                    >
                      <SelectTrigger size="sm" className="flex-1">
                        <SelectValue placeholder="Collection (optional)" />
                      </SelectTrigger>
                      <SelectContent>
                        {collections?.map((collection) => (
                          <SelectItem key={collection.id} value={collection.id}>
                            {collection.name}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </div>
                  <button
                    type="button"
                    onClick={onDraftNewArticle}
                    disabled={!targetSpaceId}
                    className="inline-flex w-full items-center justify-center gap-2 rounded-md bg-primary px-3 py-2 text-xs font-medium text-primary-foreground transition-colors hover:bg-primary/90 disabled:opacity-50"
                  >
                    <MagicWand01Icon className="h-3.5 w-3.5" />
                    Draft New Article
                  </button>
                </>
              )}
            </div>
          ) : gap.v1_gap_type === 'needs_review' ? (
            <div className="flex items-start gap-2 rounded-md border border-border/40 bg-muted/30 px-3 py-3 text-xs text-muted-foreground">
              <HelpCircleIcon className="mt-0.5 h-4 w-4 shrink-0" />
              <p className="leading-relaxed">
                Triage this gap to unlock AI suggestions. Reclassify it as a missing or weak article if
                you know the resolution path.
              </p>
            </div>
          ) : null}
        </div>
      )}

      <div className="flex flex-wrap items-center gap-2 bg-muted/20 p-4">
        <Badge variant="secondary" className={`text-xs ${STATUS_COLORS[gap.status] ?? ''}`}>
          {GAP_STATUS_LABELS[gap.status] ?? gap.status}
        </Badge>
        <button
          type="button"
          onClick={handleRegenerate}
          disabled={regenerateDisabled || gap.status !== 'open'}
          className="inline-flex items-center gap-1 rounded-md border border-border/60 px-2.5 py-1.5 text-xs font-medium text-muted-foreground hover:bg-muted disabled:opacity-50"
        >
          <ArrowReloadHorizontalIcon className="h-3 w-3" />
          {regenerateDisabled ? `Regenerate in ${regenerateSeconds}s` : 'Regenerate'}
        </button>
        <div className="flex-1" />
        {gap.status === 'open' && (
          <>
            <button
              type="button"
              onClick={() => onStatusUpdate(gap.id, 'rejected')}
              className="rounded-md border border-border/60 px-2.5 py-1.5 text-xs font-medium text-muted-foreground hover:bg-muted"
            >
              Reject
            </button>
            <button
              type="button"
              onClick={() => onStatusUpdate(gap.id, 'done')}
              className="inline-flex items-center gap-1 rounded-md bg-green-600 px-2.5 py-1.5 text-xs font-medium text-white hover:bg-green-700"
            >
              <CheckmarkCircle02Icon className="h-3 w-3" />
              Mark Done
            </button>
          </>
        )}
        {(gap.status === 'rejected' || gap.status === 'done') && (
          <button
            type="button"
            onClick={() => onStatusUpdate(gap.id, 'open')}
            className="rounded-md border border-border/60 px-2.5 py-1.5 text-xs font-medium text-muted-foreground hover:bg-muted"
          >
            Reopen
          </button>
        )}
      </div>
    </div>
  )
}
