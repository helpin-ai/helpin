import { create } from 'zustand';
import type { TeamEstimateSettings } from '@/lib/types';
import { settingsService } from '@/lib/services/settingsService';

interface TeamEstimateState {
  settings: TeamEstimateSettings[];
  loaded: boolean;
  loadForWorkspace: (workspaceId: string) => Promise<void>;
  getForTeam: (teamId: string | undefined | null) => TeamEstimateSettings | null;
  setSettings: (settings: TeamEstimateSettings[]) => void;
}

export const useTeamEstimateStore = create<TeamEstimateState>((set, get) => ({
  settings: [],
  loaded: false,

  loadForWorkspace: async (workspaceId: string) => {
    const { data } = await settingsService.getAll(workspaceId);
    if (data) {
      set({ settings: data.team_estimate_settings ?? [], loaded: true });
    }
  },

  getForTeam: (teamId: string | undefined | null) => {
    if (!teamId) return null;
    return get().settings.find((s) => s.team_id === teamId) ?? null;
  },

  setSettings: (settings: TeamEstimateSettings[]) => {
    set({ settings, loaded: true });
  },
}));
