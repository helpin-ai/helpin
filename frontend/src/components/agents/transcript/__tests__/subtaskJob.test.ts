import { describe, expect, it } from 'vitest';
import type { CodingSessionLiveToolCall } from '@/lib/pmTypes';
import type { TranscriptSegment } from '../segments';
import { isSubtaskJobFinished, subtaskJobIdFromToolCall, subtaskJobIdsFromSegments, subtaskJobProgress } from '../subtaskJob';

const counts = { total: 5, pending: 1, running: 1, retry_wait: 1, blocked: 0, completed: 1, failed: 1, cancelled: 0 };

describe('subtaskJob', () => {
  it('reads the job id from a spawn result and ignores calls without one', () => {
    const call = (summary?: string) =>
      ({ tool_call_id: 't1', tool_name: 'spawn_subtasks', args_text: '{}', status: 'completed', result: summary ? { content: '', output_summary: summary } : undefined }) as CodingSessionLiveToolCall;
    expect(subtaskJobIdFromToolCall(call('{"items":5,"job_id":"workjob_2c2082d3","state":"queued"}'))).toBe('workjob_2c2082d3');
    expect(subtaskJobIdFromToolCall(call('{"error":"subtasks are not available for this app"}'))).toBeNull();
    expect(subtaskJobIdFromToolCall(call())).toBeNull();
  });

  it('collects job ids only from spawn calls, once each', () => {
    const tool = (id: string, name: string, summary: string) =>
      ({ kind: 'tool', id, toolCall: { tool_call_id: id, tool_name: name, args_text: '{}', status: 'completed', result: { content: '', output_summary: summary } } }) as TranscriptSegment;
    const segments = [
      tool('a', 'mcp__helpin__spawn_subtasks', '{"job_id":"workjob_aa"}'),
      tool('b', 'wait_for_subtasks', '{"job_id":"workjob_aa","finished":true}'),
      tool('c', 'spawn_subtasks', '{"job_id":"workjob_aa"}'),
      tool('d', 'spawn_subtasks', '{"job_id":"workjob_bb"}'),
    ];
    expect(subtaskJobIdsFromSegments(segments)).toEqual(['workjob_aa', 'workjob_bb']);
  });

  it('counts failed items as settled and leaves zero counts out of the detail', () => {
    expect(subtaskJobProgress({ counts })).toEqual({ settled: 2, total: 5, percent: 40, detail: ['1 running', '2 waiting', '1 failed'] });
    expect(subtaskJobProgress({ counts: { ...counts, total: 0, pending: 0, running: 0, retry_wait: 0, completed: 0, failed: 0 } }).percent).toBe(0);
  });

  it('treats only terminal states as finished', () => {
    expect(isSubtaskJobFinished({ state: 'completed' })).toBe(true);
    expect(isSubtaskJobFinished({ state: 'running' })).toBe(false);
  });
});
