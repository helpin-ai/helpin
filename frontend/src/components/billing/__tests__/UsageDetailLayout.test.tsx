// @vitest-environment jsdom
import { act } from 'react';
import { createRoot } from 'react-dom/client';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { UsageDetail } from '../UsageDetail';

vi.mock('@/hooks/queries', () => ({
  useWorkspaceUsage: () => ({
    data: {
      period: '2026-08',
      period_start: '2026-08-01T00:00:00Z',
      period_end: '2026-09-01T00:00:00Z',
      mode: 'daily',
      included_credits: 5_000,
      credits_used: 10,
      features: [
        { feature_key: 'coding_run', label: 'Coding runs', cost: 10, usage: 1, credits: 10, pct: 100 },
      ],
      series: [
        { date: '2026-08-11', features: { coding_run: 10 } },
        { date: '2026-08-12', features: { coding_run: 0 } },
      ],
    },
    isLoading: false,
    isError: false,
  }),
}));

globalThis.IS_REACT_ACT_ENVIRONMENT = true;

describe('UsageDetail chart layout', () => {
  afterEach(() => {
    document.body.innerHTML = '';
  });

  it('gives each percentage-height bar a full-height containing block', () => {
    const container = document.createElement('div');
    document.body.appendChild(container);
    const root = createRoot(container);

    act(() => root.render(<UsageDetail workspaceId="workspace-1" />));

    const barColumn = container.querySelector('[title*="usage units"]');
    expect(barColumn?.classList.contains('h-full')).toBe(true);

    act(() => root.unmount());
    container.remove();
  });

  it('renders date ticks and caps each bar column at a reasonable width', () => {
    const container = document.createElement('div');
    document.body.appendChild(container);
    const root = createRoot(container);

    act(() => root.render(<UsageDetail workspaceId="workspace-1" />));

    expect(container.textContent).toContain('Aug 11');
    expect(container.textContent).toContain('Aug 12');
    const chartTrack = container.querySelector('[data-usage-chart-track]');
    expect(chartTrack?.getAttribute('style')).toContain('minmax(8px, 32px)');

    act(() => root.unmount());
    container.remove();
  });
});
