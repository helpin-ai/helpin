import { supportQueryKeys } from '@helpin-ai/support-core'

export const queryKeys = {
  user: {
    me: ['user', 'me'] as const,
  },

  organizations: {
    all: ['organizations'] as const,
    members: (orgId: string) => ['organizations', orgId, 'members'] as const,
  },

  billing: {
    org: (orgId: string) => ['billing', 'org', orgId] as const,
    cards: (orgId: string) => ['billing', 'org', orgId, 'cards'] as const,
    invoices: (orgId: string) => ['billing', 'org', orgId, 'invoices'] as const,
    usage: (wsId: string, period: string, mode: string, start = '', end = '') =>
      ['billing', 'workspace', wsId, 'usage', period, mode, start, end] as const,
    workspace: (wsId: string) => ['billing', 'workspace', wsId] as const,
  },

  dock: {
    root: (wsId: string) => ['dock', wsId] as const,
    runs: (wsId: string) => ['dock', wsId, 'runs'] as const,
    chats: (wsId: string) => ['dock', wsId, 'chats'] as const,
    run: (wsId: string, runId: string) => ['dock', wsId, 'runs', runId] as const,
    chat: (wsId: string, chatId: string) => ['dock', wsId, 'chats', chatId] as const,
  },

  workspaces: {
    all: (orgId?: string) => ['workspaces', { orgId }] as const,
    bySlug: (slug: string) => ['workspaces', 'slug', slug] as const,
    members: (wsId: string) => ['workspaces', wsId, 'members'] as const,
    memberPresence: (wsId: string) => ['workspaces', wsId, 'members', 'presence'] as const,
    assignableMembers: (wsId: string) => ['workspaces', wsId, 'assignableMembers'] as const,
    settings: (wsId: string) => ['workspaces', wsId, 'settings'] as const,
    billing: (wsId: string) => ['workspaces', wsId, 'billing'] as const,
    moduleAccess: (wsId: string) => ['workspaces', wsId, 'module-access'] as const,
    teams: (wsId: string) => ['workspaces', wsId, 'teams'] as const,
    session: (wsId: string) => ['workspaces', wsId, 'session'] as const,
    access: (wsId: string) => ['workspaces', wsId, 'access'] as const,
    setup: (wsId: string) => ['workspaces', wsId, 'setup'] as const,
  },

  automation: {
    overview: (wsId: string) => ['automation', wsId, 'overview'] as const,
    activityRoot: (wsId: string) => ['automation', wsId, 'activity'] as const,
    activity: (wsId: string, filters?: Record<string, unknown>) =>
      filters ? (['automation', wsId, 'activity', filters] as const) : (['automation', wsId, 'activity'] as const),
    flows: (wsId: string) => ['automation', wsId, 'flows'] as const,
    flowsByWorkflow: (wsId: string, workflowId: string) => ['automation', wsId, 'flows', 'workflow', workflowId] as const,
    flowTemplates: (wsId: string) => ['automation', wsId, 'templates'] as const,
    flowTemplate: (wsId: string, key: string) => ['automation', wsId, 'templates', key] as const,
    triggerCatalog: (wsId: string) => ['automation', wsId, 'library', 'triggers'] as const,
    toolCatalog: (wsId: string) => ['automation', wsId, 'library', 'tools'] as const,
    skillCatalog: (wsId: string) => ['automation', wsId, 'library', 'skills'] as const,
    agentTemplates: (wsId: string) => ['automation', wsId, 'agent-templates'] as const,
    agentTemplate: (wsId: string, id: string) => ['automation', wsId, 'agent-templates', id] as const,
    agentsRoot: (wsId: string) => ['automation', wsId, 'agents'] as const,
    agents: (wsId: string) => ['automation', wsId, 'agents'] as const,
    agentFleet: (wsId: string) => ['automation', wsId, 'agents', 'fleet'] as const,
    agent: (wsId: string, id: string) => ['automation', wsId, 'agents', id] as const,
    agentUsage: (wsId: string, id: string) => ['automation', wsId, 'agents', id, 'usage'] as const,
    runsRoot: (wsId: string) => ['automation', wsId, 'runs'] as const,
    runs: (wsId: string, page?: number, perPage?: number) => ['automation', wsId, 'runs', page, perPage] as const,
    runAttentionCount: (wsId: string) => ['automation', wsId, 'run-attention-count'] as const,
    targetRuns: (wsId: string, targetType: string, targetId: string) => ['automation', wsId, 'runs', 'target', targetType, targetId] as const,
  },

  mcp: {
    root: (wsId: string) => ['mcp', wsId] as const,
    dashboard: (wsId: string) => ['mcp', wsId, 'dashboard'] as const,
    activity: (wsId: string) => ['mcp', wsId, 'activity'] as const,
    serviceTokens: (wsId: string, principalId: string) => ['mcp', wsId, 'service-principals', principalId, 'tokens'] as const,
    authorization: (query: object) => ['mcp', 'authorization', query] as const,
    externalRoot: (wsId: string) => ['mcp', wsId, 'external'] as const,
    externalProviders: (wsId: string) => ['mcp', wsId, 'external', 'providers'] as const,
    externalServers: (wsId: string) => ['mcp', wsId, 'external', 'servers'] as const,
  },

  pm: {
    workflows: (wsId: string) => ['pm', wsId, 'workflows'] as const,
    epicStates: (wsId: string) => ['pm', wsId, 'epicStates'] as const,

    tasks: (wsId: string, filters?: Record<string, unknown>) =>
      filters ? (['pm', wsId, 'tasks', filters] as const) : (['pm', wsId, 'tasks'] as const),
    task: (wsId: string, id: string) => ['pm', wsId, 'tasks', id] as const,
    taskByDisplayId: (wsId: string, displayId: string) => ['pm', wsId, 'tasks', 'displayId', displayId] as const,
    taskActivity: (wsId: string, taskId: string) => ['pm', wsId, 'tasks', taskId, 'activity'] as const,

    board: (wsId: string, workflowId?: string, filters?: Record<string, unknown>) =>
      ['pm', wsId, 'board', workflowId, filters] as const,
    boardColumn: (wsId: string, workflowId: string, stateId: string) =>
      ['pm', wsId, 'board', workflowId, 'column', stateId] as const,

    objectives: (wsId: string) => ['pm', wsId, 'objectives'] as const,
    objective: (wsId: string, id: string) => ['pm', wsId, 'objectives', id] as const,

    epics: (wsId: string) => ['pm', wsId, 'epics'] as const,
    epic: (wsId: string, id: string) => ['pm', wsId, 'epics', id] as const,
    epicTasks: (wsId: string, epicId: string) => ['pm', wsId, 'epics', epicId, 'tasks'] as const,
    epicAssociations: (wsId: string, epicId: string) => ['pm', wsId, 'epics', epicId, 'associations'] as const,

    sprints: (wsId: string) => ['pm', wsId, 'sprints'] as const,
    sprintPlanning: (wsId: string, filters?: Record<string, unknown>) =>
      filters ? (['pm', wsId, 'sprints', 'planning', filters] as const) : (['pm', wsId, 'sprints', 'planning'] as const),
    sprint: (wsId: string, id: string) => ['pm', wsId, 'sprints', id] as const,
    sprintCloseout: (wsId: string, id: string) => ['pm', wsId, 'sprints', id, 'closeout'] as const,
    sprintCloseouts: (wsId: string, filters?: Record<string, unknown>) =>
      filters ? (['pm', wsId, 'sprints', 'closeouts', filters] as const) : (['pm', wsId, 'sprints', 'closeouts'] as const),
    sprintTasks: (wsId: string, sprintId: string) => ['pm', wsId, 'sprints', sprintId, 'tasks'] as const,
    sprintPreviewTasksRoot: (wsId: string) => ['pm', wsId, 'sprints', 'previewTasks'] as const,
    sprintPreviewTasks: (wsId: string, sprintId: string) => ['pm', wsId, 'sprints', 'previewTasks', sprintId] as const,
    sprintBacklogTasksRoot: (wsId: string) => ['pm', wsId, 'sprints', 'backlogTasks'] as const,
    sprintBacklogTasks: (wsId: string, teamId?: string) => ['pm', wsId, 'sprints', 'backlogTasks', teamId] as const,

    taskAssociations: (wsId: string, taskId: string) => ['pm', wsId, 'tasks', taskId, 'associations'] as const,
    taskRelationships: (wsId: string, taskId: string) => ['pm', wsId, 'tasks', taskId, 'relationships'] as const,

    labels: (wsId: string) => ['pm', wsId, 'labels'] as const,
    labelsWithStats: (wsId: string) => ['pm', wsId, 'labels', 'withStats'] as const,

    views: (wsId: string) => ['pm', wsId, 'views'] as const,

    templates: (wsId: string) => ['pm', wsId, 'templates'] as const,
    template: (wsId: string, id: string) => ['pm', wsId, 'templates', id] as const,
    recurringTemplates: (wsId: string, filters?: Record<string, unknown>) =>
      filters ? (['pm', wsId, 'recurringTemplates', filters] as const) : (['pm', wsId, 'recurringTemplates'] as const),
    recurringTemplate: (wsId: string, id: string) => ['pm', wsId, 'recurringTemplates', id] as const,
    taskRecurringTemplate: (wsId: string, taskId: string) => ['pm', wsId, 'tasks', taskId, 'recurringTemplate'] as const,

    automations: (wsId: string) => ['pm', wsId, 'automations'] as const,

    automationRules: (wsId: string) => ['pm', wsId, 'automationRules'] as const,
    automationRulesByWorkflow: (wsId: string, wfId: string) => ['pm', wsId, 'automationRules', 'workflow', wfId] as const,

    planningSession: (wsId: string, sessionId: string) => ['pm', wsId, 'planningSession', sessionId] as const,
    planningMessages: (wsId: string, sessionId: string) => ['pm', wsId, 'planningSession', sessionId, 'messages'] as const,
    flowRun: (wsId: string, flowRunId: string) => ['pm', wsId, 'flowRun', flowRunId] as const,
    flowNodeMessages: (wsId: string, flowRunId: string, nodeRunId: string) =>
      ['pm', wsId, 'flowRun', flowRunId, 'nodes', nodeRunId, 'messages'] as const,
    flowRuns: (wsId: string) => ['pm', wsId, 'flowRuns'] as const,
    flowTemplates: (wsId: string) => ['pm', wsId, 'flowTemplates'] as const,
    flowDBTemplates: (wsId: string) => ['pm', wsId, 'flowDBTemplates'] as const,
    flowDBTemplate: (wsId: string, templateId: string) => ['pm', wsId, 'flowDBTemplate', templateId] as const,

    comments: (wsId: string, taskId: string) => ['pm', wsId, 'tasks', taskId, 'comments'] as const,
    checklists: (wsId: string, taskId: string) => ['pm', wsId, 'tasks', taskId, 'checklists'] as const,
    attachments: (wsId: string, taskId: string) => ['pm', wsId, 'tasks', taskId, 'attachments'] as const,
    externalLinks: (wsId: string, taskId: string) => ['pm', wsId, 'tasks', taskId, 'externalLinks'] as const,
    entityExternalLinks: (wsId: string, entityType: string, entityId: string) =>
      ['pm', wsId, entityType, entityId, 'externalLinks'] as const,

    search: (wsId: string, query: string) => ['pm', wsId, 'search', query] as const,
  },

  agents: {
    all: (wsId: string) => ['agents', wsId] as const,
    detail: (wsId: string, id: string) => ['agents', wsId, id] as const,
    runs: (wsId: string, agentId: string) => ['agents', wsId, agentId, 'runs'] as const,
    knowledgeSources: (wsId: string, agentId: string) => ['agents', wsId, agentId, 'knowledge-sources'] as const,
    curatedGuidance: (wsId: string, agentId: string) => ['agents', wsId, agentId, 'curated-guidance'] as const,
    contentSources: (wsId: string) => ['agents', wsId, 'content-sources'] as const,
    contentSourcePages: (wsId: string, contentSourceId: string) => ['agents', wsId, 'content-sources', contentSourceId, 'pages'] as const,
    contentSourcePage: (wsId: string, contentSourceId: string, pageId: string) => ['agents', wsId, 'content-sources', contentSourceId, 'pages', pageId] as const,
    selectedContentSources: (wsId: string, agentId: string) => ['agents', wsId, agentId, 'content-source-selections'] as const,
  },

  git: {
    integrations: (wsId: string) => ['git', wsId, 'integrations'] as const,
    repositories: (wsId: string) => ['git', wsId, 'repositories'] as const,
    taskLinks: (wsId: string, taskId: string) => ['git', wsId, 'tasks', taskId, 'links'] as const,
    taskDeliveryTarget: (wsId: string, taskId: string) => ['git', wsId, 'tasks', taskId, 'delivery-target'] as const,
    epicDeliveryTarget: (wsId: string, epicId: string) => ['git', wsId, 'epics', epicId, 'delivery-target'] as const,
  },

  support: {
    ...supportQueryKeys,
    conversationAssignees: (wsId: string, id: string) => ['support', wsId, 'conversations', id, 'assignees'] as const,
    conversationAssociations: (wsId: string, id: string) => ['support', wsId, 'conversations', id, 'associations'] as const,
    messageEmail: (wsId: string, messageId: string) => ['support', wsId, 'messages', messageId, 'email'] as const,
    messageInfo: (wsId: string, conversationId: string, messageId: string) => ['support', wsId, 'conversations', conversationId, 'messages', messageId, 'info'] as const,
    installation: (wsId: string) => ['support', wsId, 'installation'] as const,
    routingUsage: (wsId: string) => ['support', wsId, 'routing-usage'] as const,
    inboxViews: (wsId: string) => ['support', wsId, 'inbox-views'] as const,
    inboxViewCounts: (wsId: string) => ['support', wsId, 'inbox-view-counts'] as const,
    builtinInboxViews: (wsId: string) => ['support', wsId, 'builtin-inbox-views'] as const,
    workspaceUnread: () => ['support', 'workspace-unread'] as const,
    emailRoutes: (wsId: string) => ['support', wsId, 'email-routes'] as const,
    emailSenders: (wsId: string) => ['support', wsId, 'email-senders'] as const,
    emailSenderDomains: (wsId: string) => ['support', wsId, 'email-sender-domains'] as const,
    triageRules: (wsId: string) => ['support', wsId, 'triage-rules'] as const,
    cannedResponses: (wsId: string) => ['support', wsId, 'canned-responses'] as const,
    cannedResponseSearch: (wsId: string, query: string) => ['support', wsId, 'canned-responses', 'search', query] as const,
    search: (wsId: string, filters: unknown) => ['support', wsId, 'search', filters] as const,
    tags: (wsId: string) => ['support', wsId, 'tags'] as const,
  },

  docs: {
    spaces: (wsId: string) => ['docs', wsId, 'spaces'] as const,
    space: (wsId: string, id: string) => ['docs', wsId, 'spaces', id] as const,
    collections: (wsId: string, spaceId: string) => ['docs', wsId, 'spaces', spaceId, 'collections'] as const,
    allCollections: (wsId: string) => ['docs', wsId, 'allCollections'] as const,
    collectionDeleteImpact: (wsId: string, collectionId: string) =>
      ['docs', wsId, 'collections', collectionId, 'deleteImpact'] as const,
    spaceDeleteImpact: (wsId: string, spaceId: string) =>
      ['docs', wsId, 'spaces', spaceId, 'deleteImpact'] as const,
    documents: (wsId: string, filters?: Record<string, unknown>) =>
      filters ? (['docs', wsId, 'documents', filters] as const) : (['docs', wsId, 'documents'] as const),
    document: (wsId: string, id: string) => ['docs', wsId, 'documents', id] as const,
    content: (wsId: string, docId: string) => ['docs', wsId, 'documents', docId, 'content'] as const,
    blocks: (wsId: string, docId: string) => ['docs', wsId, 'documents', docId, 'blocks'] as const,
    changeProposals: (wsId: string, docId: string) => ['docs', wsId, 'documents', docId, 'changeProposals'] as const,
    changeProposal: (wsId: string, docId: string, proposalId: string) => ['docs', wsId, 'documents', docId, 'changeProposals', proposalId] as const,
    versions: (wsId: string, docId: string) => ['docs', wsId, 'documents', docId, 'versions'] as const,
    version: (wsId: string, docId: string, versionId: string) => ['docs', wsId, 'documents', docId, 'versions', versionId] as const,
    links: (wsId: string, docId: string) => ['docs', wsId, 'documents', docId, 'links'] as const,
    linkedDocs: (wsId: string, objectType: string, objectId: string) =>
      ['docs', wsId, 'linkedDocs', objectType, objectId] as const,
    search: (wsId: string, query: string) => ['docs', wsId, 'search', query] as const,
    helpcenterConfig: (wsId: string) => ['docs', wsId, 'helpcenter', 'config'] as const,
    helpcenterLocales: (wsId: string) => ['docs', wsId, 'helpcenter', 'locales'] as const,
    helpcenterSpaceTranslations: (wsId: string, spaceId: string) =>
      ['docs', wsId, 'helpcenter', 'spaces', spaceId, 'translations'] as const,
    helpcenterCollectionTranslations: (wsId: string, collectionId: string) =>
      ['docs', wsId, 'helpcenter', 'collections', collectionId, 'translations'] as const,
    helpcenterArticleTranslations: (wsId: string, docId: string) =>
      ['docs', wsId, 'helpcenter', 'documents', docId, 'translations'] as const,
  },

  crm: {
    contacts: (wsId: string, filters?: Record<string, unknown>) =>
      filters ? (['crm', wsId, 'contacts', filters] as const) : (['crm', wsId, 'contacts'] as const),
    contact: (wsId: string, id: string) => ['crm', wsId, 'contacts', id] as const,
    contactActivities: (wsId: string, contactId: string) => ['crm', wsId, 'contacts', contactId, 'activities'] as const,
    contactAssociations: (wsId: string, contactId: string) => ['crm', wsId, 'contacts', contactId, 'associations'] as const,
    contactSupportConversations: (wsId: string, contactId: string) => ['crm', wsId, 'contacts', contactId, 'support-conversations'] as const,

    companies: (wsId: string, filters?: Record<string, unknown>) =>
      filters ? (['crm', wsId, 'companies', filters] as const) : (['crm', wsId, 'companies'] as const),
    company: (wsId: string, id: string) => ['crm', wsId, 'companies', id] as const,
    companyActivities: (wsId: string, companyId: string) => ['crm', wsId, 'companies', companyId, 'activities'] as const,
    companyAssociations: (wsId: string, companyId: string) => ['crm', wsId, 'companies', companyId, 'associations'] as const,

    deals: (wsId: string, filters?: Record<string, unknown>) =>
      filters ? (['crm', wsId, 'deals', filters] as const) : (['crm', wsId, 'deals'] as const),
    deal: (wsId: string, id: string) => ['crm', wsId, 'deals', id] as const,
    dealActivities: (wsId: string, dealId: string) => ['crm', wsId, 'deals', dealId, 'activities'] as const,
    dealAssociations: (wsId: string, dealId: string) => ['crm', wsId, 'deals', dealId, 'associations'] as const,

    pipelines: (wsId: string) => ['crm', wsId, 'pipelines'] as const,
    pipeline: (wsId: string, id: string) => ['crm', wsId, 'pipelines', id] as const,

    activities: (wsId: string, filters?: Record<string, unknown>) =>
      filters ? (['crm', wsId, 'activities', filters] as const) : (['crm', wsId, 'activities'] as const),

    imports: (wsId: string) => ['crm', wsId, 'imports'] as const,
    import: (wsId: string, id: string) => ['crm', wsId, 'imports', id] as const,

    // Phase 3
    emailAccounts: (wsId: string) => ['crm', wsId, 'emailAccounts'] as const,
    emailThreads: (wsId: string) => ['crm', wsId, 'emailThreads'] as const,
    emailMessages: (wsId: string) => ['crm', wsId, 'emailMessages'] as const,
    contactEmails: (wsId: string, contactId: string) => ['crm', wsId, 'contacts', contactId, 'emails'] as const,
    dealEmails: (wsId: string, dealId: string) => ['crm', wsId, 'deals', dealId, 'emails'] as const,
    calendarEvents: (wsId: string) => ['crm', wsId, 'calendarEvents'] as const,
    contactCalendar: (wsId: string, contactId: string) => ['crm', wsId, 'contacts', contactId, 'calendar'] as const,
    dealCalendar: (wsId: string, dealId: string) => ['crm', wsId, 'deals', dealId, 'calendar'] as const,

    // Phase 4
    enrichments: (wsId: string) => ['crm', wsId, 'enrichments'] as const,
    signals: (wsId: string) => ['crm', wsId, 'signals'] as const,
    contactSignals: (wsId: string, contactId: string) => ['crm', wsId, 'contacts', contactId, 'signals'] as const,
    contactSummary: (wsId: string, contactId: string) => ['crm', wsId, 'contacts', contactId, 'summary'] as const,
    dealSignals: (wsId: string, dealId: string) => ['crm', wsId, 'deals', dealId, 'signals'] as const,
    dealSummary: (wsId: string, dealId: string) => ['crm', wsId, 'deals', dealId, 'summary'] as const,
    healthScores: (wsId: string) => ['crm', wsId, 'healthScores'] as const,
    dealHealthScore: (wsId: string, dealId: string) => ['crm', wsId, 'deals', dealId, 'healthScore'] as const,
    suggestions: (wsId: string, status?: string) =>
      status ? (['crm', wsId, 'suggestions', { status }] as const) : (['crm', wsId, 'suggestions'] as const),
    suggestion: (wsId: string, id: string) => ['crm', wsId, 'suggestions', id] as const,
    autonomySettings: (wsId: string) => ['crm', wsId, 'autonomy-settings'] as const,
    emailSyncSettings: (wsId: string) => ['crm', wsId, 'email-sync-settings'] as const,

    // Phase 5
    writingProfiles: (wsId: string) => ['crm', wsId, 'writingProfiles'] as const,
    writingProfile: (wsId: string, memberId: string) => ['crm', wsId, 'writingProfiles', memberId] as const,

    // Phase 6
    search: (wsId: string, q: string) => ['crm', wsId, 'search', q] as const,
  },

  userNotificationSettings: {
    all: () => ['user-notification-settings'] as const,
  },

  notifications: {
    all: (wsId: string) => ['notifications', wsId] as const,
    list: (wsId: string, filter?: string) =>
      filter ? (['notifications', wsId, 'list', filter] as const) : (['notifications', wsId, 'list'] as const),
    unreadCount: (wsId: string) => ['notifications', wsId, 'unread-count'] as const,
    preferences: (wsId: string) => ['notifications', wsId, 'preferences'] as const,
    following: (wsId: string) => ['notifications', wsId, 'following'] as const,
    isFollowing: (wsId: string, entityType: string, entityId: string) =>
      ['notifications', wsId, 'following', entityType, entityId] as const,
  },
} as const
