// Workspace Settings and Team Management
import { DEFAULT_TECH_JOB_ROLES } from './defaultJobRoles'

export interface WorkspaceSettings {
  workspace_id: string
  workspace_name: string
  teams: Team[]
  people: Person[]
  general_settings: GeneralSettings
  created_at: string
  updated_at: string
}

export interface Team {
  id: string
  name: string
  description: string
  manager_id: string
  manager_name: string
  member_ids: string[]
  created_at: string
  updated_at: string
}

export interface Person {
  id: string
  name: string
  email: string
  role: 'executive' | 'manager' | 'employee' // Hierarchical role
  job_role: string // Specific job role like 'QA Engineer', 'SEO Specialist', etc.
  base_salary: number // Annual base salary for bonus calculations
  team_ids: string[] // Can be on multiple teams
  manager_id?: string // Who manages this person
  hire_date?: string // Optional hire date
  status: 'active' | 'inactive' | 'archived'
  invite_status: 'not_sent' | 'pending' | 'accepted'
  created_at: string
  updated_at: string

  // Manager-specific permissions
  can_create_goals?: boolean
  can_score_performance?: boolean

  // Account owner and evaluation controls
  is_account_owner?: boolean
  active_for_evaluation?: boolean
  active_for_bonus?: boolean
}



export interface GeneralSettings {
  quarter_start_date: string
  sprint_duration_weeks: number // Default 2
  bonus_tiers: BonusTier[]
  job_role_scoring_criteria: JobRoleScoringCriteria[]
  notifications_enabled: boolean
  auto_calculate_bonuses: boolean
  team_weight: number // Percentage weight for team performance (0-100), individual weight = 100 - team_weight
}

export interface JobRoleScoringCriteria {
  job_role: string // e.g., 'QA Engineer', 'SEO Specialist', 'Documentation Specialist'
  criteria: ScoringCriteria[]
}

export interface BonusTier {
  tier: 'A' | 'B' | 'C'
  min_score: number
  max_score: number
  salary_multiplier: number
  description: string
  editable: boolean
}

export interface ScoringCriteria {
  id: string
  name: string
  description: string
  question: string
  enabled: boolean
  weight: number // For future use
}

// Mock data generator
export const generateMockWorkspaceSettings = (workspaceId: string, workspaceName: string): WorkspaceSettings => {
  // Mock people
  const people: Person[] = [
    {
      id: 'person-1',
      name: 'Alex Chen',
      email: 'alex@teampulse.com',
      role: 'employee',
      job_role: 'Frontend Developer',
      base_salary: 0,
      team_ids: ['team-1'],
      manager_id: 'person-5',
      hire_date: '2023-01-15',
      status: 'active',
      invite_status: 'accepted',
      created_at: '2023-01-15T00:00:00Z',
      updated_at: new Date().toISOString()
    },
    {
      id: 'person-2',
      name: 'Sarah Johnson',
      email: 'sarah@teampulse.com',
      role: 'employee',
      job_role: 'QA Engineer',
      base_salary: 0,
      team_ids: ['team-1'],
      manager_id: 'person-5',
      hire_date: '2023-02-01',
      status: 'active',
      invite_status: 'accepted',
      created_at: '2023-02-01T00:00:00Z',
      updated_at: new Date().toISOString()
    },
    {
      id: 'person-3',
      name: 'Michael Rodriguez',
      email: 'michael@teampulse.com',
      role: 'employee',
      job_role: 'UX Designer',
      base_salary: 0,
      team_ids: ['team-2'],
      manager_id: 'person-6',
      hire_date: '2023-01-20',
      status: 'active',
      invite_status: 'accepted',
      created_at: '2023-01-20T00:00:00Z',
      updated_at: new Date().toISOString()
    },
    {
      id: 'person-4',
      name: 'Emily Davis',
      email: 'emily@teampulse.com',
      role: 'employee',
      job_role: 'Content Writer',
      base_salary: 0,
      team_ids: ['team-2'],
      manager_id: 'person-6',
      hire_date: '2023-03-01',
      status: 'active',
      invite_status: 'accepted',
      created_at: '2023-03-01T00:00:00Z',
      updated_at: new Date().toISOString()
    },
    {
      id: 'person-5',
      name: 'David Kim',
      email: 'david@teampulse.com',
      role: 'manager',
      job_role: 'Engineering Manager',
      base_salary: 0,
      team_ids: ['team-1'],
      hire_date: '2022-08-15',
      status: 'active',
      invite_status: 'accepted',
      can_create_goals: false,
      can_score_performance: true,
      created_at: '2022-08-15T00:00:00Z',
      updated_at: new Date().toISOString()
    },
    {
      id: 'person-6',
      name: 'Lisa Zhang',
      email: 'lisa@teampulse.com',
      role: 'manager',
      job_role: 'Design Manager',
      base_salary: 0,
      team_ids: ['team-2'],
      hire_date: '2022-09-01',
      status: 'active',
      invite_status: 'accepted',
      can_create_goals: false,
      can_score_performance: true,
      created_at: '2022-09-01T00:00:00Z',
      updated_at: new Date().toISOString()
    },
    {
      id: 'person-7',
      name: 'Robert Taylor',
      email: 'robert@teampulse.com',
      role: 'executive',
      job_role: 'CTO',
      base_salary: 0,
      team_ids: [],
      hire_date: '2021-01-01',
      status: 'active',
      invite_status: 'accepted',
      created_at: '2021-01-01T00:00:00Z',
      updated_at: new Date().toISOString()
    },
    {
      id: 'person-8',
      name: 'Maria Gonzalez',
      email: 'maria@teampulse.com',
      role: 'employee',
      job_role: 'SEO Specialist',
      base_salary: 0,
      team_ids: ['team-2'],
      manager_id: 'person-6',
      hire_date: '2023-05-15',
      status: 'active',
      invite_status: 'accepted',
      created_at: '2023-05-15T00:00:00Z',
      updated_at: new Date().toISOString()
    },
    {
      id: 'person-9',
      name: 'James Wright',
      email: 'james@teampulse.com',
      role: 'employee',
      job_role: 'Backend Developer',
      base_salary: 0,
      team_ids: ['team-1'],
      manager_id: 'person-5',
      hire_date: '2023-06-01',
      status: 'active',
      invite_status: 'accepted',
      created_at: '2023-06-01T00:00:00Z',
      updated_at: new Date().toISOString()
    },
    {
      id: 'person-10',
      name: 'Sofia Patel',
      email: 'sofia@teampulse.com',
      role: 'employee',
      job_role: 'Documentation Specialist',
      base_salary: 0,
      team_ids: ['team-1'],
      manager_id: 'person-5',
      hire_date: '2023-07-10',
      status: 'active',
      invite_status: 'accepted',
      created_at: '2023-07-10T00:00:00Z',
      updated_at: new Date().toISOString()
    }
  ]

  // Mock teams
  const teams: Team[] = [
    {
      id: 'team-1',
      name: 'Product Development',
      description: 'Frontend and backend development, UI/UX design',
      manager_id: 'person-5',
      manager_name: 'David Kim',
      member_ids: ['person-1', 'person-2', 'person-5'],
      created_at: '2023-01-01T00:00:00Z',
      updated_at: new Date().toISOString()
    },
    {
      id: 'team-2',
      name: 'Sales & Marketing',
      description: 'Customer acquisition, marketing campaigns, sales operations',
      manager_id: 'person-6',
      manager_name: 'Lisa Zhang',
      member_ids: ['person-3', 'person-4', 'person-6'],
      created_at: '2023-01-01T00:00:00Z',
      updated_at: new Date().toISOString()
    }
  ]



  // Default settings
  const generalSettings: GeneralSettings = {
    quarter_start_date: '2025-01-01', // Consistent demo data
    sprint_duration_weeks: 2,
    bonus_tiers: [
      {
        tier: 'A',
        min_score: 80,
        max_score: 100,
        salary_multiplier: 1.5,
        description: 'Exceptional performance (80-100%) - 1.5x basic salary',
        editable: true
      },
      {
        tier: 'B',
        min_score: 60,
        max_score: 79,
        salary_multiplier: 1.0,
        description: 'Good performance (60-79%) - 1.0x basic salary',
        editable: true
      },
      {
        tier: 'C',
        min_score: 0,
        max_score: 59,
        salary_multiplier: 0,
        description: 'Below expectations (0-59%) - No bonus',
        editable: true
      }
    ],
    job_role_scoring_criteria: DEFAULT_TECH_JOB_ROLES.slice(0, 12), // Use first 12 roles for mock data
    notifications_enabled: true,
    auto_calculate_bonuses: true,
    team_weight: 30 // 30% team, 70% individual (default weighting)
  }

  // Teams now use job role-based criteria instead of team-specific criteria

  return {
    workspace_id: workspaceId,
    workspace_name: workspaceName,
    teams,
    people,
    general_settings: generalSettings,
    created_at: '2023-01-01T00:00:00Z',
    updated_at: new Date().toISOString()
  }
}

// Utility functions
export const getJobRoleScoringCriteria = (jobRole: string, workspaceSettings: WorkspaceSettings): ScoringCriteria[] => {
  const roleConfig = workspaceSettings.general_settings.job_role_scoring_criteria.find(
    config => config.job_role === jobRole
  )
  return roleConfig?.criteria || []
}

export const getPersonScoringCriteria = (personId: string, workspaceSettings: WorkspaceSettings): ScoringCriteria[] => {
  const person = workspaceSettings.people.find(p => p.id === personId)
  if (!person) return []
  return getJobRoleScoringCriteria(person.job_role, workspaceSettings)
}

export const getAvailableJobRoles = (workspaceSettings?: WorkspaceSettings): string[] => {
  if (workspaceSettings) {
    return workspaceSettings.general_settings.job_role_scoring_criteria.map(role => role.job_role)
  }

  // Comprehensive fallback job roles for tech companies
  return [
    // Engineering
    'Frontend Developer',
    'Backend Developer',
    'Full Stack Developer',
    'Mobile Developer (iOS)',
    'Mobile Developer (Android)',
    'React Native Developer',
    'DevOps Engineer',
    'Site Reliability Engineer',
    'Cloud Engineer',
    'Security Engineer',
    'Data Engineer',
    'Machine Learning Engineer',
    'QA Engineer',
    'QA Automation Engineer',
    'Performance Engineer',

    // Design & UX
    'UX Designer',
    'UI Designer',
    'Product Designer',
    'UX Researcher',
    'Visual Designer',
    'Motion Designer',

    // Product & Management
    'Product Manager',
    'Technical Product Manager',
    'Engineering Manager',
    'Senior Engineering Manager',
    'Design Manager',
    'QA Manager',
    'DevOps Manager',
    'Technical Lead',
    'Architect',
    'Principal Engineer',

    // Data & Analytics
    'Data Scientist',
    'Data Analyst',
    'Business Intelligence Analyst',
    'Analytics Engineer',

    // Marketing & Content
    'Content Writer',
    'Technical Writer',
    'Documentation Specialist',
    'SEO Specialist',
    'Digital Marketing Specialist',
    'Growth Marketing Manager',
    'Developer Relations Engineer',

    // Operations & Support
    'Customer Success Manager',
    'Technical Support Engineer',
    'Sales Engineer',
    'Solutions Architect',

    // Leadership
    'CTO',
    'VP of Engineering',
    'Head of Product',
    'Head of Design'
  ]
}

export const getDefaultCriteriaForJobRole = (jobRole: string): ScoringCriteria[] => {
  const defaultCriteria: { [key: string]: ScoringCriteria[] } = {
    'Frontend Developer': [
      {
        id: 'fe-criteria-1',
        name: 'Code Quality',
        description: 'Did the code pass code review without major issues?',
        question: 'Code passed review without major issues?',
        enabled: true,
        weight: 1
      },
      {
        id: 'fe-criteria-2',
        name: 'UI Implementation',
        description: 'Was the UI implemented according to design specifications?',
        question: 'UI matches design specifications?',
        enabled: true,
        weight: 1
      },
      {
        id: 'fe-criteria-3',
        name: 'Cross-browser Compatibility',
        description: 'Does the implementation work consistently across target browsers?',
        question: 'Works across target browsers?',
        enabled: true,
        weight: 1
      }
    ],
    'Backend Developer': [
      {
        id: 'be-criteria-1',
        name: 'API Design',
        description: 'Are APIs well-designed, documented, and follow best practices?',
        question: 'APIs well-designed and documented?',
        enabled: true,
        weight: 1
      },
      {
        id: 'be-criteria-2',
        name: 'Performance Optimization',
        description: 'Is the code optimized for performance and scalability?',
        question: 'Code optimized for performance?',
        enabled: true,
        weight: 1
      },
      {
        id: 'be-criteria-3',
        name: 'Security Practices',
        description: 'Are security best practices followed in the implementation?',
        question: 'Security best practices followed?',
        enabled: true,
        weight: 1
      }
    ]
    // Add more as needed
  }

  return defaultCriteria[jobRole] || []
}

// Helper functions
export const getTeamsByManager = (managerId: string, settings: WorkspaceSettings): Team[] => {
  return settings.teams.filter(team => team.manager_id === managerId)
}

export const getPeopleByTeam = (teamId: string, settings: WorkspaceSettings): Person[] => {
  const team = settings.teams.find(t => t.id === teamId)
  if (!team) return []

  return settings.people.filter(person => person.id && team.member_ids.includes(person.id))
}

export const getManagerByPersonId = (personId: string, settings: WorkspaceSettings): Person | undefined => {
  return settings.people.find(p => p.id === personId && p.role === 'manager')
}

export const canPersonScoreTeam = (personId: string, teamId: string, settings: WorkspaceSettings): boolean => {
  const person = settings.people.find(p => p.id === personId)
  if (!person || person.role !== 'manager') return false

  return person.can_score_performance === true && person.team_ids.includes(teamId)
}

export const canPersonCreateGoals = (personId: string, settings: WorkspaceSettings): boolean => {
  const person = settings.people.find(p => p.id === personId)
  if (!person) return false

  // Executives can always create goals
  if (person.role === 'executive') return true

  // Managers with permission can create goals
  return person.role === 'manager' && person.can_create_goals === true
}

// Normalized scoring system functions
export const calculateMaxPointsPerSprint = (personId: string, settings: WorkspaceSettings): number => {
  const criteria = getPersonScoringCriteria(personId, settings)
  return criteria.length
}

export const calculateMaxQuarterlyPoints = (personId: string, settings: WorkspaceSettings): number => {
  const maxPerSprint = calculateMaxPointsPerSprint(personId, settings)
  return maxPerSprint * 6 // 6 sprints per quarter
}

export const normalizeScoreToPercentage = (rawScore: number, maxPossibleScore: number): number => {
  if (maxPossibleScore === 0) return 0
  return Math.round((rawScore / maxPossibleScore) * 100)
}

export const calculateBonusTier = (percentageScore: number, settings: WorkspaceSettings): BonusTier => {
  const tiers = settings.general_settings.bonus_tiers

  // Find the tier that matches the percentage score
  const tier = tiers.find(t => percentageScore >= t.min_score && percentageScore <= t.max_score)

  // Default to lowest tier if no match found
  return tier || tiers[tiers.length - 1]
}