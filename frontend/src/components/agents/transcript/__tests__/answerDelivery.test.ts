import { describe, expect, it } from 'vitest';

import { buildCodingSessionStreamState } from '@/components/pm/CodingSession/codingSessionStream';
import type { CodingSessionEvent } from '@/lib/pmTypes';
import { collectSegments, DOCK_SEGMENT_KINDS } from '../segments';

const answer = 'The editor supports **Mermaid**, `nwdiag`, and Excalidraw.\n\n| Format | Location |\n| --- | --- |\n| Mermaid | Diagram block |';

function event(id: string, content: string, sequence: number, persisted = false, messageType = 'assistant_turn'): CodingSessionEvent {
  return {
    id: `${persisted ? 'stored' : 'live'}:${id}`,
    session_id: 'session-1', run_id: 'run-1', sequence_no: sequence,
    timestamp: `2026-09-10T12:00:0${sequence}Z`,
    type: 'assistant.message.completed', runtime_kind: 'native_sdk',
    runtime_metadata: persisted ? { source: 'agent_run_message' } : undefined,
    payload: { message_id: id, content, message_type: messageType, ...(persisted ? { role: 'assistant', sequence_no: sequence } : {}) },
  };
}

const preamble = event('preamble', "All the evidence is in. Here's the complete picture:", 1);
const finish: CodingSessionEvent = {
  id: 'finish', session_id: 'session-1', run_id: 'run-1', sequence_no: 2,
  timestamp: '2026-09-10T12:00:02Z', type: 'tool.call.completed', runtime_kind: 'native_sdk',
  payload: { tool_call_id: 'finish-1', tool_name: 'finish_turn', args_text: JSON.stringify({ outcome: 'completed', summary: answer }) },
};
const final = event('canonical-answer', answer, 3);

function visibleAnswers(events: CodingSessionEvent[], includeLive: boolean) {
  const state = buildCodingSessionStreamState(events);
  return collectSegments(state, {
    includeLive, runtimeActive: includeLive, include: DOCK_SEGMENT_KINDS, compactAssistantProgress: true,
  });
}

describe('canonical answer delivery', () => {
  it('keeps the live final answer when later progress arrives before persistence', () => {
    const segments = visibleAnswers([
      event('canonical-answer', answer, 3, false, 'assistant_final'),
      event('late-progress', 'Here is the answer:', 4),
    ], true);
    expect(segments).toHaveLength(1);
    expect(segments[0]).toMatchObject({ kind: 'assistant', content: answer, final: true });
  });

  it('retains the canonical answer when progress is backfilled after it', () => {
    const segments = visibleAnswers([
      event('canonical-answer', answer, 3, true, 'assistant_final'),
      event('late-progress', 'Here is the answer:', 4, true),
    ], false);
    expect(segments).toHaveLength(1);
    expect(segments[0]).toMatchObject({ kind: 'assistant', content: answer, final: true });
  });

  it('shows the full live answer while keeping the finish control tool hidden', () => {
    const segments = visibleAnswers([preamble, finish, final], true);
    expect(segments).toHaveLength(1);
    expect(segments[0]).toMatchObject({ kind: 'assistant', content: answer });
  });

  it('shows the same answer after pause and reload from persisted messages', () => {
    const segments = visibleAnswers([
      event('preamble', "Here's the answer again, with that word removed:", 1, true),
      event('canonical-answer', answer, 3, true, 'assistant_final'),
    ], false);
    expect(segments).toHaveLength(1);
    expect(segments[0]).toMatchObject({ kind: 'assistant', content: answer });
  });

  it('reconciles replayed live and stored answers by identity without duplication', () => {
    const segments = visibleAnswers([
      preamble, finish, final, { ...final, id: 'replayed-answer', sequence_no: 4 },
      event('preamble', "All the evidence is in. Here's the complete picture:", 1, true),
      event('canonical-answer', answer, 3, true, 'assistant_final'),
    ], true);
    expect(segments).toHaveLength(1);
    expect(segments[0]).toMatchObject({ kind: 'assistant', content: answer });
  });

  it('keeps the first answer when a subsequent user turn receives its own answer', () => {
    const followup = event('user-followup', 'What about Excalidraw?', 4, true);
    followup.type = 'user.message.completed';
    followup.payload = { message_id: 'user-followup', role: 'user', content: 'What about Excalidraw?', sequence_no: 4 };
    const segments = visibleAnswers([
      event('preamble', 'Here is the answer:', 1, true),
      event('canonical-answer', answer, 3, true, 'assistant_final'), followup,
      event('followup-preamble', 'Here is the follow-up:', 5, true),
      event('followup-answer', 'Excalidraw is available in the editor.', 6, true, 'assistant_final'),
    ], false);
    expect(segments.map((segment) => segment.kind)).toEqual(['assistant', 'user', 'assistant']);
    expect(segments[0]).toMatchObject({ content: answer });
    expect(segments[2]).toMatchObject({ content: 'Excalidraw is available in the editor.' });
  });
});
