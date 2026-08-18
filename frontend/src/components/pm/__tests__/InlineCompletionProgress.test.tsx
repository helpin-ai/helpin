// @vitest-environment jsdom
import { act } from 'react';
import { createRoot } from 'react-dom/client';
import { describe, expect, it } from 'vitest';

import { InlineCompletionProgress } from '../InlineCompletionProgress';

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true });

describe('InlineCompletionProgress', () => {
  it('renders shared completion count, progress, and percentage', () => {
    const container = document.createElement('div');
    const root = createRoot(container);

    act(() => {
      root.render(
        <InlineCompletionProgress
          completed={2}
          total={4}
          testIdPrefix="shared"
        />,
      );
    });

    expect(container.querySelector('[data-testid="shared-count"]')?.textContent).toBe('(2/4)');
    expect(container.querySelector('[data-testid="shared-progress"]')?.getAttribute('aria-label')).toBe('2 of 4 complete');
    expect(container.querySelector<HTMLElement>('[data-slot="progress-indicator"]')?.style.transform).toBe('translateX(-50%)');
    expect(container.querySelector('[data-testid="shared-percentage"]')?.textContent).toBe('50%');

    act(() => root.unmount());
  });

  it('omits empty progress and supports a title that already contains the total', () => {
    const container = document.createElement('div');
    const root = createRoot(container);

    act(() => {
      root.render(<InlineCompletionProgress completed={0} total={0} />);
    });
    expect(container.childElementCount).toBe(0);

    act(() => {
      root.render(
        <InlineCompletionProgress
          completed={3}
          total={5}
          showCount={false}
          testIdPrefix="without-count"
        />,
      );
    });
    expect(container.querySelector('[data-testid="without-count-count"]')).toBeNull();
    expect(container.querySelector('[data-testid="without-count-percentage"]')?.textContent).toBe('60%');

    act(() => root.unmount());
  });
});
