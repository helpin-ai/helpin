export const queryKeys = {
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
  },
} as const
