import {
  parseDockChildResult,
  parseDockMediaAttachments,
  stripDockLeadingContext,
  stripDockPageContext,
  type DockChildRunResult,
} from '@/lib/dockTypes';
import type { CodingSessionStreamState } from '@/lib/pmTypes';

/** Minimal run shape the composer needs (AgentRun and CodingSession both fit). */
export interface DockRunLike {
  status: string;
  pause_reason: string;
}

export interface DockComposerState {
  visible: boolean;
  enabled: boolean;
  placeholder: string;
}

/** Only runs that are actively producing events should expose a live transcript tail. */
export function isDockTranscriptStreaming(run: DockRunLike | null): boolean {
  return run?.status === 'queued' || run?.status === 'running';
}

/**
 * Composer state for a dock chat. Mirrors the coding-session composer matrix
 * with the dock-specific cases: a chat with no backing run yet (the first
 * message starts it lazily) and a chat whose run ended (the next message
 * starts a successor run carrying the conversation forward).
 */
export function resolveDockComposerState(
  run: DockRunLike | null,
  hasStructuredInteraction: boolean,
  loading: boolean,
): DockComposerState {
  if (!run) {
    if (loading) {
      return { visible: true, enabled: false, placeholder: 'Loading chat…' };
    }
    return { visible: true, enabled: true, placeholder: 'Ask anything, or tell an agent what to do' };
  }
  if (hasStructuredInteraction) {
    return { visible: true, enabled: false, placeholder: 'Respond to the agent above' };
  }
  switch (run.status) {
    case 'completed':
    case 'failed':
    case 'cancelled':
      return { visible: true, enabled: true, placeholder: 'Send a message to continue this chat' };
    case 'queued':
      return { visible: true, enabled: false, placeholder: 'Agent is starting…' };
    case 'running':
      return { visible: true, enabled: false, placeholder: 'Agent is working…' };
    case 'paused':
      switch (run.pause_reason) {
        case 'awaiting_user_message':
          return { visible: true, enabled: true, placeholder: 'Reply…' };
        case 'human_input':
          return { visible: true, enabled: true, placeholder: 'Answer the agent…' };
        case 'authentication':
          return { visible: true, enabled: false, placeholder: 'Waiting for authentication…' };
        default:
          return { visible: true, enabled: false, placeholder: 'Use the review controls to respond' };
      }
    default:
      return { visible: true, enabled: false, placeholder: 'Agent is working…' };
  }
}

export interface DockChildResultEntry {
  sequenceNo: number;
  result: DockChildRunResult;
}

export interface DockTranscriptTransform {
  stream: CodingSessionStreamState;
  childResults: DockChildResultEntry[];
}

/**
 * Rewrites a run stream for chat display: strips <page_context> blocks from
 * user messages and extracts <child_run_result> messages into structured
 * entries (removing the raw JSON bubble from the transcript).
 */
export function transformDockStream(stream: CodingSessionStreamState, order: 'time' | 'sequence' = 'time'): DockTranscriptTransform {
  const childResults: DockChildResultEntry[] = [];
  const messages = [];
  for (const message of stream.transcript_messages) {
    if (message.role !== 'user') {
      messages.push(message);
      continue;
    }
    const childResult = parseDockChildResult(message.content);
    if (childResult) {
      childResults.push({ sequenceNo: message.sequence_no, result: childResult });
      continue;
    }
    const attachments = parseDockMediaAttachments(message.content);
    const stripped = stripDockPageContext(stripDockLeadingContext(message.content));
    if (stripped === message.content) {
      messages.push(message);
    } else {
      messages.push({ ...message, content: stripped, ...(attachments.length > 0 ? { attachments } : {}) });
    }
  }
  // Message sequence numbers are assigned in projection-arrival order, which
  // can lag conversation order (a mirrored user message may get a higher
  // sequence than the assistant reply it caused). Timestamps are
  // conversation-true, so the chat orders by time with sequence as tiebreak.
  messages.sort((a, b) => {
	if (order === 'sequence') return a.sequence_no - b.sequence_no;
    const timeDelta = Date.parse(a.timestamp) - Date.parse(b.timestamp);
    if (!Number.isNaN(timeDelta) && timeDelta !== 0) return timeDelta;
    return a.sequence_no - b.sequence_no;
  });
  return {
    stream: { ...stream, transcript_messages: messages },
    childResults,
  };
}
