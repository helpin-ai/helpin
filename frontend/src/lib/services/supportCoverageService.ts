import { api } from '../api'
import type {
  SupportCoverageGapListResponse,
  SupportCoverageGapDetail,
  SupportCoverageSummary,
  SupportConversationCoverageState,
  SupportGapSuggestion,
} from '../supportCoverageTypes'

const qs = (wsId: string) => `?workspace_id=${encodeURIComponent(wsId)}`

export const supportCoverageService = {
  getSummary: (wsId: string) =>
    api.get<SupportCoverageSummary>(`/support/coverage/summary${qs(wsId)}`),

  listGaps: (wsId: string, filters?: Record<string, string>) => {
    const params = new URLSearchParams({ workspace_id: wsId, ...filters })
    return api.get<SupportCoverageGapListResponse>(`/support/coverage/gaps?${params.toString()}`)
  },

  getGap: (wsId: string, gapId: string) =>
    api.get<SupportCoverageGapDetail>(`/support/coverage/gaps/${gapId}${qs(wsId)}`),

  regenerate: (wsId: string, gapId: string) =>
    api.post<{ status: 'queued' }>(`/support/coverage/gaps/${gapId}/regenerate${qs(wsId)}`, {}),

  updateGapStatus: (wsId: string, gapId: string, status: string, issueResolved?: boolean) =>
    api.post(`/support/coverage/gaps/${gapId}/status${qs(wsId)}`, { status, issue_resolved: issueResolved }),

  reclassifyGap: (wsId: string, gapId: string, v1GapType: string) =>
    api.post(`/support/coverage/gaps/${gapId}/reclassify${qs(wsId)}`, { v1_gap_type: v1GapType }),

  mergeGap: (wsId: string, gapId: string, targetGapId: string) =>
    api.post(`/support/coverage/gaps/${gapId}/merge${qs(wsId)}`, { target_gap_id: targetGapId }),

  createArticleDraft: (wsId: string, gapId: string, payload: { target_space_id: string; target_collection_id?: string }) =>
    api.post<SupportGapSuggestion>(`/support/coverage/gaps/${gapId}/suggestions/article-draft${qs(wsId)}`, payload),

  createArticleUpdate: (wsId: string, gapId: string, payload: { target_document_id: string }) =>
    api.post<SupportGapSuggestion>(`/support/coverage/gaps/${gapId}/suggestions/article-update${qs(wsId)}`, payload),

  applySuggestion: (
    wsId: string,
    suggestionId: string,
    payload?: { route?: string; suggestion_type?: string; target_document_id?: string }
  ) =>
    api.post(`/support/coverage/suggestions/${suggestionId}/apply${qs(wsId)}`, payload ?? {}),

  discardSuggestion: (wsId: string, suggestionId: string) =>
    api.post(`/support/coverage/suggestions/${suggestionId}/discard${qs(wsId)}`, {}),

  getConversationState: (wsId: string, conversationId: string) =>
    api.get<SupportConversationCoverageState>(`/support/coverage/conversations/${conversationId}/state${qs(wsId)}`),

  submitDocsIssueFeedback: (wsId: string, conversationId: string, docsIssue: boolean) =>
    api.post(`/support/coverage/conversations/${conversationId}/docs-issue${qs(wsId)}`, { docs_issue: docsIssue }),
}
