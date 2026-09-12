import { describe, expect, it } from 'vitest';
import { buildCodingSessionStreamState, mergeCodingSessionStreamSnapshotSeed } from '@/components/pm/CodingSession/codingSessionStream';
import type { CodingSessionStreamSnapshot, CodingSessionStreamState } from '@/lib/pmTypes';
import { resolveVisibleTurn } from '../agentTurnState';

const started = '2026-09-11T12:00:00Z';
const completed = '2026-09-11T12:00:20Z';
const body = '# Full answer\n\n' + 'Mermaid, nwdiag, Excalidraw.\n'.repeat(500);
const snapshot: CodingSessionStreamSnapshot = {
  through_sequence: 20,
  turn_state: { turn_id: 'one', phase: 'answered', started_at: started, completed_at: completed, answer_message_id: 'answer' },
  live_turn_segments: [{ kind: 'assistant_message', segment_id: 'answer', assistant_message: { message_id: 'answer', message_type: 'assistant_final', content: body, status: 'completed', started_at: completed, completed_at: completed, tool_calls: [] } }],
};
const empty: CodingSessionStreamState = { transcript_messages: [], live_turn_segments: [], live_assistant_message: null, live_reasoning_message: null, activity_events: [], completed_tool_calls: [], current_plan: null };

describe('turn recovery and visibility', () => {
  it('recovers the exact long answer from a snapshot without websocket events', () => {
    const state = buildCodingSessionStreamState([], snapshot);
    expect(state.live_turn_segments[0]).toMatchObject({ assistant_message: { content: body, message_type: 'assistant_final' } });
    expect(resolveVisibleTurn(state).answered).toBe(true);
  });
  it('does not treat answer metadata alone as visible delivery', () => {
    expect(resolveVisibleTurn({ ...empty, turn_state: snapshot.turn_state })).toMatchObject({ answered: false, answerPending: true });
  });
  it('does not lose accepted delivery to stale, empty, or regressive snapshots', () => {
    for (const incoming of [null, { through_sequence: 19 }, { through_sequence: 21, turn_state: { ...snapshot.turn_state!, phase: 'working' as const } }]) {
      const next = mergeCodingSessionStreamSnapshotSeed(snapshot, incoming);
      expect(next?.turn_state?.phase).toBe('answered');
      expect(resolveVisibleTurn(buildCodingSessionStreamState([], next)).answered).toBe(true);
    }
  });
  it('replaces completed state on a newer accepted resume', () => {
    const next = mergeCodingSessionStreamSnapshotSeed(snapshot, { through_sequence: 21, turn_state: { turn_id: 'two', phase: 'working', started_at: '2026-09-11T13:00:00Z' } });
    expect(resolveVisibleTurn(buildCodingSessionStreamState([], next))).toMatchObject({ answered: false, startedAt: '2026-09-11T13:00:00Z' });
  });
  it('does not let saved previous answers stop an optimistic follow-up before its echo arrives', () => {
    const state = { ...empty, transcript_messages: [{ event_id: 'answer', role: 'assistant' as const, message_type: 'assistant_final', content: body, timestamp: completed, sequence_no: 1 }] };
    expect(resolveVisibleTurn(state, '2026-09-11T13:00:00Z').answered).toBe(false);
  });
  it('recovers a dropped final event from durable history after terminal state arrives', () => {
    const state = { ...empty, turn_state: { ...snapshot.turn_state!, phase: 'missing_answer' as const }, transcript_messages: [{ event_id: 'answer', role: 'assistant' as const, message_type: 'assistant_final', content: body, timestamp: completed, sequence_no: 1 }] };
    expect(resolveVisibleTurn(state)).toMatchObject({ answered: true, missingAnswer: false });
  });
});
