// @vitest-environment jsdom

import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { PMConditionPill, PMFilterBar, PMFilterPill, type PMFilterCondition } from '../PMFilterControls';

const definition = { key: 'employees', label: 'Employees', options: [], kind: 'number' as const, note: '2 not numeric' };

describe('PMConditionPill', () => {
  let container: HTMLDivElement;
  let root: Root;

  beforeEach(() => {
    (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
    container = document.createElement('div');
    document.body.appendChild(container);
    root = createRoot(container);
  });

  afterEach(() => {
    act(() => root.unmount());
    container.remove();
  });

  function render(condition: PMFilterCondition, onChange = vi.fn(), onRemove = vi.fn()) {
    act(() => root.render(<PMConditionPill definition={definition} condition={condition} onChange={onChange} onRemove={onRemove} />));
    return { onChange, onRemove };
  }

  function typeInto(input: HTMLInputElement, value: string) {
    const setter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')!.set!;
    act(() => {
      setter.call(input, value);
      input.dispatchEvent(new Event('input', { bubbles: true }));
    });
  }

  it('shows the comparison, the number, and the skipped-values note', () => {
    render({ operator: 'gt', value: '50' });
    expect(container.textContent).toContain('Employees');
    expect(container.textContent).toContain('>');
    expect(container.textContent).toContain('2 not numeric');
    expect(container.querySelector<HTMLInputElement>('input[aria-label="Employees value"]')?.value).toBe('50');
  });

  it('commits a typed number on Enter, not per keystroke', () => {
    const { onChange } = render({ operator: 'gt', value: '' });
    const input = container.querySelector<HTMLInputElement>('input[aria-label="Employees value"]')!;
    typeInto(input, '75');
    expect(onChange).not.toHaveBeenCalled();
    act(() => input.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true })));
    expect(onChange).toHaveBeenCalledWith({ operator: 'gt', value: '75' });
  });

  it('renders two inputs for between and none for empty checks', () => {
    render({ operator: 'between', values: ['10', '20'] });
    expect(container.querySelectorAll('input')).toHaveLength(2);
    render({ operator: 'is_empty' });
    expect(container.querySelectorAll('input')).toHaveLength(0);
  });

  it('removes the filter', () => {
    const { onRemove } = render({ operator: 'gt', value: '1' });
    act(() => container.querySelector<HTMLButtonElement>('button[aria-label="Remove Employees filter"]')!.click());
    expect(onRemove).toHaveBeenCalled();
  });

  it('is rendered by PMFilterBar for number definitions', () => {
    act(() => root.render(
      <PMFilterBar
        definitions={[definition]}
        values={{}}
        conditions={{ employees: { operator: 'lte', value: '9' } }}
        visibleKeys={new Set(['employees'])}
        onToggle={vi.fn()}
        onRemove={vi.fn()}
        onClearAll={vi.fn()}
      />,
    ));
    expect(container.textContent).toContain('≤');
    expect(container.querySelector<HTMLInputElement>('input[aria-label="Employees value"]')?.value).toBe('9');
  });

  it('offers option operators only when the definition opts in, hiding values for empty checks', () => {
    const options = { key: 'fit', label: 'Fit', options: [{ value: 'yes', label: 'yes' }] };
    act(() => root.render(<PMFilterPill definition={options} selected={['yes']} onToggle={vi.fn()} onRemove={vi.fn()} />));
    expect(container.querySelector('button[aria-label^="Fit comparison"]')).toBeNull();

    const onModeChange = vi.fn();
    act(() => root.render(
      <PMFilterPill definition={{ ...options, optionModes: true }} selected={['yes']} mode="is_none_of" onModeChange={onModeChange} onToggle={vi.fn()} onRemove={vi.fn()} />,
    ));
    expect(container.querySelector('button[aria-label="Fit comparison: is not"]')).not.toBeNull();
    expect(container.textContent).toContain('yes');

    act(() => root.render(
      <PMFilterPill definition={{ ...options, optionModes: true }} selected={[]} mode="is_empty" onModeChange={onModeChange} onToggle={vi.fn()} onRemove={vi.fn()} />,
    ));
    expect(container.textContent).toContain('is empty');
    expect(container.textContent).not.toContain('Choose value');
  });
});
