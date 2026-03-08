import { create } from 'zustand';

export const DISPLAY_PROPERTY_LABELS = {
  story_type: 'Story Type',
  priority: 'Priority',
  severity: 'Severity',
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
  'story_type', 'priority', 'severity', 'epic', 'labels',
  'estimate', 'due_date', 'blocked', 'assignee',
];

/** Keys shown in the list view display menu */
export const LIST_PROPERTY_KEYS: DisplayPropertyKey[] = [
  'state', 'story_type', 'priority', 'severity', 'estimate',
  'assignee', 'team', 'epic', 'sprint', 'due_date', 'labels', 'updated_at',
];

export type DisplayPropertyKey = keyof typeof DISPLAY_PROPERTY_LABELS;

export type DisplayProperties = Record<DisplayPropertyKey, boolean>;

const ALL_ON: DisplayProperties = {
  story_type: true,
  priority: true,
  severity: true,
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

function loadFromStorage(workspaceId: string): { properties: DisplayProperties; showEmptyColumns: boolean } {
  try {
    const raw = localStorage.getItem(STORAGE_KEY(workspaceId));
    if (raw) {
      const parsed = JSON.parse(raw);
      return {
        properties: { ...ALL_ON, ...parsed.properties },
        showEmptyColumns: parsed.showEmptyColumns ?? true,
      };
    }
  } catch {}
  return { properties: { ...ALL_ON }, showEmptyColumns: true };
}

function saveToStorage(workspaceId: string, properties: DisplayProperties, showEmptyColumns: boolean) {
  try {
    localStorage.setItem(STORAGE_KEY(workspaceId), JSON.stringify({ properties, showEmptyColumns }));
  } catch {}
}

interface BoardDisplayState {
  workspaceId: string | null;
  properties: DisplayProperties;
  showEmptyColumns: boolean;
  init: (workspaceId: string) => void;
  toggleProperty: (key: DisplayPropertyKey) => void;
  toggleShowEmptyColumns: () => void;
}

export const useBoardDisplayStore = create<BoardDisplayState>((set, get) => ({
  workspaceId: null,
  properties: { ...ALL_ON },
  showEmptyColumns: true,

  init: (workspaceId) => {
    if (get().workspaceId === workspaceId) return;
    const loaded = loadFromStorage(workspaceId);
    set({ workspaceId, properties: loaded.properties, showEmptyColumns: loaded.showEmptyColumns });
  },

  toggleProperty: (key) => {
    const { workspaceId, properties, showEmptyColumns } = get();
    const next = { ...properties, [key]: !properties[key] };
    set({ properties: next });
    if (workspaceId) saveToStorage(workspaceId, next, showEmptyColumns);
  },

  toggleShowEmptyColumns: () => {
    const { workspaceId, properties, showEmptyColumns } = get();
    const next = !showEmptyColumns;
    set({ showEmptyColumns: next });
    if (workspaceId) saveToStorage(workspaceId, properties, next);
  },
}));
