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

/**
 * One tag's conversations, summarized like the headline numbers. A
 * conversation with several tags counts under each; an empty tag_id is the
 * row of conversations without a tag.
 */
export interface SupportReportTopic {
  tag_id: string
  name: string
  color: string | null
  conversations_created: number
  previous_conversations_created: number
  ai_involved: number
  ai_resolution_rate: number | null
  ai_handoff_rate: number | null
  first_human_response: SupportReportDuration
  resolution: SupportReportDuration
  csat_responses: number
  csat_average: number | null
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

export interface SupportReportSLATarget {
  met: number
  missed: number
  hit_rate: number | null
}

export interface SupportReportOverview {
  range: { from: string; to: string; previous_from: string; previous_to: string; timezone: string }
  summary: SupportReportSummary
  previous: SupportReportSummary
  daily: SupportReportDay[]
  topics: SupportReportTopic[]
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
  sla: {
    conversations: number
    first_response: SupportReportSLATarget
    next_response: SupportReportSLATarget
    resolution: SupportReportSLATarget
  }
  /** Workspace-wide reports only, when customers wrote portal requests in the period. */
  portal_intake?: SupportReportPortalIntake
}

/** What customers did with help center articles suggested while writing portal requests. */
export interface SupportReportPortalIntake {
  with_suggestions: number
  opened: number
  submitted_after_opening: number
  likely_deflected: number
  submitted_without_suggestions: number
}

export interface SupportReportParams {
  from: string
  to: string
  tz: string
  mailboxIds: string[]
}
