// Client-side calculation functions to preserve existing UI logic

// Inline types previously imported from ./services/types
export interface SprintGoalRecord {
  goal_id: string
  team_id: string
  sprint_index: number
  weight: number
  done: boolean
}

export interface GoalTeamContribution {
  team_id: string
  contribution_pct: number
  team: { name: string }
}

export interface CompanyGoalWithTeams {
  id: string
  title: string
  goal_type: 'metric' | 'milestone'
  baseline: number
  target: number
  unit: 'USD' | '%' | 'count'
  goal_team_contributions: GoalTeamContribution[]
  sprint_goals: SprintGoalRecord[]
}

export interface TeamGoalProgress {
  teamId: string
  teamName: string
  progressPct: number
  targetUSD: number
  currentUSD: number
  status: 'on-track' | 'at-risk'
}

export interface CompanyGoalProgress {
  id: string
  title: string
  goalType: 'metric' | 'milestone'
  baseline: number
  target: number
  unit: 'USD' | '%' | 'count'
  teamProgressList: TeamGoalProgress[]
}

// Calculate team goal progress percentage for a team/goal up to current sprint
export function calculateTeamProgressPct(
  goalId: string,
  teamId: string,
  sprintGoals: SprintGoalRecord[],
  currentSprint: number = 3
): number {
  // Get all sprint goals for this goal/team combination up to current sprint
  const relevantGoals = sprintGoals.filter(sg =>
    sg.goal_id === goalId &&
    sg.team_id === teamId &&
    sg.sprint_index <= currentSprint
  )

  if (relevantGoals.length === 0) return 0

  // Calculate weighted progress
  const totalWeight = relevantGoals.reduce((sum, sg) => sum + sg.weight, 0)
  const completedWeight = relevantGoals
    .filter(sg => sg.done)
    .reduce((sum, sg) => sum + sg.weight, 0)

  if (totalWeight === 0) return 0

  return Math.round((completedWeight / totalWeight) * 100)
}

// Calculate team target USD for metric goals
export function calculateTeamTargetUSD(
  goal: CompanyGoalWithTeams,
  teamId: string
): number {
  if (goal.goal_type !== 'metric') return 0

  const contribution = goal.goal_team_contributions.find(gtc => gtc.team_id === teamId)
  if (!contribution) return 0

  return goal.target * (contribution.contribution_pct / 100)
}

// Calculate team current USD (display proxy)
export function calculateTeamCurrentUSD(
  teamTargetUSD: number,
  progressPct: number
): number {
  return Math.round(teamTargetUSD * (progressPct / 100))
}

// Determine if team is on track vs at risk
export function calculateTeamStatus(
  progressPct: number,
  currentSprint: number = 3,
  totalSprints: number = 6
): 'on-track' | 'at-risk' {
  const expectedPct = (currentSprint / totalSprints) * 100
  return progressPct >= expectedPct ? 'on-track' : 'at-risk'
}

// Process company goals with client-side calculations
export function processCompanyGoalsWithProgress(
  goals: CompanyGoalWithTeams[],
  currentSprint: number = 3
): CompanyGoalProgress[] {
  return goals.map(goal => {
    const teamProgressList: TeamGoalProgress[] = goal.goal_team_contributions.map(contribution => {
      const progressPct = calculateTeamProgressPct(
        goal.id,
        contribution.team_id,
        goal.sprint_goals,
        currentSprint
      )

      const teamTargetUSD = calculateTeamTargetUSD(goal, contribution.team_id)
      const teamCurrentUSD = calculateTeamCurrentUSD(teamTargetUSD, progressPct)
      const status = calculateTeamStatus(progressPct, currentSprint)

      return {
        teamId: contribution.team_id,
        teamName: contribution.team.name,
        progressPct,
        targetUSD: teamTargetUSD,
        currentUSD: teamCurrentUSD,
        status
      }
    })

    return {
      id: goal.id,
      title: goal.title,
      goalType: goal.goal_type,
      baseline: goal.baseline,
      target: goal.target,
      unit: goal.unit,
      teamProgressList
    }
  })
}