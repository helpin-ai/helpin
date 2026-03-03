import { api } from '../api';
import type { AuditEntry } from '../types';

export const auditService = {
  list: (workspaceId: string, quarterId: string) =>
    api.get<AuditEntry[]>(`/audit?workspace_id=${workspaceId}&quarter_id=${quarterId}`),
};
