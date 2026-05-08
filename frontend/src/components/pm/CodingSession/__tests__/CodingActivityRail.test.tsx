// @vitest-environment jsdom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';

import { CodingActivityRail } from '../CodingActivityRail';
import type { CodingSessionEvent } from '@/lib/pmTypes';

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

function buildEvent(overrides: Partial<CodingSessionEvent> & Pick<CodingSessionEvent, 'id' | 'type' | 'sequence_no'>): CodingSessionEvent {
  return {
    id: overrides.id,
    session_id: 'session-1',
    run_id: 'run-1',
    sequence_no: overrides.sequence_no,
    timestamp: overrides.timestamp ?? '2026-04-21T09:00:00Z',
    type: overrides.type,
    runtime_kind: overrides.runtime_kind ?? 'codex',
    payload: overrides.payload ?? {},
    runtime_metadata: overrides.runtime_metadata,
  };
}

describe('CodingActivityRail', () => {
  it('keeps bottom breathing room in the scrollable activity rail', () => {
    act(() => {
      root.render(<CodingActivityRail events={[]} completedToolCalls={[]} />);
    });

    const scrollRegion = container.querySelector('[data-coding-session-activity-rail-scroll]');
    expect(scrollRegion?.className).toContain('pb-[calc(env(safe-area-inset-bottom)+4rem)]');
  });

  it('renders persisted review findings artifacts as readable history cards', () => {
    const event = buildEvent({
      id: 'artifact:review-findings-1',
      type: 'review.findings.updated',
      sequence_no: 12,
      runtime_metadata: { source: 'agent_run_artifact', artifact_type: 'review_findings' },
      payload: {
        artifact_id: 'artifact-review-findings-1',
        artifact_type: 'review_findings',
        content: {
          phase: 'code',
          title: 'Lens review findings',
          overall_correctness: 'incorrect',
          overall_explanation: 'Two issues remain before this is safe to merge.',
          overall_confidence_score: 0.91,
          findings: [
            {
              id: 'finding_1',
              title: 'Nil panic in retry path',
              body: 'The retry branch dereferences a nil client.',
              priority: 'P1',
              confidence: 0.95,
              code_location: 'server/internal/service/foo.go:42',
            },
            {
              id: 'finding_2',
              title: 'Missing regression coverage',
              body: 'The new branch has no test coverage.',
              priority: 'P2',
              confidence: 0.76,
              code_location: 'server/internal/service/foo_test.go:10',
            },
          ],
        },
      },
    });

    act(() => {
      root.render(<CodingActivityRail events={[event]} completedToolCalls={[]} />);
    });

    expect(container.textContent).toContain('Lens review findings');
    expect(container.textContent).toContain('incorrect');
    expect(container.textContent).toContain('91% confidence');
    expect(container.textContent).toContain('Nil panic in retry path');
    expect(container.textContent).toContain('Missing regression coverage');
    expect(container.textContent).toContain('server/internal/service/foo.go:42');
  });

  it('renders persisted review findings history even when finding ids are omitted', () => {
    const event = buildEvent({
      id: 'artifact:review-findings-2',
      type: 'review.findings.updated',
      sequence_no: 13,
      runtime_metadata: { source: 'agent_run_artifact', artifact_type: 'review_findings' },
      payload: {
        artifact_id: 'artifact-review-findings-2',
        artifact_type: 'review_findings',
        content: {
          title: 'Lens review findings',
          findings: [
            {
              title: 'Nil panic in retry path',
              body: 'The retry branch dereferences a nil client.',
              priority: 'P1',
              confidence: 0.95,
              code_location: 'server/internal/service/foo.go:42',
            },
          ],
        },
      },
    });

    act(() => {
      root.render(<CodingActivityRail events={[event]} completedToolCalls={[]} />);
    });

    expect(container.textContent).toContain('Lens review findings');
    expect(container.textContent).toContain('Nil panic in retry path');
    expect(container.textContent).toContain('server/internal/service/foo.go:42');
  });

  it('renders persisted review findings history when only an overall verdict is present', () => {
    const event = buildEvent({
      id: 'artifact:review-findings-3',
      type: 'review.findings.updated',
      sequence_no: 14,
      runtime_metadata: { source: 'agent_run_artifact', artifact_type: 'review_findings' },
      payload: {
        artifact_id: 'artifact-review-findings-3',
        artifact_type: 'review_findings',
        content: {
          phase: 'implementation',
          title: 'Producer Prometheus docs and alert rules added',
          summary: 'Implemented the three approved findings by adding templated producer alert rules, a runbook, and promtool unit tests.',
          overall_correctness: 'correct',
          overall_explanation: 'The requested deliverables now exist on the branch and validation passed.',
          overall_confidence_score: 0.96,
        },
      },
    });

    act(() => {
      root.render(<CodingActivityRail events={[event]} completedToolCalls={[]} />);
    });

    expect(container.textContent).toContain('Producer Prometheus docs and alert rules added');
    expect(container.textContent).toContain('Implemented the three approved findings');
    expect(container.textContent?.toLowerCase()).toContain('correct');
    expect(container.textContent).toContain('96% confidence');
  });
});
