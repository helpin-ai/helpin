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
  } = {},
) {
  act(() => {
    root.render(
      <CodingInteractionCard
        interaction={interaction ?? buildInteraction()}
        acting={null}
        onResolve={onResolve}
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
  it('renders structured review findings and approves all by default', () => {
    const onResolve = vi.fn();
    renderCard(onResolve);

    expect(container.textContent).toContain('Review findings');
    expect(container.textContent).toContain('Missing nil guard');
    expect(container.textContent).toContain('Missing regression test');
    expect(container.textContent).toContain('93% confidence');

    clickButton('Approve all');

    expect(onResolve).toHaveBeenCalledWith(
      'interaction-1',
      {
        decision: 'approve',
        selection_mode: 'all',
      },
      undefined,
    );
  });

  it('sends selected finding ids when approving a subset', () => {
    const onResolve = vi.fn();
    renderCard(onResolve);

    const checkboxes = Array.from(container.querySelectorAll('[data-slot="checkbox"]'));
    expect(checkboxes).toHaveLength(3);

    act(() => {
      checkboxes[1]!.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    });

    clickButton('Approve selected');

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

    clickButton('Approve selected');

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
    });

    expect(container.textContent).toContain('Document preview');
    expect(container.textContent).toContain('Short proposed plan.');
    expect(container.textContent).not.toContain('Should the fix include a migration?');
    expect(container.textContent).toContain('Full preview');

    clickButton('Show more');

    expect(container.textContent).toContain('Should the fix include a migration?');

    clickButton('Full preview');

    expect(onViewPreview).toHaveBeenCalledWith('task_plan_doc');
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
});
