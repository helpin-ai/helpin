// @vitest-environment jsdom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { CodingInteractionCard } from '../CodingInteractionCard';
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

function buildInteraction(overrides: Partial<CodingSessionInteraction> = {}): CodingSessionInteraction {
  return {
    interaction_id: 'interaction-1',
    interaction_kind: 'review_checkpoint',
    status: 'pending',
    request_schema_version: 'helpin.v1',
    request_payload: {
      phase: 'code',
      title: 'Review findings',
      findings: [
        {
          id: 'finding_1',
          title: 'Missing nil guard',
          body: 'A nil client will panic in the retry path.',
          priority: 'P1',
          confidence: 'high',
          code_location: 'server/internal/service/foo.go:42',
        },
        {
          id: 'finding_2',
          title: 'Missing regression test',
          body: 'The new branch is not covered by tests.',
          priority: 'P2',
          confidence: 'medium',
          code_location: 'server/internal/service/foo_test.go:10',
        },
      ],
      overall_correctness: 'incorrect',
      overall_explanation: 'Two issues need follow-up before this change is safe.',
      overall_confidence_score: 0.93,
    },
    ...overrides,
  };
}

function renderCard(
  onResolve: (interactionId: string, responsePayload: Record<string, unknown>, followupMessage?: string) => void,
  interaction?: CodingSessionInteraction,
  options: {
    availablePreviewPanelKey?: string | null;
    attachedPreview?: PublishedPreview | null;
    onViewPreview?: (panelKey: string) => void;
    compact?: boolean;
  } = {},
) {
  act(() => {
    root.render(
      <CodingInteractionCard
        interaction={interaction ?? buildInteraction()}
        acting={null}
        onResolve={onResolve}
        compact={options.compact}
        availablePreviewPanelKey={options.availablePreviewPanelKey}
        attachedPreview={options.attachedPreview}
        onViewPreview={options.onViewPreview}
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

function findButton(label: string) {
  return Array.from(container.querySelectorAll('button')).find((candidate) => candidate.textContent?.trim() === label);
}

function typeTextarea(value: string) {
  const textarea = container.querySelector('textarea');
  expect(textarea).toBeTruthy();
  act(() => {
    const valueSetter = Object.getOwnPropertyDescriptor(window.HTMLTextAreaElement.prototype, 'value')?.set;
    valueSetter?.call(textarea, value);
    textarea!.dispatchEvent(new Event('input', { bubbles: true }));
  });
}

describe('CodingInteractionCard', () => {
  it('renders structured review findings and enables one approve action after findings are selected', () => {
    const onResolve = vi.fn();
    renderCard(onResolve);

    expect(container.textContent).toContain('Review findings');
    expect(container.textContent).toContain('Missing nil guard');
    expect(container.textContent).toContain('Missing regression test');
    expect(container.textContent).toContain('93% confidence');
    expect(findButton('Approve all')).toBeUndefined();
    expect(findButton('Approve selected')).toBeUndefined();
    expect(findButton('Approve')).toBeTruthy();
    expect(findButton('Approve')?.hasAttribute('disabled')).toBe(true);

    const checkboxes = Array.from(container.querySelectorAll('[data-slot="checkbox"]'));
    expect(checkboxes).toHaveLength(3);
    act(() => {
      checkboxes[0]!.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    });

    expect(findButton('Approve')?.hasAttribute('disabled')).toBe(false);
    clickButton('Approve');

    expect(onResolve).toHaveBeenCalledWith(
      'interaction-1',
      {
        decision: 'approve',
        selection_mode: 'selected',
        selected_finding_ids: ['finding_1', 'finding_2'],
      },
      undefined,
    );
  });

  it('sends selected finding ids when approving any selected subset', () => {
    const onResolve = vi.fn();
    renderCard(onResolve);

    const checkboxes = Array.from(container.querySelectorAll('[data-slot="checkbox"]'));
    expect(checkboxes).toHaveLength(3);

    act(() => {
      checkboxes[1]!.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    });

    clickButton('Approve');

    expect(onResolve).toHaveBeenCalledWith(
      'interaction-1',
      {
        decision: 'approve',
        selection_mode: 'selected',
        selected_finding_ids: ['finding_1'],
      },
      undefined,
    );
  });

  it('sends the optional note when approving selected findings', () => {
    const onResolve = vi.fn();
    renderCard(onResolve);

    const checkboxes = Array.from(container.querySelectorAll('[data-slot="checkbox"]'));
    act(() => {
      checkboxes[1]!.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    });
    typeTextarea('Use the smaller patch.');

    clickButton('Approve');

    expect(onResolve).toHaveBeenCalledWith(
      'interaction-1',
      {
        decision: 'approve',
        message: 'Use the smaller patch.',
        selection_mode: 'selected',
        selected_finding_ids: ['finding_1'],
      },
      'Use the smaller patch.',
    );
  });

  it('sends a skip decision with the optional note', () => {
    const onResolve = vi.fn();
    renderCard(onResolve);

    typeTextarea('Not worth changing for this run.');
    clickButton('Skip');

    expect(onResolve).toHaveBeenCalledWith(
      'interaction-1',
      {
        decision: 'skip',
        message: 'Not worth changing for this run.',
        selection_mode: 'none',
        selected_finding_ids: [],
      },
      'Not worth changing for this run.',
    );
  });

  it('renders structured review findings even when finding ids are omitted', () => {
    const onResolve = vi.fn();
    renderCard(onResolve, buildInteraction({
      request_payload: {
        phase: 'code',
        title: 'Review findings',
        findings: [
          {
            title: 'Missing nil guard',
            body: 'A nil client will panic in the retry path.',
            priority: 'P1',
            code_location: 'server/internal/service/foo.go:42',
          },
          {
            title: 'Missing regression test',
            body: 'The new branch is not covered by tests.',
            priority: 'P2',
            code_location: 'server/internal/service/foo_test.go:10',
          },
        ],
        overall_correctness: 'incorrect',
      },
    }));

    expect(container.textContent).toContain('Missing nil guard');
    expect(container.textContent).toContain('Missing regression test');

    const checkboxes = Array.from(container.querySelectorAll('[data-slot="checkbox"]'));
    expect(checkboxes).toHaveLength(3);
    act(() => {
      checkboxes[0]!.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    });

    clickButton('Approve');

    expect(onResolve).toHaveBeenCalledWith(
      'interaction-1',
      {
        decision: 'approve',
        selection_mode: 'selected',
        selected_finding_ids: ['finding_1', 'finding_2'],
      },
      undefined,
    );
  });

  it('renders Codex user-input prompts as markdown without duplicating matching summaries', () => {
    const prompt = [
      'The Sentry error indicates an object with keys `{message, errors}` is being rendered.',
      '',
      '```tsx',
      'setError(err.response.data.detail)',
      '```',
    ].join('\n');
    const onResolve = vi.fn();

    renderCard(onResolve, buildInteraction({
      interaction_kind: 'request_user_input',
      request_schema_version: 'codex.v2',
      title: 'User input required',
      summary: prompt,
      request_payload: {
        questions: [
          {
            id: 'details',
            header: 'Context',
            question: prompt,
            options: [],
          },
        ],
      },
    }));

    expect(container.textContent?.match(/The Sentry error indicates/g)).toHaveLength(1);
    expect(container.querySelector('code')?.textContent).toBe('{message, errors}');
    expect(container.querySelector('pre code')?.textContent).toContain('setError(err.response.data.detail)');
    expect(container.textContent).not.toContain('`{message, errors}`');
    expect(container.textContent).not.toContain('```tsx');
  });

  it('expands approval preview inline and exposes full preview from the approval header', () => {
    const onResolve = vi.fn();
    const onViewPreview = vi.fn();
    renderCard(onResolve, buildInteraction({
      interaction_kind: 'approval_request',
      title: 'Approve planning document',
      summary: 'Review the proposed task document.',
      request_payload: {
        phase: 'task_doc',
        preview_panel_key: 'task_plan_doc',
        title: 'Approve planning document',
      },
    }), {
      availablePreviewPanelKey: 'task_plan_doc',
      attachedPreview: {
        panelKey: 'task_plan_doc',
        title: 'Task Planning Document',
        format: 'markdown',
        content: [
          '# Outcome',
          'Short proposed plan.',
          '',
          'This approval preview should show enough context for the reviewer to understand the decision without opening a separate panel. It summarizes the proposed implementation, the validation path, and the expected risk level. The collapsed state keeps the approval card compact while preserving the first useful paragraphs. Additional detail follows after the initial context so the reviewer can expand only when they need the rest of the document.',
          '',
          'The implementation keeps decision controls close to the document summary and avoids forcing a round trip to the full preview for routine approvals.',
          '',
          '## Open questions',
          'Should the fix include a migration?',
        ].join('\n'),
        replace: true,
        surroundingText: '',
      },
      onViewPreview,
      compact: true,
    });

    expect(container.firstElementChild?.className).toContain('border-amber-400/60');
    expect(container.firstElementChild?.className).not.toContain('border-l-2');
    expect(container.textContent).not.toContain('task_doc approval');
    expect(container.textContent).not.toContain('Review the proposed task document.');
    expect(container.textContent).toContain('Document preview');
    expect(container.textContent).toContain('Short proposed plan.');
    expect(container.textContent).not.toContain('Should the fix include a migration?');
    expect(container.textContent).toContain('Open full preview');
    expect(container.querySelector('[data-coding-session-approval-preview-fade]')).toBeTruthy();

    clickButton('Show full preview');

    expect(container.textContent).toContain('Should the fix include a migration?');
    expect(container.textContent).toContain('Collapse preview');
    expect(container.querySelector('[data-coding-session-approval-preview-fade]')).toBeNull();

    clickButton('Open full preview');

    expect(onViewPreview).toHaveBeenCalledWith('task_plan_doc');
  });

  it('renders structured approval JSON previews without dumping raw JSON first', () => {
    const onResolve = vi.fn();
    renderCard(onResolve, buildInteraction({
      interaction_kind: 'approval_request',
      title: 'Approve task plan',
      request_payload: {
        phase: 'plan',
        preview_panel_key: 'task_plan',
        title: 'Approve task plan',
      },
    }), {
      attachedPreview: {
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
      },
      compact: true,
    });

    expect(container.textContent).toContain('Plan summary');
    expect(container.textContent).toContain('Ship faster');
    expect(container.textContent).not.toContain('"proposed_tasks"');
  });

  it('labels approve as approve with note and sends the note with an approval decision', () => {
    const onResolve = vi.fn();
    renderCard(onResolve, buildInteraction({
      interaction_kind: 'approval_request',
      title: 'Approve PRD',
      request_payload: {
        phase: 'prd',
        title: 'Approve PRD',
      },
    }));

    typeTextarea('Looks good. Keep the scope narrow.');

    const noteInput = container.querySelector('textarea');
    expect(noteInput?.className).toContain('focus-visible:border-ring/70');
    expect(noteInput?.className).toContain('focus-visible:ring-ring/15');

    const approvalActions = container.querySelector('[data-coding-session-approval-actions]');
    expect(approvalActions?.className).toContain('mt-5');
    expect(approvalActions?.className).toContain('pt-4');
    expect(approvalActions?.className).toContain('border-t');

    clickButton('Approve with note');

    expect(onResolve).toHaveBeenCalledWith(
      'interaction-1',
      {
        decision: 'approve',
        message: 'Looks good. Keep the scope narrow.',
      },
      'Looks good. Keep the scope narrow.',
    );
  });

  it('shows permission approvals as readable permissions with raw details collapsed', () => {
    const onResolve = vi.fn();
    renderCard(onResolve, buildInteraction({
      interaction_kind: 'permissions_approval',
      title: 'Approve workspace access',
      request_payload: {
        reason: 'The agent needs to inspect project files.',
        permissions: {
          filesystem: 'workspace-write',
          network: false,
        },
      },
    }));

    expect(container.textContent).toContain('filesystem');
    expect(container.textContent).toContain('workspace-write');
    expect(container.textContent).toContain('network');
    expect(container.textContent).toContain('Not allowed');
    expect(container.querySelector('details summary')?.textContent).toContain('Raw details');
  });

  it('uses destructive treatment for command denial actions', () => {
    const onResolve = vi.fn();
    renderCard(onResolve, buildInteraction({
      interaction_kind: 'command_execution_approval',
      title: 'Approve command execution',
      request_payload: {
        command: 'npm test -- CodingInteractionCard.test.tsx',
        reason: 'Verify the card behavior.',
        availableDecisions: ['accept', 'decline', 'cancel'],
      },
    }));

    const textarea = container.querySelector('textarea');
    expect(textarea?.className).toContain('focus-visible:border-ring/70');

    const declineButton = Array.from(container.querySelectorAll('button')).find((button) => button.textContent?.trim() === 'Decline');
    const cancelButton = Array.from(container.querySelectorAll('button')).find((button) => button.textContent?.trim() === 'Cancel turn');
    expect(declineButton?.className).toContain('hover:bg-destructive/10');
    expect(cancelButton?.className).toContain('hover:bg-destructive/10');
  });
});
