import { describe, expect, it } from 'vitest';

import {
  codingSessionStageLabel,
  codingSessionStatusLabel,
  countVisibleTranscriptTurns,
  formatCodingSessionElapsed,
  isPromptTranscriptMessage,
  isStatusTranscriptMessage,
} from '../codingSessionPresentation';
import type { CodingSessionTranscriptMessage } from '@/lib/pmTypes';

function message(overrides: Partial<CodingSessionTranscriptMessage>): CodingSessionTranscriptMessage {
  return {
    event_id: overrides.event_id ?? 'event-1',
    role: overrides.role ?? 'assistant',
    content: overrides.content ?? 'Visible message',
    timestamp: overrides.timestamp ?? '2026-05-07T06:00:00Z',
    sequence_no: overrides.sequence_no ?? 1,
    message_type: overrides.message_type,
  };
}

describe('coding session presentation helpers', () => {
  it('maps backend execution stages to user-facing labels', () => {
    expect(codingSessionStageLabel('starting')).toBe('Starting runtime');
    expect(codingSessionStageLabel('codex_running')).toBe('Agent working');
    expect(codingSessionStageLabel('preparing')).toBe('Preparing workspace');
    expect(codingSessionStageLabel('unknown_stage')).toBe('Agent working');
  });

  it('layers status and execution stage into a stable status label', () => {
    expect(codingSessionStatusLabel({ status: 'queued', pauseReason: 'none' })).toBe('Queued');
    expect(codingSessionStatusLabel({ status: 'running', pauseReason: 'none', executionStage: 'codex_running' })).toBe('Agent working');
    expect(codingSessionStatusLabel({ status: 'paused', pauseReason: 'human_approval' })).toBe('Awaiting approval');
    expect(codingSessionStatusLabel({ status: 'completed', pauseReason: 'none' })).toBe('Completed');
  });

  it('keeps prompt and status messages out of visible turn counts', () => {
    const messages = [
      message({ message_type: 'developer_prompt', content: 'Internal prompt' }),
      message({ message_type: 'status', content: 'Preparing workspace and loading run context.' }),
      message({ role: 'user', content: 'Build this' }),
      message({ role: 'assistant', content: 'I will inspect the repo.' }),
      message({ role: 'assistant', content: '   ' }),
    ];

    expect(countVisibleTranscriptTurns(messages)).toBe(2);
  });

  it('identifies prompt and status transcript rows', () => {
    expect(isPromptTranscriptMessage(message({ message_type: 'system_prompt' }))).toBe(true);
    expect(isPromptTranscriptMessage(message({ message_type: 'developer_prompt' }))).toBe(true);
    expect(isStatusTranscriptMessage(message({ message_type: 'status' }))).toBe(true);
    expect(isStatusTranscriptMessage(message({ message_type: 'assistant' }))).toBe(false);
  });

  it('formats active elapsed time compactly', () => {
    expect(formatCodingSessionElapsed(9_000)).toBe('9s');
    expect(formatCodingSessionElapsed(67_000)).toBe('1m 07s');
    expect(formatCodingSessionElapsed(3_671_000)).toBe('1h 1m');
    expect(formatCodingSessionElapsed(183_660_000)).toBe('2d 3h');
  });
});
