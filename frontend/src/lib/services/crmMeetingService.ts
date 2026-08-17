import { API_BASE, api, fetchWithSessionAuth } from '../api';
import type { ApiResponse } from '../api';
import type {
  AcceptCRMMeetingActionItemRequest,
  CreateCRMMeetingRequest,
  CRMMeetingActionItem,
  CRMMeetingCapture,
  CRMMeetingDetail,
  CRMMeetingFilters,
  CRMMeetingListResponse,
  CRMMeetingSettings,
  CRMMeetingSettingsResponse,
  CRMCalendarMeetingCandidate,
  CRMCalendarMeetingCandidateListResponse,
  UpdateCRMMeetingRequest,
  UpdateCRMMeetingSettingsRequest,
  UpdateCRMCalendarMeetingCaptureRequest,
  UpdateCRMCalendarSeriesCaptureRequest,
} from '../crmMeetingTypes';

const qs = (workspaceId: string) => `?workspace_id=${encodeURIComponent(workspaceId)}`;

function filtersQuery(filters: CRMMeetingFilters = {}): string {
  const params = new URLSearchParams();
  Object.entries(filters).forEach(([key, value]) => {
    if (value !== undefined && value !== '') params.set(key, String(value));
  });
  const value = params.toString();
  return value ? `&${value}` : '';
}

async function idempotentPost<T>(path: string, body?: unknown, idempotencyKey: string = crypto.randomUUID()): Promise<ApiResponse<T>> {
  const response = await fetchWithSessionAuth(API_BASE, path, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'Idempotency-Key': idempotencyKey,
    },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  if (!response.ok) {
    const payload = await response.json().catch(() => ({ error: response.statusText }));
    return { data: null, error: payload.error ?? response.statusText, status: response.status };
  }
  return { data: await response.json(), error: null, status: response.status };
}

export const crmMeetingService = {
  list: (workspaceId: string, filters?: CRMMeetingFilters) =>
    api.get<CRMMeetingListResponse>(`/crm/meetings${qs(workspaceId)}${filtersQuery(filters)}`),
  listUpcomingCalendar: (workspaceId: string) =>
    api.get<CRMCalendarMeetingCandidateListResponse>(`/crm/meetings/calendar-upcoming${qs(workspaceId)}`),
  updateCalendarCapture: (workspaceId: string, calendarEventId: string, payload: UpdateCRMCalendarMeetingCaptureRequest) =>
    api.put<CRMCalendarMeetingCandidate>(`/crm/meetings/calendar/${calendarEventId}/capture${qs(workspaceId)}`, payload),
  updateCalendarSeriesCapture: (workspaceId: string, payload: UpdateCRMCalendarSeriesCaptureRequest) =>
    api.put<CRMCalendarMeetingCandidateListResponse>(`/crm/meetings/calendar-series/capture${qs(workspaceId)}`, payload),
  get: (workspaceId: string, meetingId: string) =>
    api.get<CRMMeetingDetail>(`/crm/meetings/${meetingId}${qs(workspaceId)}`),
  create: (payload: CreateCRMMeetingRequest, idempotencyKey?: string) => payload.start_now
    ? idempotentPost<CRMMeetingDetail>(`/crm/meetings${qs(payload.workspace_id)}`, payload, idempotencyKey)
    : api.post<CRMMeetingDetail>(`/crm/meetings${qs(payload.workspace_id)}`, payload),
  update: (workspaceId: string, meetingId: string, payload: UpdateCRMMeetingRequest) =>
    api.put<CRMMeetingDetail>(`/crm/meetings/${meetingId}${qs(workspaceId)}`, payload),
  remove: (workspaceId: string, meetingId: string) =>
    api.del(`/crm/meetings/${meetingId}${qs(workspaceId)}`),
  startCapture: (workspaceId: string, meetingId: string, idempotencyKey?: string) =>
    idempotentPost<CRMMeetingCapture>(`/crm/meetings/${meetingId}/capture${qs(workspaceId)}`, undefined, idempotencyKey),
  stopCapture: (workspaceId: string, meetingId: string) =>
    api.post<CRMMeetingCapture>(`/crm/meetings/${meetingId}/capture/stop${qs(workspaceId)}`, {}),
  retryProcessing: (workspaceId: string, meetingId: string) =>
    api.post(`/crm/meetings/${meetingId}/process${qs(workspaceId)}`, {}),
  getRecording: (workspaceId: string, meetingId: string) =>
    api.get<{ url: string }>(`/crm/meetings/${meetingId}/recording${qs(workspaceId)}`),
  deleteRecording: (workspaceId: string, meetingId: string) =>
    api.del(`/crm/meetings/${meetingId}/recording${qs(workspaceId)}`),
  acceptAction: (workspaceId: string, meetingId: string, itemId: string, payload: AcceptCRMMeetingActionItemRequest) =>
    api.post<CRMMeetingActionItem>(`/crm/meetings/${meetingId}/action-items/${itemId}/accept${qs(workspaceId)}`, payload),
  dismissAction: (workspaceId: string, meetingId: string, itemId: string) =>
    api.post<CRMMeetingActionItem>(`/crm/meetings/${meetingId}/action-items/${itemId}/dismiss${qs(workspaceId)}`, {}),
  getSettings: (workspaceId: string) =>
    api.get<CRMMeetingSettingsResponse>(`/crm/meeting-settings${qs(workspaceId)}`),
  updateSettings: (workspaceId: string, payload: UpdateCRMMeetingSettingsRequest) =>
    api.put<CRMMeetingSettings>(`/crm/meeting-settings${qs(workspaceId)}`, payload),
};
