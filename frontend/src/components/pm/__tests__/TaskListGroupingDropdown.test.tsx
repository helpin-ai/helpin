// @vitest-environment jsdom
import { act, useState } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { BoardToolbarSlot } from '../BoardToolbarSlot';
import { TaskListGroupingDropdown } from '../TaskListGroupingDropdown';

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
let root: Root;
const options = [{ value: 'workflow_state', label: 'State' }, { value: 'priority', label: 'Priority' }, { value: 'none', label: 'None' }];

beforeEach(() => {
  vi.stubGlobal('ResizeObserver', class { observe() {} unobserve() {} disconnect() {} });
  HTMLElement.prototype.scrollIntoView = vi.fn();
});
afterEach(() => { act(() => root?.unmount()); document.body.innerHTML = ''; vi.unstubAllGlobals(); });

async function render(initialOptions = options) {
  const change = vi.fn();
  const container = document.createElement('div');
  document.body.append(container);
  root = createRoot(container);
  function Toolbar() {
    const [value, setValue] = useState('workflow_state');
    return <BoardToolbarSlot><TaskListGroupingDropdown value={value} options={initialOptions}
      onChange={next => { setValue(next); change(next); }} /></BoardToolbarSlot>;
  }
  await act(async () => root.render(<Toolbar />));
  return { container, change };
}

describe('task list grouping toolbar', () => {
  it('keeps Group by visible and updates the grouping through the shared menu', async () => {
    const { container, change } = await render();
    const trigger = () => container.querySelector<HTMLButtonElement>('button')!;
    expect(trigger().getAttribute('aria-label')).toBe('Group by: State');
    await act(async () => trigger().click());
    await act(async () => document.querySelector<HTMLElement>('[cmdk-item][data-value="priority"]')!.click());
    expect(change).toHaveBeenCalledWith('priority');
    expect(trigger().getAttribute('aria-label')).toBe('Group by: Priority');
    expect(trigger().getAttribute('aria-expanded')).toBe('false');
    await act(async () => trigger().click());
    await act(async () => document.querySelector<HTMLElement>('[cmdk-item][data-value="none"]')!.click());
    expect(change).toHaveBeenLastCalledWith('none');
    expect(trigger().getAttribute('aria-label')).toBe('Group by: None');
  });

  it('renders the trigger even while grouping options are unavailable', async () => {
    const { container } = await render([]);
    expect(container.querySelector('button')?.getAttribute('aria-label')).toBe('Group by: workflow_state');
  });
});
