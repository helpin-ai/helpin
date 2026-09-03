import { useInfiniteQuery, useQuery, useMutation, useQueryClient, type QueryClient, type QueryKey } from '@tanstack/react-query';
import { toast } from 'sonner';
import { queryKeys } from '@/lib/queryKeys';
import { uploadToS3 } from '@/lib/api';
import { supportService } from '@/lib/services/supportService';
import { supportAttachmentService } from '@/lib/services/supportAttachmentService';
import { agentService } from '@/lib/services/agentService';
import { workspacesService } from '@/lib/services/workspacesService';
import { unwrap } from '@/lib/queryUtils';
import { isUpgradeRequiredError } from '@/lib/upgradeRequired';
import {
  extractConversationListConversations,
  getNextConversationIdAfterRemoval,
  isSupportConversationListQueryKey,
  patchConversationDetailPersonalRead,
  patchConversationDetailStatus,
  patchConversationPersonalReadInCache,
  patchConversationStatusInCache,
  type SupportConversationListCache,
} from '@/lib/supportQueryCache';
import {
  appendMessageToNewestPage,
  removeMessageFromPages,
  replaceMessageInPages,
  seedSupportMessagePages,
  type SupportMessagePages,
} from '@/lib/supportMessagePages';
import { useAuthStore } from '@/stores/authStore';
import { useSupportInboxStore } from '@/stores/supportInboxStore';
import type {
  AgentKnowledgeSource,
  CuratedGuidance,
  CreateCuratedGuidanceRequest,
  UpdateCuratedGuidanceRequest,
  SupportContentSource,
  SupportContentPage,
  AgentKnowledgeSourceRequest,
  CreateSupportContentSourceRequest,
  CreateSupportContentSourceFileUploadRequest,
  UpdateSupportContentSourceRequest,
  SupportInboxSettings,
  ConversationStatus,
  ConversationListResponse,
  SupportConversation,
  SupportMessagePage,
  VisitorContextResponse,
  SupportAIRewriteDraftRequest,
  CreateSupportMailboxRequest,
  UpdateSupportMailboxRequest,
  SupportInboxView,
  SupportInboxViewCount,
  CreateSupportInboxViewRequest,
  UpdateSupportInboxViewRequest,
  UpdateSupportInboxBuiltinViewRequest,
  CreateSupportEmailRouteRequest,
  SupportEmailRoute,
  CreateSupportEmailSenderRequest,
  SetSupportEmailSenderDefaultRequest,
  UpdateSupportEmailSenderRequest,
  SupportEmailSender,
  CreateSupportEmailSenderDomainRequest,
  SupportEmailSenderDomain,
  SupportTriageRule,
  CreateSupportTriageRuleRequest,
  UpdateSupportTriageRuleRequest,
  SupportCannedResponse,
  CreateCannedResponseRequest,
  UpdateCannedResponseRequest,
  SupportMessage,
  SupportTag,
  CreateConversationRequest,
  CreateConversationWithMessageRequest,
  UpdateConversationEmailRecipientsRequest,
  SupportConversationSearchParams,
  SupportConversationSearchResponse,
} from '@/lib/pmTypes';

const SUPPORT_CONVERSATIONS_PER_PAGE = 50;
const CONVERSATION_HANDOFF_DELAY_MS = 180;

export type SupportConversationFilters = {
  status?: string;
  statuses?: string;
  priority?: string;
  filter?: string;
  mailbox_id?: string | null;
  ai_state?: string;
  ai?: string;
  flow_state?: string;
  search?: string;
  assigned_to?: string;
  sort?: string;
  tag_ids?: string;
  system_tags?: string;
};

export type SupportConversationGlobalSearchFilters = SupportConversationSearchParams;

type SendMessagePayload = {
  content: string;
  client_message_id?: string;
  is_internal?: boolean;
  ai_assisted?: boolean;
  channels?: Array<'chat' | 'email'>;
  attachment_ids?: string[];
  cc_emails?: string[];
  bcc_emails?: string[];
};

type OptimisticSupportUser = {
  id?: string | null;
  full_name?: string | null;
  email?: string | null;
  avatar_url?: string | null;
};

export function buildOptimisticSupportMessage({
  workspaceId,
  conversationId,
  payload,
  user,
  now,
  optimisticId,
}: {
  workspaceId: string;
  conversationId: string;
  payload: SendMessagePayload;
  user: OptimisticSupportUser | null;
  now: string;
  optimisticId: string;
}): SupportMessage {
  return {
    id: optimisticId,
    client_message_id: optimisticId,
    workspace_id: workspaceId,
    conversation_id: conversationId,
    sender_type: 'user',
    sender_user_id: user?.id ?? undefined,
    sender_display_name: user?.full_name?.trim() || user?.email?.trim() || 'You',
    sender_avatar_url: user?.avatar_url ?? undefined,
    content: payload.content.trim() || ' ',
    message_type: 'reply',
    is_internal: Boolean(payload.is_internal),
    via_channel: payload.channels?.includes('email') ? 'email' : 'widget',
    created_at: now,
    updated_at: now,
  };
}

export function appendOptimisticSupportMessage(current: SupportMessage[] | undefined, message: SupportMessage) {
  const existing = current ?? [];
  if (existing.some((item) => item.id === message.id)) return existing;
  return [...existing, message];
}

export function reconcileOptimisticSupportMessage(
  current: SupportMessage[] | undefined,
  optimisticId: string,
  persisted: SupportMessage,
) {
  const existing = current ?? [];
  if (existing.some((item) => item.id === persisted.id)) {
    return existing.filter((item) => item.id !== optimisticId);
  }
  if (existing.some((item) => item.id === optimisticId)) {
    return existing.map((item) => item.id === optimisticId ? persisted : item);
  }
  return [...existing, persisted];
}

function invalidateSupportInboxViewCounts(queryClient: QueryClient, workspaceId: string) {
  queryClient.invalidateQueries({ queryKey: queryKeys.support.inboxViewCounts(workspaceId) });
}

async function loadConversationListPage(
  workspaceId: string,
  filters?: SupportConversationFilters & { page?: number; per_page?: number },
): Promise<ConversationListResponse> {
  const res = await supportService.listConversations(workspaceId, filters);
  if (res.error) throw new Error(res.error);
  const data = res.data;
  if (data && 'data' in data && Array.isArray(data.data)) {
    return data as ConversationListResponse;
  }
  const arr = Array.isArray(data) ? data : [];
  return {
    data: arr,
    total: arr.length,
    page: filters?.page ?? 1,
    per_page: filters?.per_page ?? SUPPORT_CONVERSATIONS_PER_PAGE,
    total_pages: 1,
    meta: {
      unread: {
        inbox: 0,
        mine: 0,
        waiting: 0,
        ai_active: 0,
        total: 0,
        my_inbox: 0,
        unassigned: 0,
      },
    },
  } satisfies ConversationListResponse;
}

// ── Installation settings ───────────────────────────────────────────

export function useChatSettings(workspaceId: string) {
  return useQuery({
    queryKey: queryKeys.support.installation(workspaceId),
    queryFn: async () => unwrap(await supportService.getInstallation(workspaceId)),
    enabled: !!workspaceId,
    staleTime: 60_000,
  });
}

export function useSupportRoutingUsage(workspaceId: string) {
  return useQuery({
    queryKey: queryKeys.support.routingUsage(workspaceId),
    queryFn: async () => unwrap(await supportService.getRoutingUsageStatus(workspaceId)),
    enabled: !!workspaceId,
    staleTime: 30_000,
    refetchInterval: 60_000,
  });
}

export function useUpdateChatSettings(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (settings: Partial<SupportInboxSettings>) =>
      supportService.updateInstallationSettings(workspaceId, settings).then(unwrap),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.support.installation(workspaceId) });
      queryClient.invalidateQueries({ queryKey: queryKeys.support.routingUsage(workspaceId) });
    },
    onError: (error: Error) => {
      toast.error('Failed to update chat settings', { description: error.message });
    },
  });
}

export function useRegenerateWidgetKey(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => supportService.regenerateWidgetKey(workspaceId).then(unwrap),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.support.installation(workspaceId) });
    },
    onError: (error: Error) => {
      toast.error('Failed to regenerate widget key', { description: error.message });
    },
  });
}

// ── Conversations ───────────────────────────────────────────────────

export function useConversations(workspaceId: string, filters?: SupportConversationFilters) {
  return useQuery({
    queryKey: [...queryKeys.support.conversations(workspaceId), filters] as const,
    queryFn: async (): Promise<ConversationListResponse> => loadConversationListPage(workspaceId, filters),
    enabled: !!workspaceId,
    staleTime: 15_000,
  });
}

export function hasSupportConversationSearchInput(filters: SupportConversationGlobalSearchFilters) {
  return Object.entries(filters).some(([key, value]) => {
    if (key === 'page' || key === 'per_page' || key === 'sort') return false;
    return typeof value === 'string' ? value.trim().length > 0 : value !== undefined && value !== null;
  });
}

export function useSupportConversationSearch(
  workspaceId: string,
  filters: SupportConversationGlobalSearchFilters,
  enabled = true,
) {
  return useQuery({
    queryKey: queryKeys.support.search(workspaceId, filters),
    queryFn: async (): Promise<SupportConversationSearchResponse> =>
      unwrap(await supportService.searchConversations(workspaceId, filters)),
    enabled: enabled && !!workspaceId && hasSupportConversationSearchInput(filters),
    staleTime: 15_000,
  });
}

export function useInfiniteConversations(workspaceId: string, filters?: SupportConversationFilters) {
  return useInfiniteQuery({
    queryKey: [...queryKeys.support.conversations(workspaceId), 'infinite', filters] as const,
    queryFn: async ({ pageParam }): Promise<ConversationListResponse> =>
      loadConversationListPage(workspaceId, {
        ...filters,
        page: pageParam as number,
        per_page: SUPPORT_CONVERSATIONS_PER_PAGE,
      }),
    initialPageParam: 1,
    getNextPageParam: (lastPage) => {
      if (lastPage.page >= lastPage.total_pages) {
        return undefined;
      }
      return lastPage.page + 1;
    },
    enabled: !!workspaceId,
    staleTime: 15_000,
  });
}

export function useUnreadStats(workspaceId: string, mailboxId?: string | null, enabled = true) {
  return useQuery({
    queryKey: [...queryKeys.support.unreadStats(workspaceId), mailboxId ?? 'all'] as const,
    queryFn: async () => unwrap(await supportService.getUnreadStats(workspaceId, mailboxId)),
    enabled: !!workspaceId && enabled,
    staleTime: 15_000,
  });
}

export function useInboxScopes(workspaceId: string, enabled = true) {
  return useQuery({
    queryKey: queryKeys.support.inboxScopes(workspaceId),
    queryFn: async () => unwrap(await supportService.listInboxScopes(workspaceId)),
    enabled: !!workspaceId && enabled,
    staleTime: 15_000,
  });
}

export function useSupportInboxViews(workspaceId: string, enabled = true) {
  return useQuery({
    queryKey: queryKeys.support.inboxViews(workspaceId),
    queryFn: async (): Promise<SupportInboxView[]> => unwrap(await supportService.listInboxViews(workspaceId)),
    enabled: !!workspaceId && enabled,
    staleTime: 30_000,
  });
}

export function useSupportBuiltinInboxViews(workspaceId: string, enabled = true) {
  return useQuery({
    queryKey: queryKeys.support.builtinInboxViews(workspaceId),
    queryFn: async (): Promise<SupportInboxView[]> => unwrap(await supportService.listBuiltinInboxViews(workspaceId)),
    enabled: !!workspaceId && enabled,
    staleTime: 30_000,
  });
}

export function useSupportInboxViewCounts(workspaceId: string, enabled = true) {
  return useQuery({
    queryKey: queryKeys.support.inboxViewCounts(workspaceId),
    queryFn: async (): Promise<SupportInboxViewCount[]> => unwrap(await supportService.listInboxViewCounts(workspaceId)),
    enabled: !!workspaceId && enabled,
    staleTime: 15_000,
    refetchOnWindowFocus: true,
  });
}

export function useCreateSupportInboxView(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (payload: CreateSupportInboxViewRequest) =>
      supportService.createInboxView(workspaceId, payload).then(unwrap),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.support.inboxViews(workspaceId) });
      queryClient.invalidateQueries({ queryKey: queryKeys.support.inboxViewCounts(workspaceId) });
    },
  });
}

export function useUpdateSupportInboxView(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, ...payload }: UpdateSupportInboxViewRequest & { id: string }) =>
      supportService.updateInboxView(workspaceId, id, payload).then(unwrap),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.support.inboxViews(workspaceId) });
      queryClient.invalidateQueries({ queryKey: queryKeys.support.inboxViewCounts(workspaceId) });
      toast.success('View updated');
    },
  });
}

export function useUpdateSupportBuiltinInboxView(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ view_key, ...payload }: UpdateSupportInboxBuiltinViewRequest) =>
      supportService.updateBuiltinInboxView(workspaceId, view_key, { view_key, ...payload }).then(unwrap),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.support.builtinInboxViews(workspaceId) });
      queryClient.invalidateQueries({ queryKey: queryKeys.support.inboxViewCounts(workspaceId) });
      toast.success('View updated');
    },
  });
}

export function useDeleteSupportInboxView(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => supportService.deleteInboxView(workspaceId, id).then(unwrap),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.support.inboxViews(workspaceId) });
      queryClient.invalidateQueries({ queryKey: queryKeys.support.inboxViewCounts(workspaceId) });
    },
  });
}

export function useSupportUnreadByWorkspace(enabled = true) {
  return useQuery({
    queryKey: queryKeys.support.workspaceUnread(),
    queryFn: async () => unwrap(await supportService.listWorkspaceUnread()),
    enabled,
    staleTime: 30_000,
    refetchInterval: 60_000,
    refetchOnWindowFocus: true,
  });
}

export function useSupportMailboxes(workspaceId: string) {
  return useQuery({
    queryKey: queryKeys.support.mailboxes(workspaceId),
    queryFn: async () => unwrap(await supportService.listMailboxes(workspaceId)),
    enabled: !!workspaceId,
    staleTime: 15_000,
  });
}

export function useMailboxMembers(workspaceId: string, mailboxId?: string | null) {
  return useQuery({
    queryKey: queryKeys.support.mailboxMembers(workspaceId, mailboxId ?? ''),
    queryFn: async () => {
      const data = unwrap(await supportService.listMailboxMembers(workspaceId, mailboxId!));
      return Array.isArray(data) ? data : [];
    },
    enabled: !!workspaceId && !!mailboxId,
    staleTime: 15_000,
  });
}

export function useSupportEmailRoutes(workspaceId: string) {
  return useQuery({
    queryKey: queryKeys.support.emailRoutes(workspaceId),
    queryFn: async (): Promise<SupportEmailRoute[]> => unwrap(await supportService.listEmailRoutes(workspaceId)),
    enabled: !!workspaceId,
    staleTime: 15_000,
    refetchInterval: (query) => {
      const routes = query.state.data;
      const waiting = routes?.some((route) => {
        if (!route.verification_sent_at) return false;
        const verificationSentAt = new Date(route.verification_sent_at).getTime();
        if (!Number.isFinite(verificationSentAt) || Date.now() - verificationSentAt > 10 * 60 * 1000) return false;
        if (!route.forwarding_verified_at) return true;
        return verificationSentAt > new Date(route.forwarding_verified_at).getTime();
      });
      return waiting ? 3_000 : false;
    },
  });
}

export function useSupportEmailSenders(workspaceId: string) {
  return useQuery({
    queryKey: queryKeys.support.emailSenders(workspaceId),
    queryFn: async (): Promise<SupportEmailSender[]> => unwrap(await supportService.listEmailSenders(workspaceId)),
    enabled: !!workspaceId,
    staleTime: 15_000,
  });
}

export function useSupportEmailSenderDomains(workspaceId: string) {
  return useQuery({
    queryKey: queryKeys.support.emailSenderDomains(workspaceId),
    queryFn: async (): Promise<SupportEmailSenderDomain[]> => unwrap(await supportService.listEmailSenderDomains(workspaceId)),
    enabled: !!workspaceId,
    staleTime: 15_000,
  });
}

export function useSupportTriageRules(workspaceId: string) {
  return useQuery({
    queryKey: queryKeys.support.triageRules(workspaceId),
    queryFn: async (): Promise<SupportTriageRule[]> => unwrap(await supportService.listTriageRules(workspaceId)),
    enabled: !!workspaceId,
    staleTime: 15_000,
  });
}

export function useCannedResponses(workspaceId: string) {
  return useQuery({
    queryKey: queryKeys.support.cannedResponses(workspaceId),
    queryFn: async (): Promise<SupportCannedResponse[]> => unwrap(await supportService.listCannedResponses(workspaceId)),
    enabled: !!workspaceId,
    staleTime: 30_000,
  });
}

export function useSupportTags(workspaceId: string) {
  return useQuery({
    queryKey: queryKeys.support.tags(workspaceId),
    queryFn: async (): Promise<SupportTag[]> => unwrap(await supportService.listTags(workspaceId)),
    enabled: !!workspaceId,
    staleTime: 30_000,
  });
}

export function useSearchCannedResponses(workspaceId: string, query: string, enabled = true) {
  const trimmed = query.trim();
  return useQuery({
    queryKey: queryKeys.support.cannedResponseSearch(workspaceId, trimmed),
    queryFn: async (): Promise<SupportCannedResponse[]> => unwrap(await supportService.searchCannedResponses(workspaceId, trimmed)),
    enabled: !!workspaceId && enabled && trimmed.length > 0,
    staleTime: 15_000,
  });
}

export function useSupportTeammatePresence(workspaceId: string, enabled = true) {
  return useQuery({
    queryKey: queryKeys.support.teammatePresence(workspaceId),
    queryFn: async () => unwrap(await supportService.listTeammatePresence(workspaceId)),
    enabled: !!workspaceId && enabled,
    staleTime: 15_000,
    refetchInterval: 30_000,
    refetchIntervalInBackground: true,
  });
}

export function useUpdateMySupportTeammatePresence(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (manualStatus: 'online' | 'away' | 'offline' | null) =>
      supportService.updateMyTeammatePresence(workspaceId, manualStatus).then(unwrap),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.support.teammatePresence(workspaceId) });
    },
    onError: (error: Error) => {
      toast.error('Failed to update support status', { description: error.message });
    },
  });
}

export function useCreateSupportEmailRoute(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (payload: CreateSupportEmailRouteRequest) =>
      supportService.createEmailRoute(workspaceId, payload).then(unwrap),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.support.emailRoutes(workspaceId) });
    },
    onError: (error: Error) => {
      toast.error('Failed to enable email forwarding', { description: error.message });
    },
  });
}

export function useSendSupportEmailRouteTest(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ routeId, sourceAddress }: { routeId: string; sourceAddress: string }) =>
      supportService.sendEmailRouteTest(workspaceId, routeId, sourceAddress).then(unwrap),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.support.emailRoutes(workspaceId) });
    },
    onError: (error: Error) => {
      toast.error('Failed to send forwarding test', { description: error.message });
    },
  });
}

export function useCreateSupportEmailSender(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (payload: CreateSupportEmailSenderRequest) =>
      supportService.createEmailSender(workspaceId, payload).then(unwrap),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.support.emailSenders(workspaceId) });
    },
    onError: (error: Error) => {
      toast.error('Failed to add sender address', { description: error.message });
    },
  });
}

export function useVerifySupportEmailSender(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (senderId: string) =>
      supportService.verifyEmailSender(workspaceId, senderId).then(unwrap),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.support.emailSenders(workspaceId) });
    },
    onError: (error: Error) => {
      toast.error('Failed to verify sender address', { description: error.message });
    },
  });
}

export function useSetDefaultSupportEmailSender(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ senderId, payload }: { senderId: string; payload: SetSupportEmailSenderDefaultRequest }) =>
      supportService.setDefaultEmailSender(workspaceId, senderId, payload).then(unwrap),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.support.emailSenders(workspaceId) });
    },
    onError: (error: Error) => {
      toast.error('Failed to update sender default', { description: error.message });
    },
  });
}

export function useUpdateSupportEmailSender(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ senderId, payload }: { senderId: string; payload: UpdateSupportEmailSenderRequest }) =>
      supportService.updateEmailSender(workspaceId, senderId, payload).then(unwrap),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.support.emailSenders(workspaceId) });
    },
    onError: (error: Error) => {
      toast.error('Failed to update sender address', { description: error.message });
    },
  });
}

export function useDisableSupportEmailSender(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (senderId: string) =>
      supportService.disableEmailSender(workspaceId, senderId).then(unwrap),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.support.emailSenders(workspaceId) });
    },
    onError: (error: Error) => {
      toast.error('Failed to disable sender address', { description: error.message });
    },
  });
}

export function useCreateSupportEmailSenderDomain(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (payload: CreateSupportEmailSenderDomainRequest) =>
      supportService.createEmailSenderDomain(workspaceId, payload).then(unwrap),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.support.emailSenderDomains(workspaceId) });
    },
    onError: (error: Error) => {
      toast.error('Failed to add sender domain', { description: error.message });
    },
  });
}

export function useVerifySupportEmailSenderDomain(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (domainId: string) =>
      supportService.verifyEmailSenderDomain(workspaceId, domainId).then(unwrap),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.support.emailSenderDomains(workspaceId) });
      queryClient.invalidateQueries({ queryKey: queryKeys.support.emailSenders(workspaceId) });
    },
    onError: (error: Error) => {
      toast.error('Failed to verify sender domain', { description: error.message });
    },
  });
}

export function useActivateSupportEmailSenderDomain(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (domainId: string) =>
      supportService.activateEmailSenderDomain(workspaceId, domainId).then(unwrap),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.support.emailSenderDomains(workspaceId) });
    },
    onError: (error: Error) => {
      toast.error('Failed to activate sender domain', { description: error.message });
    },
  });
}

export function useDeactivateSupportEmailSenderDomain(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (domainId: string) =>
      supportService.deactivateEmailSenderDomain(workspaceId, domainId).then(unwrap),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.support.emailSenderDomains(workspaceId) });
    },
    onError: (error: Error) => {
      toast.error('Failed to deactivate sender domain', { description: error.message });
    },
  });
}

export function useCreateSupportTriageRule(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (payload: CreateSupportTriageRuleRequest) =>
      supportService.createTriageRule(workspaceId, payload).then(unwrap),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.support.triageRules(workspaceId) });
    },
    onError: (error: Error) => {
      toast.error('Failed to create routing rule', { description: error.message });
    },
  });
}

export function useUpdateSupportTriageRule(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ ruleId, payload }: { ruleId: string; payload: UpdateSupportTriageRuleRequest }) =>
      supportService.updateTriageRule(workspaceId, ruleId, payload).then(unwrap),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.support.triageRules(workspaceId) });
    },
    onError: (error: Error) => {
      toast.error('Failed to update routing rule', { description: error.message });
    },
  });
}

export function useDeleteSupportTriageRule(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (ruleId: string) =>
      supportService.deleteTriageRule(workspaceId, ruleId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.support.triageRules(workspaceId) });
    },
    onError: (error: Error) => {
      toast.error('Failed to delete routing rule', { description: error.message });
    },
  });
}

export function useCreateCannedResponse(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (payload: CreateCannedResponseRequest) =>
      supportService.createCannedResponse(workspaceId, payload).then(unwrap),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.support.cannedResponses(workspaceId) });
    },
    onError: (error: Error) => {
      toast.error('Failed to create shortcut', { description: error.message });
    },
  });
}

export function useUpdateCannedResponse(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ responseId, payload }: { responseId: string; payload: UpdateCannedResponseRequest }) =>
      supportService.updateCannedResponse(workspaceId, responseId, payload).then(unwrap),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.support.cannedResponses(workspaceId) });
    },
    onError: (error: Error) => {
      toast.error('Failed to update shortcut', { description: error.message });
    },
  });
}

export function useDeleteCannedResponse(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (responseId: string) =>
      supportService.deleteCannedResponse(workspaceId, responseId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.support.cannedResponses(workspaceId) });
    },
    onError: (error: Error) => {
      toast.error('Failed to delete shortcut', { description: error.message });
    },
  });
}

export function useCreateSupportTag(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (payload: { name: string; color?: string }) =>
      supportService.createTag(workspaceId, payload).then(unwrap),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.support.tags(workspaceId) });
    },
    onError: (error: Error) => {
      toast.error('Failed to create tag', { description: error.message });
    },
  });
}

export function useUpdateSupportTag(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ tagId, payload }: { tagId: string; payload: { name?: string; color?: string } }) =>
      supportService.updateTag(workspaceId, tagId, payload).then(unwrap),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.support.tags(workspaceId) });
      queryClient.invalidateQueries({ queryKey: queryKeys.support.conversations(workspaceId) });
      invalidateSupportInboxViewCounts(queryClient, workspaceId);
    },
    onError: (error: Error) => {
      toast.error('Failed to update tag', { description: error.message });
    },
  });
}

export function useDeleteSupportTag(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (tagId: string) => supportService.deleteTag(workspaceId, tagId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.support.tags(workspaceId) });
      queryClient.invalidateQueries({ queryKey: queryKeys.support.conversations(workspaceId) });
      invalidateSupportInboxViewCounts(queryClient, workspaceId);
    },
    onError: (error: Error) => {
      toast.error('Failed to delete tag', { description: error.message });
    },
  });
}

export function useDisableSupportEmailRoute(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (routeId: string) => supportService.disableEmailRoute(workspaceId, routeId).then(unwrap),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.support.emailRoutes(workspaceId) });
    },
    onError: (error: Error) => {
      toast.error('Failed to disable email forwarding', { description: error.message });
    },
  });
}

export function useConversation(workspaceId: string, conversationId: string | null) {
  return useQuery({
    queryKey: queryKeys.support.conversation(workspaceId, conversationId ?? ''),
    queryFn: async () => unwrap(await supportService.getConversation(workspaceId, conversationId!)),
    enabled: !!workspaceId && !!conversationId,
    staleTime: 30_000,
  });
}

export function useConversationAssignees(workspaceId: string, conversationId: string | null) {
  return useQuery({
    queryKey: queryKeys.support.conversationAssignees(workspaceId, conversationId ?? ''),
    queryFn: async () => {
      const response = await supportService.listConversationAssignees(workspaceId, conversationId!);
      if (!response.error && Array.isArray(response.data) && response.data.length > 0) {
        return response.data;
      }

      const fallback = await workspacesService.listAssignableMembers(workspaceId);
      const members = unwrap(fallback);
      const currentUserID = useAuthStore.getState().user?.id;
      return members.filter((member) =>
        member.status === 'active' &&
        !!member.user_id &&
        (
          member.role === 'owner' ||
          member.role === 'admin' ||
          member.user_id === currentUserID
        ),
      );
    },
    enabled: !!workspaceId && !!conversationId,
    staleTime: 30_000,
  });
}

export function useConversationMessages(workspaceId: string, conversationId: string | null) {
  return useInfiniteQuery<SupportMessagePage, Error, SupportMessagePages, QueryKey, string | undefined>({
    queryKey: queryKeys.support.messages(workspaceId, conversationId ?? ''),
    queryFn: async ({ pageParam }) => unwrap(await supportService.listConversationMessagePage(
      workspaceId,
      conversationId!,
      20,
      pageParam,
    )),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (lastPage) => lastPage.next_cursor ?? undefined,
    enabled: !!workspaceId && !!conversationId,
    staleTime: 60_000,
  });
}

export function useSendConversationTranscript(workspaceId: string) {
  return useMutation({
    mutationFn: ({ conversationId, email, updateCustomerEmail }: { conversationId: string; email?: string; updateCustomerEmail?: boolean }) =>
      supportService.sendConversationTranscript(workspaceId, conversationId, {
        email,
        update_customer_email: updateCustomerEmail,
      }).then(unwrap),
  });
}

export function useMessageEmailDetail(workspaceId: string, messageId: string | null, enabled = true) {
  return useQuery({
    queryKey: queryKeys.support.messageEmail(workspaceId, messageId ?? ''),
    queryFn: async () => unwrap(await supportService.getMessageEmailDetail(workspaceId, messageId!)),
    enabled: enabled && !!workspaceId && !!messageId,
    staleTime: 5 * 60_000,
  });
}

export function useMessageInfo(workspaceId: string, conversationId: string, messageId: string | null, enabled = true) {
  return useQuery({
    queryKey: queryKeys.support.messageInfo(workspaceId, conversationId, messageId ?? ''),
    queryFn: async () => unwrap(await supportService.getConversationMessageInfo(workspaceId, conversationId, messageId!)),
    enabled: enabled && !!workspaceId && !!conversationId && !!messageId,
    staleTime: 60_000,
  });
}

export function useVisitorContext(workspaceId: string, conversationId: string | null) {
  return useQuery<VisitorContextResponse>({
    queryKey: queryKeys.support.visitorContext(workspaceId, conversationId ?? ''),
    queryFn: async () => unwrap(await supportService.getVisitorContext(workspaceId, conversationId!)),
    enabled: !!workspaceId && !!conversationId,
    staleTime: 60_000,
  });
}

// ── Mutations ───────────────────────────────────────────────────────

export function useSendMessage(workspaceId: string, conversationId: string | null) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (payload: SendMessagePayload) =>
      supportService.createConversationMessage(workspaceId, conversationId!, payload).then(unwrap),
    onMutate: async (payload) => {
      if (!conversationId) return { previousMessages: undefined as SupportMessagePages | undefined, optimisticId: '' };
      const key = queryKeys.support.messages(workspaceId, conversationId);
      await queryClient.cancelQueries({ queryKey: key });
      const previousMessages = queryClient.getQueryData<SupportMessagePages>(key);
      const now = new Date().toISOString();
      const optimisticId = payload.client_message_id?.trim()
        || `optimistic-${conversationId}-${crypto.randomUUID()}`;
      payload.client_message_id = optimisticId;
      const optimistic = buildOptimisticSupportMessage({
        workspaceId,
        conversationId,
        payload,
        user: useAuthStore.getState().user,
        now,
        optimisticId,
      });
      queryClient.setQueryData<SupportMessagePages>(key, (current) => appendMessageToNewestPage(current, optimistic));
      return { previousMessages, optimisticId };
    },
    onSuccess: (message, _payload, context) => {
      if (conversationId) {
        if (context?.optimisticId) {
          queryClient.setQueryData<SupportMessagePages>(
            queryKeys.support.messages(workspaceId, conversationId),
            (current) => replaceMessageInPages(current, context.optimisticId, message),
          );
        }
      }
      queryClient.invalidateQueries({ queryKey: queryKeys.support.conversations(workspaceId) });
      invalidateSupportInboxViewCounts(queryClient, workspaceId);
    },
    onError: (error: Error, _payload, context) => {
      if (conversationId && context?.previousMessages) {
        queryClient.setQueryData(queryKeys.support.messages(workspaceId, conversationId), context.previousMessages);
      }
      toast.error('Failed to send message', { description: error.message });
    },
  });
}

export function useDeleteSupportMessage(workspaceId: string, conversationId: string | null) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ messageId, undo }: { messageId: string; undo?: boolean }) =>
      supportService.deleteConversationMessage(workspaceId, conversationId!, messageId, !!undo).then(unwrap),
    onMutate: async ({ messageId }) => {
      if (!conversationId) return { previousMessages: undefined as SupportMessagePages | undefined };
      const key = queryKeys.support.messages(workspaceId, conversationId);
      await queryClient.cancelQueries({ queryKey: key });
      const previousMessages = queryClient.getQueryData<SupportMessagePages>(key);
      queryClient.setQueryData<SupportMessagePages>(key, (current) => removeMessageFromPages(current, messageId));
      return { previousMessages };
    },
    onError: (error: Error, _variables, context) => {
      if (conversationId && context?.previousMessages) {
        queryClient.setQueryData(queryKeys.support.messages(workspaceId, conversationId), context.previousMessages);
      }
      toast.error('Failed to remove message', { description: error.message });
    },
    onSettled: () => {
      if (conversationId) {
        queryClient.invalidateQueries({ queryKey: queryKeys.support.messages(workspaceId, conversationId) });
      }
      queryClient.invalidateQueries({ queryKey: queryKeys.support.conversations(workspaceId) });
      invalidateSupportInboxViewCounts(queryClient, workspaceId);
    },
  });
}

export function useRewriteSupportDraft(workspaceId: string, conversationId: string | null) {
  return useMutation({
    mutationFn: (payload: SupportAIRewriteDraftRequest) =>
      (conversationId
        ? supportService.rewriteConversationDraft(workspaceId, conversationId, payload)
        : supportService.rewriteNewDraft(workspaceId, payload)
      ).then(unwrap),
    onError: (error: Error) => {
      if (isUpgradeRequiredError(error)) return;
      toast.error('Failed to rewrite draft', { description: error.message });
    },
  });
}

export function useUploadSupportAttachment(workspaceId: string, conversationId: string | null) {
  return useMutation({
    mutationFn: async ({ file }: { file: File }) => {
      if (!conversationId) throw new Error('No conversation');

      // Step 1: Initiate — get presigned URL
      const initData = unwrap(await supportAttachmentService.initiateUpload(
        workspaceId, conversationId, {
          file_name: file.name,
          file_size: file.size,
          content_type: file.type || 'application/octet-stream',
        },
      ));

      // Step 2: Upload to S3 via presigned PUT URL
      const uploadResp = await fetch(initData.upload_url, {
        method: 'PUT',
        body: file,
        headers: {
          'Content-Type': file.type || 'application/octet-stream',
          'x-amz-acl': 'public-read',
        },
      });
      if (!uploadResp.ok) throw new Error('Upload to storage failed');

      // Step 3: Confirm upload
      await supportAttachmentService.confirmUpload(workspaceId, initData.attachment.id);

      return { id: initData.attachment.id, url: initData.public_url };
    },
    onError: (error: Error) => {
      toast.error('Failed to upload file', { description: error.message });
    },
  });
}

export function useUpdateConversationStatus(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ conversationId, status }: { conversationId: string; status: ConversationStatus }) =>
      supportService.updateConversationStatus(workspaceId, conversationId, status).then(unwrap),
    onSuccess: (data, variables) => {
      const { conversationId, status } = variables;
      const currentState = useSupportInboxStore.getState();

      const applyStatusPatch = () => {
        if (!data) return;
        const patch = {
          conversationId,
          status: data.status,
          flowState: data.flow_state,
          updatedAt: data.updated_at,
          mailboxId: data.mailbox_id ?? null,
        };
        queryClient.setQueriesData<SupportConversationListCache>(
          {
            predicate: (query) => isSupportConversationListQueryKey(query.queryKey, workspaceId),
          },
          (current) => patchConversationStatusInCache(current, patch),
        );
        queryClient.setQueryData(
          queryKeys.support.conversation(workspaceId, conversationId),
          (current: SupportConversation | undefined) => patchConversationDetailStatus(current, patch),
        );
      };

      const invalidateAfterStatusChange = () => {
        queryClient.invalidateQueries({ queryKey: queryKeys.support.conversations(workspaceId) });
        queryClient.invalidateQueries({ queryKey: queryKeys.support.conversation(workspaceId, conversationId) });
        invalidateSupportInboxViewCounts(queryClient, workspaceId);
      };

      // Auto-advance: when resolving/spamming, select the next conversation in the list
      if (status === 'resolved' || status === 'spam') {
        const { selectedConversationId, startConversationHandoff } = currentState;
        const shouldAdvanceSelection = selectedConversationId === conversationId;
        const cached = queryClient.getQueriesData<SupportConversationListCache>({
          queryKey: queryKeys.support.conversations(workspaceId),
        });
        const conversations: SupportConversation[] = cached.flatMap(([queryKey, data]) =>
          isSupportConversationListQueryKey(queryKey, workspaceId) ? extractConversationListConversations(data) : []
        );
        const nextConversationId = shouldAdvanceSelection
          ? getNextConversationIdAfterRemoval(conversations, conversationId)
          : null;

        startConversationHandoff(conversationId, nextConversationId);
        window.setTimeout(() => {
          applyStatusPatch();
          const latestState = useSupportInboxStore.getState();
          if (latestState.conversationHandoff?.fromConversationId === conversationId) {
            if (shouldAdvanceSelection && latestState.selectedConversationId === conversationId) {
              latestState.finishConversationHandoff(nextConversationId);
            } else {
              latestState.cancelConversationHandoff();
            }
          }
          invalidateAfterStatusChange();
        }, CONVERSATION_HANDOFF_DELAY_MS);
        return;
      } else if (status === 'open' && currentState.selectedConversationId === conversationId) {
        currentState.showReopenedConversationInInbox(conversationId, data?.mailbox_id ?? null);
      }

      applyStatusPatch();
      invalidateAfterStatusChange();
    },
    onError: (error: Error) => {
      toast.error('Failed to update conversation status', { description: error.message });
    },
  });
}

export function useAssignAgent(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ conversationId, agentId }: { conversationId: string; agentId: string }) =>
      supportService.assignConversationAgent(workspaceId, conversationId, { agent_id: agentId }).then(unwrap),
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({ queryKey: queryKeys.support.conversations(workspaceId) });
      queryClient.invalidateQueries({ queryKey: queryKeys.support.conversation(workspaceId, variables.conversationId) });
      invalidateSupportInboxViewCounts(queryClient, workspaceId);
    },
    onError: (error: Error) => {
      toast.error('Failed to assign agent', { description: error.message });
    },
  });
}

export function useAssignConversationUser(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ conversationId, userId }: { conversationId: string; userId: string | null }) =>
      supportService.assignConversationUser(workspaceId, conversationId, { user_id: userId }).then(unwrap),
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({ queryKey: queryKeys.support.conversations(workspaceId) });
      queryClient.invalidateQueries({ queryKey: queryKeys.support.conversation(workspaceId, variables.conversationId) });
      queryClient.invalidateQueries({ queryKey: queryKeys.support.unreadStats(workspaceId) });
      invalidateSupportInboxViewCounts(queryClient, workspaceId);
    },
    onError: (error: Error) => {
      toast.error('Failed to assign conversation', { description: error.message });
    },
  });
}

export function useCreateConversation(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (payload: CreateConversationRequest) =>
      supportService.createConversation(workspaceId, payload).then(unwrap),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.support.conversations(workspaceId) });
      queryClient.invalidateQueries({ queryKey: queryKeys.support.inboxScopes(workspaceId) });
      queryClient.invalidateQueries({ queryKey: queryKeys.support.unreadStats(workspaceId) });
      invalidateSupportInboxViewCounts(queryClient, workspaceId);
    },
    onError: (error: Error) => {
      toast.error('Failed to create conversation', { description: error.message });
    },
  });
}

export function useCreateConversationWithMessage(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (payload: CreateConversationWithMessageRequest) => {
      const data = unwrap(await supportService.createConversationWithMessage(workspaceId, payload));
      if (!data?.conversation?.id || !data?.message?.id) {
        throw new Error('Conversation send returned an invalid response');
      }
      return data;
    },
    onSuccess: (data) => {
      queryClient.invalidateQueries({ queryKey: queryKeys.support.conversations(workspaceId) });
      queryClient.invalidateQueries({ queryKey: queryKeys.support.inboxScopes(workspaceId) });
      queryClient.invalidateQueries({ queryKey: queryKeys.support.unreadStats(workspaceId) });
      if (data?.conversation?.id) {
        queryClient.setQueryData(queryKeys.support.conversation(workspaceId, data.conversation.id), data.conversation);
        queryClient.setQueryData(
          queryKeys.support.messages(workspaceId, data.conversation.id),
          seedSupportMessagePages([data.message]),
        );
      }
      invalidateSupportInboxViewCounts(queryClient, workspaceId);
    },
    onError: (error: Error) => {
      toast.error('Failed to send conversation', { description: error.message });
    },
  });
}

export function useRunConversationAgent(workspaceId: string) {
  return useMutation({
    mutationFn: (conversationId: string) =>
      supportService.runAgent(workspaceId, conversationId).then(unwrap),
    onError: (error: Error) => {
      if (isUpgradeRequiredError(error)) return;
      toast.error('Failed to run agent', { description: error.message });
    },
  });
}

export function useCreateTaskFromConversation(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ conversationId, teamId }: { conversationId: string; teamId: string }) =>
      supportService.createTaskFromConversation(workspaceId, conversationId, {
        team_id: teamId,
      }).then(unwrap),
    onSuccess: (_data, { conversationId }) => {
      queryClient.invalidateQueries({ queryKey: queryKeys.support.conversations(workspaceId) });
      queryClient.invalidateQueries({ queryKey: queryKeys.support.conversation(workspaceId, conversationId) });
      invalidateSupportInboxViewCounts(queryClient, workspaceId);
      queryClient.invalidateQueries({ queryKey: queryKeys.support.conversationAssociations(workspaceId, conversationId) });
      queryClient.invalidateQueries({ queryKey: queryKeys.pm.tasks(workspaceId) });
      queryClient.invalidateQueries({ queryKey: queryKeys.pm.board(workspaceId) });
    },
    onError: (error: Error) => {
      toast.error('Failed to create task', { description: error.message });
    },
  });
}

export function useSupportAgents(workspaceId: string) {
  return useQuery({
    queryKey: [...queryKeys.agents.all(workspaceId), 'support'] as const,
    queryFn: async () => {
      const res = await agentService.list(workspaceId);
      if (res.error) throw new Error(res.error);
      return (res.data ?? []).filter((agent) =>
        agent.allowed_targets?.includes('support_conversation') ||
        agent.preset_key === 'support_agent',
      );
    },
    enabled: !!workspaceId,
    staleTime: 60_000,
  });
}

export function useMarkConversationUnread(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (conversationId: string) =>
      supportService.markConversationUnread(workspaceId, conversationId).then(unwrap),
    onSuccess: (state, conversationId) => {
      const patch = {
        conversationId,
        unreadCount: state.unread_customer_message_count > 0 ? state.unread_customer_message_count : 1,
        version: state.version,
      };
      queryClient.setQueriesData<SupportConversationListCache>(
        {
          queryKey: queryKeys.support.conversations(workspaceId),
          predicate: (query) => isSupportConversationListQueryKey(query.queryKey, workspaceId),
        },
        (current) => patchConversationPersonalReadInCache(current, patch),
      );
      queryClient.setQueryData<SupportConversation>(
        queryKeys.support.conversation(workspaceId, conversationId),
        (current) => patchConversationDetailPersonalRead(current, patch),
      );
      queryClient.invalidateQueries({ queryKey: queryKeys.support.unreadStats(workspaceId) });
      queryClient.invalidateQueries({ queryKey: queryKeys.support.inboxScopes(workspaceId) });
      invalidateSupportInboxViewCounts(queryClient, workspaceId);
    },
    onError: (error: Error) => {
      toast.error('Failed to mark as unread', { description: error.message });
    },
  });
}

export function useMarkConversationRead(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ conversationId, throughMessageId }: { conversationId: string; throughMessageId?: string }) =>
      supportService.markConversationRead(workspaceId, conversationId, throughMessageId).then(unwrap),
    onSuccess: (state, { conversationId }) => {
      const unreadCount = state.unread_customer_message_count > 0
        ? state.unread_customer_message_count
        : state.manually_unread ? 1 : 0;
      const patch = { conversationId, unreadCount, version: state.version };
      queryClient.setQueriesData<SupportConversationListCache>(
        {
          queryKey: queryKeys.support.conversations(workspaceId),
          predicate: (query) => isSupportConversationListQueryKey(query.queryKey, workspaceId),
        },
        (current) => patchConversationPersonalReadInCache(current, patch),
      );
      queryClient.setQueryData<SupportConversation>(
        queryKeys.support.conversation(workspaceId, conversationId),
        (current) => patchConversationDetailPersonalRead(current, patch),
      );
      queryClient.invalidateQueries({ queryKey: queryKeys.support.unreadStats(workspaceId) });
      queryClient.invalidateQueries({ queryKey: queryKeys.support.inboxScopes(workspaceId) });
      invalidateSupportInboxViewCounts(queryClient, workspaceId);
    },
    onError: (error: Error) => {
      toast.error('Failed to mark conversation as read', { description: error.message });
    },
  });
}

export function useUpdateConversationSubject(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ conversationId, subject }: { conversationId: string; subject: string }) =>
      supportService.updateConversationSubject(workspaceId, conversationId, subject).then(unwrap),
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({ queryKey: queryKeys.support.conversations(workspaceId) });
      queryClient.invalidateQueries({ queryKey: queryKeys.support.conversation(workspaceId, variables.conversationId) });
      invalidateSupportInboxViewCounts(queryClient, workspaceId);
    },
    onError: (error: Error) => {
      toast.error('Failed to update subject', { description: error.message });
    },
  });
}

export function useUpdateConversationCustomerName(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ conversationId, customerName }: { conversationId: string; customerName: string }) =>
      supportService.updateConversationCustomerName(workspaceId, conversationId, { customer_name: customerName }).then(unwrap),
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({ queryKey: queryKeys.support.conversations(workspaceId) });
      queryClient.invalidateQueries({ queryKey: queryKeys.support.conversation(workspaceId, variables.conversationId) });
      invalidateSupportInboxViewCounts(queryClient, workspaceId);
    },
    onError: (error: Error) => {
      toast.error('Failed to update customer name', { description: error.message });
    },
  });
}

export function useUpdateConversationCRMCompany(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ conversationId, companyId }: { conversationId: string; companyId: string | null }) =>
      supportService.updateConversationCRMCompany(workspaceId, conversationId, { crm_company_id: companyId }).then(unwrap),
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({ queryKey: queryKeys.support.conversation(workspaceId, variables.conversationId) });
      queryClient.invalidateQueries({ queryKey: queryKeys.support.visitorContext(workspaceId, variables.conversationId) });
      queryClient.invalidateQueries({ queryKey: queryKeys.support.conversationAssociations(workspaceId, variables.conversationId) });
    },
    onError: (error: Error) => {
      toast.error('Failed to update company', { description: error.message });
    },
  });
}

export function useUpdateConversationEmailRecipients(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ conversationId, payload }: { conversationId: string; payload: UpdateConversationEmailRecipientsRequest }) =>
      supportService.updateConversationEmailRecipients(workspaceId, conversationId, payload).then(unwrap),
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({ queryKey: queryKeys.support.conversations(workspaceId) });
      queryClient.invalidateQueries({ queryKey: queryKeys.support.conversation(workspaceId, variables.conversationId) });
      queryClient.invalidateQueries({ queryKey: queryKeys.support.messages(workspaceId, variables.conversationId) });
      invalidateSupportInboxViewCounts(queryClient, workspaceId);
    },
    onError: (error: Error) => {
      toast.error('Failed to update email recipients', { description: error.message });
    },
  });
}

export function useAddConversationTag(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ conversationId, tagId }: { conversationId: string; tagId: string }) =>
      supportService.addConversationTag(workspaceId, conversationId, tagId).then(unwrap),
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({ queryKey: queryKeys.support.conversations(workspaceId) });
      queryClient.invalidateQueries({ queryKey: queryKeys.support.conversation(workspaceId, variables.conversationId) });
      invalidateSupportInboxViewCounts(queryClient, workspaceId);
    },
    onError: (error: Error) => {
      toast.error('Failed to add tag', { description: error.message });
    },
  });
}

export function useRemoveConversationTag(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ conversationId, tagId }: { conversationId: string; tagId: string }) =>
      supportService.removeConversationTag(workspaceId, conversationId, tagId).then(unwrap),
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({ queryKey: queryKeys.support.conversations(workspaceId) });
      queryClient.invalidateQueries({ queryKey: queryKeys.support.conversation(workspaceId, variables.conversationId) });
      invalidateSupportInboxViewCounts(queryClient, workspaceId);
    },
    onError: (error: Error) => {
      toast.error('Failed to remove tag', { description: error.message });
    },
  });
}

export function useDeleteConversation(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (conversationId: string) =>
      supportService.deleteConversation(workspaceId, conversationId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.support.conversations(workspaceId) });
      queryClient.invalidateQueries({ queryKey: queryKeys.support.unreadStats(workspaceId) });
      queryClient.invalidateQueries({ queryKey: queryKeys.support.inboxScopes(workspaceId) });
      invalidateSupportInboxViewCounts(queryClient, workspaceId);
    },
    onError: (error: Error) => {
      toast.error('Failed to delete conversation', { description: error.message });
    },
  });
}

export function useCreateMailbox(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (payload: CreateSupportMailboxRequest) =>
      supportService.createMailbox(workspaceId, payload).then(unwrap),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.support.mailboxes(workspaceId) });
      queryClient.invalidateQueries({ queryKey: queryKeys.support.inboxScopes(workspaceId) });
      queryClient.invalidateQueries({ queryKey: queryKeys.support.triageRules(workspaceId) });
      invalidateSupportInboxViewCounts(queryClient, workspaceId);
    },
    onError: (error: Error) => {
      toast.error('Failed to create team inbox', { description: error.message });
    },
  });
}

export function useUpdateMailbox(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ mailboxId, payload }: { mailboxId: string; payload: UpdateSupportMailboxRequest }) =>
      supportService.updateMailbox(workspaceId, mailboxId, payload).then(unwrap),
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({ queryKey: queryKeys.support.mailboxes(workspaceId) });
      queryClient.invalidateQueries({ queryKey: queryKeys.support.mailboxMembers(workspaceId, variables.mailboxId) });
      queryClient.invalidateQueries({ queryKey: queryKeys.support.inboxScopes(workspaceId) });
      queryClient.invalidateQueries({ queryKey: queryKeys.support.triageRules(workspaceId) });
      invalidateSupportInboxViewCounts(queryClient, workspaceId);
    },
    onError: (error: Error) => {
      toast.error('Failed to update team inbox', { description: error.message });
    },
  });
}

export function useArchiveMailbox(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (mailboxId: string) =>
      supportService.archiveMailbox(workspaceId, mailboxId).then(unwrap),
    onSuccess: (_data, mailboxId) => {
      queryClient.invalidateQueries({ queryKey: queryKeys.support.mailboxes(workspaceId) });
      queryClient.invalidateQueries({ queryKey: queryKeys.support.mailboxMembers(workspaceId, mailboxId) });
      queryClient.invalidateQueries({ queryKey: queryKeys.support.inboxScopes(workspaceId) });
      queryClient.invalidateQueries({ queryKey: queryKeys.support.triageRules(workspaceId) });
      invalidateSupportInboxViewCounts(queryClient, workspaceId);
    },
    onError: (error: Error) => {
      toast.error('Failed to archive team inbox', { description: error.message });
    },
  });
}

export function useReorderMailboxes(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (mailboxIds: string[]) =>
      supportService.reorderMailboxes(workspaceId, mailboxIds),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.support.mailboxes(workspaceId) });
      queryClient.invalidateQueries({ queryKey: queryKeys.support.inboxScopes(workspaceId) });
    },
    onError: (error: Error) => {
      toast.error('Failed to reorder team inboxes', { description: error.message });
    },
  });
}

export function useMoveConversation(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ conversationId, mailboxId }: { conversationId: string; mailboxId: string | null }) =>
      supportService.moveConversation(workspaceId, conversationId, mailboxId).then(unwrap),
    onSuccess: (_data, variables) => {
      const { selectedConversationId, selectConversation } = useSupportInboxStore.getState();
      if (selectedConversationId === variables.conversationId) {
        const cached = queryClient.getQueriesData<SupportConversationListCache>({
          queryKey: queryKeys.support.conversations(workspaceId),
        });
        const conversations: SupportConversation[] = cached.flatMap(([queryKey, data]) =>
          isSupportConversationListQueryKey(queryKey, workspaceId) ? extractConversationListConversations(data) : []
        );
        selectConversation(getNextConversationIdAfterRemoval(conversations, variables.conversationId));
      }

      queryClient.invalidateQueries({ queryKey: queryKeys.support.conversations(workspaceId) });
      queryClient.invalidateQueries({ queryKey: queryKeys.support.conversation(workspaceId, variables.conversationId) });
      queryClient.invalidateQueries({ queryKey: queryKeys.support.unreadStats(workspaceId) });
      queryClient.invalidateQueries({ queryKey: queryKeys.support.inboxScopes(workspaceId) });
      invalidateSupportInboxViewCounts(queryClient, workspaceId);
    },
    onError: (error: Error) => {
      toast.error('Failed to move conversation', { description: error.message });
    },
  });
}

export function useDismissConversationTriage(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (conversationId: string) =>
      supportService.dismissConversationTriage(workspaceId, conversationId).then(unwrap),
    onSuccess: (_data, conversationId) => {
      queryClient.invalidateQueries({ queryKey: queryKeys.support.conversations(workspaceId) });
      queryClient.invalidateQueries({ queryKey: queryKeys.support.conversation(workspaceId, conversationId) });
      queryClient.invalidateQueries({ queryKey: queryKeys.support.unreadStats(workspaceId) });
      queryClient.invalidateQueries({ queryKey: queryKeys.support.inboxScopes(workspaceId) });
      invalidateSupportInboxViewCounts(queryClient, workspaceId);
    },
    onError: (error: Error) => {
      toast.error('Failed to dismiss routing suggestion', { description: error.message });
    },
  });
}

export function useAgentKnowledgeSources(workspaceId: string, agentId?: string) {
  return useQuery({
    queryKey: queryKeys.agents.knowledgeSources(workspaceId, agentId ?? ''),
    queryFn: async (): Promise<AgentKnowledgeSource[]> => unwrap(await agentService.listKnowledgeSources(workspaceId, agentId!)),
    enabled: !!workspaceId && !!agentId,
    staleTime: 60_000,
    refetchInterval: (query) => {
      const sources = query.state.data as AgentKnowledgeSource[] | undefined;
      return sources?.some((source) => source.sync_status === 'queued' || source.sync_status === 'running')
        ? 2_000
        : false;
    },
  });
}

export function useCuratedGuidance(workspaceId: string, agentId?: string) {
  return useQuery({
    queryKey: queryKeys.agents.curatedGuidance(workspaceId, agentId ?? ''),
    queryFn: async (): Promise<CuratedGuidance[]> => unwrap(await agentService.listCuratedGuidance(workspaceId, agentId!)),
    enabled: !!workspaceId && !!agentId,
    staleTime: 60_000,
  });
}

export function useCreateCuratedGuidance(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ agentId, payload }: { agentId: string; payload: CreateCuratedGuidanceRequest }) =>
      agentService.createCuratedGuidance(workspaceId, agentId, payload).then(unwrap),
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({ queryKey: queryKeys.agents.curatedGuidance(workspaceId, variables.agentId) });
      toast.success('Answer guidance created');
    },
    onError: (error: Error) => toast.error('Failed to create answer guidance', { description: error.message }),
  });
}

export function useUpdateCuratedGuidance(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ agentId, guidanceId, payload }: { agentId: string; guidanceId: string; payload: UpdateCuratedGuidanceRequest }) =>
      agentService.updateCuratedGuidance(workspaceId, agentId, guidanceId, payload).then(unwrap),
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({ queryKey: queryKeys.agents.curatedGuidance(workspaceId, variables.agentId) });
      toast.success('Answer guidance updated');
    },
    onError: (error: Error) => toast.error('Failed to update answer guidance', { description: error.message }),
  });
}

export function useDeleteCuratedGuidance(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ agentId, guidanceId }: { agentId: string; guidanceId: string }) =>
      agentService.deleteCuratedGuidance(workspaceId, agentId, guidanceId).then(unwrap),
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({ queryKey: queryKeys.agents.curatedGuidance(workspaceId, variables.agentId) });
      toast.success('Answer guidance removed');
    },
    onError: (error: Error) => toast.error('Failed to remove answer guidance', { description: error.message }),
  });
}

export function useUpdateAgentKnowledgeSources(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ agentId, sources }: { agentId: string; sources: AgentKnowledgeSourceRequest[] }) =>
      agentService.updateKnowledgeSources(workspaceId, agentId, sources).then(unwrap),
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({ queryKey: queryKeys.agents.knowledgeSources(workspaceId, variables.agentId) });
    },
    onError: (error: Error) => {
      toast.error('Failed to update knowledge sources', { description: error.message });
    },
    onSettled: (_data, _error, variables) => {
      if (!variables?.agentId) {
        return;
      }
      queryClient.invalidateQueries({ queryKey: queryKeys.agents.knowledgeSources(workspaceId, variables.agentId) });
    },
  });
}

export function useReindexAgentKnowledgeSource(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ agentId, spaceId }: { agentId: string; spaceId: string }) =>
      agentService.reindexKnowledgeSource(workspaceId, agentId, spaceId).then(unwrap),
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({ queryKey: queryKeys.agents.knowledgeSources(workspaceId, variables.agentId) });
    },
    onError: (error: Error) => {
      toast.error('Failed to reindex help center docs', { description: error.message });
    },
    onSettled: (_data, _error, variables) => {
      if (!variables?.agentId) {
        return;
      }
      queryClient.invalidateQueries({ queryKey: queryKeys.agents.knowledgeSources(workspaceId, variables.agentId) });
    },
  });
}

export function useSupportContentSources(workspaceId: string) {
  return useQuery({
    queryKey: queryKeys.agents.contentSources(workspaceId),
    queryFn: async (): Promise<SupportContentSource[]> => unwrap(await agentService.listContentSources(workspaceId)),
    enabled: !!workspaceId,
    staleTime: 60_000,
    refetchInterval: (query) => {
      const sources = query.state.data as SupportContentSource[] | undefined;
      return sources?.some((source) => source.sync_status === 'queued' || source.sync_status === 'running')
        ? 2_000
        : false;
    },
  });
}

export function useSupportContentSourcePages(workspaceId: string, contentSourceId?: string) {
  return useQuery({
    queryKey: queryKeys.agents.contentSourcePages(workspaceId, contentSourceId ?? ''),
    queryFn: async (): Promise<SupportContentPage[]> => unwrap(await agentService.listContentSourcePages(workspaceId, contentSourceId!)),
    enabled: !!workspaceId && !!contentSourceId,
    staleTime: 30_000,
  });
}

export function useSupportContentSourcePage(workspaceId: string, contentSourceId: string, pageId?: string) {
  return useQuery({
    queryKey: queryKeys.agents.contentSourcePage(workspaceId, contentSourceId, pageId ?? ''),
    queryFn: async (): Promise<SupportContentPage> => unwrap(await agentService.getContentSourcePage(workspaceId, contentSourceId, pageId!)),
    enabled: !!workspaceId && !!contentSourceId && !!pageId,
    staleTime: 60_000,
  });
}

export function useCreateSupportContentSource(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (payload: CreateSupportContentSourceRequest) =>
      agentService.createContentSource(workspaceId, payload).then(unwrap),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.agents.contentSources(workspaceId) });
    },
    onError: (error: Error) => {
      toast.error('Failed to create content source', { description: error.message });
    },
  });
}

export function useCreateSupportContentSourceFile(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({ file, name }: { file: File; name: string }) => {
      const payload: CreateSupportContentSourceFileUploadRequest = {
        name,
        file_name: file.name,
        file_size: file.size,
        content_type: file.type || 'application/octet-stream',
      };
      const init = await unwrap(await agentService.createContentSourceFileUpload(workspaceId, payload));
      const upload = await uploadToS3(init.upload_url, file);
      if (!upload.ok) {
        throw new Error(upload.error ?? 'Upload failed');
      }
      return unwrap(await agentService.confirmContentSourceFileUpload(workspaceId, init.source.id));
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.agents.contentSources(workspaceId) });
    },
    onError: (error: Error) => {
      toast.error('Failed to upload file source', { description: error.message });
    },
  });
}

export function useUpdateSupportContentSource(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ contentSourceId, payload }: { contentSourceId: string; payload: UpdateSupportContentSourceRequest }) =>
      agentService.updateContentSource(workspaceId, contentSourceId, payload).then(unwrap),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.agents.contentSources(workspaceId) });
    },
    onError: (error: Error) => {
      toast.error('Failed to update content source', { description: error.message });
    },
  });
}

export function useDeleteSupportContentSource(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (contentSourceId: string) =>
      agentService.deleteContentSource(workspaceId, contentSourceId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.agents.contentSources(workspaceId) });
      queryClient.invalidateQueries({ queryKey: queryKeys.agents.all(workspaceId) });
    },
    onError: (error: Error) => {
      toast.error('Failed to delete content source', { description: error.message });
    },
  });
}

export function useReindexSupportContentSource(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (contentSourceId: string) =>
      agentService.reindexContentSource(workspaceId, contentSourceId).then(unwrap),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.agents.contentSources(workspaceId) });
    },
    onError: (error: Error) => {
      toast.error('Failed to reindex content source', { description: error.message });
    },
  });
}

export function useAgentContentSources(workspaceId: string, agentId?: string) {
  return useQuery({
    queryKey: queryKeys.agents.selectedContentSources(workspaceId, agentId ?? ''),
    queryFn: async (): Promise<string[]> => unwrap(await agentService.listAgentContentSources(workspaceId, agentId!)),
    enabled: !!workspaceId && !!agentId,
    staleTime: 60_000,
  });
}

export function useUpdateAgentContentSources(workspaceId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ agentId, contentSourceIds }: { agentId: string; contentSourceIds: string[] }) =>
      agentService.updateAgentContentSources(workspaceId, agentId, contentSourceIds).then(unwrap),
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({ queryKey: queryKeys.agents.selectedContentSources(workspaceId, variables.agentId) });
      queryClient.invalidateQueries({ queryKey: queryKeys.agents.contentSources(workspaceId) });
    },
    onError: (error: Error) => {
      toast.error('Failed to update content sources', { description: error.message });
    },
    onSettled: (_data, _error, variables) => {
      if (!variables?.agentId) {
        return;
      }
      queryClient.invalidateQueries({ queryKey: queryKeys.agents.selectedContentSources(workspaceId, variables.agentId) });
      queryClient.invalidateQueries({ queryKey: queryKeys.agents.contentSources(workspaceId) });
    },
  });
}
