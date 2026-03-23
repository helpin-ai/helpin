// @vitest-environment jsdom
import { act } from 'react';
import { createRoot } from 'react-dom/client';
import { describe, expect, it, vi } from 'vitest';

import { RecurringTemplateList } from '../RecurringTemplateList';
import type { RecurringTemplateDetail } from '@/lib/pmTypes';

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

function buildTemplate(id: string, status: RecurringTemplateDetail['template']['status']): RecurringTemplateDetail {
  return {
    template: {
      id,
      workspace_id: 'ws-1',
      title: `Template ${id}`,
      status,
      generated_count: status === 'paused' ? 2 : 5,
      failure_count: 0,
      skip_next_run: false,
      seed_payload: '{}',
      config: '{}',
      created_at: '',
      updated_at: '',
    },
    config: {
      schedule_type: 'time',
      frequency: 'weekly',
      interval: 1,
      weekdays: [1],
      due_date_mode: 'scheduled_date',
      sprint_assignment_mode: 'none',
    },
    seed: {
      name: `Seed ${id}`,
      workflow_id: 'wf-1',
      workflow_state_id: 'state-1',
    },
    rule_summary: 'Every week on Mon',
  };
}

describe('RecurringTemplateList', () => {
  it('renders lifecycle controls based on template status', () => {
    const handleDuplicate = vi.fn();
    const container = document.createElement('div');
    document.body.appendChild(container);
    const root = createRoot(container);

    act(() => {
      root.render(
        <RecurringTemplateList
          templates={[buildTemplate('active-1', 'active'), buildTemplate('paused-1', 'paused')]}
          onDuplicate={handleDuplicate}
        />,
      );
    });

    expect(container.textContent).toContain('Pause');
    expect(container.textContent).toContain('Resume');

    const duplicateButton = Array.from(container.querySelectorAll('button')).find((element) =>
      element.textContent?.includes('Duplicate'),
    );

    act(() => {
      duplicateButton?.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    });

    expect(handleDuplicate).toHaveBeenCalled();

    act(() => {
      root.unmount();
    });
    container.remove();
  });
});
