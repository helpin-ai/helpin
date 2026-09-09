import { PMFilterTrigger } from '@/components/pm/PMFilterControls';
// @vitest-environment jsdom
import { act, useState, type ReactNode } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { QuietDropdown } from '../quiet-dropdown';
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectLabel,
  SelectTrigger,
  SelectValue,
} from '../quiet-dropdown-select';
import { SidebarPopoverSelect } from '@/components/pm/SidebarPopoverSelect';

(
  globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }
).IS_REACT_ACT_ENVIRONMENT = true;
let root: Root;
const item = (value: string) =>
  document.querySelector<HTMLElement>(`[cmdk-item][data-value="${value}"]`)!;
const trigger = () =>
  document.querySelector<HTMLButtonElement>('button[aria-haspopup="dialog"]')!;
const click = async (element: HTMLElement) => {
  await act(async () => element.click());
};
async function render(children: ReactNode) {
  if (!root) {
    const container = document.createElement('div');
    document.body.append(container);
    root = createRoot(container);
  }
  await act(async () => root.render(children));
}
async function key(key: string) {
  await act(async () =>
    document
      .querySelector('[cmdk-root]')!
      .dispatchEvent(new KeyboardEvent('keydown', { key, bubbles: true })),
  );
}
beforeEach(() => {
  vi.stubGlobal(
    'ResizeObserver',
    class {
      observe() {}
      unobserve() {}
      disconnect() {}
    },
  );
  HTMLElement.prototype.scrollIntoView = vi.fn();
});
afterEach(() => {
  act(() => root?.unmount());
  root = undefined!;
  document.body.innerHTML = '';
  vi.unstubAllGlobals();
});

describe('shared PM dropdown contracts', () => {
  it('keeps same-named sidebar options independently keyboard-selectable', async () => {
    const change = vi.fn();
    await render(
      <SidebarPopoverSelect
        value="a"
        options={[
          { value: 'a', label: 'Same epic' },
          { value: 'b', label: 'Same epic' },
        ]}
        onChange={change}
        renderTrigger={() => 'Epic'}
      />,
    );
    await click(trigger());
    expect(
      document.querySelectorAll('[cmdk-item][aria-selected="true"]'),
    ).toHaveLength(1);
    await key('ArrowDown');
    await key('Enter');
    expect(change).toHaveBeenCalledWith('b');
  });

  it('resets category search before showing filter values and when navigating back', async () => {
    await render(
      <PMFilterTrigger
        definitions={[
          {
            key: 'owner',
            label: 'Owner',
            options: [{ value: 'alex', label: 'Alex' }],
          },
        ]}
        values={{}}
        visibleKeys={new Set<string>()}
        activeCount={0}
        onAdd={() => {}}
        onToggle={() => {}}
      />,
    );
    await click(trigger());
    const input = document.querySelector<HTMLInputElement>('[cmdk-input]')!;
    await act(async () => {
      Object.getOwnPropertyDescriptor(
        HTMLInputElement.prototype,
        'value',
      )!.set!.call(input, 'Owner');
      input.dispatchEvent(new Event('input', { bubbles: true }));
    });
    await click(item('owner'));
    expect(
      document.querySelector<HTMLInputElement>('[cmdk-input]')!.value,
    ).toBe('');
    expect(item('alex')).not.toBeNull();
    await click(
      document.querySelector<HTMLButtonElement>(
        '[aria-label="Back to filter fields"]',
      )!,
    );
    expect(item('owner')).not.toBeNull();
    expect(
      document.querySelector<HTMLInputElement>('[cmdk-input]')!.value,
    ).toBe('');
  });

  it('preserves multiple and partial selection without closing, and omits empty groups', async () => {
    const change = vi.fn();
    await render(
      <QuietDropdown
        label="Owners"
        trigger={<button>Owners</button>}
        multiple
        selected={['a']}
        onSelect={change}
        groups={[
          { id: 'empty', label: 'Empty', options: [] },
          {
            id: 'active',
            label: 'Active',
            options: [
              { value: 'a', label: 'Alex' },
              { value: 'b', label: 'Sam', partial: true },
              { value: 'c', label: 'Former owner', disabled: true },
            ],
          },
        ]}
      />,
    );
    await click(trigger());
    expect(document.querySelectorAll('[cmdk-group-heading]')).toHaveLength(1);
    expect(item('a').dataset.checked).toBe('true');
    expect(item('b').dataset.partial).toBe('true');
    await click(item('c'));
    expect(change).not.toHaveBeenCalled();
    await click(item('b'));
    expect(change).toHaveBeenCalledWith('b');
    expect(trigger().getAttribute('aria-expanded')).toBe('true');
  });

  it('preserves an explicit empty-value option and its callback', async () => {
    const change = vi.fn();
    await render(
      <QuietDropdown
        label="Teams"
        trigger={<button>Teams</button>}
        selected={['']}
        onSelect={change}
        options={[
          { value: '', label: 'All teams' },
          { value: 'a', label: 'Product' },
        ]}
      />,
    );
    await click(trigger());
    expect(item('__quiet_empty__').dataset.checked).toBe('true');
    await click(item('__quiet_empty__'));
    expect(change).toHaveBeenCalledWith('');
  });

  it('keeps remote and creatable search available with no results', async () => {
    await render(
      <QuietDropdown
        label="Labels"
        trigger={<button>Labels</button>}
        options={[]}
        onSelect={() => {}}
        searchMode="always"
        query="New label"
        onQueryChange={() => {}}
        empty={<button>Create New label</button>}
      />,
    );
    await click(trigger());
    const input = document.querySelector<HTMLInputElement>('[cmdk-input]')!;
    expect(input.value).toBe('New label');
    expect(
      input.closest<HTMLElement>('[data-slot="command-input-wrapper"]')!.hidden,
    ).toBe(false);
    expect(document.querySelector('[cmdk-empty]')!.textContent).toBe(
      'Create New label',
    );
  });

  it('adapts grouped form choices, skipping disabled options and preserving values by ID', async () => {
    const change = vi.fn();
    await render(
      <Select value="a" onValueChange={change}>
        <SelectTrigger>
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectGroup>
            <SelectLabel>In progress</SelectLabel>
            <SelectItem value="a">Same epic</SelectItem>
            <SelectItem value="disabled" disabled>
              Disabled
            </SelectItem>
            <SelectItem value="b">Same epic</SelectItem>
          </SelectGroup>
        </SelectContent>
      </Select>,
    );
    await click(trigger());
    expect(document.querySelector('[cmdk-group-heading]')!.textContent).toBe(
      'In progress',
    );
    await key('ArrowDown');
    await key('Enter');
    expect(change).toHaveBeenCalledWith('b');
  });

  it('resets a controlled bulk field to its mixed placeholder when dependencies change', async () => {
    const field = (value: string | undefined) => (
      <Select value={value}>
        <SelectTrigger>
          <SelectValue placeholder="Multiple" />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="a">Todo</SelectItem>
          <SelectItem value="b">Done</SelectItem>
        </SelectContent>
      </Select>
    );
    await render(field(undefined));
    expect(trigger().textContent).toBe('Multiple');
    await click(trigger());
    await click(item('b'));
    await render(field('b'));
    expect(trigger().textContent).toBe('Done');
    await render(field(undefined));
    expect(trigger().textContent).toBe('Multiple');
  });

  it('supports uncontrolled form values and opens from the arrow key', async () => {
    function Field() {
      const [changed, setChanged] = useState('');
      return (
        <>
          <Select defaultValue="a" onValueChange={setChanged} name="state">
            <SelectTrigger>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="a">Todo</SelectItem>
              <SelectItem value="b">Done</SelectItem>
            </SelectContent>
          </Select>
          <output>{changed}</output>
        </>
      );
    }
    await render(<Field />);
    await act(async () =>
      trigger().dispatchEvent(
        new KeyboardEvent('keydown', { key: 'ArrowDown', bubbles: true }),
      ),
    );
    await key('ArrowDown');
    await key('Enter');
    expect(document.querySelector('output')!.textContent).toBe('b');
    expect(
      document.querySelector<HTMLInputElement>('input[name="state"]')!.value,
    ).toBe('b');
  });
});
