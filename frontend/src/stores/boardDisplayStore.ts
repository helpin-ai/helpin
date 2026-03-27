import { create } from 'zustand';

export const DISPLAY_PROPERTY_LABELS = {
  story_type: 'Story Type',
  priority: 'Priority',
  severity: 'Severity',
  agent: 'Agent',
  contacts: 'Contacts',
  companies: 'Companies',
  deals: 'Deals',
  support: 'Support',
  epic: 'Epic',
  labels: 'Labels',
  estimate: 'Estimate',
  due_date: 'Due Date',
  blocked: 'Blocked',
  assignee: 'Assignee',
  state: 'State',
  team: 'Team',
  sprint: 'Sprint',
  updated_at: 'Last Updated',
} as const;

/** Keys shown in the Kanban board display menu */
export const BOARD_PROPERTY_KEYS: DisplayPropertyKey[] = [
  'story_type', 'priority', 'severity', 'agent', 'epic', 'sprint', 'labels',
  'estimate', 'due_date', 'blocked', 'assignee',
];

/** Keys shown in the list view display menu */
export const LIST_PROPERTY_KEYS: DisplayPropertyKey[] = [
  'state', 'story_type', 'priority', 'severity', 'estimate',
  'assignee', 'team', 'epic', 'sprint', 'contacts', 'companies', 'deals', 'support', 'due_date', 'labels', 'updated_at',
];

export type DisplayPropertyKey = keyof typeof DISPLAY_PROPERTY_LABELS;

export type DisplayProperties = Record<DisplayPropertyKey, boolean>;

const ALL_ON: DisplayProperties = {
  story_type: true,
  priority: true,
  severity: true,
  agent: true,
  contacts: false,
  companies: false,
  deals: false,
  support: false,
  epic: true,
  labels: true,
  estimate: true,
  due_date: true,
  blocked: true,
  assignee: true,
  state: true,
  team: true,
  sprint: true,
  updated_at: true,
};

const STORAGE_KEY = (wsId: string) => `pm_board_display_${wsId}`;

export type BoardGroupBy = 'status' | 'members';

function loadFromStorage(workspaceId: string): { properties: DisplayProperties; showEmptyColumns: boolean; groupBy: BoardGroupBy } {
  try {
    const raw = localStorage.getItem(STORAGE_KEY(workspaceId));
    if (raw) {
      const parsed = JSON.parse(raw);
      return {
        properties: { ...ALL_ON, ...parsed.properties },
        showEmptyColumns: parsed.showEmptyColumns ?? true,
        groupBy: parsed.groupBy === 'members' ? 'members' : 'status',
      };
    }
  } catch {}
  return { properties: { ...ALL_ON }, showEmptyColumns: true, groupBy: 'status' };
}

function saveToStorage(workspaceId: string, properties: DisplayProperties, showEmptyColumns: boolean, groupBy: BoardGroupBy) {
  try {
    localStorage.setItem(STORAGE_KEY(workspaceId), JSON.stringify({ properties, showEmptyColumns, groupBy }));
  } catch {}
}

interface BoardDisplayState {
  workspaceId: string | null;
  properties: DisplayProperties;
  showEmptyColumns: boolean;
  groupBy: BoardGroupBy;
  init: (workspaceId: string) => void;
  toggleProperty: (key: DisplayPropertyKey) => void;
  toggleShowEmptyColumns: () => void;
  setGroupBy: (value: BoardGroupBy) => void;
}

export const useBoardDisplayStore = create<BoardDisplayState>((set, get) => ({
  workspaceId: null,
  properties: { ...ALL_ON },
  showEmptyColumns: true,
  groupBy: 'status' as BoardGroupBy,

  init: (workspaceId) => {
    if (get().workspaceId === workspaceId) return;
    const loaded = loadFromStorage(workspaceId);
    set({ workspaceId, properties: loaded.properties, showEmptyColumns: loaded.showEmptyColumns, groupBy: loaded.groupBy });
  },

  toggleProperty: (key) => {
    const { workspaceId, properties, showEmptyColumns, groupBy } = get();
    const next = { ...properties, [key]: !properties[key] };
    set({ properties: next });
    if (workspaceId) saveToStorage(workspaceId, next, showEmptyColumns, groupBy);
  },

  toggleShowEmptyColumns: () => {
    const { workspaceId, properties, showEmptyColumns, groupBy } = get();
    const next = !showEmptyColumns;
    set({ showEmptyColumns: next });
    if (workspaceId) saveToStorage(workspaceId, properties, next, groupBy);
  },

  setGroupBy: (value) => {
    const { workspaceId, properties, showEmptyColumns } = get();
    set({ groupBy: value });
    if (workspaceId) saveToStorage(workspaceId, properties, showEmptyColumns, value);
  },
}));
