import { api } from '../api';
import type { BonusCalculation, FinanceSettings } from '../types';

export const bonusService = {
  getCalculations: (workspaceId: string, quarterId: string) =>
    api.get<BonusCalculation[]>(`/bonus/calculations?workspace_id=${workspaceId}&quarter_id=${quarterId}`),
  saveCalculations: (calculations: Omit<BonusCalculation, 'id'>[]) =>
    api.put('/bonus/calculations', { calculations }),
  lock: (workspaceId: string, quarterId: string) =>
    api.post('/bonus/lock', { workspace_id: workspaceId, quarter_id: quarterId }),
  unlock: (workspaceId: string, quarterId: string) =>
    api.post('/bonus/unlock', { workspace_id: workspaceId, quarter_id: quarterId }),
  getFinance: (workspaceId: string, quarterId: string) =>
    api.get<FinanceSettings>(`/bonus/finance?workspace_id=${workspaceId}&quarter_id=${quarterId}`),
  upsertFinance: (data: Partial<FinanceSettings>) =>
    api.put<FinanceSettings>('/bonus/finance', data),
  getTeamSprintData: (workspaceId: string, quarterId: string) =>
    api.get(`/bonus/team-sprint-data?workspace_id=${workspaceId}&quarter_id=${quarterId}`),
};
