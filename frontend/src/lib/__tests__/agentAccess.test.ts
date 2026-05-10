import { describe, expect, it } from 'vitest';
import type { Agent } from '@/lib/pmTypes';
import { getAgentTeamIds, isAgentAvailableForTarget, isAgentVisibleToActor } from '../agentAccess';

const baseAgent: Agent = {
  id: 'agent-1',
  workspace_id: 'workspace-1',
  name: 'Agent',
  status: 'idle',
  runtime_kind: 'native_sdk',
  skills: [],
  trigger_mode: 'manual',
  tokens_used_this_month: 0,
  allowed_tools: [],
  allowed_commands: [],
  allowed_targets: ['task'],
  approval_mode: 'always',
  max_concurrent_runs: 1,
  default_invocation_mode: 'interactive',
  is_system: false,
  created_at: '',
  updated_at: '',
};

describe('agentAccess', () => {
  it('normalizes multi-team access and legacy fallback', () => {
    expect(getAgentTeamIds({ ...baseAgent, team_ids: ['team-a', 'team-b'] })).toEqual(['team-a', 'team-b']);
    expect(getAgentTeamIds({ ...baseAgent, team_id: 'team-a' })).toEqual(['team-a']);
  });

  it('shows workspace-wide agents to everyone', () => {
    expect(isAgentVisibleToActor(baseAgent, [], false)).toBe(true);
  });

  it('shows team-scoped agents only when actor overlaps or can see all', () => {
    const agent = { ...baseAgent, team_ids: ['team-a', 'team-b'] };

    expect(isAgentVisibleToActor(agent, ['team-c'], false)).toBe(false);
    expect(isAgentVisibleToActor(agent, ['team-b'], false)).toBe(true);
    expect(isAgentVisibleToActor(agent, [], true)).toBe(true);
  });

  it('requires target type and matching target team for team-scoped agents', () => {
    const agent = { ...baseAgent, allowed_targets: ['task'], team_ids: ['team-a'] };

    expect(isAgentAvailableForTarget(agent, {
      targetType: 'task',
      targetTeamId: 'team-a',
      accessibleTeamIds: ['team-a'],
      canSeeAllAgents: false,
    })).toBe(true);

    expect(isAgentAvailableForTarget(agent, {
      targetType: 'epic',
      targetTeamId: 'team-a',
      accessibleTeamIds: ['team-a'],
      canSeeAllAgents: false,
    })).toBe(false);

    expect(isAgentAvailableForTarget(agent, {
      targetType: 'task',
      targetTeamId: 'team-b',
      accessibleTeamIds: ['team-a'],
      canSeeAllAgents: false,
    })).toBe(false);

    expect(isAgentAvailableForTarget(agent, {
      targetType: 'task',
      accessibleTeamIds: ['team-a'],
      canSeeAllAgents: false,
    })).toBe(false);
  });

  it('hides team-scoped agents from teamless workspace targets', () => {
    const agent = { ...baseAgent, allowed_targets: ['workspace'], team_ids: ['team-a'] };

    expect(isAgentAvailableForTarget(agent, {
      targetType: 'workspace',
      accessibleTeamIds: ['team-a'],
      canSeeAllAgents: false,
    })).toBe(false);
  });
});
