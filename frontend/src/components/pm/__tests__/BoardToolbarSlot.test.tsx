// @vitest-environment jsdom
import { act } from 'react';
import { createRoot } from 'react-dom/client';
import { describe, expect, it } from 'vitest';

import { BoardToolbarSlot } from '../BoardToolbarSlot';

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

describe('BoardToolbarSlot', () => {
  it('renders children inside a stable fixed-height wrapper', () => {
    const container = document.createElement('div');
    document.body.appendChild(container);
    const root = createRoot(container);

    act(() => {
      root.render(
        <BoardToolbarSlot>
          <button type="button">Control</button>
        </BoardToolbarSlot>,
      );
    });

    const slot = container.querySelector('[data-testid="board-toolbar-slot"]') as HTMLElement | null;
    expect(slot).toBeTruthy();
    expect(slot?.className).toContain('flex');
    expect(slot?.className).toContain('h-7');
    expect(slot?.className).toContain('items-center');
    expect(slot?.textContent).toContain('Control');

    act(() => {
      root.unmount();
    });
    container.remove();
  });
});
