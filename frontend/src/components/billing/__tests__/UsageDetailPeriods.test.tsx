// @vitest-environment jsdom
import { act } from 'react';
import { createRoot } from 'react-dom/client';
import { afterEach, expect, it, vi } from 'vitest';
import { UsageDetail } from '../UsageDetail';

const { useWorkspaceUsage } = vi.hoisted(() => ({ useWorkspaceUsage: vi.fn() }));
vi.mock('@/hooks/queries', () => ({ useWorkspaceUsage }));
vi.mock('@/components/ui/select', () => ({
  Select: ({ children, value, onValueChange }: { children: React.ReactNode; value: string; onValueChange: (value: string) => void }) =>
    <select aria-label="AI usage period" value={value} onChange={(event) => onValueChange(event.target.value)}>{children}</select>,
  SelectContent: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  SelectItem: ({ children, value }: { children: React.ReactNode; value: string }) => <option value={value}>{children}</option>,
  SelectTrigger: () => null,
  SelectValue: () => null,
}));
globalThis.IS_REACT_ACT_ENVIRONMENT = true;
afterEach(() => { document.body.innerHTML = ''; vi.clearAllMocks(); });

it('requests the historical allowance window and resets selection across workspaces', () => {
  const start = '2026-09-01T00:00:00Z';
  const end = '2026-10-01T00:00:00Z';
  const oldStart = '2026-08-01T00:00:00Z';
  useWorkspaceUsage.mockReturnValue({ data: {
    period_start: start, period_end: end, mode: 'daily',
    ai_usage_allowance_microusd: 1000, series: [], features: [],
    periods: [
      { id: 'new', start, end, status: 'open' },
      { id: 'old', start: oldStart, end: start, status: 'closed' },
    ],
  }, isLoading: false, isError: false });
  const container = document.createElement('div'); document.body.appendChild(container);
  const root = createRoot(container);
  act(() => root.render(<UsageDetail workspaceId="ws" periodStart={start} periodEnd={end} />));
  const select = container.querySelector('select')!;
  act(() => { select.value = 'old'; select.dispatchEvent(new Event('change', { bubbles: true })); });
  expect(useWorkspaceUsage).toHaveBeenLastCalledWith('ws', `${oldStart}..${start}`, 'daily', oldStart, start);
  act(() => root.render(<UsageDetail workspaceId="other" periodStart={start} periodEnd={end} />));
  expect(useWorkspaceUsage).toHaveBeenLastCalledWith('other', `${start}..${end}`, 'daily', start, end);
  act(() => root.unmount());
});
