// @vitest-environment jsdom
import { act } from 'react';
import { createRoot } from 'react-dom/client';
import { describe, expect, it } from 'vitest';

import { RecurringTemplateSummary } from '../RecurringTemplateSummary';

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

describe('RecurringTemplateSummary', () => {
  it('renders recurring status, rule summary, and last generated story details', () => {
    const container = document.createElement('div');
    document.body.appendChild(container);
    const root = createRoot(container);

    act(() => {
      root.render(
        <RecurringTemplateSummary
          title="Weekly Ops Review"
          status="active"
          ruleSummary="Every 2 weeks on Mon, Wed"
          nextRunAt="2026-03-20T00:00:00Z"
          generatedCount={8}
          occurrenceNumber={3}
          lastGeneratedStory={{ id: 'story-1', display_id: 321, name: 'Weekly Ops Review' }}
        />,
      );
    });

    expect(container.textContent).toContain('Weekly Ops Review');
    expect(container.textContent).toContain('active');
    expect(container.textContent).toContain('Every 2 weeks on Mon, Wed');
    expect(container.textContent).toContain('Occurrence #3');
    expect(container.textContent).toContain('8 generated');
    expect(container.textContent).toContain('321 Weekly Ops Review');

    act(() => {
      root.unmount();
    });
    container.remove();
  });
});
