import { cn } from '@/lib/utils';
import type { CodingSessionStreamState } from '@/lib/pmTypes';
import {
  collectSegments,
  hasRenderableSegments,
  DOCK_SEGMENT_KINDS,
  TranscriptSegmentView,
} from '@/components/agents/transcript';
import { useAuthStore } from '@/stores/authStore';
import { useWorkspaceMembers } from '@/hooks/useWorkspaceMembers';
import type { CodingSessionActor } from '@/lib/pmTypes';
import { groupAdjacentDockTools } from './dockTranscriptGrouping';
const DOCK_CHAT_SEGMENT_KINDS = new Set([...DOCK_SEGMENT_KINDS, 'review_decision'] as const);

/** True when the stream has at least one renderable assistant/tool segment. */
export function dockTranscriptHasContent(
  stream: CodingSessionStreamState | null,
  active: boolean,
): boolean {
  return hasRenderableSegments(stream, { includeLive: active, include: DOCK_SEGMENT_KINDS });
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
  });
  if (segments.length === 0) return null;
  const latestAssistantSegmentId = [...segments].reverse().find((segment) => segment.kind === 'assistant')?.id;
  const entries = groupAdjacentDockTools(segments);

  return (
    <div className={cn('space-y-1.5', className)}>
      {entries.map(({ key, segment, toolGroup }) => (
        <TranscriptSegmentView
          key={key}
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
      ))}
    </div>
  );
}
