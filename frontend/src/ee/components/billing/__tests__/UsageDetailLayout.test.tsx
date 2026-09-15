// @vitest-environment jsdom
import { act } from 'react';
import { createRoot } from 'react-dom/client';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { UsageDetail } from '../UsageDetail';

vi.mock('@/ee/hooks/queries/useBilling', () => ({
  useWorkspaceUsage: () => ({
    data: {
      period: '2026-08',
      period_start: '2026-08-01T00:00:00Z',
      period_end: '2026-09-01T00:00:00Z',
      mode: 'daily',
      ai_usage_allowance_microusd: 1_000,
      ai_usage_used_microusd: 10,
      ai_usage_reserved_microusd: 0,
      ai_usage_overage_microusd: 0,
      features: [
        { feature_key: 'coding_run', label: 'Coding runs', model_tiers: ['large', 'flagship'], action_count: 1, charged_microusd: 10, pct: 1 },
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

    const barColumn = container.querySelector('[data-usage-bar-column]');
    expect(barColumn?.classList.contains('h-full')).toBe(true);

    act(() => root.unmount());
    container.remove();
  });

  it('labels activity as a share of the full allowance', () => {
    const container = document.createElement('div');
    document.body.appendChild(container);
    const root = createRoot(container);

    act(() => root.render(<UsageDetail workspaceId="workspace-1" />));

    expect(container.textContent).toContain('Share of allowance');
    expect(container.textContent).toContain('1%');
    expect(container.querySelector('[aria-label*="1% of allowance"]')).not.toBeNull();

    act(() => root.unmount());
    container.remove();
  });

  it('renders square chart bars without rounded segment edges', () => {
    const container = document.createElement('div');
    document.body.appendChild(container);
    const root = createRoot(container);

    act(() => root.render(<UsageDetail workspaceId="workspace-1" />));

    const segments = Array.from(container.querySelectorAll('[data-usage-bar-segment]'));
    expect(segments.length).toBeGreaterThan(0);
    expect(segments.every((segment) => !segment.className.includes('rounded'))).toBe(true);

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

  it('shows every model size used by an activity during the period', () => {
    const container = document.createElement('div');
    document.body.appendChild(container);
    const root = createRoot(container);

    act(() => root.render(<UsageDetail workspaceId="workspace-1" />));

    expect(container.textContent).toContain('large');
    expect(container.textContent).toContain('flagship');

    act(() => root.unmount());
    container.remove();
  });
});
