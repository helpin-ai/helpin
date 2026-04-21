// @vitest-environment jsdom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { CodingInteractionCard } from '../CodingInteractionCard';
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
) {
  act(() => {
    root.render(
      <CodingInteractionCard
        interaction={interaction ?? buildInteraction()}
        acting={null}
        onResolve={onResolve}
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
});
