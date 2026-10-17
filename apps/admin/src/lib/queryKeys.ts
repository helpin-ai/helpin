export const queryKeys = {
  inboundEmails: {
    all: () => ['inbound-emails'] as const,
    list: (status: string, query: string, page: number) => ['inbound-emails', 'list', status, query, page] as const,
    detail: (id: string) => ['inbound-emails', 'detail', id] as const,
    conversations: (id: string, query: string) => ['inbound-emails', 'conversations', id, query] as const,
  },
  workspaces: {
    all: () => ['workspaces'] as const,
  },
  agents: {
    all: (workspaceId: string) => ['agents', workspaceId] as const,
  },
  support: {
    installation: (workspaceId: string) => ['support', workspaceId, 'installation'] as const,
  },
  webhookEvents: {
    list: (params: { page: number; event_type?: string; provider?: string }) =>
      ['webhook-events', params] as const,
    detail: (id: string) => ['webhook-events', id] as const,
  },
  emailQueue: {
    list: () => ['email-queue'] as const,
    diagnostics: () => ['email-diagnostics'] as const,
    conversationDiagnostics: (conversationId: string) =>
      ['email-diagnostics', 'conversation', conversationId] as const,
  },
} as const
