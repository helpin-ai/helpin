export const supportQueryKeys = {
  conversations: (wsId: string) => ['support', wsId, 'conversations'] as const,
  conversation: (wsId: string, id: string) => ['support', wsId, 'conversations', id] as const,
  messages: (wsId: string, conversationId: string) =>
    ['support', wsId, 'conversations', conversationId, 'messages'] as const,
  unreadStats: (wsId: string) => ['support', wsId, 'unread-stats'] as const,
  inboxScopes: (wsId: string) => ['support', wsId, 'inbox-scopes'] as const,
  mailboxes: (wsId: string) => ['support', wsId, 'mailboxes'] as const,
  mailboxMembers: (wsId: string, mailboxId: string) =>
    ['support', wsId, 'mailboxes', mailboxId, 'members'] as const,
  teammatePresence: (wsId: string) => ['support', wsId, 'teammates', 'presence'] as const,
  visitorContext: (wsId: string, conversationId: string) =>
    ['support', wsId, 'conversations', conversationId, 'visitor-context'] as const,
  assignees: (wsId: string, conversationId: string) =>
    ['support', wsId, 'conversations', conversationId, 'assignees'] as const,
} as const
