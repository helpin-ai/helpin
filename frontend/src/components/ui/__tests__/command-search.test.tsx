// @vitest-environment jsdom
import { act, type ComponentProps } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { Command, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList } from '../command';

let root: Root;
let container: HTMLDivElement;
let capacity: number;
let constrained: boolean;
let frameId: number;
let frames: Map<number, FrameRequestCallback>;
let resizes: Set<() => void>;
const wrapper = () => container.querySelector<HTMLElement>('[data-slot="command-input-wrapper"]')!;
const list = () => container.querySelector<HTMLElement>('[cmdk-list]')!;
const contentHeight = () => container.querySelectorAll('[cmdk-item]').length * 30 || (container.querySelector('[cmdk-empty]') ? 60 : 0);

beforeEach(() => {
  vi.stubGlobal('IS_REACT_ACT_ENVIRONMENT', true);
  capacity = 200;
  constrained = false;
  frameId = 0;
  frames = new Map();
  resizes = new Set();
  vi.stubGlobal('requestAnimationFrame', (callback: FrameRequestCallback) => { frames.set(++frameId, callback); return frameId; });
  vi.stubGlobal('cancelAnimationFrame', (id: number) => frames.delete(id));
  vi.stubGlobal('ResizeObserver', class {
    constructor(callback: () => void) { this.callback = callback; resizes.add(callback); }
    callback: () => void;
    observe() {}
    unobserve() { resizes.delete(this.callback); }
    disconnect() { resizes.delete(this.callback); }
  });
  vi.spyOn(HTMLElement.prototype, 'clientHeight', 'get').mockImplementation(function () {
    return this.hasAttribute('cmdk-list') ? Math.min(contentHeight(), capacity - (constrained && !wrapper().hidden ? 36 : 0)) : 0;
  });
  vi.spyOn(HTMLElement.prototype, 'scrollHeight', 'get').mockImplementation(function () {
    return this.hasAttribute('cmdk-list') ? contentHeight() : 0;
  });
  vi.spyOn(HTMLElement.prototype, 'offsetHeight', 'get').mockImplementation(function () {
    return this.dataset.slot === 'command-input-wrapper' && !this.hidden ? 36 : 0;
  });
  HTMLElement.prototype.scrollIntoView = vi.fn();
  container = document.createElement('div');
  document.body.append(container);
  root = createRoot(container);
});

afterEach(() => {
  act(() => root.unmount());
  container.remove();
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

async function settle() {
  await act(async () => {});
  for (let i = 0; frames.size && i < 8; i++) {
    const callbacks = [...frames.values()];
    frames.clear();
    await act(async () => callbacks.forEach(callback => callback(0)));
  }
  expect(frames.size).toBe(0);
}

async function render(count: number, inputProps: ComponentProps<typeof CommandInput> = {}, dropdown = true, onSelect = vi.fn()) {
  await act(async () => root.render(
    <div data-dropdown-content={dropdown ? '' : undefined}>
      <Command defaultValue="Option 0">
        <CommandInput placeholder="Search" {...inputProps} />
        <CommandList style={{ maxHeight: 200 }}>
          <CommandEmpty>No results</CommandEmpty>
          <CommandGroup>{Array.from({ length: count }, (_,i) => <CommandItem key={i} value={`Option ${i}`} onSelect={onSelect}>Option {i}</CommandItem>)}</CommandGroup>
        </CommandList>
      </Command>
    </div>,
  ));
  await settle();
}

async function search(value: string) {
  const input = container.querySelector<HTMLInputElement>('input')!;
  await act(async () => {
    Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')!.set!.call(input, value);
    input.dispatchEvent(new Event('input', { bubbles: true }));
  });
  await settle();
}

it.each([0, 3])('hides local search when all %i options fit', async count => {
  await render(count);
  expect(wrapper().hidden).toBe(true);
  expect(list().tabIndex).toBe(0);
});

it('supports keyboard selection from the list when search is hidden', async () => {
  const select = vi.fn();
  await render(3, {}, true, select);
  list().focus();
  await act(async () => list().dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowDown', bubbles: true })));
  await act(async () => list().dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true })));
  expect(select).toHaveBeenCalledWith('Option 1');
  expect(document.activeElement).toBe(list());
});

it('keeps search available for overflowing lists, filtered results and no matches', async () => {
  await render(10);
  expect(wrapper().hidden).toBe(false);
  await search('Option 7');
  expect(container.querySelectorAll('[cmdk-item]')).toHaveLength(1);
  expect(wrapper().hidden).toBe(false);
  await search('no matches');
  expect(container.querySelectorAll('[cmdk-item]')).toHaveLength(0);
  expect(wrapper().hidden).toBe(false);
  await search('');
  expect(container.querySelectorAll('[cmdk-item]')).toHaveLength(10);
  expect(wrapper().hidden).toBe(false);
});

it('updates when options load or the available height changes', async () => {
  await render(3);
  expect(wrapper().hidden).toBe(true);
  await render(10);
  expect(wrapper().hidden).toBe(false);
  container.querySelector<HTMLInputElement>('input')!.focus();
  await render(3);
  expect(wrapper().hidden).toBe(true);
  expect(document.activeElement).toBe(list());
  capacity = 50;
  resizes.forEach(callback => callback());
  await settle();
  expect(wrapper().hidden).toBe(false);
  capacity = 200;
  resizes.forEach(callback => callback());
  await settle();
  expect(wrapper().hidden).toBe(true);
});

it('does not oscillate when hiding search frees enough space for the full list', async () => {
  constrained = true;
  capacity = 160;
  await render(5);
  expect(wrapper().hidden).toBe(true);
  for (let i = 0; i < 3; i++) {
    resizes.forEach(callback => callback());
    await settle();
    expect(wrapper().hidden).toBe(true);
  }
});

it.each([
  [{ onValueChange: vi.fn() }, true],
  [{ value: '' }, true],
  [{ hideWhenFits: false }, true],
  [{}, false],
] as const)('preserves controlled searches, explicit overrides and command palettes (%j, %s)', async (props, dropdown) => {
  await render(3, props, dropdown);
  expect(wrapper().hidden).toBe(false);
});
