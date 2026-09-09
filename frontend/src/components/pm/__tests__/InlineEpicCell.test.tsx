// @vitest-environment jsdom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { EpicWithStats, Task } from '@/lib/pmTypes';
import { InlineEpicCell } from '../InlineEpicCell';

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

const epics = [
  { epic: { id: 'epic-a', name: 'Shared initiative', color: '#e2564a' } },
  { epic: { id: 'epic-b', name: 'Shared initiative', color: '#4e8fea', started: true } },
  { epic: { id: 'epic-c', name: 'Other initiative', started: true, completed: true } },
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

async function openPicker(epicId = 'epic-a', options = epicMap) {
  const onUpdate = vi.fn().mockResolvedValue(undefined);
  const onOpenTask = vi.fn();
  const container = document.createElement('div');
  document.body.append(container);
  root = createRoot(container);
  await act(async () => root.render(
    <div onClick={onOpenTask}>
      <InlineEpicCell task={{ id: 'task-1', epic_id: epicId } as Task} epicMap={options} onUpdate={onUpdate} />
    </div>,
  ));
  await act(async () => container.querySelector('button')!.click());
  return { onUpdate, onOpenTask };
}

const option = (id: string) => document.querySelector<HTMLElement>(`[role="option"][data-value="${id}"]`)!;

describe('task board and list epic dropdown', () => {
  it('groups epics by lifecycle in the same order as the detail rail', async () => {
    await openPicker('epic-a', new Map([...epicMap].reverse()));
    const groups = Array.from(document.querySelectorAll('[cmdk-group]')).slice(1);
    expect(groups.map((group) => group.querySelector('[cmdk-group-heading]')?.textContent))
      .toEqual(['Not started', 'In progress', 'Completed']);
    expect(groups.map((group) => group.querySelector('[role="option"]')?.getAttribute('data-value')))
      .toEqual(['epic-a', 'epic-b', 'epic-c']);
  });

  it('omits empty lifecycle groups and offers an explicit None option', async () => {
    const { onUpdate } = await openPicker('epic-a', new Map([['epic-a', epicMap.get('epic-a')!]]));
    expect(Array.from(document.querySelectorAll('[cmdk-group-heading]')).map((heading) => heading.textContent))
      .toEqual(['Not started']);
    await act(async () => option('__none__').click());
    expect(onUpdate).toHaveBeenCalledWith('task-1', { epic_id: '' });
  });

  it('shows color squares beside same-named epics and slate for an uncolored epic', async () => {
    await openPicker();
    for (const [id, color] of [['epic-a', 'rgb(226, 86, 74)'], ['epic-b', 'rgb(78, 143, 234)'], ['epic-c', 'rgb(193, 201, 211)']]) {
      expect(option(id)).not.toBeNull();
      expect(option(id).querySelector<HTMLElement>('[aria-hidden="true"][style]')!.style.backgroundColor).toBe(color);
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
    expect(onUpdate).toHaveBeenCalledWith('task-1', { epic_id: '' });
  });
});
