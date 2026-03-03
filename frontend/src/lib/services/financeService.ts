import { api } from '../api';
import type { FinanceSettings } from '../types';

export const financeService = {
  get: (workspaceId: string, quarterId: string) =>
    api.get<FinanceSettings>(`/finance?workspace_id=${workspaceId}&quarter_id=${quarterId}`),
  upsert: (data: Partial<FinanceSettings>) =>
    api.post<FinanceSettings>('/finance', data),
};
