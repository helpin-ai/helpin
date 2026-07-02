import { describe, expect, it } from 'vitest';

import { DEFAULT_AI_HANDOFF_FOLLOWUPS, formatAIHandoffFollowupOption } from '../handoffFollowups';

describe('formatAIHandoffFollowupOption', () => {
  it('uses AI follow-up terminology for handoff count options', () => {
    expect(formatAIHandoffFollowupOption(1)).toBe('1 AI follow-up');
    expect(formatAIHandoffFollowupOption(3)).toBe('3 AI follow-ups');
  });

  it('defaults to five AI follow-ups before handoff', () => {
    expect(DEFAULT_AI_HANDOFF_FOLLOWUPS).toBe(5);
  });
});
