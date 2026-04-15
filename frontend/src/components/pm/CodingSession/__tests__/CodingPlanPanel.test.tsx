// @vitest-environment jsdom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';

import { CodingPlanPanel } from '../CodingPlanPanel';
import type { AgentRunStatus, RunPlanArtifact } from '@/lib/pmTypes';

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

function render(plan: RunPlanArtifact | null, runStatus?: AgentRunStatus) {
  act(() => {
    root.render(<CodingPlanPanel plan={plan} runStatus={runStatus} />);
  });
}

const stalePlan: RunPlanArtifact = {
  note: 'Tracing the bug.',
  plan: [
    { step: 'Reproduce', status: 'in_progress' },
    { step: 'Patch the code', status: 'pending' },
    { step: 'Run validation', status: 'pending' },
  ],
};

const completedPlan: RunPlanArtifact = {
  plan: [
    { step: 'Reproduce', status: 'completed' },
    { step: 'Patch', status: 'completed' },
  ],
};

describe('CodingPlanPanel', () => {
  describe('live run (non-terminal)', () => {
    it('shows a spinner for in_progress steps and the running counter', () => {
      render(stalePlan, 'running');
      expect(container.textContent).toContain('0/3 steps');
      expect(container.querySelectorAll('.animate-spin').length).toBe(1);
      expect(container.textContent).not.toContain('not finalized');
    });

    it('uses the running counter when runStatus is omitted (backward compat)', () => {
      render(stalePlan);
      expect(container.textContent).toContain('0/3 steps');
      expect(container.querySelectorAll('.animate-spin').length).toBe(1);
    });
  });

  describe('completed run with stale plan', () => {
    it('stops the spinner, relabels the badge, and shows the "not finalized" note', () => {
      render(stalePlan, 'completed');

      expect(container.textContent).toContain('Plan not finalized');
      expect(container.querySelectorAll('.animate-spin').length).toBe(0);
      expect(container.textContent).toContain('Agent finished without publishing a final plan update');
      expect(container.textContent).not.toContain('0/3 steps');
    });

    it('collapses the details pane by default when the plan is stale', () => {
      render(stalePlan, 'completed');
      const details = container.querySelector('details');
      expect(details).not.toBeNull();
      expect(details!.open).toBe(false);
    });

    it('keeps the honest "N/M steps" counter when the agent DID finalize', () => {
      render(completedPlan, 'completed');
      expect(container.textContent).toContain('2/2 steps');
      expect(container.textContent).not.toContain('not finalized');
    });
  });

  describe('failed run', () => {
    it('shows a Run failed badge and cancels in_progress/pending steps visually', () => {
      render(stalePlan, 'failed');
      expect(container.textContent).toContain('Run failed');
      expect(container.textContent).toContain('Run ended before the agent could finish its plan.');
      expect(container.querySelectorAll('.animate-spin').length).toBe(0);
    });
  });

  describe('cancelled run', () => {
    it('shows a Run cancelled badge and cancels remaining steps', () => {
      render(stalePlan, 'cancelled');
      expect(container.textContent).toContain('Run cancelled');
      expect(container.textContent).toContain('Run ended before the agent could finish its plan.');
      expect(container.querySelectorAll('.animate-spin').length).toBe(0);
    });
  });

  describe('empty plan', () => {
    it('shows a Waiting placeholder on live runs', () => {
      render(null, 'running');
      expect(container.textContent).toContain('Waiting');
      expect(container.textContent).toContain('Waiting for the agent to publish its first plan update');
    });

    it('shows a terminal placeholder when the run ended without a plan', () => {
      render(null, 'completed');
      expect(container.textContent).toContain('Agent ended the run without publishing a plan');
    });
  });
});
