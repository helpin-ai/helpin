// Client-side calculations for sprint progress and goals

// Inline type previously imported from ./services/sprintsService
export interface FullSprintRecord {
  id: string
  workspace_id: string
  team_id: string
  quarter_id: string
  sprint_index: number
  title: string
  start_date: string
  end_date: string
  status: 'not_started' | 'active' | 'incomplete' | 'completed' | 'locked'
  goal: string
  updated_at: string
}

export interface SprintGoalData {
  id: string
  title: string
  description?: string
  assigned_team_ids: string[]
  team_goal_id?: string
  company_goal_id?: string
  kr_id: string
  status: 'planned' | 'completed'
  weight: number
  done: boolean
  created_by: string
  created_at: string
}

export interface ProcessedSprint {
  id: string
  sprint_number: number
  title: string
  start_date: string
  end_date: string
  status: 'not_started' | 'active' | 'incomplete' | 'completed' | 'locked'
  goal: string
  goals: SprintGoalData[]
  team_id: string
  workspace_id: string
  quarter_id: string
  // Sprint locking fields
  locked_at?: string | null
  locked_by?: string | null
  auto_locked?: boolean
  // Legacy compatibility fields
  quarter: string
  team_goals: any[]
  individual_scores: any[]
  created_at: string
  updated_at: string
  manager_completion_status?: { [managerId: string]: boolean }
}

// Generate mock sprint goals based on sprint number and status
export function generateSprintGoals(sprintNumber: number, status: FullSprintRecord['status']): SprintGoalData[] {
  const goalSets = [
    // Sprint 1 goals
    [
      { title: 'Set up development infrastructure', description: 'Configure CI/CD pipeline and development environments' },
      { title: 'Define project architecture', description: 'Establish technical standards and coding guidelines' },
      { title: 'Create initial wireframes', description: 'Design core user flows and interface mockups' }
    ],
    // Sprint 2 goals
    [
      { title: 'Implement user authentication', description: 'Build login, registration, and password reset functionality' },
      { title: 'Create user dashboard', description: 'Develop main user interface and navigation' },
      { title: 'Set up analytics tracking', description: 'Implement user behavior and performance monitoring' }
    ],
    // Sprint 3 goals (active)
    [
      { title: 'Improve user onboarding flow', description: 'Streamline new user experience and reduce drop-off' },
      { title: 'Implement payment gateway', description: 'Integrate Stripe for subscription payments' },
      { title: 'Fix mobile responsiveness issues', description: 'Ensure optimal experience across all devices' }
    ],
    // Sprint 4 goals
    [
      { title: 'Launch referral program', description: 'Build user referral system with rewards' },
      { title: 'Optimize database performance', description: 'Improve query speed and reduce load times' }
    ],
    // Sprint 5 goals
    [
      { title: 'Implement advanced search', description: 'Add filtering and search capabilities' },
      { title: 'Create admin panel', description: 'Build management interface for administrators' }
    ],
    // Sprint 6 goals
    [
      { title: 'Launch beta testing program', description: 'Recruit and manage beta user testing' },
      { title: 'Prepare for public launch', description: 'Final testing and launch preparations' }
    ]
  ]

  const currentGoals = goalSets[sprintNumber - 1] || []

  return currentGoals.map((goal, index) => {
    // Generate a deterministic UUID based on sprint and goal index for consistency
    const generateDeterministicUUID = (sprint: number, idx: number): string => {
      const base = `${sprint}${idx}`.padStart(4, '0');
      return `550e8400-e29b-41d4-a716-44665544${base}`;
    }

    return {
      id: generateDeterministicUUID(sprintNumber, index + 1),
      title: goal.title,
      description: goal.description,
      assigned_team_ids: ['team-1'], // Default team assignment
      kr_id: `kr-${(sprintNumber % 2) + 1}`,
      status: determineGoalStatus(sprintNumber, status, index),
      weight: (sprintNumber + index) % 5 + 1,
      done: determineGoalStatus(sprintNumber, status, index) === 'completed',
      created_by: 'system',
      created_at: new Date().toISOString()
    }
  })
}

// Determine goal completion status based on sprint status
function determineGoalStatus(sprintNumber: number, sprintStatus: FullSprintRecord['status'], goalIndex: number): 'planned' | 'completed' {
  // Completed sprints have all goals completed
  if (sprintStatus === 'completed') return 'completed'

  // Active sprint (Sprint 3) has some goals completed
  if (sprintStatus === 'active' && sprintNumber === 3) {
    return goalIndex < 2 ? 'completed' : 'planned' // First 2 goals completed
  }

  // All other cases: planned
  return 'planned'
}

// Process sprint data for UI consumption
export function processSprintsWithGoals(sprints: FullSprintRecord[]): ProcessedSprint[] {
  return sprints.map(sprint => ({
    id: sprint.id,
    sprint_number: sprint.sprint_index,
    title: sprint.title,
    start_date: sprint.start_date,
    end_date: sprint.end_date,
    status: sprint.status,
    goal: sprint.goal,
    goals: generateSprintGoals(sprint.sprint_index, sprint.status),
    team_id: sprint.team_id,
    workspace_id: sprint.workspace_id,
    quarter_id: sprint.quarter_id,
    // Legacy compatibility fields
    quarter: `Q${Math.floor(new Date().getMonth() / 3) + 1} ${new Date().getFullYear()}`, // Dynamic current quarter
    team_goals: [],
    individual_scores: [],
    created_at: sprint.updated_at,
    updated_at: sprint.updated_at,
    manager_completion_status: {}
  }))
}

// Get team information (will be replaced with real data later)
export interface TeamInfo {
  id: string
  name: string
}

export function getDefaultTeams(): TeamInfo[] {
  return [
    { id: 'dddddddd-dddd-dddd-dddd-dddddddddddd', name: 'Product Development' },
    { id: 'eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee', name: 'Sales & Marketing' },
    { id: 'ffffffff-ffff-ffff-ffff-ffffffffffff', name: 'DevOps' }
  ]
}