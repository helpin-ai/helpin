// @vitest-environment jsdom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';

import { CodingReviewHistoryPanel } from '../CodingReviewHistoryPanel';
import type { AgentRunArtifact } from '@/lib/pmTypes';

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

function buildReviewArtifact(overrides: Partial<AgentRunArtifact> = {}): AgentRunArtifact {
  return {
    id: 'artifact-1',
    workspace_id: 'ws-1',
    run_id: 'run-1',
    artifact_type: 'review_findings',
    format: 'json',
    storage_mode: 'inline',
    inline_content: JSON.stringify({
      findings: [],
    }),
    metadata: {},
    sequence_no: 1,
    created_at: '2026-03-31T10:00:00Z',
    ...overrides,
  };
}

describe('CodingReviewHistoryPanel', () => {
  it('keeps long review finding code locations inside the panel', () => {
    const longCodeLocation = 'frontend/src/components/pm/CodingSession/review-history/very/deep/path/that/should/not/create-horizontal-scroll/CodingReviewHistoryPanel.tsx:1234';

    act(() => {
      root.render(
        <CodingReviewHistoryPanel
          reviewArtifacts={[{
            artifact: buildReviewArtifact({
              inline_content: JSON.stringify({
                findings: [
                  {
                    id: 'finding_with_extremely_long_identifier_that_should_wrap',
                    title: 'Long review metadata should stay inside the panel',
                    body: 'The review history card should not force horizontal page scrolling.',
                    priority: 'P1',
                    code_location: longCodeLocation,
                  },
                ],
              }),
            }),
            decisionArtifact: null,
          }]}
        />,
      );
    });

    const reviewPanel = container.querySelector('[data-coding-session-review-history-panel]');
    const codeLocation = Array.from(container.querySelectorAll<HTMLElement>('code'))
      .find((node) => node.textContent === longCodeLocation);

    expect(reviewPanel?.className).toContain('overflow-hidden');
    expect(codeLocation?.className).toContain('break-all');
  });
});
