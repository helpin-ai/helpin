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
  CoveragePipelineHealthV2,
  CoverageSignalV2,
  CoverageTopicDetailV2,
  CoverageTopicV2,
} from '@/lib/supportCoverageTypes'
import { timeAgo } from '@/lib/utils'

interface CoverageInsightsProps {
  topics: CoverageTopicV2[]
  signals: CoverageSignalV2[]
  health: CoveragePipelineHealthV2 | null
  selectedTopic: CoverageTopicDetailV2 | null
  topicLoading: boolean
  loading: boolean
  errors: Record<string, string>
  wsSlug: string
  signalTopicSelections: Record<string, string>
  pendingSignalId: string | null
  pendingAttemptId: string | null
  canEdit: boolean
  canManage: boolean
  onSelectSignalTopic: (signalId: string, topicId: string) => void
  onOpenTopic: (topicId: string) => void
  onCloseTopic: () => void
  onAttachSignal: (signalId: string) => void
  onDismissSignal: (signalId: string) => void
  onRetryAttempt: (attemptId: string) => void
  onRefresh: () => void
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
    health,
    selectedTopic,
    topicLoading,
    loading,
    errors,
    wsSlug,
    canEdit,
    canManage,
  } = props
  const state = (surface: string) =>
    loading ? (
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
      </TabsContent>
      <TabsContent value="signals">
        <p className="mb-4 text-sm text-quiet-text-tertiary">
          These signals need human review. Attach relevant evidence to a
          customer topic, or dismiss it.
        </p>
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
      </TabsContent>
      <TabsContent value="health">
        {state('health') ??
          (!health ? (
            <EmptyState
              title="Analysis status unavailable"
              description="Refresh to check the latest analysis status."
            />
          ) : (
            <>
              <p className="text-sm font-semibold">
                {!health.rollout.capture_enabled
                  ? 'Coverage analysis is paused'
                  : !health.latest_batch
                    ? 'Waiting for the first analysis'
                    : health.healthy
                      ? 'No analysis failures'
                      : 'Some conversations need another attempt'}
              </p>
              {health.latest_batch && (
                <p className="mt-2 text-sm text-quiet-text-tertiary">
                  {health.latest_batch.succeeded_count} of{' '}
                  {health.latest_batch.candidate_count} conversations analyzed ·{' '}
                  {health.latest_batch.status.replaceAll('_', ' ')}
                  {health.latest_batch.completed_at
                    ? ` · ${timeAgo(health.latest_batch.completed_at)}`
                    : ''}
                </p>
              )}
              <div className="mt-4 divide-y divide-quiet-divider-light">
                {health.failures.map((failure) => (
                  <div
                    key={failure.id}
                    className="flex items-start justify-between gap-4 py-4"
                  >
                    <div className="min-w-0">
                      <p className="text-sm font-medium">
                        Conversation analysis needs attention
                      </p>
                      <p className="mt-1 text-xs text-quiet-text-tertiary">
                        {timeAgo(failure.created_at)} ·{' '}
                        {failure.retry_budget_used} retries
                      </p>
                      <details className="mt-2 text-xs text-quiet-text-tertiary">
                        <summary className="cursor-pointer">
                          Technical details
                        </summary>
                        <p className="mt-2 break-words">
                          {failure.failure_class || failure.stage}:{' '}
                          {failure.failure_message || 'Waiting for retry'}
                        </p>
                      </details>
                    </div>
                    {canManage && (
                      <Button
                        size="sm"
                        variant="outline"
                        disabled={Boolean(props.pendingAttemptId)}
                        onClick={() => props.onRetryAttempt(failure.id)}
                      >
                        {props.pendingAttemptId === failure.id
                          ? 'Queuing…'
                          : 'Retry'}
                      </Button>
                    )}
                  </div>
                ))}
              </div>
            </>
          ))}
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
                <a
                  href={`/w/${encodeURIComponent(wsSlug)}/support/${encodeURIComponent(finding.conversation_id)}`}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="inline-block text-sm underline underline-offset-4"
                >
                  View source conversation
                  <span className="sr-only"> (opens in a new tab)</span>
                </a>
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
