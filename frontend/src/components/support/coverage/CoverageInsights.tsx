import { Button } from '@/components/ui/button'
import { TabsContent } from '@/components/ui/tabs'
import {
  Sheet,
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
import type {
  CoverageSignalV2,
  CoverageTopicDetailV2,
  CoverageTopicV2,
} from '@/lib/supportCoverageTypes'
import { timeAgo } from '@/lib/utils'

interface InsightPagination {
  total?: number
  hasMore: boolean
  loadingMore: boolean
  loadMoreError: string | null
  loadMore: () => unknown
}

interface CoverageInsightsProps {
  topics: CoverageTopicV2[]
  signals: CoverageSignalV2[]
  selectedTopic: CoverageTopicDetailV2 | null
  topicLoading: boolean
  topicsLoading: boolean
  signalsLoading: boolean
  topicPagination: InsightPagination
  signalPagination: InsightPagination
  errors: Record<string, string>
  wsSlug: string
  signalTopicSelections: Record<string, string>
  pendingSignalId: string | null
  canEdit: boolean
  onSelectSignalTopic: (signalId: string, topicId: string) => void
  onOpenTopic: (topicId: string) => void
  onCloseTopic: () => void
  onAttachSignal: (signalId: string) => void
  onDismissSignal: (signalId: string) => void
  onRefresh: () => void
  onOpenConversationGaps: (conversationId: string) => void
}

function InsightListFooter({ count, label, pagination }: { count: number; label: string; pagination: InsightPagination }) {
  return (
    <div className="flex flex-wrap items-center justify-between gap-3 pt-3">
      <span className="text-xs text-quiet-text-tertiary">
        Showing {count}{typeof pagination.total === 'number' ? ` of ${pagination.total}` : ''} {label}
      </span>
      <div className="flex items-center gap-3">
        {pagination.loadMoreError && <span role="alert" className="text-xs text-destructive">Could not load more. Try again.</span>}
        {pagination.hasMore && (
          <Button variant="outline" size="sm" disabled={pagination.loadingMore} onClick={() => { void pagination.loadMore() }}>
            {pagination.loadingMore ? 'Loading…' : pagination.loadMoreError ? 'Try again' : 'Load more'}
          </Button>
        )}
      </div>
    </div>
  )
}

function EmptyState({
  title,
  description,
}: {
  title: string
  description: string
}) {
  return (
    <div className="space-y-2 py-8">
      <p className="text-sm font-medium">{title}</p>
      <p className="max-w-xl text-sm leading-relaxed text-quiet-text-tertiary">
        {description}
      </p>
    </div>
  )
}

export function CoverageInsights(props: CoverageInsightsProps) {
  const {
    topics,
    signals,
    selectedTopic,
    topicLoading,
    errors,
    wsSlug,
    canEdit,
  } = props
  const state = (surface: string) =>
    (surface === 'topics' ? props.topicsLoading : props.signalsLoading) ? (
      <p role="status" className="py-8 text-quiet-text-tertiary">
        Loading coverage…
      </p>
    ) : errors[surface] ? (
      <div role="alert" className="space-y-3 py-8">
        <p>{errors[surface]}</p>
        <Button variant="outline" size="sm" onClick={props.onRefresh}>
          Try again
        </Button>
      </div>
    ) : null

  return (
    <>
      <TabsContent value="topics">
        <p className="mb-4 text-sm text-quiet-text-tertiary">
          Related customer needs, grouped from analyzed conversations. Open a
          topic to compare the answers and recommended fixes.
        </p>
        {state('topics') ??
          (topics.length === 0 ? (
            <EmptyState
              title="No customer topics yet"
              description="Topics appear when analysis identifies a customer need. Your existing gaps remain available in Gaps to resolve."
            />
          ) : (
            <div className="divide-y divide-quiet-divider-light">
              {topics.map((topic) => (
                <button
                  key={topic.id}
                  type="button"
                  onClick={() => props.onOpenTopic(topic.id)}
                  className="block w-full min-w-0 py-4 text-left transition-colors hover:bg-quiet-row-hover focus-visible:outline-2 focus-visible:outline-ring"
                >
                  <div className="flex items-start justify-between gap-4">
                    <span className="break-words text-sm font-semibold">
                      {topic.title}
                    </span>
                    <span className="shrink-0 text-xs capitalize text-quiet-text-tertiary">
                      {topic.status}
                    </span>
                  </div>
                  <p className="mt-1 line-clamp-2 text-sm text-quiet-text-secondary">
                    {topic.customer_need}
                  </p>
                  <p className="mt-2 text-xs text-quiet-text-tertiary">
                    Based on {topic.conversation_count} conversations ·{' '}
                    {topic.customer_count} customers · {topic.finding_count}{' '}
                    findings
                  </p>
                </button>
              ))}
            </div>
          ))}
        {!props.topicsLoading && !errors.topics && topics.length > 0 && (
          <InsightListFooter count={topics.length} label="topics" pagination={props.topicPagination} />
        )}
      </TabsContent>
      <TabsContent value="signals">
        <p className="mb-4 text-sm text-quiet-text-tertiary">
          These signals need human review. Attach relevant evidence to a
          customer topic, or dismiss it.
        </p>
        {canEdit && signals.length > 0 && props.topicPagination.hasMore && (
          <Button className="mb-3" size="sm" variant="ghost" disabled={props.topicPagination.loadingMore} onClick={() => { void props.topicPagination.loadMore() }}>
            {props.topicPagination.loadingMore ? 'Loading topics…' : 'Load more topics to choose from'}
          </Button>
        )}
        {state('signals') ??
          (signals.length === 0 ? (
            <EmptyState
              title="Nothing waiting for review"
              description="Signals that need your judgment will appear here as customers use support."
            />
          ) : (
            <div className="divide-y divide-quiet-divider-light">
              {signals.map((signal) => (
                <div key={signal.id} className="space-y-3 py-4">
                  <p className="break-words text-sm font-semibold">
                    {signal.normalized_query}
                  </p>
                  <p className="text-xs text-quiet-text-tertiary">
                    {signal.source_kind.replaceAll('_', ' ')} ·{' '}
                    {timeAgo(signal.observed_at)}
                  </p>
                  {canEdit && (
                    <div className="flex flex-wrap items-center gap-2">
                      {topics.length > 0 ? (
                        <Select
                          value={props.signalTopicSelections[signal.id] ?? ''}
                          onValueChange={(value) =>
                            props.onSelectSignalTopic(signal.id, value)
                          }
                          disabled={Boolean(props.pendingSignalId)}
                        >
                          <SelectTrigger
                            className="w-full sm:w-72"
                            aria-label={`Topic for ${signal.normalized_query}`}
                          >
                            <SelectValue placeholder="Choose a customer topic" />
                          </SelectTrigger>
                          <SelectContent>
                            {topics.map((topic) => (
                              <SelectItem key={topic.id} value={topic.id}>
                                {topic.title}
                              </SelectItem>
                            ))}
                          </SelectContent>
                        </Select>
                      ) : (
                        <span className="text-xs text-quiet-text-tertiary">
                          A topic must be available before you can attach this
                          signal.
                        </span>
                      )}
                      <Button
                        size="sm"
                        variant="outline"
                        disabled={
                          !props.signalTopicSelections[signal.id] ||
                          Boolean(props.pendingSignalId)
                        }
                        onClick={() => props.onAttachSignal(signal.id)}
                      >
                        Attach to topic
                      </Button>
                      <Button
                        size="sm"
                        variant="ghost"
                        disabled={Boolean(props.pendingSignalId)}
                        onClick={() => props.onDismissSignal(signal.id)}
                      >
                        Dismiss
                      </Button>
                      {props.pendingSignalId === signal.id && (
                        <span
                          role="status"
                          className="text-xs text-quiet-text-tertiary"
                        >
                          Saving…
                        </span>
                      )}
                    </div>
                  )}
                </div>
              ))}
            </div>
          ))}
        {!props.signalsLoading && !errors.signals && signals.length > 0 && (
          <InsightListFooter count={signals.length} label="signals" pagination={props.signalPagination} />
        )}
      </TabsContent>
      <Sheet
        open={Boolean(selectedTopic) || topicLoading}
        onOpenChange={(open) => {
          if (!open) props.onCloseTopic()
        }}
      >
        <SheetContent className="overflow-y-auto p-6 data-[side=right]:w-full data-[side=right]:sm:max-w-2xl">
          <SheetTitle className="pr-8 text-xl">
            {selectedTopic?.topic.title ?? 'Loading topic…'}
          </SheetTitle>
          <SheetDescription>
            Evidence and recommended fixes from support conversations.
          </SheetDescription>
          {topicLoading && (
            <p role="status" className="py-8 text-sm">
              Loading evidence…
            </p>
          )}
          {selectedTopic?.findings.length === 0 && (
            <EmptyState
              title="No findings available"
              description="New findings will appear here after related conversations are analyzed."
            />
          )}
          {selectedTopic?.findings.map((finding) => (
            <article
              key={finding.id}
              className="space-y-4 border-t border-quiet-divider-strong py-5"
            >
              <div>
                <h3 className="text-xs font-semibold uppercase tracking-wide text-quiet-text-tertiary">
                  Customer need
                </h3>
                <p className="mt-1 text-sm leading-relaxed">
                  {finding.customer_need}
                </p>
              </div>
              <dl className="space-y-4 text-sm">
                {finding.ai_answer && (
                  <div>
                    <dt className="font-medium">AI answer</dt>
                    <dd className="mt-1 whitespace-pre-wrap text-quiet-text-secondary">
                      {finding.ai_answer}
                    </dd>
                  </div>
                )}
                {finding.ai_failure && (
                  <div>
                    <dt className="font-medium">Where support fell short</dt>
                    <dd className="mt-1 text-quiet-text-secondary">
                      {finding.ai_failure}
                    </dd>
                  </div>
                )}
                <div>
                  <dt className="font-medium">Human resolution</dt>
                  <dd className="mt-1 text-quiet-text-secondary">
                    {finding.human_answer || 'No human answer observed.'}
                  </dd>
                </div>
                <div className="border-t border-quiet-divider-light pt-4">
                  <dt className="font-semibold">
                    Recommended fix · {finding.fix_type.replaceAll('_', ' ')}
                  </dt>
                  <dd className="mt-1 space-y-2 text-quiet-text-secondary">
                    {finding.fix_target && <p>{finding.fix_target}</p>}
                    <p>{finding.suggested_change || finding.rationale}</p>
                    {finding.suggested_change && finding.rationale && (
                      <p className="text-xs">{finding.rationale}</p>
                    )}
                  </dd>
                </div>
              </dl>
              {finding.conversation_id ? (
                <div className="flex flex-wrap items-center gap-4">
                  <Button size="sm" variant="outline" onClick={() => props.onOpenConversationGaps(finding.conversation_id!)}>
                    View open gaps
                  </Button>
                <a
                  href={`/w/${encodeURIComponent(wsSlug)}/support/${encodeURIComponent(finding.conversation_id)}`}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="inline-block text-sm underline underline-offset-4"
                >
                  View source conversation
                  <span className="sr-only"> (opens in a new tab)</span>
                </a>
                </div>
              ) : (
                <p className="text-xs text-quiet-text-tertiary">
                  Source conversation unavailable.
                </p>
              )}
            </article>
          ))}
        </SheetContent>
      </Sheet>
    </>
  )
}
