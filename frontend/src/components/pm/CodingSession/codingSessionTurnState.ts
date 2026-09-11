import type { CodingSessionTurnState } from '@/lib/pmTypes';

export function applyCodingSessionTurnState(
  current: CodingSessionTurnState | undefined,
  type: string,
  payload: Record<string, unknown>,
  timestamp: string,
): { state: CodingSessionTurnState | undefined; accept: boolean } {
  const id = typeof payload.turn_id === 'string' ? payload.turn_id : '';
  if (!id || payload.completion_mode !== 'explicit') return { state: current, accept: true };
  const startedAt = typeof payload.turn_started_at === 'string' ? payload.turn_started_at : timestamp;
  if (current && current.turn_id !== id && Date.parse(startedAt) <= Date.parse(current.started_at)) {
    return { state: current, accept: false };
  }
  const state: CodingSessionTurnState = current?.turn_id === id
    ? { ...current }
    : { turn_id: id, phase: 'working', started_at: startedAt };
  if (state.phase === 'answered') {
    return { state, accept: !type.startsWith('assistant.message.') || payload.message_type === 'assistant_final' };
  }
  if (type === 'assistant.message.completed' && payload.message_type === 'assistant_final'
    && (typeof payload.content === 'string' ? payload.content : typeof payload.text === 'string' ? payload.text : '').trim()) {
    state.phase = 'answered';
    state.answer_message_id = typeof payload.message_id === 'string' ? payload.message_id : undefined;
    state.completed_at = typeof payload.answer_completed_at === 'string' ? payload.answer_completed_at : timestamp;
  } else if (type === 'run.paused') {
    state.phase = payload.pause_reason === 'awaiting_user_message' ? 'missing_answer' : 'waiting';
    state.completed_at = timestamp;
  } else if (type === 'run.completed' || type === 'run.failed' || type === 'run.cancelled') {
    state.phase = type === 'run.completed' ? 'missing_answer' : type === 'run.failed' ? 'failed' : 'cancelled';
    state.completed_at = timestamp;
  }
  return { state, accept: true };
}
