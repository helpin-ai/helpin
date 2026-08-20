export const supportQueryKeys = {
  conversations: (wsId: string) => ['support', wsId, 'conversations'] as const,
  conversationPages: (wsId: string, filters?: object) =>
    ['support', wsId, 'conversations', { ...filters, pagination: 'infinite' }] as const,
  search: (wsId: string, filters?: object) =>
    ['support', wsId, 'search', { ...filters, pagination: 'infinite' }] as const,
  conversation: (wsId: string, id: string) => ['support', wsId, 'conversations', id] as const,
  aiRunInteractions: (wsId: string, id: string) =>
    ['support', wsId, 'conversations', id, 'ai-run-interactions'] as const,
  agentRuns: (wsId: string, conversationId: string) =>
    ['support', wsId, 'conversations', conversationId, 'agent-runs'] as const,
  agentRunMessages: (wsId: string, runId: string) =>
    ['support', wsId, 'agent-runs', runId, 'messages'] as const,
  dockChat: (wsId: string, chatId: string) => ['support', wsId, 'dock-chats', chatId] as const,
  dockChatMessages: (wsId: string, chatId: string) =>
    ['support', wsId, 'dock-chats', chatId, 'messages'] as const,
  dockRunInteractions: (wsId: string, chatId: string) =>
    ['support', wsId, 'dock-chats', chatId, 'run-interactions'] as const,
  messages: (wsId: string, conversationId: string) =>
    ['support', wsId, 'conversations', conversationId, 'messages'] as const,
  messageInfo: (wsId: string, conversationId: string, messageId: string) =>
    ['support', wsId, 'conversations', conversationId, 'messages', messageId, 'info'] as const,
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
  inboxViewCounts: (wsId: string) => ['support', wsId, 'inbox-view-counts'] as const,
  builtinInboxViews: (wsId: string) => ['support', wsId, 'builtin-inbox-views'] as const,
  inboxViews: (wsId: string) => ['support', wsId, 'inbox-views'] as const,
  cannedResponses: (wsId: string) => ['support', wsId, 'canned-responses'] as const,
  tags: (wsId: string) => ['support', wsId, 'tags'] as const,
  installation: (wsId: string) => ['support', wsId, 'installation'] as const,
} as const
