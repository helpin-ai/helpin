// @vitest-environment jsdom

import React, { act } from 'react';
import { createRoot } from 'react-dom/client';
import { describe, expect, it, vi } from 'vitest';
import { SetupRailNav } from '../SetupRailNav';
import type { SetupJourney } from '@/lib/setupTypes';

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

const journeys: SetupJourney[] = [
  { key: 'foundation', title: 'Workspace essentials', description: '', accent: 'indigo', scope: 'foundation', maturity: 'ready', completed_count: 1, total_count: 3, tasks: [] },
  { key: 'customer_support', title: 'Scale customer support', description: '', accent: 'emerald', scope: 'active', maturity: 'ready', completed_count: 2, total_count: 6, tasks: [] },
];

describe('SetupRailNav', () => {
  it('shows journey progress and selects a journey', () => {
    const container = document.createElement('div');
    const root = createRoot(container);
    const onSelect = vi.fn();

    act(() => root.render(<SetupRailNav journeys={journeys} activeJourneyKey="foundation" onSelect={onSelect} />));

    expect(container.textContent).toContain('Workspace essentials');
    expect(container.textContent).toContain('1/3');
    expect(container.querySelector('[aria-current="location"]')?.textContent).toContain('Workspace essentials');

    const support = Array.from(container.querySelectorAll('button')).find((button) => button.textContent?.includes('Scale customer support'));
    act(() => support?.click());
    expect(onSelect).toHaveBeenCalledWith('customer_support');

    act(() => root.unmount());
  });
});
