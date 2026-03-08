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
} as const
