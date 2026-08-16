import { useRef } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { crmMeetingService } from '@/lib/services/crmMeetingService';
import { unwrap } from '@/lib/queryUtils';
import type {
  AcceptCRMMeetingActionItemRequest,
  CreateCRMMeetingRequest,
  CRMMeetingFilters,
  UpdateCRMMeetingRequest,
  UpdateCRMMeetingSettingsRequest,
  UpdateCRMCalendarMeetingCaptureRequest,
} from '@/lib/crmMeetingTypes';

export const crmMeetingKeys = {
  all: (workspaceId: string) => ['crm', workspaceId, 'meetings'] as const,
  list: (workspaceId: string, filters?: CRMMeetingFilters) => ['crm', workspaceId, 'meetings', filters ?? {}] as const,
  detail: (workspaceId: string, meetingId: string) => ['crm', workspaceId, 'meetings', meetingId] as const,
  settings: (workspaceId: string) => ['crm', workspaceId, 'meeting-settings'] as const,
  upcomingCalendar: (workspaceId: string) => ['crm', workspaceId, 'meetings', 'calendar-upcoming'] as const,
};

export function useCRMMeetings(workspaceId: string, filters?: CRMMeetingFilters) {
  return useQuery({
    queryKey: crmMeetingKeys.list(workspaceId, filters),
    queryFn: async () => unwrap(await crmMeetingService.list(workspaceId, filters)),
    enabled: Boolean(workspaceId),
    refetchInterval: (query) => query.state.data?.data.some((meeting) =>
      ['joining', 'waiting', 'recording', 'finalizing', 'processing'].includes(meeting.status)) ? 10_000 : false,
  });
}

export function useUpcomingCalendarMeetings(workspaceId: string) {
  return useQuery({
    queryKey: crmMeetingKeys.upcomingCalendar(workspaceId),
    queryFn: async () => unwrap(await crmMeetingService.listUpcomingCalendar(workspaceId)),
    enabled: Boolean(workspaceId),
    staleTime: 60_000,
  });
}

export function useUpdateCalendarMeetingCapture(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({ calendarEventId, payload }: { calendarEventId: string; payload: UpdateCRMCalendarMeetingCaptureRequest }) =>
      unwrap(await crmMeetingService.updateCalendarCapture(workspaceId, calendarEventId, payload)),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: crmMeetingKeys.all(workspaceId) });
      queryClient.invalidateQueries({ queryKey: crmMeetingKeys.upcomingCalendar(workspaceId) });
    },
  });
}

export function useCRMMeeting(workspaceId: string, meetingId: string) {
  return useQuery({
    queryKey: crmMeetingKeys.detail(workspaceId, meetingId),
    queryFn: async () => unwrap(await crmMeetingService.get(workspaceId, meetingId)),
    enabled: Boolean(workspaceId && meetingId),
    refetchInterval: (query) => {
      const status = query.state.data?.meeting.status;
      return status && ['joining', 'waiting', 'recording', 'finalizing', 'processing'].includes(status) ? 5_000 : false;
    },
  });
}

export function useCreateCRMMeeting(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({ payload, idempotencyKey }: { payload: CreateCRMMeetingRequest; idempotencyKey: string }) =>
      unwrap(await crmMeetingService.create(payload, idempotencyKey)),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: crmMeetingKeys.all(workspaceId) }),
  });
}

export function useUpdateCRMMeeting(workspaceId: string, meetingId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (payload: UpdateCRMMeetingRequest) => unwrap(await crmMeetingService.update(workspaceId, meetingId, payload)),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: crmMeetingKeys.all(workspaceId) });
      queryClient.invalidateQueries({ queryKey: crmMeetingKeys.detail(workspaceId, meetingId) });
    },
  });
}

function useMeetingCommand(workspaceId: string, meetingId: string, command: () => Promise<unknown>, afterSuccess?: () => void) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: command,
    onSuccess: () => {
      afterSuccess?.();
      queryClient.invalidateQueries({ queryKey: crmMeetingKeys.all(workspaceId) });
      queryClient.invalidateQueries({ queryKey: crmMeetingKeys.detail(workspaceId, meetingId) });
    },
  });
}

export function useStartMeetingCapture(workspaceId: string, meetingId: string) {
  const idempotencyKey = useRef(crypto.randomUUID());
  return useMeetingCommand(
    workspaceId,
    meetingId,
    async () => unwrap(await crmMeetingService.startCapture(workspaceId, meetingId, idempotencyKey.current)),
    () => { idempotencyKey.current = crypto.randomUUID(); },
  );
}

export function useStopMeetingCapture(workspaceId: string, meetingId: string) {
  return useMeetingCommand(workspaceId, meetingId, async () => unwrap(await crmMeetingService.stopCapture(workspaceId, meetingId)));
}

export function useRetryMeetingProcessing(workspaceId: string, meetingId: string) {
  return useMeetingCommand(workspaceId, meetingId, async () => unwrap(await crmMeetingService.retryProcessing(workspaceId, meetingId)));
}

export function useAcceptMeetingAction(workspaceId: string, meetingId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({ itemId, payload }: { itemId: string; payload: AcceptCRMMeetingActionItemRequest }) =>
      unwrap(await crmMeetingService.acceptAction(workspaceId, meetingId, itemId, payload)),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: crmMeetingKeys.detail(workspaceId, meetingId) }),
  });
}

export function useDismissMeetingAction(workspaceId: string, meetingId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (itemId: string) => unwrap(await crmMeetingService.dismissAction(workspaceId, meetingId, itemId)),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: crmMeetingKeys.detail(workspaceId, meetingId) }),
  });
}

export function useCRMMeetingSettings(workspaceId: string) {
  return useQuery({
    queryKey: crmMeetingKeys.settings(workspaceId),
    queryFn: async () => unwrap(await crmMeetingService.getSettings(workspaceId)),
    enabled: Boolean(workspaceId),
  });
}

export function useUpdateCRMMeetingSettings(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (payload: UpdateCRMMeetingSettingsRequest) => unwrap(await crmMeetingService.updateSettings(workspaceId, payload)),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: crmMeetingKeys.settings(workspaceId) }),
  });
}
