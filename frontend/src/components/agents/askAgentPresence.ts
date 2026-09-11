import type { AskAgentAvatarState } from '@/components/agents/AskAgentAvatar';
import type { CodingSessionStreamState } from '@/lib/pmTypes';
import { resolveVisibleTurn } from './dock/agentTurnState';

interface AskAgentRunLike {
  status?: string | null;
}

interface AskAgentPresenceInput {
  run?: AskAgentRunLike | null;
  stream?: Pick<CodingSessionStreamState, 'live_turn_segments'> & Partial<Pick<CodingSessionStreamState, 'transcript_messages' | 'turn_state'>> | null;
  sending?: boolean;
  error?: boolean | string | null;
}

export function isAskAgentSpeaking(
  stream?: Pick<CodingSessionStreamState, 'live_turn_segments'> | null,
): boolean {
  if (!stream) return false;
  const latest = stream.live_turn_segments[stream.live_turn_segments.length - 1];
  return latest?.kind === 'assistant_message'
    && latest.assistant_message.status === 'streaming'
    && latest.assistant_message.content.trim().length > 0;
}

/** Maps the runtime's observable state to the mascot's intentionally small state vocabulary. */
export function deriveAskAgentAvatarState({
  run,
  stream,
  sending = false,
  error = false,
}: AskAgentPresenceInput): AskAgentAvatarState {
  if (error || run?.status === 'failed') return 'error';
  if (!sending && stream && resolveVisibleTurn({ ...stream, transcript_messages: stream.transcript_messages ?? [] }).answered) return 'idle';
  if (isAskAgentSpeaking(stream)) return 'speaking';
  if (sending || run?.status === 'queued' || run?.status === 'running') return 'thinking';
  return 'idle';
}
