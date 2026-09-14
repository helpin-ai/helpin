import { describe, expect, it } from 'vitest';

import { buildUsageChart, selectUsageDateTickIndexes } from '../UsageDetail';
import type { UsageResponse } from '@/ee/lib/billingTypes';

const usage: UsageResponse = {
  period: '2026-06-21..2026-07-21',
  period_start: '2026-06-21T00:00:00Z',
  period_end: '2026-07-21T00:00:00Z',
  mode: 'daily',
  ai_usage_allowance_microusd: 100,
  ai_usage_used_microusd: 30,
  ai_usage_reserved_microusd: 0,
  ai_usage_overage_microusd: 0,
  features: [
    { feature_key: 'coding_run', label: 'Coding runs', charged_microusd: 30, action_count: 3, pct: 30 },
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

  it('measures chart values against the full allowance, not total consumed usage', () => {
    const chart = buildUsageChart({
      ...usage,
      ai_usage_allowance_microusd: 1_000,
      series: [{ date: '2026-06-21', features: { coding_run: 25 } }],
    }, 'daily');

    expect(chart.points[0]?.total).toBe(2.5);
  });

  it('preserves usage above 100 percent for extra usage and Founder reporting', () => {
    const chart = buildUsageChart({
      ...usage,
      series: [{ date: '2026-06-21', features: { coding_run: 125 } }],
    }, 'daily');

    expect(chart.points[0]?.total).toBe(125);
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
