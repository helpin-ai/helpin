// Inline types previously imported from ./teamGoalTypes
export interface ScoringCriteria {
  on_time_delivery: {
    target_date: string
    status: 'pass' | 'fail' | 'pending'
    notes?: string
  }
  zero_critical_defects: {
    defect_count: number
    status: 'pass' | 'fail' | 'pending'
    notes?: string
  }
  process_compliance: {
    compliance_percentage: number
    status: 'pass' | 'fail' | 'pending'
    notes?: string
  }
  early_finish: {
    completion_date?: string
    status: 'pass' | 'fail' | 'pending'
    notes?: string
  }
}

export interface SprintTask {
  id: string
  sprint_milestone_id?: string
  title: string
  description: string
  assignee_id?: string
  assignee_name?: string
  status: 'todo' | 'in_progress' | 'review' | 'done'
  priority: 'high' | 'medium' | 'low'
  estimated_hours: number
  actual_hours?: number
  due_date?: string
  labels: string[]
  created_at: string
  updated_at: string
}

export interface Sprint {
  id: string
  workspace_id: string
  team_id: string // NEW: associate sprint with a team
  sprint_number: number
  title: string
  goal: string
  status: 'planning' | 'active' | 'completed' | 'cancelled'
  start_date: string
  end_date: string
  team_goals: SprintTeamGoal[]
  capacity: SprintCapacity
  scoring: ScoringCriteria
  velocity: number // story points completed
  tasks: SprintTask[]
  retrospective?: SprintRetrospective
  created_at: string
  updated_at: string
}

export interface SprintTeamGoal {
  team_goal_id: string
  team_goal_title: string
  team_id: string
  team_name: string
  sprint_target: number
  commitment_level: 'stretch' | 'committed' | 'conservative'
  tasks: SprintTask[]
  progress_percentage: number
}

export interface SprintCapacity {
  total_hours: number
  allocated_hours: number
  available_hours: number
  team_members: TeamMemberCapacity[]
  utilization_percentage: number
  overcommitment_risk: 'low' | 'medium' | 'high'
}

export interface TeamMemberCapacity {
  member_id: string
  member_name: string
  role: string
  total_hours: number
  allocated_hours: number
  availability_percentage: number // for PTO, meetings, etc.
  skills: string[]
  current_tasks: number
}

export interface SprintRetrospective {
  what_went_well: string[]
  what_could_improve: string[]
  action_items: string[]
  team_satisfaction: number // 1-5
  velocity_analysis: string
  process_improvements: string[]
}

export interface SprintPlanningData {
  sprint_goal: string
  team_goal_selections: {
    team_goal_id: string
    target_value: number
    commitment_level: 'stretch' | 'committed' | 'conservative'
  }[]
  capacity_planning: {
    total_capacity_hours: number
    buffer_percentage: number
    risk_mitigation: string[]
  }
  success_criteria: {
    primary_deliverables: string[]
    quality_gates: string[]
    stakeholder_expectations: string[]
  }
}

// Mock data functions
export const generateMockSprints = (workspaceId: string, teams: {id: string, name: string}[]): Sprint[] => {
  const currentDate = new Date()
  const sprints: Sprint[] = []

  // For each team, generate 6 sprints (2 completed, 1 active, 3 planned)
  for (const team of teams) {
    for (let i = 1; i <= 6; i++) {
      const startDate = new Date(currentDate)
      startDate.setDate(currentDate.getDate() + (i - 3) * 14) // Sprint 3 is current

      const endDate = new Date(startDate)
      endDate.setDate(startDate.getDate() + 13)

      const status = i < 3 ? 'completed' : i === 3 ? 'active' : 'planning'

      sprints.push({
        id: `sprint-${team.id}-${i}`,
        workspace_id: workspaceId,
        team_id: team.id,
        sprint_number: i,
        title: `Sprint ${i} - 2025-Q1`,
        goal: getSprintGoal(i),
        status,
        start_date: startDate.toISOString().split('T')[0],
        end_date: endDate.toISOString().split('T')[0],
        team_goals: generateSprintTeamGoals(i),
        capacity: generateSprintCapacity(),
        scoring: generateScoringCriteria(status),
        velocity: status === 'completed' ? Math.floor(Math.random() * 20) + 40 : 0,
        tasks: [],
        retrospective: status === 'completed' ? generateRetrospective() : undefined,
        created_at: new Date(startDate.getTime() - 7 * 24 * 60 * 60 * 1000).toISOString(),
        updated_at: new Date().toISOString()
      })
    }
  }
  return sprints
}

const getSprintGoal = (sprintNumber: number): string => {
  const goals = [
    'Establish foundation and development infrastructure',
    'Complete core feature development and testing framework',
    'Integration testing and performance optimization',
    'Launch preparation and documentation',
    'Launch execution and performance monitoring',
    'Post-launch optimization and feedback integration'
  ]
  return goals[sprintNumber - 1] || 'Sprint goal'
}

const generateSprintTeamGoals = (sprintNumber: number): SprintTeamGoal[] => {
  return [
    {
      team_goal_id: 'tg-1',
      team_goal_title: 'Launch Premium Features Suite',
      team_id: 'team-1',
      team_name: 'Product Development',
      sprint_target: 50000 + (sprintNumber - 1) * 25000,
      commitment_level: sprintNumber <= 2 ? 'committed' : sprintNumber <= 4 ? 'stretch' : 'conservative',
      tasks: [],
      progress_percentage: sprintNumber <= 3 ? 100 : sprintNumber === 4 ? 65 : 0
    },
    {
      team_goal_id: 'tg-2',
      team_goal_title: 'Drive Premium Conversion Campaign',
      team_id: 'team-2',
      team_name: 'Sales & Marketing',
      sprint_target: 30000 + (sprintNumber - 1) * 15000,
      commitment_level: 'committed',
      tasks: [],
      progress_percentage: sprintNumber <= 3 ? 100 : sprintNumber === 4 ? 45 : 0
    }
  ]
}

const generateSprintCapacity = (): SprintCapacity => {
  const teamMembers: TeamMemberCapacity[] = [
    {
      member_id: 'tm-1',
      member_name: 'Alex Chen',
      role: 'Senior Developer',
      total_hours: 80,
      allocated_hours: 72,
      availability_percentage: 90,
      skills: ['React', 'TypeScript', 'Node.js'],
      current_tasks: 5
    },
    {
      member_id: 'tm-2',
      member_name: 'Sarah Johnson',
      role: 'Product Designer',
      total_hours: 80,
      allocated_hours: 68,
      availability_percentage: 85,
      skills: ['UI/UX', 'Figma', 'User Research'],
      current_tasks: 3
    },
    {
      member_id: 'tm-3',
      member_name: 'Michael Rodriguez',
      role: 'DevOps Engineer',
      total_hours: 80,
      allocated_hours: 76,
      availability_percentage: 95,
      skills: ['AWS', 'Docker', 'CI/CD'],
      current_tasks: 4
    }
  ]

  const totalHours = teamMembers.reduce((sum, member) => sum + member.total_hours, 0)
  const allocatedHours = teamMembers.reduce((sum, member) => sum + member.allocated_hours, 0)
  const utilizationPercentage = (allocatedHours / totalHours) * 100

  return {
    total_hours: totalHours,
    allocated_hours: allocatedHours,
    available_hours: totalHours - allocatedHours,
    team_members: teamMembers,
    utilization_percentage: Math.round(utilizationPercentage),
    overcommitment_risk: utilizationPercentage > 90 ? 'high' : utilizationPercentage > 75 ? 'medium' : 'low'
  }
}

const generateScoringCriteria = (status: Sprint['status']): ScoringCriteria => {
  if (status === 'completed') {
    return {
      on_time_delivery: { target_date: '2024-01-15', status: 'pass', notes: 'Delivered on schedule' },
      zero_critical_defects: { defect_count: 0, status: 'pass', notes: 'No critical issues found' },
      process_compliance: { compliance_percentage: 95, status: 'pass', notes: 'Excellent process adherence' },
      early_finish: { completion_date: '2024-01-14', status: 'pass', notes: 'Completed 1 day early' }
    }
  }

  if (status === 'active') {
    return {
      on_time_delivery: { target_date: '2024-01-29', status: 'pending' },
      zero_critical_defects: { defect_count: 1, status: 'pending', notes: '1 minor defect being addressed' },
      process_compliance: { compliance_percentage: 88, status: 'pending', notes: 'On track but needs attention' },
      early_finish: { status: 'pending', notes: 'Possible if current pace continues' }
    }
  }

  return {
    on_time_delivery: { target_date: '2024-02-12', status: 'pending' },
    zero_critical_defects: { defect_count: 0, status: 'pending' },
    process_compliance: { compliance_percentage: 0, status: 'pending' },
    early_finish: { status: 'pending' }
  }
}

const generateRetrospective = (): SprintRetrospective => {
  return {
    what_went_well: [
      'Strong team collaboration and communication',
      'Effective use of pair programming for complex features',
      'Good stakeholder feedback integration'
    ],
    what_could_improve: [
      'Better estimation accuracy for UI components',
      'More thorough testing of edge cases',
      'Earlier identification of integration dependencies'
    ],
    action_items: [
      'Create component complexity estimation guidelines',
      'Add automated edge case testing to CI pipeline',
      'Implement dependency mapping in sprint planning'
    ],
    team_satisfaction: 4,
    velocity_analysis: 'Velocity improved 15% from previous sprint due to reduced context switching',
    process_improvements: [
      'Daily standups moved to async for better focus time',
      'Code review checklist updated with security guidelines'
    ]
  }
}