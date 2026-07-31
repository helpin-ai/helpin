import { api } from '../api';
import type { MemberSetupPreference, SetupGoalKey, SetupView } from '../setupTypes';

export const setupService = {
  get: (workspaceId: string) => api.get<SetupView>(`/workspaces/${workspaceId}/setup`),
  updateGoals: (workspaceId: string, goals: SetupGoalKey[]) =>
    api.put<SetupView>(`/workspaces/${workspaceId}/setup/goals`, { goals }),
  updatePreference: (workspaceId: string, sidebarDismissed: boolean) =>
    api.patch<MemberSetupPreference>(`/workspaces/${workspaceId}/setup/me`, { sidebar_dismissed: sidebarDismissed }),
  startRecommendation: (workspaceId: string, taskKey: string) =>
    api.post<void>(`/workspaces/${workspaceId}/setup/recommendations/${encodeURIComponent(taskKey)}/start`),
};
