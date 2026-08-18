import { describe, expect, it } from 'vitest';
import { calculateAIUsagePercent, formatAIUsagePercent } from '../aiUsage';

describe('formatAIUsagePercent', () => {
  it('formats ordinary, tiny, and over-budget percentages', () => {
    expect(formatAIUsagePercent(72)).toBe('72%');
    expect(formatAIUsagePercent(0.004)).toBe('<0.01%');
    expect(formatAIUsagePercent(112.4)).toBe('112.4%');
  });
});

describe('calculateAIUsagePercent', () => {
  it('preserves fractional usage instead of rounding it away', () => {
    expect(calculateAIUsagePercent(244_315, 150_000_000)).toBeCloseTo(0.1628767, 6);
    expect(formatAIUsagePercent(calculateAIUsagePercent(244_315, 150_000_000))).toBe('0.2%');
  });
});
