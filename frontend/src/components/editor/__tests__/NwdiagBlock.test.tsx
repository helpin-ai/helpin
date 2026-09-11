// @vitest-environment jsdom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { NwdiagBlock } from '../NwdiagBlock';

const state = vi.hoisted(() => ({ render: vi.fn(), theme: 'light' }));
vi.mock('@/lib/nwdiagRenderer', () => ({ renderNwdiagSvg: state.render }));
vi.mock('next-themes', () => ({ useTheme: () => ({ resolvedTheme: state.theme }) }));
(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

let root: Root;
afterEach(() => {
  act(() => root?.unmount());
  document.body.innerHTML = '';
  state.render.mockReset();
  state.theme = 'light';
});

function mount(source: string) {
  const container = document.createElement('div');
  document.body.append(container);
  root = createRoot(container);
  act(() => root.render(<NwdiagBlock source={source} />));
  return container;
}

describe('NwdiagBlock', () => {
  it('ignores stale renders and renders the current theme', async () => {
    let finishOld: (svg: string) => void = () => {};
    state.render.mockImplementationOnce(() => new Promise<string>((resolve) => { finishOld = resolve; }));
    state.render.mockResolvedValue('<svg><text>Current dark diagram</text></svg>');
    const container = mount('old source');
    expect(container.querySelector('[data-nwdiag-status="loading"]')).toBeTruthy();
    state.theme = 'dark';
    await act(async () => root.render(<NwdiagBlock source="new source" />));
    await act(async () => finishOld('<svg><text>Stale diagram</text></svg>'));
    expect(state.render).toHaveBeenLastCalledWith('new source', 'dark');
    expect(container.textContent).toContain('Current dark diagram');
    expect(container.textContent).not.toContain('Stale');
  });

  it('shows syntax errors and recovers when source is corrected', async () => {
    state.render.mockRejectedValueOnce(new Error('Line 1: Expected nwdiag'));
    const container = mount('bad source');
    await act(async () => undefined);
    expect(container.querySelector('[role="alert"]')?.textContent).toContain('Line 1');
    state.render.mockResolvedValue('<svg><text>Corrected</text></svg>');
    await act(async () => root.render(<NwdiagBlock source="corrected source" />));
    expect(container.querySelector('[data-nwdiag-status="ready"]')).toBeTruthy();
    expect(container.querySelector('[role="alert"]')).toBeNull();
  });

  it('explains empty source without invoking the renderer', () => {
    const container = mount('  ');
    expect(container.querySelector('[role="alert"]')?.textContent).toContain('empty');
    expect(state.render).not.toHaveBeenCalled();
  });
});
