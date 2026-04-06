// @vitest-environment jsdom
import { act } from 'react';
import { createRoot } from 'react-dom/client';
import { afterEach, describe, expect, it } from 'vitest';

import { CodingPreviewPanels } from '../CodingSession/CodingPreviewPanels';
import type { PublishedPreview } from '../runPreviews';

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true });

describe('CodingPreviewPanels', () => {
  afterEach(() => {
    document.body.innerHTML = '';
  });

  it('renders legacy Atlas task-plan previews with proposed_stories as structured UI', () => {
    const container = document.createElement('div');
    document.body.appendChild(container);
    const root = createRoot(container);

    const previewsByKey = new Map<string, PublishedPreview>([
      ['task_plan', {
        panelKey: 'task_plan',
        title: 'Task Plan',
        format: 'json',
        replace: true,
        surroundingText: '',
        content: {
          summary: 'Break the epic into slices.',
          proposed_stories: [
            {
              ref: 'story_1',
              title: 'User can connect Stripe',
              story_type: 'feature',
              description: 'Add the Stripe connection flow.',
              acceptance_criteria: ['User can complete OAuth'],
              dependency_refs: [],
              implementation_brief: {
                files_to_modify: [
                  { path: 'frontend/src/pages/settings/Stripe.tsx' },
                ],
              },
            },
          ],
        },
      }],
    ]);

    act(() => {
      root.render(<CodingPreviewPanels previewsByKey={previewsByKey} />);
    });

    expect(container.textContent).toContain('Task Plan');
    expect(container.textContent).toContain('Break the epic into slices.');
    expect(container.textContent).toContain('1 tasks');
    expect(container.textContent).toContain('User can connect Stripe');
    expect(container.textContent).toContain('Feature');
    expect(container.textContent).not.toContain('proposed_stories');

    act(() => {
      root.unmount();
    });
    container.remove();
  });
});
