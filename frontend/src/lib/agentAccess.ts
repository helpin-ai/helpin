import type { Agent, AgentTargetType } from '@/lib/pmTypes';

function normalizeIds(ids: Array<string | null | undefined>): string[] {
  const seen = new Set<string>();
  const result: string[] = [];
  for (const id of ids) {
    const trimmed = id?.trim();
    if (!trimmed || seen.has(trimmed)) continue;
    seen.add(trimmed);
    result.push(trimmed);
  }
  return result;
}

export function getAgentTeamIds(agent: Pick<Agent, 'team_id' | 'team_ids'>): string[] {
  const teamIds = normalizeIds(agent.team_ids ?? []);
  if (teamIds.length > 0) return teamIds;
  return normalizeIds([agent.team_id]);
}

export function isAgentVisibleToActor(
  agent: Pick<Agent, 'team_id' | 'team_ids'>,
  accessibleTeamIds: string[] | Set<string>,
  canSeeAllAgents: boolean,
): boolean {
  if (canSeeAllAgents) return true;
  const agentTeamIds = getAgentTeamIds(agent);
  if (agentTeamIds.length === 0) return true;
  const accessible = accessibleTeamIds instanceof Set ? accessibleTeamIds : new Set(accessibleTeamIds);
  return agentTeamIds.some((teamId) => accessible.has(teamId));
}

export function isAgentAvailableForTarget(
  agent: Pick<Agent, 'allowed_targets' | 'team_id' | 'team_ids'>,
  options: {
    targetType: AgentTargetType;
    targetTeamId?: string | null;
    accessibleTeamIds: string[] | Set<string>;
    canSeeAllAgents: boolean;
  },
): boolean {
  if (!isAgentVisibleToActor(agent, options.accessibleTeamIds, options.canSeeAllAgents)) {
    return false;
  }
  if (!agent.allowed_targets?.includes(options.targetType)) {
    return false;
  }
  const agentTeamIds = getAgentTeamIds(agent);
  if (agentTeamIds.length === 0) {
    return true;
  }
  const targetTeamId = options.targetTeamId?.trim();
  return Boolean(targetTeamId && agentTeamIds.includes(targetTeamId));
}
