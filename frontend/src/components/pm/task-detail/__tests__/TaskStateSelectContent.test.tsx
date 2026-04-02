// @vitest-environment jsdom
import { act } from 'react';
import { createRoot } from 'react-dom/client';
import { describe, expect, it } from 'vitest';

import { TaskStateSelectContent } from '../TaskStateSelectContent';

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

describe('TaskStateSelectContent', () => {
  it('renders the state label and color indicator', () => {
    const container = document.createElement('div');
    document.body.appendChild(container);
    const root = createRoot(container);

    act(() => {
      root.render(
        <TaskStateSelectContent
          stateType="started"
          label="In Progress"
          color="#123456"
        />,
      );
    });

    expect(container.textContent).toContain('In Progress');
    const colorDot = container.querySelector('[data-testid="state-color-dot"]') as HTMLElement | null;
    expect(colorDot).toBeTruthy();
    expect(colorDot?.style.backgroundColor).toBe('rgb(18, 52, 86)');
    expect(colorDot?.className).toContain('h-3');
    expect(colorDot?.className).toContain('w-3');
    expect(container.querySelector('svg')).toBeNull();

    act(() => {
      root.unmount();
    });
    container.remove();
  });
});
