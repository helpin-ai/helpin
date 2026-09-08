// @vitest-environment jsdom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { EpicWithStats, Task } from '@/lib/pmTypes';
import { InlineEpicCell } from '../InlineEpicCell';

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

const epics = [
  { epic: { id: 'epic-a', name: 'Shared initiative', color: '#e2564a' } },
  { epic: { id: 'epic-b', name: 'Shared initiative', color: '#4e8fea' } },
  { epic: { id: 'epic-c', name: 'Other initiative' } },
] as EpicWithStats[];
const epicMap = new Map(epics.map(({ epic }) => [epic.id, epic]));
let root: Root;

beforeEach(() => {
  vi.stubGlobal('ResizeObserver', class { observe() {} unobserve() {} disconnect() {} });
  HTMLElement.prototype.scrollIntoView = vi.fn();
});

afterEach(() => {
  act(() => root.unmount());
  document.body.innerHTML = '';
  vi.unstubAllGlobals();
});

async function openPicker(epicId = 'epic-a') {
  const onUpdate = vi.fn().mockResolvedValue(undefined);
  const onOpenTask = vi.fn();
  const container = document.createElement('div');
  document.body.append(container);
  root = createRoot(container);
  await act(async () => root.render(
    <div onClick={onOpenTask}>
      <InlineEpicCell task={{ id: 'task-1', epic_id: epicId } as Task} epics={epics} epicMap={epicMap} onUpdate={onUpdate} />
    </div>,
  ));
  await act(async () => container.querySelector('button')!.click());
  return { onUpdate, onOpenTask };
}

const option = (id: string) => document.querySelector<HTMLElement>(`[role="option"][data-value="${id}"]`)!;

describe('task list epic dropdown', () => {
  it('shows separate colored badges for same-named epics and slate for an uncolored epic', async () => {
    await openPicker();
    for (const [id, color] of [['epic-a', '#e2564a'], ['epic-b', '#4e8fea'], ['epic-c', '#788596']]) {
      expect(option(id)).not.toBeNull();
      expect(option(id).querySelector<HTMLElement>('[title]')!.style.getPropertyValue('--epic-color')).toBe(color);
    }
  });

  it('highlights only the hovered ID without changing which epic is assigned', async () => {
    await openPicker();
    for (const id of ['epic-b', 'epic-a']) {
      await act(async () => option(id).dispatchEvent(new MouseEvent('pointermove', { bubbles: true })));
      const highlighted = document.querySelectorAll('[role="option"][aria-selected="true"]');
      expect(highlighted).toHaveLength(1);
      expect(highlighted[0].getAttribute('data-value')).toBe(id);
      expect(option('epic-a').getAttribute('data-checked')).toBe('true');
      expect(option('epic-b').getAttribute('data-checked')).toBe('false');
    }
  });

  it('searches by name and assigns the chosen duplicate by ID without opening the task', async () => {
    const { onUpdate, onOpenTask } = await openPicker();
    const input = document.querySelector<HTMLInputElement>('input')!;
    await act(async () => {
      Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')!.set!.call(input, 'Shared initiative');
      input.dispatchEvent(new Event('input', { bubbles: true }));
    });
    expect(document.querySelectorAll('[role="option"]')).toHaveLength(2);
    expect(option('epic-c')).toBeNull();
    await act(async () => option('epic-b').click());
    expect(onUpdate).toHaveBeenCalledTimes(1);
    expect(onUpdate).toHaveBeenCalledWith('task-1', { epic_id: 'epic-b' });
    expect(onOpenTask).not.toHaveBeenCalled();
  });

  it('supports keyboard selection of the second same-named epic', async () => {
    const { onUpdate } = await openPicker();
    const input = document.querySelector<HTMLInputElement>('input')!;
    await act(async () => input.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowDown', bubbles: true })));
    await act(async () => input.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true })));
    expect(onUpdate).toHaveBeenCalledWith('task-1', { epic_id: 'epic-b' });
  });

  it('clears the assignment when the currently assigned epic is selected', async () => {
    const { onUpdate } = await openPicker();
    await act(async () => option('epic-a').click());
    expect(onUpdate).toHaveBeenCalledWith('task-1', { epic_id: undefined });
  });
});
