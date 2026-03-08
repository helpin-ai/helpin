import { create } from 'zustand';
import type { TeamFieldVisibility } from '@/lib/types';
import { settingsService } from '@/lib/services/settingsService';

const DEFAULT_VISIBILITY: Omit<TeamFieldVisibility, 'id' | 'team_id' | 'created_at' | 'updated_at'> = {
  priority: true,
  story_type: true,
  severity: true,
  labels: true,
  epic: true,
  sprint: true,
  estimate: true,
  due_date: true,
  blocked: true,
  delivery: true,
  dev_history: true,
};

type VisibilityField = keyof typeof DEFAULT_VISIBILITY;

interface TeamFieldVisibilityState {
  settings: TeamFieldVisibility[];
  loaded: boolean;
  loadForWorkspace: (workspaceId: string) => Promise<void>;
  getForTeam: (teamId: string | undefined | null) => TeamFieldVisibility | typeof DEFAULT_VISIBILITY;
  isFieldVisible: (teamId: string | undefined | null, field: VisibilityField) => boolean;
  setSettings: (settings: TeamFieldVisibility[]) => void;
}

export const useTeamFieldVisibilityStore = create<TeamFieldVisibilityState>((set, get) => ({
  settings: [],
  loaded: false,

  loadForWorkspace: async (workspaceId: string) => {
    const { data } = await settingsService.getAll(workspaceId);
    if (data) {
      set({ settings: data.team_field_visibility ?? [], loaded: true });
    }
  },

  getForTeam: (teamId: string | undefined | null) => {
    if (!teamId) return DEFAULT_VISIBILITY;
    return get().settings.find((s) => s.team_id === teamId) ?? DEFAULT_VISIBILITY;
  },

  isFieldVisible: (teamId: string | undefined | null, field: VisibilityField) => {
    const vis = get().getForTeam(teamId);
    return vis[field];
  },

  setSettings: (settings: TeamFieldVisibility[]) => {
    set({ settings, loaded: true });
  },
}));
