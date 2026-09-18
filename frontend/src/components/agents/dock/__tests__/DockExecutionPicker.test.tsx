// @vitest-environment jsdom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { DockExecutionPicker } from '../DockExecutionPicker';

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

vi.mock('@/components/ui/dropdown-menu', () => ({
  DropdownMenu: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  DropdownMenuTrigger: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  DropdownMenuContent: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  DropdownMenuCheckboxItem: ({ children, checked, disabled, onCheckedChange }: {
    children: React.ReactNode;
    checked: boolean;
    disabled?: boolean;
    onCheckedChange: (checked: boolean) => void;
  }) => (
    <button type="button" disabled={disabled} data-checked={String(checked)} onClick={() => onCheckedChange(!checked)}>
      {children}
    </button>
  ),
}));

vi.mock('@/components/ui/tooltip', () => ({
  Tooltip: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  TooltipTrigger: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  TooltipContent: ({ children }: { children: React.ReactNode }) => <span>{children}</span>,
}));

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

describe('DockExecutionPicker', () => {
  it('shows compact state and lets the user enable execution', () => {
    const onChange = vi.fn();
    act(() => root.render(<DockExecutionPicker enabled={false} onChange={onChange} />));

    expect(container.querySelector('[aria-label="Code and Python tools disabled"]')).not.toBeNull();
    const choice = [...container.querySelectorAll('button')].find((button) => button.textContent?.includes('Code & Python'));
    expect(choice?.dataset.checked).toBe('false');
    act(() => choice?.click());
    expect(onChange).toHaveBeenCalledWith(true);
  });
});
