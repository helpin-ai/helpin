import { create } from 'zustand';

export const DEAL_DISPLAY_PROPERTY_LABELS = {
  amount: 'Amount',
  probability: 'Probability',
  close_date: 'Close Date',
  owner: 'Owner',
  stage_type: 'Stage Type',
  created_at: 'Created',
} as const;

export type DealDisplayPropertyKey = keyof typeof DEAL_DISPLAY_PROPERTY_LABELS;

export type DealDisplayProperties = Record<DealDisplayPropertyKey, boolean>;

/** Keys shown in the Kanban board display menu */
export const DEAL_BOARD_PROPERTY_KEYS: DealDisplayPropertyKey[] = [
  'amount', 'probability', 'close_date', 'owner',
];

/** Keys shown in the list view display menu */
export const DEAL_LIST_PROPERTY_KEYS: DealDisplayPropertyKey[] = [
  'amount', 'probability', 'close_date', 'owner', 'stage_type', 'created_at',
];

const ALL_ON: DealDisplayProperties = {
  amount: true,
  probability: true,
  close_date: true,
  owner: true,
  stage_type: true,
  created_at: true,
};

const STORAGE_KEY = (wsId: string) => `crm_deal_display_${wsId}`;

function loadFromStorage(workspaceId: string): { properties: DealDisplayProperties; showEmptyStages: boolean } {
  try {
    const raw = localStorage.getItem(STORAGE_KEY(workspaceId));
    if (raw) {
      const parsed = JSON.parse(raw);
      return {
        properties: { ...ALL_ON, ...parsed.properties },
        showEmptyStages: parsed.showEmptyStages ?? true,
      };
    }
  } catch {}
  return { properties: { ...ALL_ON }, showEmptyStages: true };
}

function saveToStorage(workspaceId: string, properties: DealDisplayProperties, showEmptyStages: boolean) {
  try {
    localStorage.setItem(STORAGE_KEY(workspaceId), JSON.stringify({ properties, showEmptyStages }));
  } catch {}
}

interface DealDisplayState {
  workspaceId: string | null;
  properties: DealDisplayProperties;
  showEmptyStages: boolean;
  init: (workspaceId: string) => void;
  toggleProperty: (key: DealDisplayPropertyKey) => void;
  toggleShowEmptyStages: () => void;
}

export const useDealDisplayStore = create<DealDisplayState>((set, get) => ({
  workspaceId: null,
  properties: { ...ALL_ON },
  showEmptyStages: true,

  init: (workspaceId) => {
    if (get().workspaceId === workspaceId) return;
    const loaded = loadFromStorage(workspaceId);
    set({ workspaceId, properties: loaded.properties, showEmptyStages: loaded.showEmptyStages });
  },

  toggleProperty: (key) => {
    const { workspaceId, properties, showEmptyStages } = get();
    const next = { ...properties, [key]: !properties[key] };
    set({ properties: next });
    if (workspaceId) saveToStorage(workspaceId, next, showEmptyStages);
  },

  toggleShowEmptyStages: () => {
    const { workspaceId, properties, showEmptyStages } = get();
    const next = !showEmptyStages;
    set({ showEmptyStages: next });
    if (workspaceId) saveToStorage(workspaceId, properties, next);
  },
}));
