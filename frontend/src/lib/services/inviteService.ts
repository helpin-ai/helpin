import { api } from '../api';
import type { Invitation, InviteInfo } from '../types';

export const inviteService = {
  send: (data: { workspace_id: string; email: string; role: string }) =>
    api.post<Invitation>(`/invitations?workspace_id=${encodeURIComponent(data.workspace_id)}`, data),
  list: (workspaceId: string) =>
    api.get<Invitation[]>(`/invitations?workspace_id=${workspaceId}`),
  getInfo: (token: string) =>
    api.get<InviteInfo>(`/invitations/info?token=${token}`),
  accept: (token: string) =>
    api.post<{ message: string }>('/invitations/accept', { token }),
  resend: (id: string, workspaceId: string) =>
    api.post<{ message: string }>(`/invitations/${id}/resend?workspace_id=${encodeURIComponent(workspaceId)}`),
  revoke: (id: string, workspaceId: string) =>
    api.del<{ message: string }>(`/invitations/${id}?workspace_id=${encodeURIComponent(workspaceId)}`),
};
