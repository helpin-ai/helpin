import { describe, expect, it } from 'vitest';
import { formatAIUsagePercent } from '../aiUsage';

describe('formatAIUsagePercent', () => {
  it('formats ordinary, tiny, and over-budget percentages', () => {
    expect(formatAIUsagePercent(72)).toBe('72%');
    expect(formatAIUsagePercent(0.004)).toBe('<0.01%');
    expect(formatAIUsagePercent(112.4)).toBe('112.4%');
  });
});
