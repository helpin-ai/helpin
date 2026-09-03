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
    approvalStatesByPreviewKey?: Map<string, {
      interaction: CodingSessionInteraction;
      status: 'pending' | 'approved' | 'changes_requested';
      title?: string;
      summary?: string;
      previewPanelKey: string;
      resolvedAt?: string;
      resolvedBy?: string;
      note?: string;
    }>;
    resolverNamesByUserId?: Map<string, string>;
    onReviewApproval?: (interactionId: string) => void;
    openPreviewPanelKey?: string | null;
    openPreviewRequestId?: number;
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
        approvalStatesByPreviewKey={options.approvalStatesByPreviewKey}
        resolverNamesByUserId={options.resolverNamesByUserId}
        onReviewApproval={options.onReviewApproval}
        openPreviewPanelKey={options.openPreviewPanelKey}
        openPreviewRequestId={options.openPreviewRequestId}
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
  it('renders preview-bound approvals as transcript navigation instead of duplicate decisions', () => {
    const onReviewApproval = vi.fn();
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
      onReviewApproval,
    });

    expect(container.textContent).toContain('Approve PRD');
    expect(container.textContent).toContain('Review the latest draft.');
    expect(container.textContent).toContain('Waiting for approval');
    expect(container.textContent).toContain('Open doc');
    expect(container.textContent).toContain('Review');
    expect(container.textContent).not.toContain('Draft body');
    expect(container.textContent).not.toContain('Optional note');
    expect(container.textContent).not.toContain('Request changes');
    expect(container.textContent!.indexOf('Approve PRD')).toBeLessThan(
      container.textContent!.indexOf('Waiting for approval'),
    );
    const reviewButton = Array.from(container.querySelectorAll('button')).find((candidate) => candidate.textContent?.trim() === 'Review');
    const openDocButton = Array.from(container.querySelectorAll('button')).find((candidate) => candidate.textContent?.trim() === 'Open doc');
    expect(reviewButton?.className).toContain('bg-primary/10');
    expect(openDocButton?.className).toContain('border-border/70');

    clickButton('Open doc');

    expect(document.body.querySelector('[role="dialog"]')).toBeTruthy();
    expect(document.body.textContent).toContain('Draft body');

    act(() => {
      root.render(null);
    });
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
      onReviewApproval,
    });

    clickButton('Review');

    expect(onReviewApproval).toHaveBeenCalledWith('interaction-1');
  });

  it('opens a requested preview dialog from outside the right pane', () => {
    renderPanels({
      openPreviewPanelKey: 'prd_draft',
      openPreviewRequestId: 1,
    });

    expect(document.body.querySelector('[role="dialog"]')).toBeTruthy();
    expect(document.body.textContent).toContain('Draft body');
  });

  it('shows task-plan-doc approvals as compact actions without duplicating the preview body', () => {
    const onReviewApproval = vi.fn();
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
      onReviewApproval,
    });

    expect(container.textContent).toContain('Approve planning document');
    expect(container.textContent).toContain('Waiting for approval');
    expect(container.textContent).toContain('Open doc');
    expect(container.textContent).toContain('Review');
    expect(container.textContent).not.toContain('Plan draft');

    clickButton('Open doc');

    expect(document.body.querySelector('[role="dialog"]')).toBeTruthy();
    expect(document.body.textContent).toContain('Plan draft');

    clickButton('Review');

    expect(onReviewApproval).toHaveBeenCalledWith('interaction-2');
  });

  it('keeps resolved task document approvals compact in the right pane', () => {
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
      approvalStatesByPreviewKey: new Map([
        ['task_plan_doc', {
          interaction: {
            interaction_id: 'interaction-3',
            interaction_kind: 'approval_request',
            status: 'resolved',
            request_schema_version: 'helpin.v1',
            request_payload: {
              phase: 'task_doc',
              preview_panel_key: 'task_plan_doc',
              title: 'Approve planning document',
            },
            response_payload: {
              decision: 'approve',
              message: 'Looks ready.',
            },
            resolved_at: '2026-05-07T10:00:00Z',
            resolved_by: 'user-1',
          },
          status: 'approved',
          title: 'Approve planning document',
          previewPanelKey: 'task_plan_doc',
          resolvedAt: '2026-05-07T10:00:00Z',
          resolvedBy: 'user-1',
          note: 'Looks ready.',
        }],
      ]),
      resolverNamesByUserId: new Map([['user-1', 'John Doe']]),
    });

    expect(container.textContent).toContain('Approve planning document');
    expect(container.textContent).toContain('Approved');
    expect(container.textContent).toContain('John Doe approved');
    expect(container.textContent).toContain('Looks ready.');
    expect(container.textContent).toContain('Open doc');
    expect(container.textContent).not.toContain('Review');
    expect(container.textContent).not.toContain('Plan draft');

    clickButton('Open doc');

    expect(document.body.querySelector('[role="dialog"]')).toBeTruthy();
    expect(document.body.textContent).toContain('Plan draft');
  });

  it('shows requested changes for resolved task document approvals', () => {
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
      approvalStatesByPreviewKey: new Map([
        ['task_plan_doc', {
          interaction: {
            interaction_id: 'interaction-4',
            interaction_kind: 'approval_request',
            status: 'resolved',
            request_schema_version: 'helpin.v1',
            request_payload: {
              phase: 'task_doc',
              preview_panel_key: 'task_plan_doc',
              title: 'Approve planning document',
            },
            response_payload: {
              decision: 'request_changes',
              message: 'Split this into two smaller tasks.',
            },
            resolved_at: '2026-05-07T10:00:00Z',
            resolved_by: 'user-2',
          },
          status: 'changes_requested',
          title: 'Approve planning document',
          previewPanelKey: 'task_plan_doc',
          resolvedAt: '2026-05-07T10:00:00Z',
          resolvedBy: 'user-2',
          note: 'Split this into two smaller tasks.',
        }],
      ]),
      resolverNamesByUserId: new Map([['user-2', 'Priya Singh']]),
    });

    expect(container.textContent).toContain('Changes requested');
    expect(container.textContent).toContain('Priya Singh requested changes');
    expect(container.textContent).toContain('Split this into two smaller tasks.');
    expect(container.textContent).toContain('Open doc');
    expect(container.textContent).not.toContain('Review');
    expect(container.textContent).not.toContain('Plan draft');
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

  it('attaches task-plan approval to the normalized task-plan preview', () => {
    const onReviewApproval = vi.fn();
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
        interaction_id: 'interaction-task-plan',
        interaction_kind: 'approval_request',
        status: 'pending',
        request_schema_version: 'helpin.v1',
        request_payload: {
          phase: 'plan',
          preview_panel_key: 'task_plan',
          title: 'Approve task plan',
          summary: 'Review the latest plan.',
        },
      } satisfies CodingSessionInteraction,
      onReviewApproval,
    });

    expect(container.textContent).toContain('Approve task plan');
    expect(container.textContent).toContain('Waiting for approval');
    expect(container.textContent).not.toContain('Ship faster');

    clickButton('Review');

    expect(onReviewApproval).toHaveBeenCalledWith('interaction-task-plan');
  });
});
