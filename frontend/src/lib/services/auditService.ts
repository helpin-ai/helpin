import { api } from '../api';
import type { RewardAuditEntry } from '../types';

export const rewardAuditService = {
  list: (workspaceId: string, quarterId: string) =>
    api.get<RewardAuditEntry[]>(`/rewards/audit?workspace_id=${workspaceId}&quarter_id=${quarterId}`),
};
