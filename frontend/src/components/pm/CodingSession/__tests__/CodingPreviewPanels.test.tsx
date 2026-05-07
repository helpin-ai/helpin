// @vitest-environment jsdom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { CodingPreviewPanels } from '../CodingPreviewPanels';
import type { PublishedPreview } from '@/components/pm/runPreviews';
import type { CodingSessionInteraction } from '@/lib/pmTypes';

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

let container: HTMLDivElement;
let root: Root;

beforeEach(() => {
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
});

afterEach(() => {
  act(() => {
    root.unmount();
  });
  container.remove();
});

function renderPanels(
  options: {
    previews?: Map<string, PublishedPreview>;
    interaction?: CodingSessionInteraction | null;
    onResolveInteraction?: (interactionId: string, responsePayload: Record<string, unknown>, followupMessage?: string) => void;
  } = {},
) {
  const previews = options.previews ?? new Map<string, PublishedPreview>([
    ['prd_draft', {
      panelKey: 'prd_draft',
      title: 'PRD Draft',
      format: 'markdown',
      content: '# Problem\nDraft body',
      replace: true,
      surroundingText: '',
    }],
  ]);

  act(() => {
    root.render(
      <CodingPreviewPanels
        previewsByKey={previews}
        attachedApprovalInteraction={options.interaction ?? null}
        acting={null}
        onResolveInteraction={options.onResolveInteraction}
      />,
    );
  });
}

function clickButton(label: string) {
  const button = Array.from(container.querySelectorAll('button')).find((candidate) => candidate.textContent?.trim() === label);
  expect(button).toBeTruthy();
  act(() => {
    button!.dispatchEvent(new MouseEvent('click', { bubbles: true }));
  });
}

function clickButtonByAriaLabel(label: string) {
  const button = container.querySelector(`button[aria-label="${label}"]`);
  expect(button).toBeTruthy();
  act(() => {
    button!.dispatchEvent(new MouseEvent('click', { bubbles: true }));
  });
}

describe('CodingPreviewPanels', () => {
  it('renders attached approval controls for preview-bound approval requests', () => {
    const onResolveInteraction = vi.fn();
    renderPanels({
      interaction: {
        interaction_id: 'interaction-1',
        interaction_kind: 'approval_request',
        status: 'pending',
        request_schema_version: 'helpin.v1',
        request_payload: {
          phase: 'prd',
          preview_panel_key: 'prd_draft',
          title: 'Approve PRD',
          summary: 'Review the latest draft.',
        },
      } satisfies CodingSessionInteraction,
      onResolveInteraction,
    });

    expect(container.textContent).toContain('PRD Draft');
    expect(container.textContent).toContain('Approve PRD');
    expect(container.textContent).toContain('Review the latest draft.');

    clickButton('Approve');

    expect(onResolveInteraction).toHaveBeenCalledWith(
      'interaction-1',
      { decision: 'approve' },
      undefined,
    );
  });

  it('clarifies note behavior and uses note-aware approve labels in preview approval controls', () => {
    const onResolveInteraction = vi.fn();
    renderPanels({
      interaction: {
        interaction_id: 'interaction-note',
        interaction_kind: 'approval_request',
        status: 'pending',
        request_schema_version: 'helpin.v1',
        request_payload: {
          phase: 'prd',
          preview_panel_key: 'prd_draft',
          title: 'Approve PRD',
        },
      } satisfies CodingSessionInteraction,
      onResolveInteraction,
    });

    expect(container.textContent).toContain('Sent with your decision.');

    const textarea = container.querySelector('textarea');
    expect(textarea).toBeTruthy();
    act(() => {
      const valueSetter = Object.getOwnPropertyDescriptor(window.HTMLTextAreaElement.prototype, 'value')?.set;
      valueSetter?.call(textarea, 'Approved, but keep the rollout staged.');
      textarea!.dispatchEvent(new Event('input', { bubbles: true }));
    });

    clickButton('Approve with note');

    expect(onResolveInteraction).toHaveBeenCalledWith(
      'interaction-note',
      {
        decision: 'approve',
        message: 'Approved, but keep the rollout staged.',
      },
      'Approved, but keep the rollout staged.',
    );
  });

  it('renders request changes for task-plan-doc style generic previews', () => {
    const onResolveInteraction = vi.fn();
    renderPanels({
      previews: new Map<string, PublishedPreview>([
        ['task_plan_doc', {
          panelKey: 'task_plan_doc',
          title: 'Task Planning Document',
          format: 'markdown',
          content: '# Outcome\nPlan draft',
          replace: true,
          surroundingText: '',
        }],
      ]),
      interaction: {
        interaction_id: 'interaction-2',
        interaction_kind: 'approval_request',
        status: 'pending',
        request_schema_version: 'helpin.v1',
        request_payload: {
          phase: 'task_doc',
          preview_panel_key: 'task_plan_doc',
          title: 'Approve planning document',
        },
      } satisfies CodingSessionInteraction,
      onResolveInteraction,
    });

    clickButton('Request changes');

    expect(onResolveInteraction).toHaveBeenCalledWith(
      'interaction-2',
      { decision: 'request_changes' },
      undefined,
    );
  });

  it('renders the PRD expand action and opens the dialog', () => {
    renderPanels();

    clickButtonByAriaLabel('Open PRD Draft');

    expect(document.body.querySelector('[role="dialog"]')).toBeTruthy();
    expect(document.body.textContent).toContain('Draft body');
  });

  it('renders the task-plan expand action and opens the dialog', () => {
    renderPanels({
      previews: new Map<string, PublishedPreview>([
        ['task_plan', {
          panelKey: 'task_plan',
          title: 'Task Plan',
          format: 'json',
          content: {
            summary: 'Plan summary',
            proposed_tasks: [
              { ref: 'T1', title: 'Ship faster', description: 'Do the work' },
            ],
          },
          replace: true,
          surroundingText: '',
        }],
      ]),
    });

    clickButtonByAriaLabel('Open Task Plan');

    expect(document.body.querySelector('[role="dialog"]')).toBeTruthy();
    expect(document.body.textContent).toContain('Ship faster');
  });

  it('attaches story-plan approval aliases to the normalized task-plan preview', () => {
    const onResolveInteraction = vi.fn();
    renderPanels({
      previews: new Map<string, PublishedPreview>([
        ['task_plan', {
          panelKey: 'task_plan',
          title: 'Task Plan',
          format: 'json',
          content: {
            summary: 'Plan summary',
            proposed_tasks: [
              { ref: 'T1', title: 'Ship faster', description: 'Do the work' },
            ],
          },
          replace: true,
          surroundingText: '',
        }],
      ]),
      interaction: {
        interaction_id: 'interaction-story-plan',
        interaction_kind: 'approval_request',
        status: 'pending',
        request_schema_version: 'helpin.v1',
        request_payload: {
          phase: 'plan',
          preview_panel_key: 'story_plan',
          title: 'Approve task plan',
          summary: 'Review the latest plan.',
        },
      } satisfies CodingSessionInteraction,
      onResolveInteraction,
    });

    expect(container.textContent).toContain('Task Plan');
    expect(container.textContent).toContain('Approve task plan');

    clickButton('Approve');

    expect(onResolveInteraction).toHaveBeenCalledWith(
      'interaction-story-plan',
      { decision: 'approve' },
      undefined,
    );
  });
});
