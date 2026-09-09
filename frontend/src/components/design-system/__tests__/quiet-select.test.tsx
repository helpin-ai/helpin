// @vitest-environment jsdom
import { act, useState, type ReactNode } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { QuietFilterDropdown } from '../quiet-select';

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

const options = [
  { value: 'all', label: 'All statuses' },
  { value: 'draft', label: 'Draft' },
  { value: 'live', label: 'Accepting signals' },
  { value: 'stopped', label: 'Stopped', disabled: true },
];
let root: Root;

async function render(children: ReactNode) {
  const container = document.createElement('div');
  document.body.append(container);
  root = createRoot(container);
  await act(async () => root.render(children));
}

const trigger = () => document.querySelector<HTMLButtonElement>('button[aria-haspopup="dialog"]')!;
const item = (value: string) => document.querySelector<HTMLElement>(`[cmdk-item][data-value="${value}"]`)!;
async function click(element: HTMLElement) { await act(async () => element.click()); }
async function search(value: string) {
  const input = document.querySelector<HTMLInputElement>('input[cmdk-input]')!;
  await act(async () => {
    Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')!.set!.call(input, value);
    input.dispatchEvent(new Event('input', { bubbles: true }));
  });
  return input;
}

beforeEach(() => {
  vi.stubGlobal('ResizeObserver', class { observe() {} unobserve() {} disconnect() {} });
  HTMLElement.prototype.scrollIntoView = vi.fn();
});

afterEach(() => {
  act(() => root?.unmount());
  document.body.innerHTML = '';
  vi.unstubAllGlobals();
});

describe('QuietFilterDropdown interactions', () => {
  it('searches labels without case sensitivity and selects one value with the keyboard', async () => {
    function Filter() {
      const [value, setValue] = useState('all');
      return <QuietFilterDropdown label="Status" value={value} onChange={setValue} options={options} />;
    }
    await render(<Filter />);
    await click(trigger());
    const input = await search('ACCEPTING');
    expect(document.querySelectorAll('[cmdk-item]')).toHaveLength(1);
    await act(async () => input.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true })));
    expect(trigger().getAttribute('aria-label')).toBe('Status: Accepting signals');
    expect(trigger().getAttribute('aria-expanded')).toBe('false');
  });

  it('keeps multiple selections while searching, toggles values, and clears with focus restored', async () => {
    function Filter() {
      const [selected, setSelected] = useState<string[]>([]);
      return <QuietFilterDropdown multiple label="Status" emptyLabel="All statuses" selected={selected} onChange={setSelected} options={options} />;
    }
    await render(<Filter />);
    expect(trigger().textContent).toBe('All statuses');
    await click(trigger());
    await click(item('draft'));
    expect(trigger().getAttribute('aria-expanded')).toBe('true');
    await search('accepting');
    await click(item('live'));
    expect(trigger().getAttribute('aria-label')).toBe('Status: 2 selected');
    expect(trigger().textContent).toBe('Status: 2 selected');
    await search('');
    expect(item('draft').getAttribute('data-checked')).toBe('true');
    expect(item('live').getAttribute('data-checked')).toBe('true');
    await click(item('draft'));
    expect(trigger().getAttribute('aria-label')).toBe('Status: Accepting signals');
    await click(trigger());
    await click(document.querySelector<HTMLButtonElement>('[aria-label="Clear Status filter"]')!);
    expect(trigger().getAttribute('aria-label')).toBe('Status: All statuses');
    expect(document.activeElement).toBe(trigger());
    expect(document.querySelector('[aria-label="Clear Status filter"]')).toBeNull();
  });

  it('shows no results and preserves a selection that is absent from the available options', async () => {
    const onChange = vi.fn();
    await render(<QuietFilterDropdown multiple label="Owner" selected={['former-member']} onChange={onChange} options={[]} />);
    expect(trigger().getAttribute('aria-label')).toBe('Owner: former-member');
    await click(trigger());
    await search('missing');
    expect(document.querySelector('[cmdk-empty]')?.textContent).toBe('No results');
    expect(onChange).not.toHaveBeenCalled();
  });

  it('uses option IDs to distinguish duplicate labels', async () => {
    const onChange = vi.fn();
    await render(<QuietFilterDropdown label="Owner" value="a" onChange={onChange} options={[{ value: 'a', label: 'Alex' }, { value: 'b', label: 'Alex' }]} />);
    await click(trigger());
    await search('Alex');
    expect(document.querySelectorAll('[cmdk-item]')).toHaveLength(2);
    await click(item('b'));
    expect(onChange).toHaveBeenCalledWith('b');
  });

  it('prevents disabled filters, clear actions, and options from changing the value', async () => {
    const onChange = vi.fn();
    await render(<QuietFilterDropdown multiple disabled label="Status" selected={['draft']} onChange={onChange} options={options} />);
    await click(trigger());
    await click(document.querySelector<HTMLButtonElement>('[aria-label="Clear Status filter"]')!);
    expect(trigger().getAttribute('aria-expanded')).toBe('false');
    expect(onChange).not.toHaveBeenCalled();
    await act(async () => root.render(<QuietFilterDropdown label="Status" value="draft" onChange={onChange} options={options} />));
    await click(trigger());
    await click(item('stopped'));
    expect(onChange).not.toHaveBeenCalled();
  });

  it('supports keyboard selection when search is turned off', async () => {
    const onChange = vi.fn();
    await render(<QuietFilterDropdown label="Status" value="all" onChange={onChange} options={options} searchable={false} />);
    await click(trigger());
    expect(document.querySelector('input')).toBeNull();
    const command = document.querySelector<HTMLElement>('[cmdk-root]')!;
    await act(async () => command.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowDown', bubbles: true })));
    await act(async () => command.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true })));
    expect(onChange).toHaveBeenCalledWith('draft');
  });
});
