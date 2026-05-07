import { describe, expect, it } from 'vitest';

import {
  shouldShowCodingSessionPlanPanel,
  shouldShowCodingSessionSidePanel,
} from '../codingSessionLayout';
import type { RunPlanArtifact } from '@/lib/pmTypes';

const populatedPlan: RunPlanArtifact = {
  plan: [{ step: 'Inspect repository', status: 'in_progress' }],
};

describe('coding session layout decisions', () => {
  it('hides the plan panel until the agent publishes a real plan', () => {
    expect(shouldShowCodingSessionPlanPanel(null)).toBe(false);
    expect(shouldShowCodingSessionPlanPanel({ plan: [] })).toBe(false);
    expect(shouldShowCodingSessionPlanPanel(populatedPlan)).toBe(true);
  });

  it('keeps the side panel hidden when there is no plan or preview content', () => {
    expect(shouldShowCodingSessionSidePanel(null, 0)).toBe(false);
  });

  it('shows the side panel for plans or preview content', () => {
    expect(shouldShowCodingSessionSidePanel(populatedPlan, 0)).toBe(true);
    expect(shouldShowCodingSessionSidePanel(null, 1)).toBe(true);
  });
});
