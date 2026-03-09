export const queryKeys = {
  user: {
    me: ['user', 'me'] as const,
  },

  organizations: {
    all: ['organizations'] as const,
  },

  workspaces: {
    all: (orgId?: string) => ['workspaces', { orgId }] as const,
    bySlug: (slug: string) => ['workspaces', 'slug', slug] as const,
    members: (wsId: string) => ['workspaces', wsId, 'members'] as const,
    assignableMembers: (wsId: string) => ['workspaces', wsId, 'assignableMembers'] as const,
    settings: (wsId: string) => ['workspaces', wsId, 'settings'] as const,
    teams: (wsId: string) => ['workspaces', wsId, 'teams'] as const,
    quarters: (wsId: string) => ['workspaces', wsId, 'quarters'] as const,
    session: (wsId: string) => ['workspaces', wsId, 'session'] as const,
    access: (wsId: string) => ['workspaces', wsId, 'access'] as const,
  },

  pm: {
    workflows: (wsId: string) => ['pm', wsId, 'workflows'] as const,
    epicStates: (wsId: string) => ['pm', wsId, 'epicStates'] as const,

    stories: (wsId: string, filters?: Record<string, unknown>) =>
      filters ? (['pm', wsId, 'stories', filters] as const) : (['pm', wsId, 'stories'] as const),
    story: (wsId: string, id: string) => ['pm', wsId, 'stories', id] as const,
    storyByDisplayId: (wsId: string, displayId: string) => ['pm', wsId, 'stories', 'displayId', displayId] as const,
    storyActivity: (wsId: string, storyId: string) => ['pm', wsId, 'stories', storyId, 'activity'] as const,

    board: (wsId: string, workflowId?: string, filters?: Record<string, unknown>) =>
      ['pm', wsId, 'board', workflowId, filters] as const,
    boardColumn: (wsId: string, workflowId: string, stateId: string) =>
      ['pm', wsId, 'board', workflowId, 'column', stateId] as const,

    objectives: (wsId: string) => ['pm', wsId, 'objectives'] as const,
    objective: (wsId: string, id: string) => ['pm', wsId, 'objectives', id] as const,

    epics: (wsId: string) => ['pm', wsId, 'epics'] as const,
    epic: (wsId: string, id: string) => ['pm', wsId, 'epics', id] as const,
    epicStories: (wsId: string, epicId: string) => ['pm', wsId, 'epics', epicId, 'stories'] as const,

    sprints: (wsId: string) => ['pm', wsId, 'sprints'] as const,
    sprint: (wsId: string, id: string) => ['pm', wsId, 'sprints', id] as const,
    sprintStories: (wsId: string, sprintId: string) => ['pm', wsId, 'sprints', sprintId, 'stories'] as const,

    labels: (wsId: string) => ['pm', wsId, 'labels'] as const,
    labelsWithStats: (wsId: string) => ['pm', wsId, 'labels', 'withStats'] as const,

    views: (wsId: string) => ['pm', wsId, 'views'] as const,

    templates: (wsId: string) => ['pm', wsId, 'templates'] as const,
    template: (wsId: string, id: string) => ['pm', wsId, 'templates', id] as const,

    automations: (wsId: string) => ['pm', wsId, 'automations'] as const,

    comments: (wsId: string, storyId: string) => ['pm', wsId, 'stories', storyId, 'comments'] as const,
    checklists: (wsId: string, storyId: string) => ['pm', wsId, 'stories', storyId, 'checklists'] as const,
    attachments: (wsId: string, storyId: string) => ['pm', wsId, 'stories', storyId, 'attachments'] as const,
    externalLinks: (wsId: string, storyId: string) => ['pm', wsId, 'stories', storyId, 'externalLinks'] as const,

    search: (wsId: string, query: string) => ['pm', wsId, 'search', query] as const,
  },

  rewards: {
    quarters: (wsId: string) => ['rewards', wsId, 'quarters'] as const,
    sprints: (wsId: string) => ['rewards', wsId, 'sprints'] as const,
    goals: (wsId: string) => ['rewards', wsId, 'goals'] as const,
    bonus: (wsId: string) => ['rewards', wsId, 'bonus'] as const,
    drafts: (wsId: string) => ['rewards', wsId, 'drafts'] as const,
  },

  agents: {
    all: (wsId: string) => ['agents', wsId] as const,
    detail: (wsId: string, id: string) => ['agents', wsId, id] as const,
    runs: (wsId: string, agentId: string) => ['agents', wsId, agentId, 'runs'] as const,
  },

  git: {
    integrations: (wsId: string) => ['git', wsId, 'integrations'] as const,
    repositories: (wsId: string) => ['git', wsId, 'repositories'] as const,
    storyLinks: (wsId: string, storyId: string) => ['git', wsId, 'stories', storyId, 'links'] as const,
  },

  support: {
    tickets: (wsId: string) => ['support', wsId, 'tickets'] as const,
    ticket: (wsId: string, id: string) => ['support', wsId, 'tickets', id] as const,
    messages: (wsId: string, ticketId: string) => ['support', wsId, 'tickets', ticketId, 'messages'] as const,
  },

  docs: {
    spaces: (wsId: string) => ['docs', wsId, 'spaces'] as const,
    space: (wsId: string, id: string) => ['docs', wsId, 'spaces', id] as const,
    collections: (wsId: string, spaceId: string) => ['docs', wsId, 'spaces', spaceId, 'collections'] as const,
    documents: (wsId: string, filters?: Record<string, unknown>) =>
      filters ? (['docs', wsId, 'documents', filters] as const) : (['docs', wsId, 'documents'] as const),
    document: (wsId: string, id: string) => ['docs', wsId, 'documents', id] as const,
    content: (wsId: string, docId: string) => ['docs', wsId, 'documents', docId, 'content'] as const,
    versions: (wsId: string, docId: string) => ['docs', wsId, 'documents', docId, 'versions'] as const,
    links: (wsId: string, docId: string) => ['docs', wsId, 'documents', docId, 'links'] as const,
    linkedDocs: (wsId: string, objectType: string, objectId: string) =>
      ['docs', wsId, 'linkedDocs', objectType, objectId] as const,
    search: (wsId: string, query: string) => ['docs', wsId, 'search', query] as const,
    helpcenterConfig: (wsId: string) => ['docs', wsId, 'helpcenter', 'config'] as const,
  },

  crm: {
    contacts: (wsId: string, filters?: Record<string, unknown>) =>
      filters ? (['crm', wsId, 'contacts', filters] as const) : (['crm', wsId, 'contacts'] as const),
    contact: (wsId: string, id: string) => ['crm', wsId, 'contacts', id] as const,
    contactActivities: (wsId: string, contactId: string) => ['crm', wsId, 'contacts', contactId, 'activities'] as const,
    contactAssociations: (wsId: string, contactId: string) => ['crm', wsId, 'contacts', contactId, 'associations'] as const,

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

    properties: (wsId: string, objectType?: string) =>
      objectType ? (['crm', wsId, 'properties', objectType] as const) : (['crm', wsId, 'properties'] as const),
    propertyGroups: (wsId: string, objectType?: string) =>
      objectType ? (['crm', wsId, 'propertyGroups', objectType] as const) : (['crm', wsId, 'propertyGroups'] as const),

    lists: (wsId: string, filters?: Record<string, unknown>) =>
      filters ? (['crm', wsId, 'lists', filters] as const) : (['crm', wsId, 'lists'] as const),
    list: (wsId: string, id: string) => ['crm', wsId, 'lists', id] as const,
    listMembers: (wsId: string, listId: string) => ['crm', wsId, 'lists', listId, 'members'] as const,

    imports: (wsId: string) => ['crm', wsId, 'imports'] as const,
    import: (wsId: string, id: string) => ['crm', wsId, 'imports', id] as const,
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
