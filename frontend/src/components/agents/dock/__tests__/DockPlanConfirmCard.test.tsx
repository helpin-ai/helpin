// @vitest-environment jsdom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { DockPlanConfirmCard } from '../DockPlanConfirmCard';

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

let container: HTMLDivElement;
let root: Root;

beforeEach(() => {
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
});

afterEach(() => {
  act(() => root.unmount());
  container.remove();
});

describe('DockPlanConfirmCard', () => {
  it('renders a sub-agent launch as a divider-based approval without nested cards', () => {
    act(() => {
      root.render(
        <DockPlanConfirmCard
          payload={{
            title: 'Launch a sub-agent',
            summary: 'Launch a sub-agent to review the implementation.',
            action: {
              steps: [
                {
                  use_command_agent: true,
                  target: { type: 'repository', id: 'repo-123' },
                  instructions: 'Review the implementation and report concrete issues.',
                },
                {
                  agent_id: 'lens-agent',
                  instructions: 'Verify the first review independently.',
                  depends_on_step_indexes: [0],
                },
              ],
            },
          }}
          onDecision={vi.fn().mockResolvedValue({ error: null })}
        />,
      );
    });

    const shell = container.querySelector<HTMLElement>('[data-agent-dock-plan-confirm]');
    expect(shell?.className).toContain('border-y');
    expect(shell?.className).not.toContain('rounded-lg');
    expect(shell?.className).not.toContain('bg-amber-50/60');
    expect(container.textContent).toContain('Needs your approval');
    expect(container.textContent).toContain('Launch a sub-agent to review the implementation.');

    const stepList = container.querySelector('[data-agent-dock-plan-steps]');
    expect(stepList?.className).toContain('divide-y');
    const stepRows = Array.from(stepList?.querySelectorAll(':scope > li') ?? []);
    expect(stepRows).toHaveLength(2);
    for (const row of stepRows) {
      expect(row.className).not.toContain('rounded-md');
      expect(row.className).not.toContain('bg-background/80');
      expect(row.className).not.toContain('border-border/60');
    }
  });
});
