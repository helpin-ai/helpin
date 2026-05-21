import { useEffect, useState } from 'react'
import Markdown from 'react-markdown'
import { Badge } from '@/components/ui/badge'
import { AgentAvatar } from '@/components/agents/AgentAvatar'
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
  Loading01Icon,
  MagicWand01Icon,
} from '@/lib/icons'
import type { SupportCoverageGapDetail } from '@/lib/supportCoverageTypes'
import { GAP_STATUS_LABELS, V1_GAP_TYPE_LABELS } from '@/lib/supportCoverageTypes'
import { timeAgo } from '@/lib/utils'
import { EvidenceConversationCard } from './EvidenceConversationCard'
import { groupEvidenceByConversation } from './evidenceGrouping'
import { GapAddSplitButton, type GapAddRoute } from './GapAddSplitButton'
import { storeCoverageHandoffContent } from './coverageHandoff'
import { buildCoverageCollectionOptions } from './coverageCollectionOptions'
import {
  EVIDENCE_TYPE_LABELS,
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

const RECOMMENDATION_TYPE_LABELS: Record<string, string> = {
  create_article: 'Create article',
  update_article: 'Update article',
  update_website_page: 'Update website page',
  create_website_page: 'Create website page',
  add_data: 'Add data',
  add_action: 'Add action',
  define_policy: 'Define policy',
  improve_workflow: 'Improve workflow',
  no_fix: 'No fix',
}

function recommendationLabel(type: string): string {
  return RECOMMENDATION_TYPE_LABELS[type] ?? coverageTopicLabel(type)
}

function documentationAgentAction(gap: SupportCoverageGapDetail): { label: string; cta: string; description: string } {
  const primaryRecommendation =
    gap.recommendations.find((rec) => rec.priority === 'primary') ?? gap.recommendations[0]
  switch (primaryRecommendation?.recommendation_type) {
    case 'add_data':
      return {
        label: 'Route missing data',
        cta: 'Route gap',
        description: 'Summarize the missing data, customer impact, and docs or owner handoff needed.',
      }
    case 'add_action':
      return {
        label: 'Route missing action',
        cta: 'Route gap',
        description: 'Document the unavailable action or prepare a product handoff when docs are not enough.',
      }
    case 'define_policy':
      return {
        label: 'Define policy guidance',
        cta: 'Draft policy',
        description: 'Turn repeated support ambiguity into clear internal or customer-facing policy guidance.',
      }
    case 'improve_workflow':
      return {
        label: 'Improve workflow guidance',
        cta: 'Draft workflow',
        description: 'Capture the process gap and update internal guidance or hand off the workflow fix.',
      }
    case 'no_fix':
      return {
        label: 'Review no-fix gap',
        cta: 'Review',
        description: 'Summarize why documentation may not fix this and ask for a human decision.',
      }
  }

  if (gap.gap_kind === 'data') {
    return {
      label: 'Route missing data',
      cta: 'Route gap',
      description: 'Summarize the missing data, customer impact, and docs or owner handoff needed.',
    }
  }
  if (gap.gap_kind === 'action') {
    return {
      label: 'Route missing action',
      cta: 'Route gap',
      description: 'Document the unavailable action or prepare a product handoff when docs are not enough.',
    }
  }

  switch (gap.v1_gap_type) {
    case 'missing_article':
      return {
        label: 'Write with Quill',
        cta: 'Write doc',
        description: 'Draft the right new doc and choose where it belongs.',
      }
    case 'weak_article':
      return {
        label: 'Improve with Quill',
        cta: 'Improve doc',
        description: 'Use the evidence to strengthen the linked article without creating duplicates.',
      }
    case 'outdated_or_conflicting_article':
      return {
        label: 'Fix docs with Quill',
        cta: 'Fix docs',
        description: 'Find stale or conflicting guidance and prepare a reviewed update.',
      }
    case 'needs_review':
      return {
        label: 'Investigate with Quill',
        cta: 'Investigate',
        description: 'Clarify the gap and propose the right documentation action before drafting.',
      }
    default:
      return {
        label: 'Run Quill',
        cta: 'Start agent',
        description: 'Turn this support pattern into the right documentation work.',
      }
  }
}

export function GapDetailPane({
  gap,
  wsSlug,
  loading,
  canGenerate,
  canRunDocumentationAgent,
  startingDocumentationAgent,
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
  onRunDocumentationAgent,
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
  canRunDocumentationAgent: boolean
  startingDocumentationAgent: boolean
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
  onRunDocumentationAgent: (gapId: string) => void
  onApplySuggestion: (suggestionId: string, override?: { route?: GapAddRoute; target_document_id?: string }) => void
  onDiscardSuggestion: (suggestionId: string) => void
  onSetConfirmSuggestion: (suggestionId: string | null) => void
  onStatusUpdate: (gapId: string, status: string) => void
  onRegenerate: (gapId: string) => void
}) {
  const [regenerateLockedUntil, setRegenerateLockedUntil] = useState<number | null>(null)
  const [quickDraftOpen, setQuickDraftOpen] = useState(false)
  const [now, setNow] = useState(() => Date.now())

  useEffect(() => {
    if (!regenerateLockedUntil) return
    const timer = window.setInterval(() => setNow(Date.now()), 1000)
    return () => window.clearInterval(timer)
  }, [regenerateLockedUntil])

  useEffect(() => {
    setQuickDraftOpen(false)
  }, [gap.id])

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
  const draftEditorDocumentID =
    draftSuggestion?.target_document_id ?? gap.related_articles[0]?.document_id ?? undefined
  const draftEditorHref =
    draftSuggestion && draftEditorDocumentID
      ? `/w/${wsSlug}/docs/documents/${draftEditorDocumentID}?from_gap=${encodeURIComponent(gap.id)}&from_suggestion=${encodeURIComponent(draftSuggestion.id)}`
      : ''
  const collectionOptions = buildCoverageCollectionOptions(targetSpaceId, collections ?? [])
  const explanation = gap.analysis_explanation
  const recommendations = gap.recommendations ?? []
  const docsAgentAction = documentationAgentAction(gap)
  const canQuickDraft =
    canGenerate &&
    !draftSuggestion &&
    !appliedSuggestion &&
    gap.status === 'open' &&
    (canSuggestImprovements || canDraftNewArticle)
  const quickDraftButtonLabel = canSuggestImprovements ? 'Quick improvement draft' : 'Quick article draft'

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
            <dd className="mt-0.5 truncate font-medium">{gap.topic_title || coverageTopicLabel(gap.issue_key)}</dd>
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
            href={`/w/${wsSlug}/docs/documents/${gap.related_articles[0].document_id}`}
            target="_blank"
            rel="noopener noreferrer"
            className="inline-flex shrink-0 items-center gap-1 text-xs font-medium text-primary hover:underline"
          >
            Open in Editor
            <ArrowUpRight01Icon className="h-3 w-3" />
          </a>
        </div>
      )}

      {gap.status === 'open' && (
        <div className="p-4">
          <div className="space-y-3 rounded-md border border-border/40 bg-muted/20 p-3">
            <div className="flex items-start gap-3">
              <AgentAvatar
                agent={{ name: 'Quill', preset_key: 'documentation_agent' }}
                className="h-8 w-8 shrink-0 rounded-none border-0 bg-transparent shadow-none"
                genericBare
              />
              <div className="min-w-0 flex-1">
                <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
                  Suggested coverage action
                </p>
                <p className="mt-1 text-sm font-medium">{docsAgentAction.label}</p>
                <p className="mt-0.5 text-xs leading-relaxed text-muted-foreground">
                  {docsAgentAction.description}
                </p>
              </div>
              {canRunDocumentationAgent && (
                <button
                  type="button"
                  onClick={() => onRunDocumentationAgent(gap.id)}
                  disabled={startingDocumentationAgent}
                  className="inline-flex shrink-0 items-center gap-1.5 rounded-md bg-primary px-2.5 py-1.5 text-xs font-medium text-primary-foreground transition-colors hover:bg-primary/90 disabled:opacity-50"
                >
                  {startingDocumentationAgent ? (
                    <Loading01Icon className="h-3.5 w-3.5 animate-spin" />
                  ) : (
                    <MagicWand01Icon className="h-3.5 w-3.5" />
                  )}
                  {startingDocumentationAgent ? 'Starting...' : docsAgentAction.cta}
                </button>
              )}
            </div>

            {canQuickDraft && (
              <div className="border-t border-border/50 pt-3">
                {generating ? (
                  <div className="flex items-center gap-2 rounded-md border border-border/40 bg-background/70 px-3 py-2.5 text-xs text-muted-foreground">
                    <Loading01Icon className="h-4 w-4 animate-spin" />
                    Generating quick draft...
                  </div>
                ) : generateError ? (
                  <div className="rounded-md border border-red-200 bg-red-50 p-3">
                    <div className="mb-2 flex items-start gap-2">
                      <AlertCircleIcon className="mt-0.5 h-4 w-4 shrink-0 text-red-600" />
                      <p className="text-xs leading-relaxed text-red-800">
                        Could not generate a quick draft. This may be a temporary service issue.
                      </p>
                    </div>
                    <button
                      type="button"
                      onClick={canSuggestImprovements ? onSuggestImprovements : onDraftNewArticle}
                      className="inline-flex items-center gap-1 text-xs font-medium text-red-700 hover:underline"
                    >
                      <ArrowReloadHorizontalIcon className="h-3 w-3" />
                      Try quick draft again
                    </button>
                  </div>
                ) : quickDraftOpen && canDraftNewArticle ? (
                  <div className="space-y-2">
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
                            <SelectTrigger size="sm" className="flex-1 bg-background">
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
                            <SelectTrigger size="sm" className="flex-1 bg-background">
                              <SelectValue placeholder="Collection (optional)" />
                            </SelectTrigger>
                            <SelectContent>
                              {collectionOptions.map((collection) => (
                                <SelectItem key={collection.id} value={collection.id}>
                                  {collection.label}
                                </SelectItem>
                              ))}
                            </SelectContent>
                          </Select>
                        </div>
                        <div className="flex justify-end gap-2">
                          <button
                            type="button"
                            onClick={() => setQuickDraftOpen(false)}
                            className="rounded-md border border-border/60 px-2.5 py-1.5 text-xs font-medium text-muted-foreground hover:bg-muted"
                          >
                            Cancel
                          </button>
                          <button
                            type="button"
                            onClick={onDraftNewArticle}
                            disabled={!targetSpaceId}
                            className="inline-flex items-center gap-1.5 rounded-md border border-border/60 bg-background px-2.5 py-1.5 text-xs font-medium text-foreground hover:bg-muted disabled:opacity-50"
                          >
                            <MagicWand01Icon className="h-3.5 w-3.5" />
                            Create quick draft
                          </button>
                        </div>
                      </>
                    )}
                  </div>
                ) : (
                  <div className="flex items-center justify-between gap-3">
                    <p className="text-xs leading-relaxed text-muted-foreground">
                      Need a faster draft? Generate one without agent investigation.
                    </p>
                    <button
                      type="button"
                      onClick={canSuggestImprovements ? onSuggestImprovements : () => setQuickDraftOpen(true)}
                      className="inline-flex shrink-0 items-center gap-1.5 rounded-md border border-border/60 bg-background px-2.5 py-1.5 text-xs font-medium text-muted-foreground hover:bg-muted"
                    >
                      <MagicWand01Icon className="h-3.5 w-3.5" />
                      {quickDraftButtonLabel}
                    </button>
                  </div>
                )}
              </div>
            )}
          </div>
        </div>
      )}

      {explanation && (
        <div className="p-4">
          <h4 className="mb-2 text-xs font-semibold uppercase tracking-wide text-muted-foreground">
            Why this matters
          </h4>
          <div className="space-y-2 rounded-md border border-border/40 bg-muted/20 p-3 text-xs leading-relaxed">
            {explanation.customer_need && (
              <p>
                <span className="font-medium text-muted-foreground">Customer needed: </span>
                <span className="break-words">{explanation.customer_need}</span>
              </p>
            )}
            {explanation.ai_failure && (
              <p>
                <span className="font-medium text-muted-foreground">AI missed: </span>
                <span className="break-words">{explanation.ai_failure}</span>
              </p>
            )}
            {explanation.human_resolution && (
              <p>
                <span className="font-medium text-muted-foreground">Human resolved by: </span>
                <span className="break-words">{explanation.human_resolution}</span>
              </p>
            )}
            {explanation.decision_reason && (
              <p>
                <span className="font-medium text-muted-foreground">Reason: </span>
                <span className="break-words">{explanation.decision_reason}</span>
              </p>
            )}
          </div>
        </div>
      )}

      {recommendations.length > 0 && (
        <div className="p-4">
          <h4 className="mb-2 text-xs font-semibold uppercase tracking-wide text-muted-foreground">
            Recommendations
          </h4>
          <div className="space-y-2">
            {recommendations.map((rec) => {
              const isPrimary = rec.priority === 'primary'
              const linkedSuggestion = rec.suggestion_id
                ? gap.suggestions.find((suggestion) => suggestion.id === rec.suggestion_id)
                : null
              return (
                <div key={rec.id} className="rounded-md border border-border/40 bg-card p-3 text-xs">
                  <div className="mb-2 flex items-start justify-between gap-2">
                    <div className="min-w-0">
                      <p className="font-semibold leading-tight">{recommendationLabel(rec.recommendation_type)}</p>
                      {(rec.target_title || rec.target_url) && (
                        <p className="mt-0.5 truncate text-muted-foreground">
                          Target: {rec.target_title || rec.target_url}
                        </p>
                      )}
                    </div>
                    {isPrimary && (
                      <Badge variant="secondary" className="shrink-0 text-[10px] uppercase tracking-wide">
                        Primary
                      </Badge>
                    )}
                  </div>
                  {rec.rationale && (
                    <p className="leading-relaxed">
                      <span className="font-medium text-muted-foreground">Why: </span>
                      <span className="break-words">{rec.rationale}</span>
                    </p>
                  )}
                  {rec.suggested_change && (
                    <p className="mt-1.5 leading-relaxed">
                      <span className="font-medium text-muted-foreground">Suggested change: </span>
                      <span className="break-words">{rec.suggested_change}</span>
                    </p>
                  )}
                  {rec.implementation_notes && (
                    <p className="mt-1.5 leading-relaxed text-muted-foreground">
                      {rec.implementation_notes}
                    </p>
                  )}
                  {rec.target_url && (
                    <a
                      href={rec.target_url}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="mt-2 inline-flex items-center gap-1 font-medium text-primary hover:underline"
                    >
                      Open target
                      <ArrowUpRight01Icon className="h-2.5 w-2.5" />
                    </a>
                  )}
                  {linkedSuggestion && linkedSuggestion.status === 'draft' && (
                    <p className="mt-2 text-muted-foreground">Review Add is available in the draft below.</p>
                  )}
                </div>
              )
            })}
          </div>
        </div>
      )}

      {gap.evidence.length > 0 && (() => {
        const groups = groupEvidenceByConversation(gap.evidence)
        return (
          <div className="p-4">
            <h4 className="mb-2 text-xs font-semibold uppercase tracking-wide text-muted-foreground">
              Evidence
            </h4>
            <div className="max-h-72 space-y-2 overflow-y-auto">
              {groups.map((group) => {
                if (group.kind === 'conversation') {
                  const head = group.items[0]
                  const label =
                    EVIDENCE_TYPE_LABELS[head.evidence_type] ?? head.evidence_type
                  return (
                    <EvidenceConversationCard
                      key={`conv-${group.conversationId}`}
                      conversationId={group.conversationId}
                      wsSlug={wsSlug}
                      evidenceTypeLabel={label}
                      evidenceItems={group.items}
                    />
                  )
                }
                const ev = group.item
                return (
                  <div
                    key={ev.id}
                    className="rounded-md border border-border/40 bg-muted/30 p-2.5 text-xs"
                  >
                    <div className="flex items-center justify-between gap-2">
                      <p className="font-medium text-muted-foreground">
                        {EVIDENCE_TYPE_LABELS[ev.evidence_type] ?? ev.evidence_type}
                        <span className="ml-2 font-normal">{timeAgo(ev.created_at)}</span>
                      </p>
                      {ev.document_id && (
                        <a
                          href={`/w/${wsSlug}/docs/documents/${ev.document_id}`}
                          target="_blank"
                          rel="noopener noreferrer"
                          className="inline-flex items-center gap-0.5 text-primary hover:underline"
                        >
                          Article
                          <ArrowUpRight01Icon className="h-2.5 w-2.5" />
                        </a>
                      )}
                    </div>
                    {ev.excerpt && (
                      <div className="prose-chat mt-1.5 text-sm leading-relaxed">
                        <Markdown>{ev.excerpt}</Markdown>
                      </div>
                    )}
                  </div>
                )
              })}
            </div>
          </div>
        )
      })()}

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
                href={`/w/${wsSlug}/docs/documents/${appliedSuggestion.result_document_id}`}
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
                {Boolean(draftEditorHref && draftSuggestion.content) && (
                  <a
                    href={draftEditorHref}
                    onClick={() =>
                      storeCoverageHandoffContent(gap.id, draftSuggestion.id, draftSuggestion.content)
                    }
                    className="inline-flex items-center gap-1 rounded-md border border-border/60 px-2.5 py-1.5 text-xs font-medium text-muted-foreground hover:bg-muted"
                  >
                    Open in editor
                    <ArrowUpRight01Icon className="h-3 w-3" />
                  </a>
                )}
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
