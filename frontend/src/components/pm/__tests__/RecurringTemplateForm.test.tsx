// @vitest-environment jsdom
import { act } from 'react';
import { createRoot } from 'react-dom/client';
import { describe, expect, it, vi } from 'vitest';

import { RecurringTemplateForm } from '../RecurringTemplateForm';

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

describe('RecurringTemplateForm', () => {
  it('switches between time-based and completion-based controls', () => {
    const container = document.createElement('div');
    document.body.appendChild(container);
    const root = createRoot(container);

    act(() => {
      root.render(
        <RecurringTemplateForm
          initialValue={{ title: 'Weekly Ops Review' }}
          onSubmit={() => {}}
        />,
      );
    });

    expect(container.textContent).toContain('Frequency');
    expect(container.textContent).toContain('Weekdays');

    const completionButton = Array.from(container.querySelectorAll('button')).find((element) =>
      element.textContent?.includes('On completion'),
    );

    act(() => {
      completionButton?.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    });

    expect(container.textContent).toContain('Completion event');
    expect(container.textContent).not.toContain('Weekdays');

    act(() => {
      root.unmount();
    });
    container.remove();
  });

  it('submits normalized recurring config values', () => {
    const handleSubmit = vi.fn();
    const container = document.createElement('div');
    document.body.appendChild(container);
    const root = createRoot(container);

    act(() => {
      root.render(
        <RecurringTemplateForm
          initialValue={{
            title: 'Sprint Planning',
            config: {
              schedule_type: 'time',
              frequency: 'weekly',
              interval: 2,
              weekdays: [1, 3],
              due_date_mode: 'offset_days',
              due_offset_days: 3,
              sprint_assignment_mode: 'current_sprint',
            },
          }}
          onSubmit={handleSubmit}
        />,
      );
    });

    const saveButton = Array.from(container.querySelectorAll('button')).find((element) =>
      element.textContent?.includes('Save recurrence'),
    );

    act(() => {
      saveButton?.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    });

    expect(handleSubmit).toHaveBeenCalledWith(
      expect.objectContaining({
        title: 'Sprint Planning',
        config: expect.objectContaining({
          schedule_type: 'time',
          frequency: 'weekly',
          interval: 2,
          weekdays: [1, 3],
          due_date_mode: 'offset_days',
          due_offset_days: 3,
          sprint_assignment_mode: 'current_sprint',
        }),
      }),
    );

    act(() => {
      root.unmount();
    });
    container.remove();
  });
});
