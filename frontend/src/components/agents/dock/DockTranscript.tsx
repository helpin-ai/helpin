import { MarkdownContent } from '@/components/pm/CodingSession/MarkdownContent';
import { describeToolCall } from '@/components/pm/CodingSession/toolCallPresentation';
import { Cancel01Icon, Loading01Icon, Tick01Icon } from '@/lib/icons';
import { isToolName } from '@/lib/toolNames';
import { cn } from '@/lib/utils';
import type {
  CodingSessionLiveToolCall,
  CodingSessionStreamState,
} from '@/lib/pmTypes';

type DockSegment =
  | { kind: 'assistant'; id: string; content: string; streaming?: boolean }
  | { kind: 'tool'; id: string; toolCall: CodingSessionLiveToolCall };

/**
 * Flatten a reconciled run stream into an ordered list of assistant-text and
 * tool-call segments for compact inline rendering in the Ask Agents dock.
 *
 * Completed turns come from `transcript_messages` (preferring their
 * `turn_segments` so text and tool calls stay interleaved in order). While the
 * run is still active we append `live_turn_segments` for the in-flight turn;
 * once terminal those are already folded into the transcript, so we skip them
 * to avoid double-rendering the final answer.
 */
function collectDockSegments(
  stream: CodingSessionStreamState,
  includeLive: boolean,
): DockSegment[] {
  const out: DockSegment[] = [];

  for (const message of stream.transcript_messages) {
    if (message.role !== 'assistant') continue;
    const segments = message.turn_segments?.length ? message.turn_segments : null;
    if (segments) {
      for (const segment of segments) {
        if (segment.kind === 'assistant_message') {
          const content = segment.assistant_message.content.trim();
          if (content) out.push({ kind: 'assistant', id: segment.segment_id, content });
        } else if (segment.kind === 'tool_call' && !isToolName(segment.tool_call.tool_name, 'update_plan')) {
          out.push({ kind: 'tool', id: segment.segment_id, toolCall: segment.tool_call });
        }
      }
      continue;
    }
    if (message.content.trim()) {
      out.push({ kind: 'assistant', id: message.event_id, content: message.content.trim() });
    }
    for (const toolCall of message.tool_calls ?? []) {
      if (isToolName(toolCall.tool_name, 'update_plan')) continue;
      out.push({ kind: 'tool', id: toolCall.tool_call_id, toolCall });
    }
  }

  if (includeLive) {
    for (const segment of stream.live_turn_segments) {
      if (segment.kind === 'assistant_message') {
        const content = segment.assistant_message.content.trim();
        if (content) {
          out.push({
            kind: 'assistant',
            id: `live-${segment.segment_id}`,
            content,
            streaming: segment.assistant_message.status === 'streaming',
          });
        }
      } else if (segment.kind === 'tool_call' && !isToolName(segment.tool_call.tool_name, 'update_plan')) {
        out.push({ kind: 'tool', id: `live-${segment.segment_id}`, toolCall: segment.tool_call });
      }
    }
  }

  return out;
}

/** True when the stream has at least one renderable assistant/tool segment. */
export function dockTranscriptHasContent(
  stream: CodingSessionStreamState | null,
  active: boolean,
): boolean {
  if (!stream) return false;
  return collectDockSegments(stream, active).length > 0;
}

/**
 * Renders an agent run's output inline inside the Ask Agents dock — assistant
 * messages as markdown plus a compact row per tool call — so a one-shot agent's
 * result is readable in the bar without opening the full session sheet.
 */
export function DockTranscript({
  stream,
  active,
  className,
}: {
  stream: CodingSessionStreamState | null;
  /** True while the run is still executing — controls live-turn inclusion. */
  active: boolean;
  className?: string;
}) {
  if (!stream) return null;
  const segments = collectDockSegments(stream, active);
  if (segments.length === 0) return null;

  return (
    <div className={cn('space-y-1.5', className)}>
      {segments.map((segment) =>
        segment.kind === 'assistant' ? (
          <MarkdownContent
            key={segment.id}
            content={segment.content}
            streaming={segment.streaming}
            className="text-[13px] leading-6 text-foreground/90"
          />
        ) : (
          <ToolCallRow key={segment.id} toolCall={segment.toolCall} />
        ),
      )}
    </div>
  );
}

function ToolCallRow({ toolCall }: { toolCall: CodingSessionLiveToolCall }) {
  const { primaryLabel } = describeToolCall(toolCall);
  const failed = toolCall.status === 'failed';
  const running = toolCall.status === 'running';
  return (
    <div className="flex items-center gap-1.5 text-[11px] text-muted-foreground">
      {running ? (
        <Loading01Icon className="h-3 w-3 shrink-0 animate-spin text-orange-500" />
      ) : failed ? (
        <Cancel01Icon className="h-3 w-3 shrink-0 text-destructive" />
      ) : (
        <Tick01Icon className="h-3 w-3 shrink-0 text-emerald-600 dark:text-emerald-400" />
      )}
      <span className={cn('min-w-0 flex-1 truncate', failed && 'text-destructive')}>
        {primaryLabel}
      </span>
    </div>
  );
}
