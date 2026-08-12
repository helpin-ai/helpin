import { describe, expect, it } from 'vitest';

import { buildUsageChart, selectUsageDateTickIndexes } from '../UsageDetail';
import type { UsageResponse } from '@/lib/billingTypes';

const usage: UsageResponse = {
  period: '2026-06-21..2026-07-21',
  period_start: '2026-06-21T00:00:00Z',
  period_end: '2026-07-21T00:00:00Z',
  mode: 'daily',
  included_credits: 5_000,
  credits_used: 30,
  features: [
    { feature_key: 'coding_run', label: 'Coding runs', cost: 10, usage: 3, credits: 30, pct: 100 },
  ],
  series: [
    { date: '2026-06-21', features: { coding_run: 10 } },
    { date: '2026-06-22', features: { coding_run: 20 } },
  ],
};

describe('buildUsageChart', () => {
  it('builds cumulative points from daily API data', () => {
    const chart = buildUsageChart(usage, 'cumulative');

    expect(chart.points.map((point) => point.total)).toEqual([10, 30]);
  });

  it('does not double-count data that is already cumulative from the API', () => {
    const chart = buildUsageChart({
      ...usage,
      mode: 'cumulative',
      series: [
        { date: '2026-06-21', features: { coding_run: 10 } },
        { date: '2026-06-22', features: { coding_run: 30 } },
      ],
    }, 'cumulative');

    expect(chart.points.map((point) => point.total)).toEqual([10, 30]);
  });
});

describe('selectUsageDateTickIndexes', () => {
  it('shows every date when the series is sparse', () => {
    expect(selectUsageDateTickIndexes(4)).toEqual([0, 1, 2, 3]);
  });

  it('keeps the first and last dates with evenly spaced intermediate ticks', () => {
    expect(selectUsageDateTickIndexes(31, 6)).toEqual([0, 6, 12, 18, 24, 30]);
  });
});
