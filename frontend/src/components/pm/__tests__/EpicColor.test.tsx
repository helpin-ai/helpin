// @vitest-environment jsdom
import { act, type ReactNode } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { ColorPicker, EPIC_PRESET_COLORS, PRESET_COLORS } from '../ColorPicker';
import { EpicBadge } from '../EpicBadge';
import { EpicColorControl } from '../EpicColorControl';
import { DEFAULT_EPIC_COLOR, resolveEpicColor } from '../epicColor';

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

let root: Root;
function render(node: ReactNode) {
  const container = document.createElement('div');
  document.body.append(container);
  root = createRoot(container);
  act(() => root.render(node));
  return container;
}

function click(label: string) {
  const button = Array.from(document.querySelectorAll('button')).find((element) => element.getAttribute('aria-label') === label || element.textContent === label);
  expect(button, label).toBeDefined();
  act(() => button!.click());
}

afterEach(() => {
  if (root) act(() => root.unmount());
  document.body.innerHTML = '';
});

describe('epic colors', () => {
  it.each([
    [undefined, DEFAULT_EPIC_COLOR], [null, DEFAULT_EPIC_COLOR], ['', DEFAULT_EPIC_COLOR],
    ['invalid', DEFAULT_EPIC_COLOR], ['url(example.com)', DEFAULT_EPIC_COLOR],
    ['#ABC', '#aabbcc'], ['AABBCC', '#aabbcc'], [' #123456 ', '#123456'],
  ])('resolves %s to %s', (input, expected) => {
    expect(resolveEpicColor(input)).toBe(expected);
  });

  it('renders the full epic name with a safe fallback color', () => {
    const name = 'A very long epic name that must remain available when truncated';
    const container = render(<EpicBadge name={name} color="not a color" />);
    const badge = container.querySelector<HTMLElement>('[title]')!;
    expect(badge.title).toBe(name);
    expect(badge.textContent).toBe(name);
    expect(badge.querySelector<HTMLElement>('[aria-hidden="true"]')?.style.backgroundColor).toBe('rgb(193, 201, 211)');
    expect(badge.style.backgroundColor).toBe('');
    expect(badge.style.color).toBe('');
  });

  it.each([
    ['#000000', 'rgb(0, 0, 0)'],
    ['#ffffff', 'rgb(255, 255, 255)'],
    ['#5e6ad2', 'rgb(94, 106, 210)'],
    ['#e2564a', 'rgb(226, 86, 74)'],
    ['#ffff00', 'rgb(255, 255, 0)'],
    ['#0000ff', 'rgb(0, 0, 255)'],
    ['#abc', 'rgb(170, 187, 204)'],
  ])('shows the exact %s in a square beside neutral text in both themes', (color, background) => {
    const container = render(<EpicBadge name="Epic" color={color} />);
    const badge = container.querySelector<HTMLElement>('[title]')!;
    for (const dark of [false, true]) {
      container.classList.toggle('dark', dark);
      expect(badge.querySelector<HTMLElement>('[aria-hidden="true"]')?.style.backgroundColor).toBe(background);
      expect(badge.style.backgroundColor).toBe('');
      expect(badge.style.color).toBe('');
    }
  });

  it('exposes the selected square and sends preset changes', () => {
    const onChange = vi.fn();
    render(<ColorPicker value={DEFAULT_EPIC_COLOR} onChange={onChange} shape="square" presets={EPIC_PRESET_COLORS} />);
    const selected = document.querySelector('[aria-pressed="true"]')!;
    expect(selected.getAttribute('aria-label')).toBe(`Select color ${DEFAULT_EPIC_COLOR}`);
    expect(selected.className).toContain('rounded-[4px]');
    click(`Select color ${EPIC_PRESET_COLORS[8]}`);
    expect(onChange).toHaveBeenCalledWith(EPIC_PRESET_COLORS[8]);
  });

  it('keeps the default picker circular for other callers', () => {
    render(<ColorPicker value={PRESET_COLORS[0]} onChange={vi.fn()} />);
    expect(document.querySelector('[aria-pressed="true"]')!.className).toContain('rounded-full');
  });

  it('normalizes custom hex input and rejects invalid input', () => {
    const onChange = vi.fn();
    render(<ColorPicker value={DEFAULT_EPIC_COLOR} onChange={onChange} shape="square" />);
    click('Custom color');
    const input = document.querySelector<HTMLInputElement>('[aria-label="Hex color"]')!;
    const changeInput = (value: string) => {
      act(() => {
        Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')!.set!.call(input, value);
        input.dispatchEvent(new Event('input', { bubbles: true }));
      });
      act(() => input.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true })));
    };
    changeInput('abc');
    expect(onChange).toHaveBeenLastCalledWith('#aabbcc');
    onChange.mockClear();
    changeInput('invalid');
    expect(input.getAttribute('aria-invalid')).toBe('true');
    expect(onChange).not.toHaveBeenCalled();
  });

  it('previews without saving until Apply, and discards cancelled drafts', () => {
    const onChange = vi.fn();
    render(<EpicColorControl value={DEFAULT_EPIC_COLOR} onChange={onChange} />);
    click('Change epic color');
    click(`Select color ${EPIC_PRESET_COLORS[8]}`);
    expect(onChange).not.toHaveBeenCalled();
    click('Cancel');
    click('Change epic color');
    expect(document.querySelector('[aria-pressed="true"]')!.getAttribute('aria-label')).toBe(`Select color ${DEFAULT_EPIC_COLOR}`);
    click(`Select color ${EPIC_PRESET_COLORS[1]}`);
    click('Apply');
    expect(onChange).toHaveBeenCalledTimes(1);
    expect(onChange).toHaveBeenCalledWith(EPIC_PRESET_COLORS[1]);
  });

  it('shows a static square to viewers', () => {
    const container = render(<EpicColorControl value="#123456" />);
    expect(container.querySelector('button')).toBeNull();
    expect(container.querySelector('[aria-label="Epic color: #123456"]')).not.toBeNull();
  });

  it('edits inside a clickable row without opening the epic', () => {
    const onOpenEpic = vi.fn();
    const onChange = vi.fn();
    render(<div onClick={onOpenEpic}><EpicColorControl compact value={DEFAULT_EPIC_COLOR} onChange={onChange} /></div>);
    click('Change epic color');
    click(`Select color ${EPIC_PRESET_COLORS[8]}`);
    click('Apply');
    expect(onChange).toHaveBeenCalledWith(EPIC_PRESET_COLORS[8]);
    expect(onOpenEpic).not.toHaveBeenCalled();
  });
});
