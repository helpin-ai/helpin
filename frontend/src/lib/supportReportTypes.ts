export interface SupportReportDuration {
  count: number
  median_seconds: number | null
  p90_seconds: number | null
}

export interface SupportReportSummary {
  conversations_created: number
  conversations_resolved: number
  resolved_by_ai: number
  resolved_by_human: number
  first_response: SupportReportDuration
  first_human_response: SupportReportDuration
  resolution: SupportReportDuration
  ai_involved: number
  ai_resolved: number
  ai_resolved_confirmed: number
  ai_resolved_assumed: number
  ai_handoffs: number
  ai_involvement_rate: number | null
  ai_resolution_rate: number | null
  ai_handoff_rate: number | null
  csat_responses: number
  csat_average: number | null
  csat_positive_rate: number | null
}

export interface SupportReportDay {
  date: string
  conversations_created: number
  conversations_resolved: number
  resolved_by_ai: number
  median_first_human_response_seconds: number | null
}

export interface SupportReportChannel {
  channel: string
  conversations_created: number
  conversations_resolved: number
  first_human_response: SupportReportDuration
}

export interface SupportReportTeammate {
  user_id: string
  name: string
  avatar_url?: string
  resolved: number
  replies_sent: number
  first_human_response: SupportReportDuration
  csat_responses: number
  csat_average: number | null
}

export interface SupportReportCSATComment {
  conversation_id: string
  display_id: number
  subject: string
  rating: number
  comment: string
  channel: string
  resolved_by: 'ai' | 'human'
  submitted_at: string
}

export interface SupportReportOverview {
  range: { from: string; to: string; previous_from: string; previous_to: string; timezone: string }
  summary: SupportReportSummary
  previous: SupportReportSummary
  backlog: { open: number; waiting_on_customer: number; unassigned: number }
  daily: SupportReportDay[]
  channels: SupportReportChannel[]
  teammates: SupportReportTeammate[]
  csat: {
    distribution: [number, number, number, number, number]
    ai_average: number | null
    ai_responses: number
    human_average: number | null
    human_responses: number
    recent_comments: SupportReportCSATComment[]
  }
}

export interface SupportReportParams {
  from: string
  to: string
  tz: string
  mailboxIds: string[]
}
