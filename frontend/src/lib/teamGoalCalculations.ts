import type { CompanyGoal, Sprint } from './simplifiedTypes';

/**
 * Shared utilities for team goal calculations based on KR-linked sprint goals
 */

/**
 * Calculate team progress percentage up to current sprint
 * Formula: (sum of weight of KR-linked SprintGoals for this team that are done, across sprints <= current)
 *         / (sum of weight of all KR-linked SprintGoals for this team across sprints <= current) * 100
 */
export function calculateTeamProgressPct(
  companyGoal: CompanyGoal,
  teamId: string,
  sprints: Sprint[],
  currentSprintIndex: number
): number {
  const krId = `kr1-${companyGoal.id}`;

  // Get all KR-linked sprint goals for this team up to current sprint
  const relevantGoals = sprints
    .filter(sprint => sprint.sprint_number <= currentSprintIndex)
    .flatMap(sprint =>
      sprint.goals.filter(goal =>
        goal.kr_id === krId &&
        goal.assigned_team_ids.includes(teamId)
      )
    );

  if (relevantGoals.length === 0) {
    return 0;
  }

  const totalWeight = relevantGoals.reduce((sum, goal) => sum + goal.weight, 0);
  const doneWeight = relevantGoals
    .filter(goal => goal.done)
    .reduce((sum, goal) => sum + goal.weight, 0);

  if (totalWeight === 0) {
    return 0;
  }

  const progressPct = (doneWeight / totalWeight) * 100;
  return Math.max(0, Math.min(100, Math.round(progressPct)));
}

/**
 * Calculate expected progress percentage for status determination
 */
export function calculateExpectedPct(currentSprintIndex: number): number {
  const completedSprints = Math.max(0, currentSprintIndex - 1);
  return Math.round((completedSprints / 6) * 100);
}

/**
 * Calculate team target value for metric goals
 */
export function calculateTeamTarget(
  companyGoal: CompanyGoal,
  teamContributionPct: number
): number {
  if (companyGoal.goal_type !== 'metric') {
    return 0; // Not applicable for milestone goals
  }

  return Math.round(companyGoal.target * (teamContributionPct / 100));
}

/**
 * Calculate team current value for metric goals (display proxy)
 */
export function calculateTeamCurrent(
  teamTarget: number,
  teamProgressPct: number
): number {
  return Math.round(teamTarget * (teamProgressPct / 100));
}

/**
 * Determine team goal status based on progress vs expected
 */
export function getTeamGoalStatus(
  teamProgressPct: number,
  expectedPct: number
): 'on-track' | 'at-risk' | 'not-started' {
  if (teamProgressPct === 0) {
    return 'not-started';
  }

  return teamProgressPct >= expectedPct ? 'on-track' : 'at-risk';
}

/**
 * Get current sprint index (you may need to adjust this based on your actual sprint tracking)
 */
export function getCurrentSprintIndex(sprints: Sprint[]): number {
  // Find the active sprint, or default to 3 if none found
  const activeSprint = sprints.find(sprint => sprint.status === 'active');
  return activeSprint?.sprint_number || 3;
}

/**
 * Get team contribution percentage from company goal
 */
export function getTeamContributionPct(
  companyGoal: CompanyGoal,
  teamId: string
): number {
  const contribution = companyGoal.contributions?.find(c => c.teamId === teamId);
  return contribution?.pct || 0;
}