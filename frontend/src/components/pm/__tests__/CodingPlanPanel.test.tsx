// @vitest-environment jsdom
import { act } from 'react';
import { createRoot } from 'react-dom/client';
import { afterEach, describe, expect, it } from 'vitest';

import { CodingPlanPanel } from '../CodingSession/CodingPlanPanel';

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true });

describe('CodingPlanPanel', () => {
  afterEach(() => {
    document.body.innerHTML = '';
  });

  it('renders a waiting state when no plan has been published yet', () => {
    const container = document.createElement('div');
    document.body.appendChild(container);
    const root = createRoot(container);

    act(() => {
      root.render(<CodingPlanPanel plan={null} />);
    });

    expect(container.textContent).toContain('Agent plan');
    expect(container.textContent).toContain('Waiting');
    expect(container.textContent).toContain('Waiting for the agent to publish its first plan update.');

    act(() => {
      root.unmount();
    });
  });
});
