import { describe, expect, it, vi } from 'vitest';

import type { PMAutomation } from '@/lib/pmTypes';
import {
  buildSprintAutomationDismissRequest,
  dismissSprintAutomationPrompt,
  shouldPromptSprintAutomation,
} from '../sprintAutomationPrompt';

const teamId = 'team-123';
const workspaceId = 'workspace-456';

const makeAutomation = (overrides: Partial<PMAutomation> = {}): PMAutomation => ({
  id: 'automation-1',
  workspace_id: workspaceId,
  automation_type: 'sprint_auto_create',
  enabled: true,
  team_id: teamId,
  config_int: 2,
  config_int2: 3,
  config_int3: 1,
  created_at: '2026-03-17T00:00:00Z',
  updated_at: '2026-03-17T00:00:00Z',
  ...overrides,
});

const prompt = {
  teamId,
  teamName: 'Engineering',
  sprintCount: 2,
  weeks: 3,
  startDay: 1,
  moveUnfinished: true,
} as const;

describe('sprintAutomationPrompt helpers', () => {
  it('shows the prompt when no sprint_auto_create automation exists', () => {
    expect(shouldPromptSprintAutomation([], teamId)).toBe(true);
  });

  it('shows the prompt when automations are undefined', () => {
    expect(shouldPromptSprintAutomation(undefined, teamId)).toBe(true);
  });

  it('suppresses the prompt when a disabled sprint_auto_create automation exists for the team', () => {
    expect(shouldPromptSprintAutomation([makeAutomation({ enabled: false })], teamId)).toBe(false);
  });

  it('suppresses the prompt when an enabled sprint_auto_create automation exists for the team', () => {
    expect(shouldPromptSprintAutomation([makeAutomation({ enabled: true })], teamId)).toBe(false);
  });

  it('still shows the prompt for a different team or automation type', () => {
    expect(
      shouldPromptSprintAutomation(
        [makeAutomation({ team_id: 'team-other' }), makeAutomation({ automation_type: 'epic_auto_start' })],
        teamId,
      ),
    ).toBe(true);
  });

  it('builds a disabled sprint_auto_create dismissal request from the prompt', () => {
    expect(buildSprintAutomationDismissRequest(workspaceId, prompt)).toEqual({
      workspace_id: workspaceId,
      automation_type: 'sprint_auto_create',
      enabled: false,
      team_id: teamId,
      config_int: 2,
      config_int2: 3,
      config_int3: 1,
    });
  });

  it('does not introduce sprint_move_unfinished in the dismissal request', () => {
    expect(buildSprintAutomationDismissRequest(workspaceId, prompt)).not.toHaveProperty(
      'sprint_move_unfinished',
    );
  });

  it('sends the disabled dismissal request through the injected upsert function', async () => {
    const upsert = vi.fn().mockResolvedValue({ data: { ok: true }, error: null });

    await dismissSprintAutomationPrompt({ workspaceId, prompt, upsert });

    expect(upsert).toHaveBeenCalledWith(workspaceId, {
      workspace_id: workspaceId,
      automation_type: 'sprint_auto_create',
      enabled: false,
      team_id: teamId,
      config_int: 2,
      config_int2: 3,
      config_int3: 1,
    });
  });

  it('throws when the dismissal upsert returns an error', async () => {
    const upsert = vi.fn().mockResolvedValue({ data: null, error: 'failed to save' });

    await expect(
      dismissSprintAutomationPrompt({ workspaceId, prompt, upsert }),
    ).rejects.toThrow('failed to save');
  });
});
