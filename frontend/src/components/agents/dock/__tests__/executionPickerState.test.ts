import { describe, expect, it } from 'vitest';

import { resolveExecutionPickerDisabled } from '../executionPickerState';

const idle = { changingExecution: false, sending: false, pendingEcho: false, runStatus: 'completed' };

describe('execution picker availability', () => {
  it('locks the picker while a message is being sent so the user cannot cancel the run it starts', () => {
    expect(resolveExecutionPickerDisabled({ ...idle, sending: true })).toBe(true);
    expect(resolveExecutionPickerDisabled({ ...idle, pendingEcho: true })).toBe(true);
    expect(resolveExecutionPickerDisabled({ ...idle, changingExecution: true })).toBe(true);
  });

  it('locks the picker in both directions while a run is queued or running', () => {
    for (const runStatus of ['queued', 'pending', 'running']) {
      expect(resolveExecutionPickerDisabled({ ...idle, runStatus })).toBe(true);
    }
  });

  it('allows changes only when the chat is idle or paused', () => {
    for (const runStatus of [undefined, null, 'completed', 'failed', 'cancelled', 'paused']) {
      expect(resolveExecutionPickerDisabled({ ...idle, runStatus })).toBe(false);
    }
  });
});
