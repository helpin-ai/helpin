import { Fragment, type ReactNode } from 'react';
import { cn } from '@/lib/utils';
import type { CodingSessionStreamState } from '@/lib/pmTypes';
import {
  collectSegments,
  DOCK_SEGMENT_KINDS,
  TranscriptSegmentView,
} from '@/components/agents/transcript';
import { useAuthStore } from '@/stores/authStore';
import { useWorkspaceMembers } from '@/hooks/useWorkspaceMembers';
import type { CodingSessionActor } from '@/lib/pmTypes';
import { groupAdjacentDockTools } from './dockTranscriptGrouping';
const DOCK_CHAT_SEGMENT_KINDS = new Set([...DOCK_SEGMENT_KINDS, 'review_decision'] as const);

export interface DockSubAgentTimelineItem {
  id: string;
  createdAt: string;
  /** Exact sequence of the hidden result marker, when the attempt settled. */
  resultSequence?: number;
  runCount: number;
  content: ReactNode;
}

function transcriptSegmentTimes(stream: CodingSessionStreamState): Map<string, number> {
  const times = new Map<string, number>();
  const remember = (id: string | undefined, value: string | undefined) => {
    if (!id || !value) return;
    const timestamp = Date.parse(value);
    if (Number.isFinite(timestamp)) times.set(id, timestamp);
  };

  for (const message of stream.transcript_messages) {
    remember(message.event_id, message.timestamp);
    remember(message.message_id, message.timestamp);
    for (const segment of message.turn_segments ?? []) {
      remember(segment.segment_id, message.timestamp);
      if (segment.kind === 'assistant_message') {
        remember(segment.assistant_message.message_id, segment.assistant_message.started_at ?? message.timestamp);
      } else {
        remember(segment.tool_call.tool_call_id, segment.tool_call.started_at ?? message.timestamp);
      }
    }
  }
  for (const segment of stream.live_turn_segments) {
    if (segment.kind === 'assistant_message') {
      remember(segment.segment_id, segment.assistant_message.started_at);
      remember(segment.assistant_message.message_id, segment.assistant_message.started_at);
    } else {
      remember(segment.segment_id, segment.tool_call.started_at);
      remember(segment.tool_call.tool_call_id, segment.tool_call.started_at);
    }
  }
  return times;
}

function segmentTimestamp(segment: ReturnType<typeof collectSegments>[number], times: Map<string, number>): number | null {
  if (segment.kind === 'user' || segment.kind === 'status' || segment.kind === 'context' || segment.kind === 'review_decision') {
    const timestamp = Date.parse(segment.message.timestamp);
    return Number.isFinite(timestamp) ? timestamp : null;
  }
  if (segment.kind === 'assistant') return times.get(segment.id) ?? (segment.messageId ? times.get(segment.messageId) : undefined) ?? null;
  if (segment.kind === 'tool') {
    const timestamp = Date.parse(segment.toolCall.started_at ?? '');
    return Number.isFinite(timestamp) ? timestamp : times.get(segment.id) ?? times.get(segment.toolCall.tool_call_id) ?? null;
  }
  const timestamp = Date.parse(segment.reasoning.started_at ?? '');
  return Number.isFinite(timestamp) ? timestamp : times.get(segment.id) ?? null;
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
  workspaceId,
  fallbackActor,
  showUserMessages = true,
  compactAssistantProgress = false,
  subAgentRuns = [],
  className,
}: {
  stream: CodingSessionStreamState | null;
  /** True while the run is still executing — controls live-turn inclusion. */
  active: boolean;
  /** Workspace used to resolve the author of each human message. */
  workspaceId?: string;
  /** Actor for older messages that predate per-message attribution. */
  fallbackActor?: CodingSessionActor | null;
  /** Main chat shows user turns; embedded execution strips stay agent-only. */
  showUserMessages?: boolean;
  /** Root Ask chat keeps only the latest assistant prose in each interval. */
  compactAssistantProgress?: boolean;
  /** Delegated work inserted between the messages surrounding its launch. */
  subAgentRuns?: DockSubAgentTimelineItem[];
  className?: string;
}) {
  const user = useAuthStore((state) => state.user);
  const { members } = useWorkspaceMembers(workspaceId);
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
    includeLive: active,
    include: showUserMessages ? DOCK_CHAT_SEGMENT_KINDS : DOCK_SEGMENT_KINDS,
    compactAssistantProgress,
  });
  if (segments.length === 0 && subAgentRuns.length === 0) return null;
  const latestAssistantSegmentId = [...segments].reverse().find((segment) => segment.kind === 'assistant')?.id;
  const entries = groupAdjacentDockTools(segments);
  const times = transcriptSegmentTimes(stream);
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
      {entries.map(({ key, segment, toolGroup }, index) => (
        <Fragment key={key}>
          {runsByBoundary.has(index) ? <SubAgentTimelineGroup items={runsByBoundary.get(index)!} /> : null}
          <TranscriptSegmentView
            segment={segment}
            options={{
              expandable: true,
              toolGroup,
              collapseLongAssistantContent: segment.kind !== 'assistant' || segment.id !== latestAssistantSegmentId,
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
        </Fragment>
      ))}
      {runsByBoundary.has(entries.length) ? <SubAgentTimelineGroup items={runsByBoundary.get(entries.length)!} /> : null}
    </div>
  );
}
