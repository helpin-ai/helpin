// Consolidated Helpin Types for Performance-Based Bonuses

export interface CompanyGoal {
  id: string
  title: string
  description: string
  workspace_id: string
  target_value: number
  current_value: number
  progress_percentage: number
  unit: 'currency' | 'percentage' | 'number' | 'custom'
  unit_label?: string // for custom types
  status: 'on-track' | 'at-risk' | 'behind' | 'completed'
  priority: 'high' | 'medium' | 'low'
  category: string
  quarter: string // e.g., "Q1 2024"
  start_date: string
  end_date: string
  team_assignments: TeamAssignment[]
  success_criteria: string
  created_by: 'manager' | 'executive'
  // Enhanced details
  sprint_milestones: SprintMilestone[]
  team_contributions: TeamContribution[]
  key_results: KeyResult[]
  recent_updates: GoalUpdate[]
  created_at: string
  updated_at: string

  // New fields for auto-populated modal
  goal_type: 'metric' | 'milestone'
  baseline: number
  target: number
  unit_type: 'USD' | '%' | 'count'
  data_source: 'finance' | 'manual'
  contributions: GoalContribution[]
  metric_snapshots?: MetricSnapshot[]
}

export interface GoalContribution {
  teamId: string
  pct: number
}

export interface MetricSnapshot {
  sprintIndex: number
  current: number
  recordedAt: string
}

export interface TeamAssignment {
  team_id: string
  team_name: string
  contribution_percentage: number
  target_value: number
  rationale?: string
}

// Enhanced team goal that gets generated from assignments
export interface TeamGoal {
  id: string
  company_goal_id: string
  company_goal_title: string
  team_id: string
  team_name: string
  title: string
  description: string
  target_value: number
  current_value: number
  contribution_percentage: number
  status: 'not_started' | 'in_progress' | 'at_risk' | 'completed'
  progress_percentage: number
  current_sprint_number: number
  team_strategy?: string
  team_notes?: string
  risk_factors: string[]
  blockers: string[]
  sprint_breakdown: SprintBreakdown[]
  key_results: KeyResult[]
  created_at: string
  updated_at: string
}

export interface SprintBreakdown {
  sprint_number: number
  target_value: number
  description: string
  key_tasks: string[]
}

export interface TeamContribution {
  team_id: string
  team_name: string
  contribution_percentage: number
  current_progress: number
  target_progress: number
  status: 'ahead' | 'on-track' | 'behind'
  risk_level: 'low' | 'medium' | 'high'
  risk_reason?: string
}

export interface SprintMilestone {
  sprint_number: number
  target_value: number
  actual_value?: number
  completion_date?: string
  status: 'not-started' | 'in-progress' | 'completed' | 'missed'
}

export interface KeyResult {
  id: string
  title: string
  target_value: number
  current_value: number
  unit: string
  completion_percentage: number
}

export interface GoalUpdate {
  id: string
  message: string
  created_by: string
  created_at: string
  type: 'progress' | 'risk' | 'milestone' | 'general'
}

export interface SprintGoal {
  id: string
  title: string
  description?: string
  assigned_team_ids: string[]
  team_goal_id?: string // Links to specific team goal
  company_goal_id?: string // Links to company goal
  kr_id: string // Links to company goal KR
  status: 'planned' | 'completed'
  weight: number // 1-5 importance rating
  done: boolean // Yes/no completion status
  created_by: string
  created_at: string
}

export interface Sprint {
  id: string
  workspace_id: string
  team_id: string // Associate sprint with a team
  sprint_number: number // 1-6 per quarter
  quarter: string
  title: string
  start_date: string
  end_date: string
  goal: string // Legacy field - keeping for backward compatibility
  goals: SprintGoal[] // New multiple goals system
  team_goals: SprintTeamGoal[] // Links to team goals
  status: 'not_started' | 'active' | 'incomplete' | 'completed'
  individual_scores: IndividualSprintScore[]
  manager_completion_status?: { [managerId: string]: boolean } // Track which managers have completed scoring
  capacity?: SprintCapacity
  scoring?: ScoringCriteria
  velocity?: number
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
  availability_percentage: number
  skills: string[]
  current_tasks: number
}

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

export interface SprintRetrospective {
  what_went_well: string[]
  what_could_improve: string[]
  action_items: string[]
  team_satisfaction: number // 1-5
  velocity_analysis: string
  process_improvements: string[]
}

export interface IndividualSprintScore {
  employee_id: string
  employee_name: string
  team_id: string
  job_role?: string
  // Variable yes/no checks per person per sprint
  yes_count: number         // Number of "yes" checks (>= 0)
  total_count: number       // Total number of checks (>= 0, yes_count <= total_count)
  // Legacy fields for backward compatibility
  on_time_delivery?: boolean // Did person finish every assigned task on or before due date?
  logic_quality?: boolean    // Zero logical defects from their work?
  polish_quality?: boolean   // Zero visual/UX flaws from their work?
  // New job role-based scoring
  criteria_scores?: { [criteriaId: string]: boolean | null } // true = Yes, false = No, null = N/A
  total_score: number       // 0-N points per sprint based on job role criteria
  comments?: string         // Manager's notes/feedback for this sprint
}

export interface QuarterlyPerformance {
  employee_id: string
  employee_name: string
  team_id: string
  quarter: string
  individual_total_score: number // Raw points earned
  max_possible_score: number // Maximum points possible for this person's role
  percentage_score: number // Normalized percentage (0-100%)
  team_goal_achieved: boolean
  base_tier: 'A' | 'B' | 'C'
  final_tier: 'A' | 'B' | 'C' // After automatic overrides
  bonus_multiplier: number // 1.5, 1.0, or 0
  override_applied: 'star_performer' | 'poor_performer' | 'none'
}

// Mock data generators
export const generateMockCompanyGoals = (workspaceId: string, _quarterId?: string): CompanyGoal[] => {
  // Use consistent demo data for predictable showcase
  const demoQuarter = '2025-Q1'
  const start_date = '2025-01-01'
  const end_date = '2025-03-31'

  return [
    {
      id: 'cg-1',
      title: `Increase MRR by 40% in ${demoQuarter}`,
      description: 'Grow monthly recurring revenue from $250k to $350k through new customer acquisition and upsells',
      workspace_id: workspaceId,
      target_value: 350000,
      current_value: 287500,
      progress_percentage: 82,
      unit: 'currency',
      status: 'on-track',
      priority: 'high',
      category: 'Revenue Growth',
      quarter: demoQuarter,
      start_date,
      end_date,
      team_assignments: [
        { team_id: 'team-1', team_name: 'Product Development', contribution_percentage: 60, target_value: 210000, rationale: 'Primary feature development and platform enhancements' },
        { team_id: 'team-2', team_name: 'Sales & Marketing', contribution_percentage: 40, target_value: 140000, rationale: 'Customer acquisition and conversion optimization' }
      ],
      success_criteria: 'Achieve $350k MRR with sustainable growth rate and customer retention above 95%',
      created_by: 'executive',

      // New modal fields
      goal_type: 'metric',
      baseline: 250000,
      target: 350000,
      unit_type: 'USD',
      data_source: 'finance',
      contributions: [
        { teamId: 'team-1', pct: 60 },
        { teamId: 'team-2', pct: 40 }
      ],
      metric_snapshots: [
        { sprintIndex: 1, current: 262000, recordedAt: '2024-01-14T00:00:00Z' },
        { sprintIndex: 2, current: 275000, recordedAt: '2024-01-28T00:00:00Z' }
      ],
      sprint_milestones: [
        { sprint_number: 1, target_value: 260000, actual_value: 262000, completion_date: '2024-01-14', status: 'completed' },
        { sprint_number: 2, target_value: 280000, actual_value: 275000, completion_date: '2024-01-28', status: 'completed' },
        { sprint_number: 3, target_value: 300000, actual_value: 287500, status: 'in-progress' },
        { sprint_number: 4, target_value: 320000, status: 'not-started' },
        { sprint_number: 5, target_value: 335000, status: 'not-started' },
        { sprint_number: 6, target_value: 350000, status: 'not-started' }
      ],
      team_contributions: [
        {
          team_id: 'team-1',
          team_name: 'Product Development',
          contribution_percentage: 60,
          current_progress: 172500,
          target_progress: 210000,
          status: 'on-track',
          risk_level: 'low'
        },
        {
          team_id: 'team-2',
          team_name: 'Sales & Marketing',
          contribution_percentage: 40,
          current_progress: 115000,
          target_progress: 140000,
          status: 'on-track',
          risk_level: 'medium',
          risk_reason: 'Market competition increasing'
        }
      ],
      key_results: [
        {
          id: 'kr1-cg1',
          title: 'New customer acquisition',
          target_value: 150,
          current_value: 112,
          unit: 'customers',
          completion_percentage: 75
        },
        {
          id: 'kr2-cg1',
          title: 'Average contract value increase',
          target_value: 15,
          current_value: 12,
          unit: 'percentage',
          completion_percentage: 80
        }
      ],
      recent_updates: [
        {
          id: 'u1-cg1',
          message: 'Sprint 3 in progress with strong pipeline development',
          created_by: 'Sarah Johnson',
          created_at: '2024-02-25T09:30:00Z',
          type: 'progress'
        }
      ],
      created_at: start_date + 'T00:00:00Z',
      updated_at: new Date().toISOString()
    },
    {
      id: 'cg-2',
      title: 'Launch Enterprise Security Features',
      description: 'Deliver SOC2 compliance and enterprise-grade security features to capture enterprise market',
      workspace_id: workspaceId,
      target_value: 100,
      current_value: 65,
      progress_percentage: 65,
      unit: 'percentage',
      status: 'at-risk',
      priority: 'high',
      category: 'Product Development',
      quarter: demoQuarter,
      start_date,
      end_date,
      team_assignments: [
        { team_id: 'team-1', team_name: 'Product Development', contribution_percentage: 80, target_value: 80, rationale: 'Core security feature development' },
        { team_id: 'team-3', team_name: 'DevOps', contribution_percentage: 20, target_value: 20, rationale: 'Infrastructure and compliance setup' }
      ],
      success_criteria: 'SOC2 Type II certification completed and enterprise security dashboard launched',
      created_by: 'executive',

      // New modal fields
      goal_type: 'milestone',
      baseline: 0,
      target: 100,
      unit_type: '%',
      data_source: 'manual',
      contributions: [
        { teamId: 'team-1', pct: 80 },
        { teamId: 'team-3', pct: 20 }
      ],
      sprint_milestones: [
        { sprint_number: 1, target_value: 15, actual_value: 18, completion_date: '2024-01-14', status: 'completed' },
        { sprint_number: 2, target_value: 30, actual_value: 32, completion_date: '2024-01-28', status: 'completed' },
        { sprint_number: 3, target_value: 50, actual_value: 45, status: 'in-progress' },
        { sprint_number: 4, target_value: 70, status: 'not-started' },
        { sprint_number: 5, target_value: 85, status: 'not-started' },
        { sprint_number: 6, target_value: 100, status: 'not-started' }
      ],
      team_contributions: [
        {
          team_id: 'team-1',
          team_name: 'Product Development',
          contribution_percentage: 80,
          current_progress: 52,
          target_progress: 80,
          status: 'behind',
          risk_level: 'high',
          risk_reason: 'Security audit requirements more complex than expected'
        },
        {
          team_id: 'team-3',
          team_name: 'DevOps',
          contribution_percentage: 20,
          current_progress: 13,
          target_progress: 20,
          status: 'on-track',
          risk_level: 'low'
        }
      ],
      key_results: [
        {
          id: 'kr1-cg2',
          title: 'SOC2 certification progress',
          target_value: 100,
          current_value: 65,
          unit: 'percentage',
          completion_percentage: 65
        }
      ],
      recent_updates: [
        {
          id: 'u1-cg2',
          message: 'Security audit revealed additional requirements, adjusting timeline',
          created_by: 'Mike Chen',
          created_at: '2024-02-22T14:15:00Z',
          type: 'risk'
        }
      ],
      created_at: start_date + 'T00:00:00Z',
      updated_at: new Date().toISOString()
    }
  ]
}

export const generateMockSprints = (workspaceId: string, _quarterId: string, teamGoals: TeamGoal[] = []): Sprint[] => {
  const sprints: Sprint[] = []

  // For demo data, use consistent 2025-Q1
  const quarter = '2025-Q1'
  const quarterNum = 1;
  const year = 2025;

  // Calculate first month of the quarter
  const quarterStartMonth = (quarterNum - 1) * 3;

  // Create 6 sprints with 2-week durations for the quarter
  const sprintDates = [
    { start: new Date(year, quarterStartMonth, 1), end: new Date(year, quarterStartMonth, 14) },     // First 2 weeks
    { start: new Date(year, quarterStartMonth, 15), end: new Date(year, quarterStartMonth, 28) },    // Second 2 weeks
    { start: new Date(year, quarterStartMonth, 29), end: new Date(year, quarterStartMonth + 1, 11) }, // Third 2 weeks
    { start: new Date(year, quarterStartMonth + 1, 12), end: new Date(year, quarterStartMonth + 1, 25) }, // Fourth 2 weeks
    { start: new Date(year, quarterStartMonth + 1, 26), end: new Date(year, quarterStartMonth + 2, 8) },  // Fifth 2 weeks
    { start: new Date(year, quarterStartMonth + 2, 9), end: new Date(year, quarterStartMonth + 2, 22) }   // Sixth 2 weeks
  ]

  // Create mock teams - using the same IDs as getDefaultTeams()
  const teams = ['dddddddd-dddd-dddd-dddd-dddddddddddd', 'eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee', 'ffffffff-ffff-ffff-ffff-ffffffffffff']

  // Generate 6 sprints per quarter for each team
  for (const teamId of teams) {
    for (let i = 1; i <= 6; i++) {
      const startDate = sprintDates[i-1].start
      const endDate = sprintDates[i-1].end

      // Determine status based on dates and current progress
      let status: 'not_started' | 'active' | 'incomplete' | 'completed'

      // Force Sprint 3 to be active for UI demonstration
      if (i === 3) {
        status = 'active'
      } else if (i < 3) {
        status = 'completed' // First two sprints are completed
      } else {
        status = 'not_started' // Later sprints not started yet
      }

      // Find team goals for this team
      const teamGoalsForTeam = teamGoals.filter(tg => tg.team_id === teamId)

      // Generate a deterministic UUID for sprint based on team and sprint number
      const generateSprintUUID = (_team: string, sprintNum: number): string => {
        // For consistency with database, use simple pattern: sprintNum repeated with dashes
        const sprintHex = sprintNum.toString().repeat(8);
        return `${sprintHex}-1111-1111-1111-111111111111`;
      }

      sprints.push({
        id: generateSprintUUID(teamId, i),
        workspace_id: workspaceId,
        team_id: teamId,
        sprint_number: i,
        quarter,
        title: `Sprint ${i} - 2025-Q1`,
        start_date: startDate.toISOString().split('T')[0],
        end_date: endDate.toISOString().split('T')[0],
        goal: getSprintGoal(i), // Legacy field
        goals: generateMockSprintGoals(i, status, teamGoalsForTeam),
        team_goals: generateSprintTeamGoals(i, teamGoalsForTeam),
        status,
        individual_scores: generateMockIndividualScores(i, status),
        created_at: startDate.toISOString(),
        updated_at: new Date().toISOString()
      })
    }
  }

  return sprints
}

const getSprintGoal = (sprintNumber: number): string => {
  const goals = [
    'Foundation setup and infrastructure improvements',
    'Core feature development and testing',
    'Integration testing and performance optimization',
    'Security implementation and compliance work',
    'Launch preparation and documentation',
    'Launch execution and post-launch optimization'
  ]
  return goals[sprintNumber - 1] || `Sprint ${sprintNumber} goals`
}

const generateMockSprintGoals = (sprintNumber: number, status: Sprint['status'], teamGoals: TeamGoal[] = []): SprintGoal[] => {
  const goalSets = [
    // Sprint 1 goals
    [
      { title: 'Set up development infrastructure', description: 'Configure CI/CD pipeline and development environments', teams: ['team-1'] },
      { title: 'Define project architecture', description: 'Establish technical standards and coding guidelines', teams: ['team-1'] },
      { title: 'Create initial wireframes', description: 'Design core user flows and interface mockups', teams: ['team-2'] }
    ],
    // Sprint 2 goals
    [
      { title: 'Implement user authentication', description: 'Build login, registration, and password reset functionality', teams: ['team-1'] },
      { title: 'Create user dashboard', description: 'Develop main user interface and navigation', teams: ['team-1', 'team-2'] },
      { title: 'Set up analytics tracking', description: 'Implement user behavior and performance monitoring', teams: ['team-3'] }
    ],
    // Sprint 3 goals (active)
    [
      { title: 'Improve user onboarding flow', description: 'Streamline new user experience and reduce drop-off', teams: ['team-1', 'team-2'] },
      { title: 'Implement payment gateway', description: 'Integrate Stripe for subscription payments', teams: ['team-1'] },
      { title: 'Fix mobile responsiveness issues', description: 'Ensure optimal experience across all devices', teams: ['team-2'] }
    ],
    // Sprint 4 goals
    [
      { title: 'Launch referral program', description: 'Build user referral system with rewards', teams: ['team-1', 'team-2'] },
      { title: 'Optimize database performance', description: 'Improve query speed and reduce load times', teams: ['team-3'] }
    ],
    // Sprint 5 goals
    [
      { title: 'Implement advanced search', description: 'Add filtering and search capabilities', teams: ['team-1'] },
      { title: 'Create admin panel', description: 'Build management interface for administrators', teams: ['team-1'] }
    ],
    // Sprint 6 goals
    [
      { title: 'Launch beta testing program', description: 'Recruit and manage beta user testing', teams: ['team-2'] },
      { title: 'Prepare for public launch', description: 'Final testing and launch preparations', teams: ['team-1', 'team-2', 'team-3'] }
    ]
  ]

  const currentGoals = goalSets[sprintNumber - 1] || []

  return currentGoals.map((goal, index) => {
    const teamGoal = teamGoals.find(tg => goal.teams.includes(tg.team_id))

    // Generate a deterministic UUID based on sprint and goal index for consistency
    const generateDeterministicUUID = (sprint: number, idx: number): string => {
      const base = `${sprint}${idx}`.padStart(4, '0');
      return `550e8400-e29b-41d4-a716-44665544${base}`;
    }

    return {
      id: generateDeterministicUUID(sprintNumber, index + 1),
      title: goal.title,
      description: goal.description,
      assigned_team_ids: goal.teams,
      team_goal_id: teamGoal?.id,
      company_goal_id: teamGoal?.company_goal_id,
      kr_id: index < 2 ? `kr1-cg-${(sprintNumber % 2) + 1}` : `kr${(index % 3) + 1}`,
      status: determineGoalStatus(sprintNumber, status, index),
      weight: (sprintNumber + index) % 5 + 1,
      done: determineGoalStatus(sprintNumber, status, index) === 'completed',
      created_by: 'person-5',
      created_at: new Date().toISOString()
    }
  })
}

const determineGoalStatus = (sprintNumber: number, sprintStatus: Sprint['status'], goalIndex: number): 'planned' | 'completed' => {
  // Completed sprints have all goals completed
  if (sprintStatus === 'completed') return 'completed'

  // Active sprint (Sprint 3) has some goals completed
  if (sprintStatus === 'active' && sprintNumber === 3) {
    return goalIndex < 2 ? 'completed' : 'planned' // First 2 goals completed
  }

  // All other cases: planned
  return 'planned'
}

// Generate sprint team goals that link to actual team goals
const generateSprintTeamGoals = (sprintNumber: number, teamGoals: TeamGoal[]): SprintTeamGoal[] => {
  return teamGoals.map(teamGoal => {
    const sprintBreakdown = teamGoal.sprint_breakdown.find(sb => sb.sprint_number === sprintNumber)

    return {
      team_goal_id: teamGoal.id,
      team_goal_title: teamGoal.title,
      team_id: teamGoal.team_id,
      team_name: teamGoal.team_name,
      sprint_target: sprintBreakdown?.target_value || 0,
      commitment_level: sprintNumber <= 2 ? 'committed' : sprintNumber <= 4 ? 'stretch' : 'conservative',
      tasks: [],
      progress_percentage: sprintNumber <= 2 ? 100 : sprintNumber === 3 ? 65 : 0
    }
  })
}

const generateMockIndividualScores = (sprintNumber: number, status: Sprint['status']): IndividualSprintScore[] => {
  if (status === 'not_started') return []

  const employees = [
    { id: 'emp-1', name: 'Alex Chen', team: 'team-1' },
    { id: 'emp-2', name: 'Sarah Johnson', team: 'team-1' },
    { id: 'emp-3', name: 'Michael Rodriguez', team: 'team-2' },
    { id: 'emp-4', name: 'Emily Davis', team: 'team-2' }
  ]

  // Specific patterns as requested: 0, 3, 5 with deterministic results
  const checkPatterns = [
    { total: 3, yes: 2 },  // Alex Chen: 2/3
    { total: 5, yes: 4 },  // Sarah Johnson: 4/5
    { total: 0, yes: 0 },  // Michael Rodriguez: 0/0 (no checks)
    { total: 5, yes: 5 }   // Emily Davis: 5/5 (perfect)
  ]

  return employees.map((emp, index) => {
    // Use consistent pattern per employee across all sprints for easy testing
    const pattern = checkPatterns[index % checkPatterns.length]

    // Vary slightly by sprint to show progression/regression
    let totalCount = pattern.total
    let yesCount = pattern.yes

    // Add some sprint-based variation while keeping recognizable patterns
    if (totalCount > 0) {
      // Adjust based on sprint status
      if (status === 'completed' && sprintNumber <= 2) {
        // Earlier completed sprints had slightly different counts
        totalCount = Math.max(0, totalCount + (sprintNumber - 3))
        yesCount = Math.min(totalCount, Math.max(0, yesCount + (sprintNumber - 3)))
      }
    }

    // Simulate different performance patterns for legacy fields
    const onTime = Math.random() > 0.2 // 80% on-time rate
    const logic = Math.random() > 0.15 // 85% logic quality rate
    const polish = Math.random() > 0.25 // 75% polish quality rate

    return {
      employee_id: emp.id,
      employee_name: emp.name,
      team_id: emp.team,
      yes_count: yesCount,
      total_count: totalCount,
      on_time_delivery: onTime,
      logic_quality: logic,
      polish_quality: polish,
      total_score: yesCount // Use yes_count as the main score now
    }
  })
}

// Check if sprint scoring is complete for a specific manager's team members
export const isManagerScoringComplete = (sprint: Sprint, managerTeamMemberIds: string[]): boolean => {
  if (!sprint.individual_scores || sprint.individual_scores.length === 0) {
    return false
  }

  // Check if all of this manager's team members have scores
  const scoredEmployeeIds = sprint.individual_scores.map(score => score.employee_id)
  const allMembersScored = managerTeamMemberIds.every(memberId => scoredEmployeeIds.includes(memberId))

  if (!allMembersScored) return false

  // Check if all scores for this manager's team have at least some criteria scored
  const managerTeamScores = sprint.individual_scores.filter(score =>
    managerTeamMemberIds.includes(score.employee_id)
  )

  return managerTeamScores.every(score => score.total_score > 0)
}

// Check if sprint scoring is complete for ALL managers
export const isSprintScoringComplete = (sprint: Sprint, allTeamMemberIds: string[]): boolean => {
  if (!sprint.individual_scores || sprint.individual_scores.length === 0) {
    return false
  }

  // Check if all team members have scores
  const scoredEmployeeIds = sprint.individual_scores.map(score => score.employee_id)
  const allMembersScored = allTeamMemberIds.every(memberId => scoredEmployeeIds.includes(memberId))

  if (!allMembersScored) return false

  // Check if all scores have at least some criteria scored (not all zeros)
  return sprint.individual_scores.every(score => score.total_score > 0)
}

// Mark manager as having completed their scoring
export const markManagerScoringComplete = (
  sprint: Sprint,
  managerId: string,
  managerTeamMemberIds: string[]
): Sprint => {
  const updatedSprint = { ...sprint }

  if (!updatedSprint.manager_completion_status) {
    updatedSprint.manager_completion_status = {}
  }

  // Mark this manager as complete if their scoring is done
  if (isManagerScoringComplete(sprint, managerTeamMemberIds)) {
    updatedSprint.manager_completion_status[managerId] = true
  }

  return updatedSprint
}

// Mark sprint as completed (only if ALL managers have completed scoring)
export const completeSprintScoring = (sprint: Sprint, allManagerIds: string[], allTeamMemberIds: string[]): Sprint => {
  // Check if all managers have completed their part
  const allManagersComplete = allManagerIds.every(managerId =>
    sprint.manager_completion_status?.[managerId] === true
  )

  // Check if overall scoring is complete
  if (allManagersComplete && isSprintScoringComplete(sprint, allTeamMemberIds)) {
    return { ...sprint, status: 'completed' }
  }
  return sprint
}

// Generate team goals from company goals
export const generateMockTeamGoals = (_workspaceId: string, companyGoals: CompanyGoal[]): TeamGoal[] => {
  const teamGoals: TeamGoal[] = []

  for (const companyGoal of companyGoals) {
    for (const assignment of companyGoal.team_assignments) {
      const teamGoalId = `tg-${companyGoal.id}-${assignment.team_id}`

      teamGoals.push({
        id: teamGoalId,
        company_goal_id: companyGoal.id,
        company_goal_title: companyGoal.title,
        team_id: assignment.team_id,
        team_name: assignment.team_name,
        title: `${assignment.team_name}: ${companyGoal.title}`,
        description: `${assignment.team_name} contribution to ${companyGoal.title}. ${assignment.rationale || ''}`,
        target_value: assignment.target_value,
        current_value: Math.round(assignment.target_value * 0.75), // Mock current progress
        contribution_percentage: assignment.contribution_percentage,
        status: 'in_progress',
        progress_percentage: 75,
        current_sprint_number: 3,
        team_strategy: `Focus on high-impact deliverables for ${companyGoal.title}`,
        risk_factors: ['Resource allocation challenges', 'Dependency on external teams'],
        blockers: [],
        sprint_breakdown: generateSprintBreakdownForTeam(assignment.target_value),
        key_results: [
          {
            id: `kr-${teamGoalId}-1`,
            title: `${assignment.team_name} Deliverables`,
            target_value: assignment.target_value,
            current_value: Math.round(assignment.target_value * 0.75),
            unit: companyGoal.unit,
            completion_percentage: 75
          }
        ],
        created_at: companyGoal.created_at,
        updated_at: new Date().toISOString()
      })
    }
  }

  return teamGoals
}

const generateSprintBreakdownForTeam = (targetValue: number): SprintBreakdown[] => {
  const progressionRates = [0.08, 0.15, 0.22, 0.25, 0.20, 0.10] // Sprint 1-6 percentages
  let cumulativeValue = 0

  const sprintDescriptions = [
    'Foundation & Planning',
    'Initial Implementation',
    'Core Development',
    'Feature Completion',
    'Optimization & Testing',
    'Final Push & Polish'
  ]

  return progressionRates.map((rate, index) => {
    const sprintValue = Math.round(targetValue * rate)
    cumulativeValue += sprintValue

    return {
      sprint_number: index + 1,
      target_value: cumulativeValue,
      description: sprintDescriptions[index],
      key_tasks: [`Sprint ${index + 1} tasks for team goal`]
    }
  })
}

export const calculateQuarterlyPerformance = (
  employeeId: string,
  sprints: Sprint[],
  workspaceSettings: any
): QuarterlyPerformance => {
  const demoQuarter = '2025-Q1' // Consistent demo data
  const employee = sprints[0]?.individual_scores?.find(score => score.employee_id === employeeId)
  if (!employee) throw new Error('Employee not found')

  // Sum individual scores across all completed sprints
  const individualTotal = sprints
    .filter(sprint => sprint.status === 'completed')
    .reduce((total, sprint) => {
      const score = sprint.individual_scores?.find(s => s.employee_id === employeeId)
      return total + (score?.total_score || 0)
    }, 0)

  // Calculate max possible score for this person's job role
  const person = workspaceSettings?.people.find((p: any) => p.id === employeeId)
  const maxPerSprint = person ? workspaceSettings.general_settings.job_role_scoring_criteria
    .find((role: any) => role.job_role === person.job_role)?.criteria.length || 0 : 0
  const maxQuarterlyScore = maxPerSprint * 6 // 6 sprints per quarter

  // Normalize to percentage
  const percentageScore = maxQuarterlyScore > 0 ? Math.round((individualTotal / maxQuarterlyScore) * 100) : 0

  // Mock team goal achievement (would be calculated from company goals)
  const teamGoalAchieved = true // Placeholder

  // Calculate base tier using percentage thresholds
  let baseTier: 'A' | 'B' | 'C' = 'C'
  if (percentageScore >= 75) baseTier = 'A'
  else if (percentageScore >= 50) baseTier = 'B'

  // Apply automatic overrides
  let finalTier = baseTier
  let overrideApplied: 'star_performer' | 'poor_performer' | 'none' = 'none'

  // Star performer override: High percentage (>=75%) + team goal missed -> Force Tier A
  if (percentageScore >= 75 && !teamGoalAchieved) {
    finalTier = 'A'
    overrideApplied = 'star_performer'
  }

  // Poor performer override: Low percentage (<50%) + team goal hit -> Force Tier C
  if (percentageScore < 50 && teamGoalAchieved) {
    finalTier = 'C'
    overrideApplied = 'poor_performer'
  }

  const bonusMultiplier = finalTier === 'A' ? 1.5 : finalTier === 'B' ? 1.0 : 0

  return {
    employee_id: employeeId,
    employee_name: employee.employee_name,
    team_id: employee.team_id,
    quarter: demoQuarter,
    individual_total_score: individualTotal,
    max_possible_score: maxQuarterlyScore,
    percentage_score: percentageScore,
    team_goal_achieved: teamGoalAchieved,
    base_tier: baseTier,
    final_tier: finalTier,
    bonus_multiplier: bonusMultiplier,
    override_applied: overrideApplied
  }
}