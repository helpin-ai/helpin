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
  velocity: number // points completed
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
