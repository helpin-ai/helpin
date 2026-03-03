import { api } from '../api';
import type { BonusCalculation, FinanceSettings } from '../types';

export const bonusService = {
  getCalculations: (workspaceId: string, quarterId: string) =>
    api.get<BonusCalculation[]>(`/bonus/calculations?workspace_id=${workspaceId}&quarter_id=${quarterId}`),
  saveCalculations: (calculations: Omit<BonusCalculation, 'id'>[]) =>
    api.post('/bonus/calculations', { calculations }),
  lock: (workspaceId: string, quarterId: string) =>
    api.post('/bonus/lock', { workspace_id: workspaceId, quarter_id: quarterId }),
  unlock: (workspaceId: string, quarterId: string) =>
    api.post('/bonus/unlock', { workspace_id: workspaceId, quarter_id: quarterId }),
  getFinance: (workspaceId: string, quarterId: string) =>
    api.get<FinanceSettings>(`/finance?workspace_id=${workspaceId}&quarter_id=${quarterId}`),
  upsertFinance: (data: Partial<FinanceSettings>) =>
    api.post<FinanceSettings>('/finance', data),
  getTeamSprintData: (sprintId: string) =>
    api.get(`/bonus/team-sprint-data?sprint_id=${encodeURIComponent(sprintId)}`),
};
