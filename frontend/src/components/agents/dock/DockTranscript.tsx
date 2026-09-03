import { Fragment, useState, type ReactNode } from 'react';
import { cn } from '@/lib/utils';
import type { CodingSessionStreamState } from '@/lib/pmTypes';
import {
  collectSegments,
  DOCK_SEGMENT_KINDS,
  segmentTimestamp,
  transcriptSegmentTimes,
  TranscriptSegmentView,
} from '@/components/agents/transcript';
import { useAuthStore } from '@/stores/authStore';
import { useWorkspaceMembers } from '@/hooks/queries/useWorkspaces';
import type { AgentRunMessage, CodingSessionActor, CodingSessionTranscriptMessage } from '@/lib/pmTypes';
import { dockChatService } from '@/lib/services/dockChatService';
import { formatCodingSessionElapsed } from '@/components/pm/CodingSession/codingSessionPresentation';
import { groupAdjacentDockTools } from './dockTranscriptGrouping';
import { buildDockWorkingTimeline } from './dockWorkingGroups';
import { DockWorkingGroup } from './DockWorkingGroup';
import { mergePersistedChatMessages } from './dockChatTimeline';
import { DisclosureChevron } from '@/components/agents/transcript/DisclosureChevron';
const DOCK_CHAT_SEGMENT_KINDS = new Set([...DOCK_SEGMENT_KINDS, 'review_decision', 'status'] as const);
const DOCK_WORKING_SEGMENT_KINDS = new Set([...DOCK_CHAT_SEGMENT_KINDS, 'reasoning'] as const);
const dockWorkDetailCache = new Map<string, AgentRunMessage[]>();

export interface DockSubAgentTimelineItem {
  id: string;
  createdAt: string;
  /** Exact sequence of the hidden result marker, when the attempt settled. */
  resultSequence?: number;
  runCount: number;
  content: ReactNode;
}

function transcriptSegmentSequences(stream: CodingSessionStreamState): Map<string, number> {
  const sequences = new Map<string, number>();
  const remember = (id: string | undefined, sequence: number) => {
    if (id) sequences.set(id, sequence);
  };
  for (const message of stream.transcript_messages) {
    remember(message.event_id, message.sequence_no);
    remember(message.message_id, message.sequence_no);
    for (const segment of message.turn_segments ?? []) {
      remember(segment.segment_id, message.sequence_no);
      if (segment.kind === 'assistant_message') {
        remember(segment.assistant_message.message_id, message.sequence_no);
      } else {
        remember(segment.tool_call.tool_call_id, message.sequence_no);
      }
    }
  }
  return sequences;
}

function segmentSequence(
  segment: ReturnType<typeof collectSegments>[number],
  sequences: Map<string, number>,
): number | null {
  if (segment.kind === 'user' || segment.kind === 'status' || segment.kind === 'context' || segment.kind === 'review_decision') {
    return segment.message.sequence_no;
  }
  if (segment.kind === 'assistant') return sequences.get(segment.id) ?? (segment.messageId ? sequences.get(segment.messageId) : undefined) ?? null;
  if (segment.kind === 'tool') return sequences.get(segment.id) ?? sequences.get(segment.toolCall.tool_call_id) ?? null;
  return sequences.get(segment.id) ?? null;
}

function SubAgentTimelineGroup({ items }: { items: DockSubAgentTimelineItem[] }) {
  const runCount = items.reduce((count, item) => count + Math.max(item.runCount, 1), 0);
  return (
    <section className="divide-y divide-border/60 border-y border-border/70 py-2" data-agent-dock-sub-agent-runs>
      <div className="px-1 pb-2 text-[11px] font-medium uppercase tracking-wide text-muted-foreground">
        Sub-agent runs{runCount > 1 ? ` · ${runCount}` : ''}
      </div>
      {items.map((item) => (
        <div key={item.id} className="py-2 first:pt-0 last:pb-0">
          {item.content}
        </div>
      ))}
    </section>
  );
}

function DockWorkDisclosure({
  workspaceId,
  chatId,
  summary,
}: {
  workspaceId: string;
  chatId: string;
  summary: NonNullable<CodingSessionTranscriptMessage['dock_work_summary']>;
}) {
  const cacheKey = `${workspaceId}:${chatId}:${summary.message_id}`;
  const [open, setOpen] = useState(false);
  const [messages, setMessages] = useState<AgentRunMessage[] | null>(() => dockWorkDetailCache.get(cacheKey) ?? null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(false);

  const loadWork = async () => {
    if (messages || loading) return;
    const cached = dockWorkDetailCache.get(cacheKey);
    if (cached) {
      setMessages(cached);
      return;
    }
    setLoading(true);
    setError(false);
    const response = await dockChatService.getMessageWorkDetail(workspaceId, chatId, summary.message_id);
    setLoading(false);
    if (!response.data) {
      setError(true);
      return;
    }
    dockWorkDetailCache.set(cacheKey, response.data.messages);
    setMessages(response.data.messages);
  };

  const toggle = () => {
    const nextOpen = !open;
    setOpen(nextOpen);
    if (nextOpen) void loadWork();
  };

  const workStream = messages ? mergePersistedChatMessages(null, messages) : null;
  return (
    <section className="py-1" data-dock-work-disclosure>
      <button
        type="button"
        className="inline-flex items-center gap-1.5 text-xs font-medium text-muted-foreground hover:text-foreground"
        aria-expanded={open}
        onClick={toggle}
      >
        <span>Worked for {formatCodingSessionElapsed(summary.duration_ms)}</span>
        <DisclosureChevron open={open} />
      </button>
      {open ? (
        <div className="mt-2 border-l border-border/70 pl-3">
          {loading ? <div className="text-xs text-muted-foreground">Loading work…</div> : null}
          {error ? (
            <button type="button" className="text-xs text-destructive" onClick={() => void loadWork()}>
              Couldn’t load work. Retry
            </button>
          ) : null}
          {workStream ? (
            <DockTranscript
              stream={workStream}
              active={false}
              useRuntimeTimeline={false}
              workspaceId={workspaceId}
              showUserMessages={false}
              compactAssistantProgress
              historyWorkOnly
            />
          ) : null}
        </div>
      ) : null}
    </section>
  );
}

/**
 * Renders an agent run's output inline inside the Ask Agents dock — assistant
 * messages as markdown plus a compact one-line row per tool call — so a
 * one-shot agent's result is readable in the bar without opening the full
 * session sheet. The main chat also shows user turns; embedded execution
 * strips remain assistant/tool-only. All shared segments stay flat and
 * Tool-call rows remain permanently concise; long prose and reasoning can
 * still disclose when the surrounding surface permits it.
 */
export function DockTranscript({
  stream,
  active,
  useRuntimeTimeline = active,
  workspaceId,
  chatId,
  fallbackActor,
  showUserMessages = true,
  compactAssistantProgress = false,
  completedRun = false,
  historyWorkOnly = false,
  subAgentRuns = [],
  className,
}: {
  stream: CodingSessionStreamState | null;
  /** True while the run is still executing — controls live-turn inclusion. */
  active: boolean;
  /** Reconcile retained runtime ordering independently from live styling. */
  useRuntimeTimeline?: boolean;
  /** Workspace used to resolve the author of each human message. */
  workspaceId?: string;
  /** Conversation used to lazily fetch completed work details. */
  chatId?: string;
  /** Actor for older messages that predate per-message attribution. */
  fallbackActor?: CodingSessionActor | null;
  /** Main chat shows user turns; embedded execution strips stay agent-only. */
  showUserMessages?: boolean;
  /** Root Ask chat groups full progress into independently expandable work. */
  compactAssistantProgress?: boolean;
  /** Collapse already-loaded standalone run activity before each final response. */
  completedRun?: boolean;
  /** Render progress and tools without repeating the completed final answer. */
  historyWorkOnly?: boolean;
  /** Delegated work inserted between the messages surrounding its launch. */
  subAgentRuns?: DockSubAgentTimelineItem[];
  className?: string;
}) {
  const user = useAuthStore((state) => state.user);
  const { data: members = [] } = useWorkspaceMembers(workspaceId ?? '');
  const actorsById = new Map<string, CodingSessionActor>(
    members.map((member) => [member.user_id, {
      id: member.user_id,
      email: member.email,
      full_name: member.full_name,
      avatar_url: member.avatar_url,
      avatar_style: member.avatar_style,
      avatar_seed: member.avatar_seed,
      avatar_background_mode: member.avatar_background_mode,
      avatar_background_color: member.avatar_background_color,
    }]),
  );
  if (!stream) return null;
  const segments = collectSegments(stream, {
    includeLive: useRuntimeTimeline,
    runtimeActive: active,
    include: showUserMessages
      ? (compactAssistantProgress ? DOCK_WORKING_SEGMENT_KINDS : DOCK_CHAT_SEGMENT_KINDS)
      : DOCK_SEGMENT_KINDS,
    compactAssistantProgress: false,
  });
  if (segments.length === 0 && subAgentRuns.length === 0) return null;
  const latestAssistantSegmentId = [...segments].reverse().find((segment) => segment.kind === 'assistant')?.id;
  const times = transcriptSegmentTimes(stream);
  const workingTimeline = compactAssistantProgress
    ? buildDockWorkingTimeline(segments, active, {
        collapseCompletedWork: completedRun,
        timestampForSegment: (segment) => segmentTimestamp(segment, times),
      })
    : null;
  const entries = workingTimeline
    ? workingTimeline.map((entry) => entry.kind === 'working_group'
      ? { key: entry.key, segment: entry.segments[0], workingGroup: entry }
      : { key: entry.key, segment: entry.segment })
    : groupAdjacentDockTools(segments);
  const assistantPresentations = new Map<number, { presentation: 'progress' | 'final'; separator: boolean }>();
  let intervalStart = 0;
  let intervalAssistantIndexes: number[] = [];
  const finalizeInterval = (end: number, allowFinal: boolean) => {
    for (const index of intervalAssistantIndexes) {
      assistantPresentations.set(index, { presentation: 'progress', separator: false });
    }
    if (!allowFinal) return;
    const finalIndex = [...intervalAssistantIndexes].reverse().find((index) => {
      const segment = entries[index]?.segment;
      return segment?.kind === 'assistant' && !segment.streaming;
    });
    if (finalIndex === undefined) return;
    const separator = entries.slice(intervalStart, finalIndex).some((candidate) => (
      'workingGroup' in candidate
      || candidate.segment.kind === 'reasoning'
      || candidate.segment.kind === 'assistant'
    ));
    assistantPresentations.set(finalIndex, { presentation: 'final', separator });
    intervalStart = end;
  };
  for (let index = 0; index < entries.length; index += 1) {
    const segment = entries[index].segment;
    if (segment.kind === 'user' || segment.kind === 'review_decision') {
      finalizeInterval(index, true);
      intervalStart = index + 1;
      intervalAssistantIndexes = [];
      continue;
    }
    if (segment.kind === 'assistant') intervalAssistantIndexes.push(index);
  }
  finalizeInterval(entries.length, !active);
  const sequences = transcriptSegmentSequences(stream);
  const runsByBoundary = new Map<number, DockSubAgentTimelineItem[]>();
  for (const item of [...subAgentRuns].sort((left, right) => {
    const timeDelta = Date.parse(left.createdAt) - Date.parse(right.createdAt);
    return Number.isFinite(timeDelta) && timeDelta !== 0 ? timeDelta : left.id.localeCompare(right.id);
  })) {
    const itemTimestamp = Date.parse(item.createdAt);
    let boundary = entries.length;
    if (Number.isFinite(itemTimestamp)) {
      const laterEntry = entries.findIndex((entry) => {
        const timestamp = segmentTimestamp(entry.segment, times);
        return timestamp !== null && timestamp > itemTimestamp;
      });
      if (laterEntry >= 0) boundary = laterEntry;
    }
    if (item.resultSequence !== undefined) {
      const resultBoundary = entries.findIndex((entry) => {
        const sequence = segmentSequence(entry.segment, sequences);
        return sequence !== null && sequence >= item.resultSequence!;
      });
      if (resultBoundary >= 0) boundary = Math.min(boundary, resultBoundary);
    }
    const existing = runsByBoundary.get(boundary) ?? [];
    existing.push(item);
    runsByBoundary.set(boundary, existing);
  }

  return (
    <div className={cn('space-y-1.5', className)}>
      {entries.map((entry, index) => {
        const workingGroup = 'workingGroup' in entry ? entry.workingGroup : undefined;
        const followsUserMessage = index > 0 && entries[index - 1].segment.kind === 'user';
        const assistantPresentation = assistantPresentations.get(index);
        if (historyWorkOnly && (
          entry.segment.kind === 'user'
          || entry.segment.kind === 'status'
          || assistantPresentation?.presentation === 'final'
        )) return null;
        if (
          entry.segment.kind === 'status'
          && entry.segment.message.dock_work_summary
          && workspaceId
          && chatId
        ) {
          return (
            <DockWorkDisclosure
              key={entry.key}
              workspaceId={workspaceId}
              chatId={chatId}
              summary={entry.segment.message.dock_work_summary}
            />
          );
        }
        return (
          <Fragment key={entry.key}>
            {runsByBoundary.has(index) ? <SubAgentTimelineGroup items={runsByBoundary.get(index)!} /> : null}
            <div
              className={cn(
                followsUserMessage && 'pt-2',
                assistantPresentation?.separator && 'mt-3 border-t border-border/60 pt-4',
              )}
              data-after-user-message={followsUserMessage ? 'true' : undefined}
              data-assistant-presentation={assistantPresentation?.presentation}
              data-final-response-separator={assistantPresentation?.separator ? 'true' : undefined}
            >
              {workingGroup ? (
                <DockWorkingGroup
                  id={workingGroup.key}
                  segments={workingGroup.segments}
                  active={workingGroup.active}
                  completedDurationMs={workingGroup.durationMs}
                >
                  {workingGroup.segments.map((segment) => (
                    <TranscriptSegmentView
                      key={segment.id}
                      segment={segment}
                      options={{
                        expandable: true,
                        collapseLongAssistantContent: false,
                        showToolDetails: true,
                        showReasoningDetails: true,
                        assistantPresentation: segment.kind === 'assistant' ? 'progress' : undefined,
                        fallbackUserLabel: 'You',
                      }}
                    />
                  ))}
                </DockWorkingGroup>
              ) : (
                <TranscriptSegmentView
                  segment={entry.segment}
                  options={{
                    expandable: true,
                    toolGroup: 'toolGroup' in entry ? entry.toolGroup : undefined,
                    collapseLongAssistantContent: assistantPresentation?.presentation !== 'final'
                      && (entry.segment.kind !== 'assistant' || entry.segment.id !== latestAssistantSegmentId),
                    assistantPresentation: assistantPresentation?.presentation,
                    fallbackUserLabel: 'You',
                    resolveActor: (message) => {
                      const attributedUserId = message.resolver_user_id ?? message.actor_user_id;
                      if (attributedUserId) {
                        const actor = actorsById.get(attributedUserId);
                        if (actor) return actor;
                      }
                      if (fallbackActor) return actorsById.get(fallbackActor.id) ?? fallbackActor;
                      return user ? {
                          id: user.id,
                          email: user.email,
                          full_name: user.full_name,
                          avatar_url: user.avatar_url,
                          avatar_style: user.avatar_style,
                          avatar_seed: user.avatar_seed,
                          avatar_background_mode: user.avatar_background_mode,
                          avatar_background_color: user.avatar_background_color,
                        } : null;
                    },
                  }}
                />
              )}
            </div>
          </Fragment>
        );
      })}
      {runsByBoundary.has(entries.length) ? <SubAgentTimelineGroup items={runsByBoundary.get(entries.length)!} /> : null}
    </div>
  );
}
