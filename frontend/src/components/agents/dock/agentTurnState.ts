import type { CodingSessionStreamState } from '@/lib/pmTypes';
import { findLastMatchingIndex } from './findLastMatchingIndex';
import type { TranscriptSegment } from '../transcript/segments';

// Shared by row presentation and working groups. Legacy inference is restricted
// to settled intervals that contain no explicit progress/final markers.
export function finalAssistantIndex(segments: TranscriptSegment[], allowLegacy: boolean): number {
  for (let i = segments.length - 1; i >= 0; i--) {
    const segment = segments[i];
    if (segment.kind === 'assistant' && segment.final && !segment.streaming) return i;
  }
  if (!allowLegacy || segments.some((s) => s.kind === 'assistant' && (s.progress || s.final))) return -1;
  for (let i = segments.length - 1; i >= 0; i--) {
    const segment = segments[i];
    if (segment.kind === 'assistant' && !segment.streaming) return i;
  }
  return -1;
}

export function resolveVisibleTurn(stream: Pick<CodingSessionStreamState, 'transcript_messages' | 'live_turn_segments' | 'turn_state'> | null, localStartedAt?: string) {
  const messages = stream?.transcript_messages ?? [];
  const boundary = findLastMatchingIndex(messages, (m) => m.role === 'user'
    || m.message_type === 'review_checkpoint_resolution' || m.message_type === 'approval_request_resolution');
  const userAt = boundary >= 0 ? messages[boundary].timestamp : undefined;
  const boundaryAt = Math.max(Date.parse(userAt ?? '') || 0, Date.parse(localStartedAt ?? '') || 0);
  const state = stream?.turn_state;
  const stateCurrent = state && boundaryAt <= (Date.parse(state.completed_at ?? state.started_at) || 0);
  const answerBoundary = Math.max(boundaryAt, stateCurrent ? Date.parse(state.started_at) || 0 : 0);
  const answer = [...messages.slice(boundary + 1)].reverse().find((m) => m.message_type === 'assistant_final' && m.content.trim()
    && (Date.parse(m.timestamp) || 0) >= answerBoundary);
  const liveAnswer = [...(stream?.live_turn_segments ?? [])].reverse().find((s) => s.kind === 'assistant_message'
    && s.assistant_message.message_type === 'assistant_final'
    && s.assistant_message.status === 'completed' && s.assistant_message.content.trim()
    && (Date.parse(s.assistant_message.completed_at ?? s.assistant_message.started_at ?? '') || 0) >= answerBoundary);
  const answered = !!answer || !!liveAnswer;
  return {
    answered,
    explicit: !!state || !!answer || messages.slice(boundary + 1).some((m) => m.message_type === 'assistant_progress'),
    answerPending: !!(stateCurrent && state.phase === 'answered' && !answered),
    missingAnswer: !!(stateCurrent && state.phase === 'missing_answer' && !answered),
    startedAt: stateCurrent ? state.started_at : localStartedAt || userAt,
    completedAt: stateCurrent ? state.completed_at : answer?.timestamp,
  };
}
