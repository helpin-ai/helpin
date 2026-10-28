import { api, API_BASE } from '../api'
import { fetchWithSessionAuth } from '@helpin-ai/support-core'
import type {
  SupportReportOverview,
  SupportReportParams,
  SupportReportSLAConversationsPage,
  SupportReportSLAMetric,
  SupportReportSLAOutcome,
} from '../supportReportTypes'

function reportQuery(wsId: string, params: SupportReportParams): string {
  const query = new URLSearchParams({ workspace_id: wsId, from: params.from, to: params.to, tz: params.tz })
  if (params.mailboxIds.length > 0) query.set('mailbox_ids', params.mailboxIds.join(','))
  return query.toString()
}

export const supportReportsService = {
  overview: (wsId: string, params: SupportReportParams) =>
    api.get<SupportReportOverview>(`/support/reports/overview?${reportQuery(wsId, params)}`),

  slaConversations: (wsId: string, params: SupportReportParams, metric: SupportReportSLAMetric, outcome: SupportReportSLAOutcome, page: number) =>
    api.get<SupportReportSLAConversationsPage>(
      `/support/reports/sla/conversations?${reportQuery(wsId, params)}&${new URLSearchParams({ metric, outcome, page: String(page) })}`,
    ),

  /** Downloads the conversation export as a CSV blob. */
  exportConversations: async (wsId: string, params: SupportReportParams): Promise<{ blob: Blob; filename: string }> => {
    const response = await fetchWithSessionAuth(API_BASE, `/support/reports/conversations.csv?${reportQuery(wsId, params)}`)
    if (!response.ok) {
      let message = 'Could not export conversations.'
      try {
        const body = await response.json()
        if (typeof body?.error === 'string') message = body.error
      } catch {
        // Keep the generic message for non-JSON errors.
      }
      throw new Error(message)
    }
    const disposition = response.headers.get('Content-Disposition') ?? ''
    const filename = /filename="([^"]+)"/.exec(disposition)?.[1] ?? 'support-conversations.csv'
    return { blob: await response.blob(), filename }
  },
}
